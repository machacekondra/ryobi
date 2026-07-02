package database

import (
	"context"
	"testing"
)

func TestInMemoryClient_SaveAndGet(t *testing.T) {
	client := NewInMemoryClient()
	ctx := context.Background()

	obj := &Object{
		Metadata: Metadata{
			ID:           "/api/v1/ryobi/environments/dev",
			ResourceType: "ryobi/environments",
			RootScope:    "/api/v1",
		},
		Data: map[string]any{
			"name": "dev",
			"properties": map[string]any{
				"compute": map[string]any{"kind": "podman"},
			},
		},
	}

	err := client.Save(ctx, obj)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if obj.ETag == "" {
		t.Fatal("ETag should be set after save")
	}

	got, err := client.Get(ctx, "/api/v1/ryobi/environments/dev")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	var data map[string]any
	if err := got.As(&data); err != nil {
		t.Fatalf("As failed: %v", err)
	}

	if data["name"] != "dev" {
		t.Errorf("expected name=dev, got %v", data["name"])
	}
}

func TestInMemoryClient_Query(t *testing.T) {
	client := NewInMemoryClient()
	ctx := context.Background()

	for _, name := range []string{"dev", "staging", "prod"} {
		obj := &Object{
			Metadata: Metadata{
				ID:           "/api/v1/ryobi/environments/" + name,
				ResourceType: "ryobi/environments",
				RootScope:    "/api/v1",
			},
			Data: map[string]any{"name": name},
		}
		if err := client.Save(ctx, obj); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	result, err := client.Query(ctx, Query{
		RootScope:    "/api/v1",
		ResourceType: "ryobi/environments",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(result.Items))
	}
}

func TestInMemoryClient_Delete(t *testing.T) {
	client := NewInMemoryClient()
	ctx := context.Background()

	obj := &Object{
		Metadata: Metadata{
			ID:           "/api/v1/ryobi/environments/test",
			ResourceType: "ryobi/environments",
			RootScope:    "/api/v1",
		},
		Data: map[string]any{"name": "test"},
	}
	_ = client.Save(ctx, obj)

	err := client.Delete(ctx, "/api/v1/ryobi/environments/test")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = client.Get(ctx, "/api/v1/ryobi/environments/test")
	if err == nil {
		t.Fatal("expected ErrNotFound after delete")
	}
}

func TestInMemoryClient_ConcurrencyCheck(t *testing.T) {
	client := NewInMemoryClient()
	ctx := context.Background()

	obj := &Object{
		Metadata: Metadata{
			ID:           "/api/v1/ryobi/environments/test",
			ResourceType: "ryobi/environments",
			RootScope:    "/api/v1",
		},
		Data: map[string]any{"name": "test"},
	}
	_ = client.Save(ctx, obj)
	firstETag := obj.ETag

	// Save again to change the etag
	_ = client.Save(ctx, obj)

	// Try saving with old etag
	obj.Data = map[string]any{"name": "updated"}
	err := client.Save(ctx, obj, WithETag(firstETag))
	if err == nil {
		t.Fatal("expected ErrConcurrency")
	}
}
