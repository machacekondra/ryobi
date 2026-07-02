package terraform

import (
	"context"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/ryobi-project/ryobi/pkg/recipes"
)

// Executor is the interface for executing Terraform operations.
type Executor interface {
	// Deploy runs terraform init + apply and returns the resulting state.
	Deploy(ctx context.Context, options Options) (*tfjson.State, error)

	// Delete runs terraform init + destroy.
	Delete(ctx context.Context, options Options) error
}

// Options holds the options for a Terraform execution.
type Options struct {
	// RootDir is the working directory for Terraform.
	RootDir string

	// EnvConfig holds environment-level configuration (providers, recipe config).
	EnvConfig *recipes.Configuration

	// EnvRecipe holds the recipe definition from the environment.
	EnvRecipe *recipes.EnvironmentDefinition

	// ResourceRecipe holds metadata about the resource being deployed.
	ResourceRecipe *recipes.ResourceMetadata
}
