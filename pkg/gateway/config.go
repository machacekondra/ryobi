package gateway

import "fmt"

// Config holds the server configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"databaseProvider"`
	Queue    QueueConfig    `yaml:"queueProvider"`
	Secret   SecretConfig   `yaml:"secretProvider"`
	Logging   LoggingConfig   `yaml:"logging"`
	Terraform TerraformExecConfig `yaml:"terraform"`
	Worker    WorkerConfig   `yaml:"workerServer"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Address returns the listening address.
func (s ServerConfig) Address() string {
	if s.Host == "" && s.Port == 0 {
		return "0.0.0.0:9000"
	}
	host := s.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := s.Port
	if port == 0 {
		port = 9000
	}
	return host + ":" + fmt.Sprintf("%d", port)
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Provider   string         `yaml:"provider"`
	PostgreSQL PostgresConfig `yaml:"postgresql"`
}

// PostgresConfig holds PostgreSQL connection settings.
type PostgresConfig struct {
	URL string `yaml:"url"`
}

// QueueConfig holds queue settings.
type QueueConfig struct {
	Provider string `yaml:"provider"`
}

// SecretConfig holds secret provider settings.
type SecretConfig struct {
	Provider string           `yaml:"provider"`
	File     FileSecretConfig `yaml:"file"`
}

// FileSecretConfig holds file-based secret provider settings.
type FileSecretConfig struct {
	Directory string `yaml:"directory"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level string `yaml:"level"`
}

// TerraformExecConfig holds Terraform execution settings.
type TerraformExecConfig struct {
	RootDir      string `yaml:"rootDir"`
	StateBackend string `yaml:"stateBackend"`
}

// WorkerConfig holds async worker settings.
type WorkerConfig struct {
	MaxConcurrency int `yaml:"maxOperationConcurrency"`
	MaxRetryCount  int `yaml:"maxOperationRetryCount"`
}
