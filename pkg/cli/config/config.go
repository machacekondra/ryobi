package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultServerURL = "http://localhost:9000"
	configFileName   = "config.yaml"
	configDirName    = ".ryobi"
)

// Config holds CLI configuration.
type Config struct {
	CurrentEnvironment string       `yaml:"currentEnvironment,omitempty"`
	Server             ServerConfig `yaml:"server"`
}

// ServerConfig holds server connection settings.
type ServerConfig struct {
	URL string `yaml:"url"`
}

// Load reads the CLI configuration from ~/.ryobi/config.yaml.
// Returns a default config if the file doesn't exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return defaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Server.URL == "" {
		cfg.Server.URL = DefaultServerURL
	}

	return &cfg, nil
}

// Save writes the CLI configuration to ~/.ryobi/config.yaml.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, configDirName, configFileName), nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			URL: DefaultServerURL,
		},
	}
}
