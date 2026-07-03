package yaml

import (
	"testing"
)

func TestParse_Application(t *testing.T) {
	input := `
apiVersion: ryobi/v1
kind: Application
metadata:
  name: my-app
  environment: prod

resources:
  - name: postgres-db
    type: Applications.Datastores/postgresDatabases
    recipe: default
  - name: redis-cache
    type: Applications.Datastores/redisCaches
    recipe: default
    parameters:
      sku: Standard
`

	docs, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	doc := docs[0]
	if doc.Kind != KindApplication {
		t.Errorf("expected kind Application, got %s", doc.Kind)
	}
	if doc.Metadata.Name != "my-app" {
		t.Errorf("expected name my-app, got %s", doc.Metadata.Name)
	}
	if doc.Metadata.Environment != "prod" {
		t.Errorf("expected environment prod, got %s", doc.Metadata.Environment)
	}
	if len(doc.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(doc.Resources))
	}
	if doc.Resources[0].Name != "postgres-db" {
		t.Errorf("expected first resource postgres-db, got %s", doc.Resources[0].Name)
	}
	if doc.Resources[1].Parameters["sku"] != "Standard" {
		t.Errorf("expected sku=Standard, got %v", doc.Resources[1].Parameters["sku"])
	}
}

func TestParse_Environment(t *testing.T) {
	input := `
apiVersion: ryobi/v1
kind: Environment
metadata:
  name: prod
providers:
  azure:
    scope: /subscriptions/sub-123/resourceGroups/my-rg
recipes:
  Applications.Datastores/postgresDatabases:
    default:
      templateKind: terraform
      templatePath: ghcr.io/myorg/recipes/postgres:1.0
`

	docs, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	doc := docs[0]
	if doc.Kind != KindEnvironment {
		t.Errorf("expected kind Environment, got %s", doc.Kind)
	}
	if doc.Providers["azure"].Scope != "/subscriptions/sub-123/resourceGroups/my-rg" {
		t.Errorf("unexpected azure scope: %s", doc.Providers["azure"].Scope)
	}
	recipeDef := doc.Recipes["Applications.Datastores/postgresDatabases"]["default"]
	if recipeDef.TemplatePath != "ghcr.io/myorg/recipes/postgres:1.0" {
		t.Errorf("unexpected template path: %s", recipeDef.TemplatePath)
	}
}

func TestParse_MultiDocument(t *testing.T) {
	input := `
apiVersion: ryobi/v1
kind: Environment
metadata:
  name: dev
---
apiVersion: ryobi/v1
kind: Application
metadata:
  name: my-app
  environment: dev
resources:
  - name: db
    type: Datastores/postgres
    recipe: default
`

	docs, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	if docs[0].Kind != KindEnvironment {
		t.Errorf("expected first doc to be Environment, got %s", docs[0].Kind)
	}
	if docs[1].Kind != KindApplication {
		t.Errorf("expected second doc to be Application, got %s", docs[1].Kind)
	}
}

func TestValidate_ApplicationMissingEnv_Allowed(t *testing.T) {
	doc := &Document{
		APIVersion: APIVersionV1,
		Kind:       KindApplication,
		Metadata:   Metadata{Name: "test"},
	}
	// Environment is now optional — placement engine decides
	if err := Validate(doc); err != nil {
		t.Errorf("environment should be optional, got error: %v", err)
	}
}

func TestValidate_ResourceMissingFields(t *testing.T) {
	doc := &Document{
		APIVersion: APIVersionV1,
		Kind:       KindApplication,
		Metadata:   Metadata{Name: "test", Environment: "dev"},
		Resources:  []ResourceSpec{{Name: "db"}},
	}
	err := Validate(doc)
	if err == nil {
		t.Error("expected validation error for resource missing type")
	}
}

func TestValidate_ValidApplication(t *testing.T) {
	doc := &Document{
		APIVersion: APIVersionV1,
		Kind:       KindApplication,
		Metadata:   Metadata{Name: "test", Environment: "dev"},
		Resources: []ResourceSpec{
			{Name: "db", Type: "Datastores/postgres", Recipe: "default"},
		},
	}
	if err := Validate(doc); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}
