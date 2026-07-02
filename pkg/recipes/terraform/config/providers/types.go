package providers

import (
	"context"

	"github.com/ryobi-project/ryobi/pkg/recipes"
)

// Provider is the interface for generating Terraform provider configuration.
type Provider interface {
	// BuildConfig generates the provider configuration block.
	BuildConfig(ctx context.Context, envConfig *recipes.Configuration) (map[string]any, error)
}
