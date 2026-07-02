package terraform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"

	"github.com/ryobi-project/ryobi/pkg/recipes/terraform/config"
	"github.com/ryobi-project/ryobi/pkg/recipes/terraform/config/backends"
	"github.com/ryobi-project/ryobi/pkg/recipes/terraform/config/providers"
)

// executor implements the Executor interface.
type executor struct {
	terraformPath string
	backend       backends.Backend
}

// NewExecutor creates a new Terraform executor.
// terraformPath is the path to the terraform binary.
// backend is the state backend to use (PostgreSQL or local).
func NewExecutor(terraformPath string, backend backends.Backend) Executor {
	return &executor{
		terraformPath: terraformPath,
		backend:       backend,
	}
}

// Deploy runs terraform init + apply and returns the resulting state.
func (e *executor) Deploy(ctx context.Context, options Options) (*tfjson.State, error) {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Deploying recipe",
		"template", options.EnvRecipe.TemplatePath,
		"resource", options.ResourceRecipe.ResourceID)

	workDir, err := e.prepareWorkDir(options)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare working directory: %w", err)
	}

	// Generate Terraform configuration
	if err := e.generateConfig(ctx, workDir, options); err != nil {
		return nil, fmt.Errorf("failed to generate terraform config: %w", err)
	}

	// Create terraform executor
	tf, err := tfexec.NewTerraform(workDir, e.terraformPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform executor: %w", err)
	}

	// Set environment variables (if any)
	if envVars := e.buildEnvVars(options); len(envVars) > 0 {
		if err := tf.SetEnv(envVars); err != nil {
			return nil, fmt.Errorf("failed to set environment variables: %w", err)
		}
	}

	logger.Info("Running terraform init")
	if err := tf.Init(ctx); err != nil {
		return nil, fmt.Errorf("terraform init failed: %w", err)
	}

	logger.Info("Running terraform apply")
	if err := tf.Apply(ctx); err != nil {
		return nil, fmt.Errorf("terraform apply failed: %w", err)
	}

	// Get the resulting state
	logger.Info("Reading terraform state")
	state, err := tf.Show(ctx)
	if err != nil {
		return nil, fmt.Errorf("terraform show failed: %w", err)
	}

	logger.Info("Recipe deployed successfully")
	return state, nil
}

// Delete runs terraform init + destroy.
func (e *executor) Delete(ctx context.Context, options Options) error {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Deleting recipe",
		"template", options.EnvRecipe.TemplatePath,
		"resource", options.ResourceRecipe.ResourceID)

	workDir, err := e.prepareWorkDir(options)
	if err != nil {
		return fmt.Errorf("failed to prepare working directory: %w", err)
	}

	// Generate Terraform configuration
	if err := e.generateConfig(ctx, workDir, options); err != nil {
		return fmt.Errorf("failed to generate terraform config: %w", err)
	}

	tf, err := tfexec.NewTerraform(workDir, e.terraformPath)
	if err != nil {
		return fmt.Errorf("failed to create terraform executor: %w", err)
	}

	if envVars := e.buildEnvVars(options); len(envVars) > 0 {
		if err := tf.SetEnv(envVars); err != nil {
			return fmt.Errorf("failed to set environment variables: %w", err)
		}
	}

	logger.Info("Running terraform init")
	if err := tf.Init(ctx); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	logger.Info("Running terraform destroy")
	if err := tf.Destroy(ctx); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	logger.Info("Recipe destroyed successfully")
	return nil
}

// prepareWorkDir creates a unique working directory for this Terraform execution.
func (e *executor) prepareWorkDir(options Options) (string, error) {
	rootDir := options.RootDir
	if rootDir == "" {
		rootDir = os.TempDir()
	}

	workDir := filepath.Join(rootDir, "recipes", sanitizeName(options.ResourceRecipe.ResourceID))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create working directory: %w", err)
	}
	return workDir, nil
}

// generateConfig creates the main.tf.json configuration file.
func (e *executor) generateConfig(ctx context.Context, workDir string, options Options) error {
	tfConfig := config.New()

	// Resolve template path — convert relative paths to absolute
	templatePath := options.EnvRecipe.TemplatePath
	if isLocalPath(templatePath) {
		absPath, err := filepath.Abs(templatePath)
		if err != nil {
			return fmt.Errorf("failed to resolve template path %q: %w", templatePath, err)
		}
		templatePath = absPath
	}

	// Set the module source and parameters
	params := mergeParams(options.EnvRecipe.Parameters, options.ResourceRecipe.Parameters)
	tfConfig.SetModule("recipe", templatePath, options.EnvRecipe.TemplateVersion, params)

	// Configure state backend
	if e.backend != nil {
		backendType, backendConfig, err := e.backend.BuildBackend(options.ResourceRecipe)
		if err != nil {
			return fmt.Errorf("failed to build backend config: %w", err)
		}
		tfConfig.SetBackend(backendType, backendConfig)
	}

	// Configure cloud providers
	if options.EnvConfig != nil {
		if _, ok := options.EnvConfig.Providers["azure"]; ok {
			azureConfig, err := providers.NewAzureProvider().BuildConfig(ctx, options.EnvConfig)
			if err != nil {
				return fmt.Errorf("failed to build azure provider config: %w", err)
			}
			tfConfig.AddProvider(providers.ProviderNameAzureRM, azureConfig)
		}

		if _, ok := options.EnvConfig.Providers["aws"]; ok {
			awsConfig, err := providers.NewAWSProvider().BuildConfig(ctx, options.EnvConfig)
			if err != nil {
				return fmt.Errorf("failed to build aws provider config: %w", err)
			}
			tfConfig.AddProvider(providers.ProviderNameAWS, awsConfig)
		}

		if _, ok := options.EnvConfig.Providers["kubernetes"]; ok {
			k8sConfig := providers.BuildKubernetesConfig(options.EnvConfig)
			tfConfig.AddProvider("kubernetes", k8sConfig)
		}
	}

	// Add result output to capture module outputs
	tfConfig.AddResultOutput("recipe")

	// Write configuration to disk
	return tfConfig.Save(workDir)
}

// buildEnvVars constructs environment variables for the Terraform process.
// Note: TF_IN_AUTOMATION is managed by terraform-exec internally and must not be set here.
func (e *executor) buildEnvVars(options Options) map[string]string {
	envVars := map[string]string{}

	// Pass through additional env vars from recipe config
	if options.EnvConfig != nil {
		for k, v := range options.EnvConfig.RecipeConfig.Env.AdditionalProperties {
			envVars[k] = v
		}
	}

	return envVars
}

// mergeParams merges environment recipe params with resource-level params.
// Resource params take precedence.
func mergeParams(envParams, resourceParams map[string]any) map[string]any {
	merged := make(map[string]any)
	for k, v := range envParams {
		merged[k] = v
	}
	for k, v := range resourceParams {
		merged[k] = v
	}
	return merged
}

// isLocalPath returns true if the path refers to a local filesystem path
// rather than a registry, git, or HTTP module source.
func isLocalPath(path string) bool {
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.HasPrefix(path, "/") {
		return true
	}
	return false
}

// sanitizeName creates a filesystem-safe name from a resource ID.
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
