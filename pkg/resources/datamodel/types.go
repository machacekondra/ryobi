package datamodel

import "time"

// Environment represents a deployment target.
type Environment struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Type       string                `json:"type"`
	Properties EnvironmentProperties `json:"properties"`
	CreatedAt  time.Time             `json:"createdAt,omitempty"`
	UpdatedAt  time.Time             `json:"updatedAt,omitempty"`
}

// EnvironmentProperties holds environment configuration.
type EnvironmentProperties struct {
	Providers    map[string]ProviderConfig          `json:"providers,omitempty"`
	Recipes      map[string]map[string]RecipeConfig  `json:"recipes,omitempty"`
	RecipeConfig *RecipeGlobalConfig                `json:"recipeConfig,omitempty"`
}

// ProviderConfig holds cloud provider configuration.
type ProviderConfig struct {
	Scope string `json:"scope,omitempty"`
}

// RecipeConfig defines a recipe template.
type RecipeConfig struct {
	TemplateKind string         `json:"templateKind"`
	TemplatePath string         `json:"templatePath"`
	Parameters   map[string]any `json:"parameters,omitempty"`
}

// RecipeGlobalConfig holds global recipe configuration.
type RecipeGlobalConfig struct {
	Terraform *TerraformConfig `json:"terraform,omitempty"`
}

// TerraformConfig holds Terraform-specific configuration.
type TerraformConfig struct {
	Providers      map[string]map[string]any `json:"providers,omitempty"`
	Authentication map[string]any            `json:"authentication,omitempty"`
}

// Application represents a deployed application.
type Application struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Type       string                `json:"type"`
	Properties ApplicationProperties `json:"properties"`
	CreatedAt  time.Time             `json:"createdAt,omitempty"`
	UpdatedAt  time.Time             `json:"updatedAt,omitempty"`
}

// ApplicationProperties holds application configuration.
type ApplicationProperties struct {
	Environment string            `json:"environment"`
	Status      ApplicationStatus `json:"status,omitempty"`
}

// ApplicationStatus represents the current status of an application.
type ApplicationStatus struct {
	State string `json:"state,omitempty"`
}

// Resource represents a Terraform-managed resource within an application.
type Resource struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Type       string             `json:"type"`
	Properties ResourceProperties `json:"properties"`
	CreatedAt  time.Time          `json:"createdAt,omitempty"`
	UpdatedAt  time.Time          `json:"updatedAt,omitempty"`
}

// ResourceProperties holds resource configuration.
type ResourceProperties struct {
	ResourceType string           `json:"resourceType"`
	RecipeName   string           `json:"recipeName"`
	Parameters   map[string]any   `json:"parameters,omitempty"`
	Connections  []ConnectionRef  `json:"connections,omitempty"`
	Status       ResourceStatus   `json:"status,omitempty"`
}

// ConnectionRef references another resource's outputs.
type ConnectionRef struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Key    string `json:"key,omitempty"`
}

// ResourceStatus represents the status of a Terraform-managed resource.
type ResourceStatus struct {
	State           string         `json:"state,omitempty"`
	Outputs         map[string]any `json:"outputs,omitempty"`
	OutputResources []string       `json:"outputResources,omitempty"`
}

// Resource state constants.
const (
	StatePending   = "Pending"
	StateDeploying = "Deploying"
	StateSucceeded = "Succeeded"
	StateFailed    = "Failed"
	StateDeleting  = "Deleting"
)

// Resource type constants.
const (
	EnvironmentResourceType = "ryobi/environments"
	ApplicationResourceType = "ryobi/applications"
	ResourceResourceType    = "ryobi/resources"
)
