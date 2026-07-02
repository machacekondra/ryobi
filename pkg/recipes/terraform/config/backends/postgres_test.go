package backends

import (
	"testing"

	"github.com/ryobi-project/ryobi/pkg/recipes"
)

func TestPostgresBackend_BuildBackend(t *testing.T) {
	backend := NewPostgresBackend("postgres://ryobi:pass@localhost:5432/ryobi")

	resourceMeta := &recipes.ResourceMetadata{
		ResourceID: "/api/v1/ryobi/applications/myapp/ryobi/resources/postgres-db",
	}

	backendType, config, err := backend.BuildBackend(resourceMeta)
	if err != nil {
		t.Fatalf("BuildBackend failed: %v", err)
	}

	if backendType != "pg" {
		t.Errorf("expected backend type 'pg', got %q", backendType)
	}

	connStr, ok := config["conn_str"].(string)
	if !ok || connStr != "postgres://ryobi:pass@localhost:5432/ryobi" {
		t.Errorf("unexpected conn_str: %v", config["conn_str"])
	}

	schemaName, ok := config["schema_name"].(string)
	if !ok || schemaName == "" {
		t.Errorf("schema_name should be non-empty, got %v", config["schema_name"])
	}

	if len(schemaName) <= len(SchemaNamePrefix) {
		t.Errorf("schema_name should include prefix + hash, got %q", schemaName)
	}
}

func TestPostgresBackend_DeterministicSchema(t *testing.T) {
	backend := NewPostgresBackend("postgres://localhost/db")

	meta := &recipes.ResourceMetadata{
		ResourceID: "/api/v1/apps/myapp/resources/db",
	}

	_, config1, _ := backend.BuildBackend(meta)
	_, config2, _ := backend.BuildBackend(meta)

	if config1["schema_name"] != config2["schema_name"] {
		t.Error("same resource ID should produce same schema name")
	}
}

func TestPostgresBackend_UniqueSchemas(t *testing.T) {
	backend := NewPostgresBackend("postgres://localhost/db")

	_, config1, _ := backend.BuildBackend(&recipes.ResourceMetadata{ResourceID: "/resource/a"})
	_, config2, _ := backend.BuildBackend(&recipes.ResourceMetadata{ResourceID: "/resource/b"})

	if config1["schema_name"] == config2["schema_name"] {
		t.Error("different resource IDs should produce different schema names")
	}
}
