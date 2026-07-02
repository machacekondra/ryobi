package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTerraformConfig_SetModule(t *testing.T) {
	cfg := New()
	cfg.SetModule("recipe", "ghcr.io/myorg/recipes/postgres:1.0", "", map[string]any{
		"sku": "Standard",
	})

	if len(cfg.Module) != 1 {
		t.Fatalf("expected 1 module, got %d", len(cfg.Module))
	}

	mod := cfg.Module["recipe"]
	if mod.Source != "ghcr.io/myorg/recipes/postgres:1.0" {
		t.Errorf("unexpected source: %s", mod.Source)
	}
	if mod.Parameters["sku"] != "Standard" {
		t.Errorf("unexpected parameter sku: %v", mod.Parameters["sku"])
	}
}

func TestTerraformConfig_SetBackend(t *testing.T) {
	cfg := New()
	cfg.SetBackend("pg", map[string]any{
		"conn_str":    "postgres://localhost/db",
		"schema_name": "tfstate_abc123",
	})

	if cfg.Terraform == nil {
		t.Fatal("Terraform block should be set")
	}
	if cfg.Terraform.Backend == nil {
		t.Fatal("Backend should be set")
	}

	pgConfig, ok := cfg.Terraform.Backend["pg"].(map[string]any)
	if !ok {
		t.Fatal("pg backend config should be a map")
	}
	if pgConfig["conn_str"] != "postgres://localhost/db" {
		t.Errorf("unexpected conn_str: %v", pgConfig["conn_str"])
	}
}

func TestTerraformConfig_Save(t *testing.T) {
	dir := t.TempDir()

	cfg := New()
	cfg.SetModule("recipe", "hashicorp/consul/aws", "0.1.0", map[string]any{
		"region": "us-east-1",
	})
	cfg.SetBackend("local", map[string]any{
		"path": "/tmp/state.tfstate",
	})
	cfg.AddProvider("aws", map[string]any{
		"region": "us-east-1",
	})
	cfg.AddResultOutput("recipe")

	if err := cfg.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "main.tf.json"))
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}

	// Verify terraform block exists
	if _, ok := parsed["terraform"]; !ok {
		t.Error("expected 'terraform' block in output")
	}

	// Verify module block exists
	if _, ok := parsed["module"]; !ok {
		t.Error("expected 'module' block in output")
	}

	// Verify provider block exists
	if _, ok := parsed["provider"]; !ok {
		t.Error("expected 'provider' block in output")
	}

	// Verify output block exists
	if _, ok := parsed["output"]; !ok {
		t.Error("expected 'output' block in output")
	}
}
