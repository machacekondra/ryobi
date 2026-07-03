package placement

import (
	"testing"
)

func TestPlace_ConstraintRegion(t *testing.T) {
	engine := NewEngine()

	envs := []EnvironmentInfo{
		{Name: "us-env", Connected: true, Static: StaticCapabilities{Region: "us-east-1"}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
		{Name: "eu-env", Connected: true, Static: StaticCapabilities{Region: "eu-west-1"}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	}

	result, err := engine.Place(PlacementRequest{
		ResourceType: "Ryobi.Compute/containers",
		Constraints:  Constraints{Region: "eu-west-1"},
	}, envs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "eu-env" {
		t.Errorf("expected eu-env, got %s", result.EnvironmentName)
	}
}

func TestPlace_ConstraintSovereignty(t *testing.T) {
	engine := NewEngine()

	envs := []EnvironmentInfo{
		{Name: "us-env", Connected: true, Static: StaticCapabilities{Sovereignty: "us"}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
		{Name: "eu-env", Connected: true, Static: StaticCapabilities{Sovereignty: "eu"}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	}

	result, err := engine.Place(PlacementRequest{
		ResourceType: "Ryobi.Compute/containers",
		Constraints:  Constraints{Sovereignty: "eu"},
	}, envs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "eu-env" {
		t.Errorf("expected eu-env, got %s", result.EnvironmentName)
	}
}

func TestPlace_ConstraintCapabilities(t *testing.T) {
	engine := NewEngine()

	envs := []EnvironmentInfo{
		{Name: "basic", Connected: true, Static: StaticCapabilities{Capabilities: []string{"standard"}}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
		{Name: "gpu", Connected: true, Static: StaticCapabilities{Capabilities: []string{"standard", "gpu"}}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	}

	result, err := engine.Place(PlacementRequest{
		ResourceType: "Ryobi.Compute/containers",
		Constraints:  Constraints{Capabilities: []string{"gpu"}},
	}, envs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "gpu" {
		t.Errorf("expected gpu, got %s", result.EnvironmentName)
	}
}

func TestPlace_PreferCheapest(t *testing.T) {
	engine := NewEngine()

	envs := []EnvironmentInfo{
		{Name: "expensive", Connected: true, Static: StaticCapabilities{CostPerHour: 2.0}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
		{Name: "cheap", Connected: true, Static: StaticCapabilities{CostPerHour: 0.1}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	}

	result, err := engine.Place(PlacementRequest{
		ResourceType: "Ryobi.Compute/containers",
		Preferences:  Preferences{Cost: "minimize"},
	}, envs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "cheap" {
		t.Errorf("expected cheap, got %s", result.EnvironmentName)
	}
}

func TestPlace_PreferMostResources(t *testing.T) {
	engine := NewEngine()

	envs := []EnvironmentInfo{
		{Name: "busy", Connected: true, Dynamic: DynamicCapabilities{AvailableCPUMillicores: 1000, AvailableMemoryMB: 2048}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
		{Name: "idle", Connected: true, Dynamic: DynamicCapabilities{AvailableCPUMillicores: 32000, AvailableMemoryMB: 65536}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	}

	result, err := engine.Place(PlacementRequest{
		ResourceType: "Ryobi.Compute/containers",
		Preferences:  Preferences{AvailableResources: "maximize"},
	}, envs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "idle" {
		t.Errorf("expected idle, got %s", result.EnvironmentName)
	}
}

func TestPlace_NoMatchingEnvironment(t *testing.T) {
	engine := NewEngine()

	envs := []EnvironmentInfo{
		{Name: "us-env", Connected: true, Static: StaticCapabilities{Region: "us-east-1"}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	}

	_, err := engine.Place(PlacementRequest{
		ResourceType: "Ryobi.Compute/containers",
		Constraints:  Constraints{Region: "eu-west-1"},
	}, envs)

	if err == nil {
		t.Error("expected error for no matching environment")
	}
}

func TestPlace_DisconnectedEnvSkipped(t *testing.T) {
	engine := NewEngine()

	envs := []EnvironmentInfo{
		{Name: "offline", Connected: false, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
		{Name: "online", Connected: true, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	}

	result, err := engine.Place(PlacementRequest{
		ResourceType: "Ryobi.Compute/containers",
	}, envs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "online" {
		t.Errorf("expected online, got %s", result.EnvironmentName)
	}
}

func TestPlace_UnsupportedResourceType(t *testing.T) {
	engine := NewEngine()

	envs := []EnvironmentInfo{
		{Name: "env1", Connected: true, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	}

	_, err := engine.Place(PlacementRequest{
		ResourceType: "Ryobi.Compute/virtualMachines",
	}, envs)

	if err == nil {
		t.Error("expected error for unsupported resource type")
	}
}
