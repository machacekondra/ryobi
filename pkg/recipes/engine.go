package recipes

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
)

// Engine orchestrates recipe execution.
type Engine interface {
	// Execute deploys a recipe for the given resource.
	Execute(ctx context.Context, envConfig *Configuration, envRecipe *EnvironmentDefinition, resourceRecipe *ResourceMetadata) (*RecipeOutput, error)

	// Delete destroys a recipe-managed resource.
	Delete(ctx context.Context, envConfig *Configuration, envRecipe *EnvironmentDefinition, resourceRecipe *ResourceMetadata) error
}

// ResolveRecipe looks up a recipe definition from an environment's recipe registry.
func ResolveRecipe(envRecipes map[string]map[string]EnvironmentDefinition, resourceType string, recipeName string) (*EnvironmentDefinition, error) {
	logger := logr.Discard()
	_ = logger

	typeRecipes, ok := envRecipes[resourceType]
	if !ok {
		return nil, fmt.Errorf("no recipes registered for resource type %q", resourceType)
	}

	recipe, ok := typeRecipes[recipeName]
	if !ok {
		return nil, fmt.Errorf("recipe %q not found for resource type %q", recipeName, resourceType)
	}

	return &recipe, nil
}
