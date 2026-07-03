package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EnvConfig is the configuration for the environment agent.
type EnvConfig struct {
	// Name of the environment to register.
	Name string `yaml:"name"`

	// Server is the ryobid connection settings.
	Server ServerConfig `yaml:"server"`

	// Providers defines cloud provider scopes.
	Providers map[string]ProviderCfg `yaml:"providers"`

	// TerraformProviders configures Terraform provider blocks.
	TerraformProviders map[string]map[string]any `yaml:"terraformProviders"`

	// Recipes maps resource types to recipe definitions.
	Recipes []RecipeCfg `yaml:"recipes"`

	// Capabilities describes this environment's static properties for placement.
	Capabilities CapabilitiesCfg `yaml:"capabilities"`

	// Management UI settings.
	Management ManagementCfg `yaml:"management"`

	// Terraform execution settings.
	Terraform TerraformCfg `yaml:"terraform"`
}

type ManagementCfg struct {
	Enabled bool   `yaml:"enabled"`
	Address string `yaml:"address"`
}

type CapabilitiesCfg struct {
	Region       string   `yaml:"region" json:"region"`
	Sovereignty  string   `yaml:"sovereignty" json:"sovereignty"`
	Capabilities []string `yaml:"capabilities" json:"capabilities"`
	CostPerHour  float64  `yaml:"costPerHour" json:"costPerHour"`
	MaxReplicas  int32    `yaml:"maxReplicas" json:"maxReplicas"`
}

type ServerConfig struct {
	GRPCAddress string `yaml:"grpcAddress"`
}

type ProviderCfg struct {
	Scope string `yaml:"scope"`
}

type RecipeCfg struct {
	ResourceType string            `yaml:"resourceType" json:"resourceType"`
	RecipeName   string            `yaml:"recipeName" json:"recipeName"`
	TemplatePath string            `yaml:"templatePath" json:"templatePath"`
	Parameters   map[string]string `yaml:"parameters,omitempty" json:"parameters,omitempty"`
}

type TerraformCfg struct {
	BinaryPath   string `yaml:"binaryPath"`
	WorkDir      string `yaml:"workDir"`
	StateBackend string `yaml:"stateBackend"`
	DatabaseURL  string `yaml:"databaseUrl"`
}

// LoadConfig reads and parses the environment agent config file.
func LoadConfig(path string) (*EnvConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	var cfg EnvConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Defaults
	if cfg.Server.GRPCAddress == "" {
		cfg.Server.GRPCAddress = "localhost:9001"
	}
	if cfg.Terraform.BinaryPath == "" {
		cfg.Terraform.BinaryPath = "terraform"
	}
	if cfg.Terraform.WorkDir == "" {
		home, _ := os.UserHomeDir()
		cfg.Terraform.WorkDir = filepath.Join(home, ".ryobi", "terraform")
	}
	if cfg.Management.Address == "" {
		cfg.Management.Address = "0.0.0.0:9090"
	}

	// Resolve relative template paths to absolute
	configDir, _ := filepath.Abs(filepath.Dir(path))
	for i := range cfg.Recipes {
		tp := cfg.Recipes[i].TemplatePath
		if !filepath.IsAbs(tp) {
			cfg.Recipes[i].TemplatePath = filepath.Join(configDir, tp)
		}
	}

	return &cfg, nil
}
