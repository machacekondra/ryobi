package yaml

// Document represents a parsed YAML application or environment definition.
type Document struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`

	// Application fields
	Resources []ResourceSpec `yaml:"resources,omitempty"`

	// Environment fields
	Providers    map[string]ProviderSpec             `yaml:"providers,omitempty"`
	Recipes      map[string]map[string]RecipeDefSpec `yaml:"recipes,omitempty"`
	RecipeConfig *RecipeConfigSpec                   `yaml:"recipeConfig,omitempty"`
}

// Metadata holds resource metadata.
type Metadata struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment,omitempty"`
}

// ResourceSpec defines a Terraform-managed resource in an application.
type ResourceSpec struct {
	Name        string          `yaml:"name"`
	Type        string          `yaml:"type"`
	Recipe      string          `yaml:"recipe,omitempty"`
	Parameters  map[string]any  `yaml:"parameters,omitempty"`
	Connections []ConnectionSpec `yaml:"connections,omitempty"`
	Placement   *PlacementSpec  `yaml:"placement,omitempty"`
}

// PlacementSpec defines constraints and preferences for automatic placement.
type PlacementSpec struct {
	Constraints ConstraintsSpec `yaml:"constraints,omitempty"`
	Preferences PreferencesSpec `yaml:"preferences,omitempty"`
}

// ConstraintsSpec defines hard requirements for placement.
type ConstraintsSpec struct {
	Region       string   `yaml:"region,omitempty"`
	Sovereignty  string   `yaml:"sovereignty,omitempty"`
	Capabilities []string `yaml:"capabilities,omitempty"`
}

// PreferencesSpec defines soft preferences for placement scoring.
type PreferencesSpec struct {
	Cost               string `yaml:"cost,omitempty"`               // "minimize"
	Latency            string `yaml:"latency,omitempty"`            // "minimize"
	AvailableResources string `yaml:"availableResources,omitempty"` // "maximize"
}

// ConnectionSpec defines a connection to another resource's outputs.
type ConnectionSpec struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"`
	Key    string `yaml:"key,omitempty"`
}

// ProviderSpec defines a cloud provider scope.
type ProviderSpec struct {
	Scope string `yaml:"scope,omitempty"`
}

// RecipeDefSpec defines a recipe template in an environment.
type RecipeDefSpec struct {
	TemplateKind string         `yaml:"templateKind"`
	TemplatePath string         `yaml:"templatePath"`
	Parameters   map[string]any `yaml:"parameters,omitempty"`
}

// RecipeConfigSpec holds global recipe configuration.
type RecipeConfigSpec struct {
	Terraform *TerraformSpec `yaml:"terraform,omitempty"`
}

// TerraformSpec holds Terraform provider configuration.
type TerraformSpec struct {
	Providers      map[string]map[string]any `yaml:"providers,omitempty"`
	Authentication map[string]any            `yaml:"authentication,omitempty"`
}
