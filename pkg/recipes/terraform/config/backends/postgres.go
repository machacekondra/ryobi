package backends

import (
	"crypto/sha256"
	"fmt"

	"github.com/ryobi-project/ryobi/pkg/recipes"
)

const (
	BackendPG         = "pg"
	SchemaNamePrefix  = "tfstate_"
)

// PostgresBackend implements the Backend interface using PostgreSQL's native pg backend.
type PostgresBackend struct {
	connStr string
}

// NewPostgresBackend creates a new PostgresBackend with the given connection string.
func NewPostgresBackend(connStr string) *PostgresBackend {
	return &PostgresBackend{connStr: connStr}
}

// BuildBackend returns the Terraform pg backend configuration.
// Each resource gets a unique schema name derived from its resource ID.
func (b *PostgresBackend) BuildBackend(resourceRecipe *recipes.ResourceMetadata) (string, map[string]any, error) {
	schemaName := generateSchemaName(resourceRecipe.ResourceID)

	config := map[string]any{
		"conn_str":    b.connStr,
		"schema_name": schemaName,
	}

	return BackendPG, config, nil
}

// generateSchemaName creates a unique, deterministic schema name from a resource ID.
func generateSchemaName(resourceID string) string {
	hash := sha256.Sum256([]byte(resourceID))
	return fmt.Sprintf("%s%x", SchemaNamePrefix, hash[:8])
}
