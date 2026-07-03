package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/hashicorp/terraform-exec/tfexec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ryobi-project/ryobi/pkg/grpcapi"
	"github.com/ryobi-project/ryobi/pkg/recipes"
	tfconfig "github.com/ryobi-project/ryobi/pkg/recipes/terraform/config"
	"github.com/ryobi-project/ryobi/pkg/recipes/terraform/config/backends"
	"github.com/ryobi-project/ryobi/pkg/version"
)

func main() {
	logger := stdr.New(nil)

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: ryobi-env <config.yaml>\n")
		os.Exit(1)
	}

	configPath := os.Args[1]
	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Starting ryobi-env agent",
		"version", version.Version,
		"environment", cfg.Name,
		"server", cfg.Server.GRPCAddress)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx = logr.NewContext(ctx, logger)

	// Connect to ryobid gRPC server
	conn, err := grpc.NewClient(cfg.Server.GRPCAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to server: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := grpcapi.NewEnvironmentServiceClient(conn)

	// Register the environment
	if err := registerEnvironment(ctx, client, cfg, logger); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register environment: %v\n", err)
		os.Exit(1)
	}

	// Ensure we unregister on shutdown
	defer func() {
		logger.Info("Unregistering environment", "environment", cfg.Name)
		// Use a fresh context since ctx is cancelled on signal
		unregCtx, unregCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer unregCancel()
		_, err := client.Unregister(unregCtx, &grpcapi.UnregisterRequest{
			EnvironmentName: cfg.Name,
		})
		if err != nil {
			logger.Error(err, "Failed to unregister environment")
		} else {
			logger.Info("Environment unregistered", "environment", cfg.Name)
		}
	}()

	// Start heartbeat goroutine
	go runHeartbeat(ctx, client, cfg, logger)

	// Start status watcher
	watcher := newStatusWatcher(client, cfg, logger)
	go watcher.Run(ctx)

	// Build recipe lookup table
	recipeIndex := buildRecipeIndex(cfg)

	// Watch for resource events
	logger.Info("Watching for resource events...")
	if err := watchAndProcess(ctx, client, cfg, recipeIndex, watcher, logger); err != nil {
		if ctx.Err() != nil {
			logger.Info("Shutting down")
		} else {
			fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)
			os.Exit(1)
		}
	}
}

func registerEnvironment(ctx context.Context, client grpcapi.EnvironmentServiceClient, cfg *EnvConfig, logger logr.Logger) error {
	req := &grpcapi.RegisterRequest{
		EnvironmentName: cfg.Name,
		Config: &grpcapi.EnvironmentConfig{
			Providers:          make(map[string]*grpcapi.ProviderConfig),
			TerraformProviders: make(map[string]string),
		},
		Recipes: make([]*grpcapi.RecipeRegistration, 0, len(cfg.Recipes)),
		Capabilities: &grpcapi.EnvironmentCapabilities{
			Region:       cfg.Capabilities.Region,
			Sovereignty:  cfg.Capabilities.Sovereignty,
			Capabilities: cfg.Capabilities.Capabilities,
			CostPerHour:  cfg.Capabilities.CostPerHour,
			MaxReplicas:  cfg.Capabilities.MaxReplicas,
		},
	}

	for name, p := range cfg.Providers {
		req.Config.Providers[name] = &grpcapi.ProviderConfig{Scope: p.Scope}
	}

	for name, providerCfg := range cfg.TerraformProviders {
		data, _ := json.Marshal(providerCfg)
		req.Config.TerraformProviders[name] = string(data)
	}

	for _, recipe := range cfg.Recipes {
		req.Recipes = append(req.Recipes, &grpcapi.RecipeRegistration{
			ResourceType:      recipe.ResourceType,
			RecipeName:        recipe.RecipeName,
			TemplatePath:      recipe.TemplatePath,
			DefaultParameters: recipe.Parameters,
		})
	}

	resp, err := client.Register(ctx, req)
	if err != nil {
		return fmt.Errorf("gRPC register failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}

	logger.Info("Environment registered", "id", resp.EnvironmentId)
	return nil
}

type recipeEntry struct {
	templatePath string
	parameters   map[string]string
}

func buildRecipeIndex(cfg *EnvConfig) map[string]recipeEntry {
	index := make(map[string]recipeEntry)
	for _, r := range cfg.Recipes {
		key := r.ResourceType + "|" + r.RecipeName
		index[key] = recipeEntry{
			templatePath: r.TemplatePath,
			parameters:   r.Parameters,
		}
	}
	return index
}

func watchAndProcess(ctx context.Context, client grpcapi.EnvironmentServiceClient, cfg *EnvConfig, recipeIndex map[string]recipeEntry, watcher *statusWatcher, logger logr.Logger) error {
	stream, err := client.WatchResources(ctx, &grpcapi.WatchRequest{
		EnvironmentName: cfg.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to open watch stream: %w", err)
	}

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		logger.Info("Received resource event",
			"operation", event.Operation.String(),
			"resource", event.ResourceName,
			"type", event.ResourceType,
			"recipe", event.RecipeName)

		// Process the event
		go func(ev *grpcapi.ResourceEvent) {
			result := processEvent(ctx, ev, cfg, recipeIndex, logger)

			// Report result back to ryobid
			_, reportErr := client.ReportResult(ctx, result)
			if reportErr != nil {
				logger.Error(reportErr, "Failed to report result", "operationId", ev.OperationId)
			}

			// Register/unregister resource for status watching
			if result.Success && ev.Operation == grpcapi.OperationType_DEPLOY {
				watcher.Register(ev.ResourceId, ev.ResourceName, ev.ResourceType, ev.Parameters)
			} else if ev.Operation == grpcapi.OperationType_DELETE {
				watcher.Unregister(ev.ResourceId)
			}
		}(event)
	}
}

func processEvent(ctx context.Context, event *grpcapi.ResourceEvent, cfg *EnvConfig, recipeIndex map[string]recipeEntry, logger logr.Logger) *grpcapi.ReportResultRequest {
	// Look up recipe
	key := event.ResourceType + "|" + event.RecipeName
	recipe, ok := recipeIndex[key]
	if !ok {
		return &grpcapi.ReportResultRequest{
			OperationId:  event.OperationId,
			ResourceId:   event.ResourceId,
			Success:      false,
			ErrorMessage: fmt.Sprintf("recipe %q not found for resource type %q", event.RecipeName, event.ResourceType),
		}
	}

	// Prepare working directory
	workDir := filepath.Join(cfg.Terraform.WorkDir, "recipes", sanitizeName(event.ResourceId))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return &grpcapi.ReportResultRequest{
			OperationId:  event.OperationId,
			ResourceId:   event.ResourceId,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to create work dir: %v", err),
		}
	}

	// Generate main.tf.json
	tfCfg := tfconfig.New()

	// Merge parameters: recipe defaults + event parameters
	params := make(map[string]any)
	for k, v := range recipe.parameters {
		params[k] = v
	}
	for k, v := range event.Parameters {
		// Try to unmarshal JSON values, fall back to string
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err != nil {
			params[k] = v
		} else {
			params[k] = parsed
		}
	}

	tfCfg.SetModule("recipe", recipe.templatePath, "", params)

	// Configure state backend
	var stateBackend backends.Backend
	if cfg.Terraform.DatabaseURL != "" {
		stateBackend = backends.NewPostgresBackend(cfg.Terraform.DatabaseURL)
	} else {
		stateBackend = backends.NewLocalBackend(cfg.Terraform.WorkDir)
	}

	backendType, backendConfig, err := stateBackend.BuildBackend(&recipes.ResourceMetadata{
		Name:       event.ResourceName,
		ResourceID: event.ResourceId,
	})
	if err == nil {
		tfCfg.SetBackend(backendType, backendConfig)
	}

	// Add terraform provider configs
	for name, providerCfg := range cfg.TerraformProviders {
		tfCfg.AddProvider(name, providerCfg)
	}

	tfCfg.AddResultOutput("recipe")

	if err := tfCfg.Save(workDir); err != nil {
		return &grpcapi.ReportResultRequest{
			OperationId:  event.OperationId,
			ResourceId:   event.ResourceId,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to save terraform config: %v", err),
		}
	}

	// Execute terraform
	tf, err := tfexec.NewTerraform(workDir, cfg.Terraform.BinaryPath)
	if err != nil {
		return &grpcapi.ReportResultRequest{
			OperationId:  event.OperationId,
			ResourceId:   event.ResourceId,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to create terraform executor: %v", err),
		}
	}

	logger.Info("Running terraform init", "workDir", workDir)
	if err := tf.Init(ctx); err != nil {
		return &grpcapi.ReportResultRequest{
			OperationId:  event.OperationId,
			ResourceId:   event.ResourceId,
			Success:      false,
			ErrorMessage: fmt.Sprintf("terraform init failed: %v", err),
		}
	}

	if event.Operation == grpcapi.OperationType_DEPLOY {
		logger.Info("Running terraform apply", "resource", event.ResourceName)
		if err := tf.Apply(ctx); err != nil {
			return &grpcapi.ReportResultRequest{
				OperationId:  event.OperationId,
				ResourceId:   event.ResourceId,
				Success:      false,
				ErrorMessage: fmt.Sprintf("terraform apply failed: %v", err),
			}
		}

		// Extract outputs
		outputs := make(map[string]string)
		state, err := tf.Show(ctx)
		if err == nil && state != nil && state.Values != nil {
			for name, out := range state.Values.Outputs {
				if out != nil {
					data, _ := json.Marshal(out.Value)
					outputs[name] = string(data)
				}
			}
		}

		logger.Info("Terraform apply succeeded", "resource", event.ResourceName, "outputs", len(outputs))
		return &grpcapi.ReportResultRequest{
			OperationId: event.OperationId,
			ResourceId:  event.ResourceId,
			Success:     true,
			Outputs:     outputs,
		}

	} else {
		logger.Info("Running terraform destroy", "resource", event.ResourceName)
		if err := tf.Destroy(ctx); err != nil {
			return &grpcapi.ReportResultRequest{
				OperationId:  event.OperationId,
				ResourceId:   event.ResourceId,
				Success:      false,
				ErrorMessage: fmt.Sprintf("terraform destroy failed: %v", err),
			}
		}

		logger.Info("Terraform destroy succeeded", "resource", event.ResourceName)
		return &grpcapi.ReportResultRequest{
			OperationId: event.OperationId,
			ResourceId:  event.ResourceId,
			Success:     true,
		}
	}
}

func runHeartbeat(ctx context.Context, client grpcapi.EnvironmentServiceClient, cfg *EnvConfig, logger logr.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := client.Heartbeat(ctx, &grpcapi.HeartbeatRequest{
				EnvironmentName:        cfg.Name,
				AvailableCpuMillicores: 0, // TODO: collect from system/k8s
				AvailableMemoryMb:      0, // TODO: collect from system/k8s
				RunningResources:       0, // TODO: track locally
			})
			if err != nil {
				logger.Error(err, "Heartbeat failed")
			}
		}
	}
}

func sanitizeName(id string) string {
	result := make([]byte, 0, len(id))
	for _, c := range id {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, byte(c))
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}
