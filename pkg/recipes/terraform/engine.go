package terraform

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"github.com/ryobi-project/ryobi/pkg/recipes"
)

// RecipeEngine implements recipes.Engine using Terraform.
type RecipeEngine struct {
	executor Executor
	rootDir  string
}

// NewRecipeEngine creates a new Terraform recipe engine.
func NewRecipeEngine(executor Executor, rootDir string) *RecipeEngine {
	return &RecipeEngine{
		executor: executor,
		rootDir:  rootDir,
	}
}

// Execute deploys a recipe using Terraform.
func (e *RecipeEngine) Execute(ctx context.Context, envConfig *recipes.Configuration, envRecipe *recipes.EnvironmentDefinition, resourceRecipe *recipes.ResourceMetadata) (*recipes.RecipeOutput, error) {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Executing Terraform recipe",
		"recipe", envRecipe.Name,
		"template", envRecipe.TemplatePath,
		"resource", resourceRecipe.ResourceID)

	opts := Options{
		RootDir:        e.rootDir,
		EnvConfig:      envConfig,
		EnvRecipe:      envRecipe,
		ResourceRecipe: resourceRecipe,
	}

	state, err := e.executor.Deploy(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("terraform deploy failed: %w", err)
	}

	// Extract outputs from Terraform state
	output := &recipes.RecipeOutput{
		Values: make(map[string]any),
	}

	if state != nil && state.Values != nil {
		for name, outputValue := range state.Values.Outputs {
			if outputValue != nil {
				output.Values[name] = outputValue.Value
			}
		}
	}

	logger.Info("Terraform recipe executed successfully",
		"outputs", len(output.Values))

	return output, nil
}

// Delete destroys a Terraform-managed recipe.
func (e *RecipeEngine) Delete(ctx context.Context, envConfig *recipes.Configuration, envRecipe *recipes.EnvironmentDefinition, resourceRecipe *recipes.ResourceMetadata) error {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Deleting Terraform recipe",
		"recipe", envRecipe.Name,
		"resource", resourceRecipe.ResourceID)

	opts := Options{
		RootDir:        e.rootDir,
		EnvConfig:      envConfig,
		EnvRecipe:      envRecipe,
		ResourceRecipe: resourceRecipe,
	}

	if err := e.executor.Delete(ctx, opts); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	logger.Info("Terraform recipe deleted successfully")
	return nil
}
