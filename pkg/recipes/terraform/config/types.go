package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// TerraformConfig represents the generated main.tf.json configuration.
type TerraformConfig struct {
	Terraform *TerraformDefinition        `json:"terraform,omitempty"`
	Provider  map[string][]map[string]any `json:"provider,omitempty"`
	Module    map[string]ModuleConfig     `json:"module,omitempty"`
	Output    map[string]any              `json:"output,omitempty"`
}

// TerraformDefinition holds the terraform block.
type TerraformDefinition struct {
	Backend          map[string]any                    `json:"backend,omitempty"`
	RequiredProviders map[string]RequiredProviderConfig `json:"required_providers,omitempty"`
}

// RequiredProviderConfig defines a required provider.
type RequiredProviderConfig struct {
	Source  string `json:"source"`
	Version string `json:"version,omitempty"`
}

// ModuleConfig defines a Terraform module reference.
type ModuleConfig struct {
	Source     string         `json:"source"`
	Version   string         `json:"version,omitempty"`
	Parameters map[string]any `json:"-"`
}

// MarshalJSON custom marshals ModuleConfig to inline parameters.
func (m ModuleConfig) MarshalJSON() ([]byte, error) {
	result := map[string]any{
		"source": m.Source,
	}
	if m.Version != "" {
		result["version"] = m.Version
	}
	for k, v := range m.Parameters {
		result[k] = v
	}
	return json.Marshal(result)
}

// New creates a new TerraformConfig with defaults.
func New() *TerraformConfig {
	return &TerraformConfig{
		Provider: make(map[string][]map[string]any),
		Module:   make(map[string]ModuleConfig),
		Output:   make(map[string]any),
	}
}

// SetModule sets the recipe module configuration.
func (c *TerraformConfig) SetModule(name, source, version string, params map[string]any) {
	c.Module[name] = ModuleConfig{
		Source:     source,
		Version:   version,
		Parameters: params,
	}
}

// SetBackend sets the Terraform backend configuration.
func (c *TerraformConfig) SetBackend(backendType string, config map[string]any) {
	if c.Terraform == nil {
		c.Terraform = &TerraformDefinition{}
	}
	c.Terraform.Backend = map[string]any{
		backendType: config,
	}
}

// AddProvider adds a provider configuration.
func (c *TerraformConfig) AddProvider(name string, config map[string]any) {
	c.Provider[name] = append(c.Provider[name], config)
}

// AddOutput adds an output definition.
func (c *TerraformConfig) AddOutput(name string, value any) {
	c.Output[name] = value
}

// AddResultOutput adds a standard result output that captures module outputs.
func (c *TerraformConfig) AddResultOutput(moduleName string) {
	c.Output["result"] = map[string]any{
		"value": "${module." + moduleName + "}",
	}
}

// Save writes the configuration to a main.tf.json file in the given directory.
func (c *TerraformConfig) Save(dir string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "main.tf.json"), data, 0644)
}
