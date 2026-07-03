package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ryobi-project/ryobi/pkg/cli/config"
	"github.com/ryobi-project/ryobi/pkg/cli/connections"
	"github.com/ryobi-project/ryobi/pkg/cli/output"
	cliyaml "github.com/ryobi-project/ryobi/pkg/cli/yaml"
	"github.com/ryobi-project/ryobi/pkg/version"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ryobi",
		Short: "Ryobi - Cloud-native application platform",
		Long:  "Ryobi is a platform for deploying and managing cloud-native applications using Terraform recipes.",
	}

	rootCmd.PersistentFlags().String("server", "", "Server URL (default: from config or http://localhost:9000)")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(newEnvCmd())
	rootCmd.AddCommand(newAppCmd())
	rootCmd.AddCommand(newResourceCmd())
	rootCmd.AddCommand(newRecipeCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getClient(cmd *cobra.Command) *connections.Client {
	serverURL, _ := cmd.Flags().GetString("server")
	if serverURL == "" {
		cfg, _ := config.Load()
		if cfg != nil {
			serverURL = cfg.Server.URL
		}
	}
	if serverURL == "" {
		serverURL = config.DefaultServerURL
	}
	return connections.New(serverURL)
}

// --- version ---

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ryobi version %s (commit: %s, channel: %s)\n", version.Version, version.Commit, version.Channel)
		},
	}
}

// --- deploy ---

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [file]",
		Short: "Deploy an application from a YAML file",
		Args:  cobra.ExactArgs(1),
		RunE:  runDeploy,
	}
	cmd.Flags().StringP("environment", "e", "", "Override target environment")
	cmd.Flags().Bool("wait", true, "Wait for async operations to complete")
	return cmd
}

func runDeploy(cmd *cobra.Command, args []string) error {
	client := getClient(cmd)
	ctx := cmd.Context()
	wait, _ := cmd.Flags().GetBool("wait")
	envOverride, _ := cmd.Flags().GetString("environment")

	docs, err := cliyaml.ParseFile(args[0])
	if err != nil {
		return err
	}

	// Process environments first, then applications
	for _, doc := range docs {
		if err := cliyaml.Validate(&doc); err != nil {
			return fmt.Errorf("validation error: %w", err)
		}
	}

	for _, doc := range docs {
		if doc.Kind == cliyaml.KindEnvironment {
			if err := deployEnvironment(ctx, client, &doc); err != nil {
				return err
			}
		}
	}

	for _, doc := range docs {
		if doc.Kind == cliyaml.KindApplication {
			if envOverride != "" {
				doc.Metadata.Environment = envOverride
			}
			if err := deployApplication(ctx, client, &doc, wait); err != nil {
				return err
			}
		}
	}

	return nil
}

func deployEnvironment(ctx context.Context, client *connections.Client, doc *cliyaml.Document) error {
	output.PrintStatus("Creating environment %q...", doc.Metadata.Name)

	body := map[string]any{
		"name": doc.Metadata.Name,
		"properties": map[string]any{
			"providers":    doc.Providers,
			"recipes":      doc.Recipes,
			"recipeConfig": doc.RecipeConfig,
		},
	}

	_, statusCode, err := client.Put(ctx, "/api/v1/environments/"+doc.Metadata.Name, body)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	if statusCode >= 400 {
		return fmt.Errorf("failed to create environment %q (status %d)", doc.Metadata.Name, statusCode)
	}

	output.PrintSuccess("Environment %q created successfully", doc.Metadata.Name)
	return nil
}

func deployApplication(ctx context.Context, client *connections.Client, doc *cliyaml.Document, wait bool) error {
	// Create/update the application
	output.PrintStatus("Creating application %q...", doc.Metadata.Name)

	appBody := map[string]any{
		"name": doc.Metadata.Name,
		"properties": map[string]any{
			"environment": doc.Metadata.Environment,
		},
	}

	_, statusCode, err := client.Put(ctx, "/api/v1/applications/"+doc.Metadata.Name, appBody)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	if statusCode >= 400 {
		return fmt.Errorf("failed to create application %q (status %d)", doc.Metadata.Name, statusCode)
	}

	output.PrintSuccess("Application %q created", doc.Metadata.Name)

	// Deploy each resource
	var operationURLs []string
	for _, res := range doc.Resources {
		output.PrintStatus("Deploying resource %q (%s)...", res.Name, res.Type)

		props := map[string]any{
			"resourceType": res.Type,
			"parameters":   res.Parameters,
			"connections":  res.Connections,
		}
		if res.Recipe != "" {
			props["recipeName"] = res.Recipe
		}
		if res.Placement != nil {
			props["placement"] = res.Placement
		}
		resBody := map[string]any{
			"name":       res.Name,
			"properties": props,
		}

		path := fmt.Sprintf("/api/v1/applications/%s/resources/%s", doc.Metadata.Name, res.Name)
		respBody, statusCode, err := client.Put(ctx, path, resBody)
		if err != nil {
			return fmt.Errorf("failed to deploy resource %q: %w", res.Name, err)
		}

		if statusCode == http.StatusAccepted {
			// Extract operation URL from Location header
			var respMap map[string]any
			_ = json.Unmarshal(respBody, &respMap)

			// The Location header is in the response headers, but we get it from our response
			// For 202, we need to check the body or headers
			output.PrintStatus("Resource %q deployment queued", res.Name)
			// We'll poll based on resource status instead
			operationURLs = append(operationURLs, path)
		} else if statusCode >= 400 {
			return fmt.Errorf("failed to deploy resource %q (status %d)", res.Name, statusCode)
		} else {
			output.PrintSuccess("Resource %q deployed", res.Name)
		}
	}

	if wait && len(operationURLs) > 0 {
		output.PrintStatus("Waiting for deployments to complete...")
		var failed bool
		for _, url := range operationURLs {
			if err := waitForResource(ctx, client, url); err != nil {
				output.PrintError("%v", err)
				failed = true
			}
		}
		if failed {
			return fmt.Errorf("application %q deployment completed with errors", doc.Metadata.Name)
		}
	}

	output.PrintSuccess("Application %q deployed successfully", doc.Metadata.Name)
	return nil
}

func waitForResource(ctx context.Context, client *connections.Client, resourcePath string) error {
	timeout, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	for {
		select {
		case <-timeout.Done():
			return fmt.Errorf("timed out waiting for resource")
		default:
		}

		body, _, err := client.Get(timeout, resourcePath)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		var resource map[string]any
		if err := json.Unmarshal(body, &resource); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		props, _ := resource["properties"].(map[string]any)
		status, _ := props["status"].(map[string]any)
		state, _ := status["state"].(string)

		switch state {
		case "Succeeded":
			return nil
		case "Failed":
			errMsg, _ := status["error"].(string)
			if errMsg != "" {
				return fmt.Errorf("%s", errMsg)
			}
			return fmt.Errorf("deployment failed (no details available)")
		default:
			time.Sleep(2 * time.Second)
		}
	}
}

// --- env ---

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environments",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getClient(cmd)
			body, _, err := client.Get(cmd.Context(), "/api/v1/environments")
			if err != nil {
				return err
			}

			var resp connections.ListResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				return output.PrintJSON(json.RawMessage(body))
			}

			headers := []string{"NAME", "PROVIDERS"}
			var rows [][]string
			for _, item := range resp.Value {
				var env map[string]any
				_ = json.Unmarshal(item, &env)
				name, _ := env["name"].(string)
				providers := ""
				if props, ok := env["properties"].(map[string]any); ok {
					if p, ok := props["providers"].(map[string]any); ok {
						for k := range p {
							if providers != "" {
								providers += ", "
							}
							providers += k
						}
					}
				}
				rows = append(rows, []string{name, providers})
			}
			output.PrintTable(headers, rows)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show [name]",
		Short: "Show environment details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getClient(cmd)
			body, statusCode, err := client.Get(cmd.Context(), "/api/v1/environments/"+args[0])
			if err != nil {
				return err
			}
			if statusCode == http.StatusNotFound {
				return fmt.Errorf("environment %q not found", args[0])
			}
			return output.PrintJSON(json.RawMessage(body))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete [name]",
		Short: "Delete an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getClient(cmd)
			_, statusCode, err := client.Delete(cmd.Context(), "/api/v1/environments/"+args[0])
			if err != nil {
				return err
			}
			if statusCode >= 400 && statusCode != http.StatusNoContent {
				return fmt.Errorf("failed to delete environment (status %d)", statusCode)
			}
			output.PrintSuccess("Environment %q deleted", args[0])
			return nil
		},
	})

	return cmd
}

// --- app ---

func newAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage applications",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List applications",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getClient(cmd)
			body, _, err := client.Get(cmd.Context(), "/api/v1/applications")
			if err != nil {
				return err
			}

			var resp connections.ListResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				return output.PrintJSON(json.RawMessage(body))
			}

			headers := []string{"NAME", "ENVIRONMENT"}
			var rows [][]string
			for _, item := range resp.Value {
				var app map[string]any
				_ = json.Unmarshal(item, &app)
				name, _ := app["name"].(string)
				env := ""
				if props, ok := app["properties"].(map[string]any); ok {
					env, _ = props["environment"].(string)
				}
				rows = append(rows, []string{name, env})
			}
			output.PrintTable(headers, rows)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show [name]",
		Short: "Show application details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getClient(cmd)
			body, statusCode, err := client.Get(cmd.Context(), "/api/v1/applications/"+args[0])
			if err != nil {
				return err
			}
			if statusCode == http.StatusNotFound {
				return fmt.Errorf("application %q not found", args[0])
			}
			return output.PrintJSON(json.RawMessage(body))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete [name]",
		Short: "Delete an application",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getClient(cmd)
			_, statusCode, err := client.Delete(cmd.Context(), "/api/v1/applications/"+args[0])
			if err != nil {
				return err
			}
			if statusCode >= 400 && statusCode != http.StatusNoContent {
				return fmt.Errorf("failed to delete application (status %d)", statusCode)
			}
			output.PrintSuccess("Application %q deleted", args[0])
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status [name]",
		Short: "Show application status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getClient(cmd)
			// Get resources for this application
			body, _, err := client.Get(cmd.Context(), "/api/v1/applications/"+args[0]+"/resources")
			if err != nil {
				return err
			}

			var resp connections.ListResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				return output.PrintJSON(json.RawMessage(body))
			}

			fmt.Printf("Application: %s\n\n", args[0])
			headers := []string{"RESOURCE", "TYPE", "RECIPE", "STATE"}
			var rows [][]string
			for _, item := range resp.Value {
				var res map[string]any
				_ = json.Unmarshal(item, &res)
				name, _ := res["name"].(string)
				props, _ := res["properties"].(map[string]any)
				resType, _ := props["resourceType"].(string)
				recipe, _ := props["recipeName"].(string)
				state := ""
				if status, ok := props["status"].(map[string]any); ok {
					state, _ = status["state"].(string)
				}
				rows = append(rows, []string{name, resType, recipe, state})
			}
			output.PrintTable(headers, rows)
			return nil
		},
	})

	return cmd
}

// --- resource ---

func newResourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Manage application resources",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, _ := cmd.Flags().GetString("app")
			if app == "" {
				return fmt.Errorf("--app flag is required")
			}
			client := getClient(cmd)
			body, _, err := client.Get(cmd.Context(), "/api/v1/applications/"+app+"/resources")
			if err != nil {
				return err
			}

			var resp connections.ListResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				return output.PrintJSON(json.RawMessage(body))
			}

			headers := []string{"NAME", "TYPE", "RECIPE", "STATE"}
			var rows [][]string
			for _, item := range resp.Value {
				var res map[string]any
				_ = json.Unmarshal(item, &res)
				name, _ := res["name"].(string)
				props, _ := res["properties"].(map[string]any)
				resType, _ := props["resourceType"].(string)
				recipe, _ := props["recipeName"].(string)
				state := ""
				if status, ok := props["status"].(map[string]any); ok {
					state, _ = status["state"].(string)
				}
				rows = append(rows, []string{name, resType, recipe, state})
			}
			output.PrintTable(headers, rows)
			return nil
		},
	}
	listCmd.Flags().String("app", "", "Application name (required)")
	cmd.AddCommand(listCmd)

	showCmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Show resource details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, _ := cmd.Flags().GetString("app")
			if app == "" {
				return fmt.Errorf("--app flag is required")
			}
			client := getClient(cmd)
			path := fmt.Sprintf("/api/v1/applications/%s/resources/%s", app, args[0])
			body, statusCode, err := client.Get(cmd.Context(), path)
			if err != nil {
				return err
			}
			if statusCode == http.StatusNotFound {
				return fmt.Errorf("resource %q not found", args[0])
			}
			return output.PrintJSON(json.RawMessage(body))
		},
	}
	showCmd.Flags().String("app", "", "Application name (required)")
	cmd.AddCommand(showCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, _ := cmd.Flags().GetString("app")
			if app == "" {
				return fmt.Errorf("--app flag is required")
			}
			client := getClient(cmd)
			path := fmt.Sprintf("/api/v1/applications/%s/resources/%s", app, args[0])
			_, statusCode, err := client.Delete(cmd.Context(), path)
			if err != nil {
				return err
			}
			if statusCode >= 400 && statusCode != http.StatusAccepted && statusCode != http.StatusNoContent {
				return fmt.Errorf("failed to delete resource (status %d)", statusCode)
			}
			output.PrintSuccess("Resource %q deletion initiated", args[0])
			return nil
		},
	}
	deleteCmd.Flags().String("app", "", "Application name (required)")
	cmd.AddCommand(deleteCmd)

	return cmd
}

// --- recipe ---

func newRecipeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe",
		Short: "Manage recipes",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recipes registered in an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			envName, _ := cmd.Flags().GetString("environment")
			if envName == "" {
				return fmt.Errorf("--environment flag is required")
			}
			client := getClient(cmd)
			body, statusCode, err := client.Get(cmd.Context(), "/api/v1/environments/"+envName)
			if err != nil {
				return err
			}
			if statusCode == http.StatusNotFound {
				return fmt.Errorf("environment %q not found", envName)
			}

			var env map[string]any
			_ = json.Unmarshal(body, &env)
			props, _ := env["properties"].(map[string]any)
			recipes, _ := props["recipes"].(map[string]any)

			headers := []string{"RESOURCE TYPE", "RECIPE", "TEMPLATE"}
			var rows [][]string
			for resType, recipeMap := range recipes {
				if rm, ok := recipeMap.(map[string]any); ok {
					for name, def := range rm {
						template := ""
						if d, ok := def.(map[string]any); ok {
							template, _ = d["templatePath"].(string)
						}
						rows = append(rows, []string{resType, name, template})
					}
				}
			}
			output.PrintTable(headers, rows)
			return nil
		},
	}
	listCmd.Flags().StringP("environment", "e", "", "Environment name (required)")
	cmd.AddCommand(listCmd)

	return cmd
}
