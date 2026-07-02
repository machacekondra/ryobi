package recipes

// Configuration holds the environment-level recipe configuration.
type Configuration struct {
	Providers    map[string]ProviderConfig `json:"providers,omitempty"`
	RecipeConfig RecipeConfigProperties    `json:"recipeConfig,omitempty"`
}

// ProviderConfig holds cloud provider scope.
type ProviderConfig struct {
	Scope string `json:"scope,omitempty"`
}

// RecipeConfigProperties holds recipe-level config (TF providers, env vars).
type RecipeConfigProperties struct {
	Terraform TerraformConfigProperties `json:"terraform,omitempty"`
	Env       EnvironmentVariables      `json:"env,omitempty"`
}

// TerraformConfigProperties holds Terraform provider configurations.
type TerraformConfigProperties struct {
	Providers      map[string][]map[string]any `json:"providers,omitempty"`
	Authentication map[string]any              `json:"authentication,omitempty"`
}

// EnvironmentVariables holds additional env vars to pass to Terraform.
type EnvironmentVariables struct {
	AdditionalProperties map[string]string `json:"additionalProperties,omitempty"`
}

// EnvironmentDefinition defines a recipe template.
type EnvironmentDefinition struct {
	Name            string         `json:"name"`
	ResourceType    string         `json:"resourceType"`
	TemplatePath    string         `json:"templatePath"`
	TemplateVersion string         `json:"templateVersion,omitempty"`
	Parameters      map[string]any `json:"parameters,omitempty"`
}

// ResourceMetadata holds metadata about the resource being deployed.
type ResourceMetadata struct {
	Name          string         `json:"name"`
	ApplicationID string         `json:"applicationId,omitempty"`
	EnvironmentID string         `json:"environmentId,omitempty"`
	ResourceID    string         `json:"resourceId"`
	Parameters    map[string]any `json:"parameters,omitempty"`
}

// RecipeOutput holds the result of a recipe execution.
type RecipeOutput struct {
	Values          map[string]any `json:"values,omitempty"`
	Secrets         map[string]any `json:"secrets,omitempty"`
	Resources       []string       `json:"resources,omitempty"`
}
