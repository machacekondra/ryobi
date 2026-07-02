package backends

import "github.com/ryobi-project/ryobi/pkg/recipes"

// Backend is the interface for Terraform state backend configuration.
type Backend interface {
	// BuildBackend returns the backend configuration block for the given resource.
	BuildBackend(resourceRecipe *recipes.ResourceMetadata) (backendType string, config map[string]any, error error)
}
