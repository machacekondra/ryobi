package placement

import (
	"testing"
)

var testEnvs = []EnvironmentInfo{
	{Name: "us-cheap", Connected: true, Static: StaticCapabilities{Region: "us-east-1", Sovereignty: "us", CostPerHour: 0.30, Capabilities: []string{"standard", "gpu"}}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	{Name: "eu-expensive", Connected: true, Static: StaticCapabilities{Region: "eu-west-1", Sovereignty: "eu", CostPerHour: 0.80}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	{Name: "eu-cheap", Connected: true, Static: StaticCapabilities{Region: "eu-west-1", Sovereignty: "eu", CostPerHour: 0.40}, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
}

func TestPlace_RuleConstraintRegion(t *testing.T) {
	engine := NewEngine()
	rules := []PlacementRule{
		{Name: "eu-only", Properties: PlacementRuleProperties{ResourceType: "Ryobi.Compute/containers", Constraints: Constraints{Region: "eu-west-1"}}},
	}

	result, err := engine.Place(PlacementRequest{ResourceType: "Ryobi.Compute/containers"}, rules, testEnvs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "eu-expensive" && result.EnvironmentName != "eu-cheap" {
		t.Errorf("expected an EU env, got %s", result.EnvironmentName)
	}
	if result.RuleName != "eu-only" {
		t.Errorf("expected rule eu-only, got %s", result.RuleName)
	}
}

func TestPlace_RuleConstraintRegionAndPreferCost(t *testing.T) {
	engine := NewEngine()
	rules := []PlacementRule{
		{Name: "eu-cheapest", Properties: PlacementRuleProperties{
			ResourceType: "Ryobi.Compute/containers",
			Constraints:  Constraints{Region: "eu-west-1"},
			Preferences:  Preferences{Cost: "minimize"},
		}},
	}

	result, err := engine.Place(PlacementRequest{ResourceType: "Ryobi.Compute/containers"}, rules, testEnvs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "eu-cheap" {
		t.Errorf("expected eu-cheap, got %s", result.EnvironmentName)
	}
}

func TestPlace_RuleConstraintSovereignty(t *testing.T) {
	engine := NewEngine()
	rules := []PlacementRule{
		{Name: "us-sovereign", Properties: PlacementRuleProperties{
			ResourceType: "Ryobi.Compute/containers",
			Constraints:  Constraints{Sovereignty: "us"},
		}},
	}

	result, err := engine.Place(PlacementRequest{ResourceType: "Ryobi.Compute/containers"}, rules, testEnvs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "us-cheap" {
		t.Errorf("expected us-cheap, got %s", result.EnvironmentName)
	}
}

func TestPlace_RuleConstraintCapabilities(t *testing.T) {
	engine := NewEngine()
	rules := []PlacementRule{
		{Name: "gpu-required", Properties: PlacementRuleProperties{
			ResourceType: "Ryobi.Compute/containers",
			Constraints:  Constraints{Capabilities: []string{"gpu"}},
		}},
	}

	result, err := engine.Place(PlacementRequest{ResourceType: "Ryobi.Compute/containers"}, rules, testEnvs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "us-cheap" {
		t.Errorf("expected us-cheap (only one with gpu), got %s", result.EnvironmentName)
	}
}

func TestPlace_RulePriority(t *testing.T) {
	engine := NewEngine()
	rules := []PlacementRule{
		{Name: "low-priority", Properties: PlacementRuleProperties{ResourceType: "Ryobi.Compute/containers", Priority: 1, Constraints: Constraints{Region: "us-east-1"}}},
		{Name: "high-priority", Properties: PlacementRuleProperties{ResourceType: "Ryobi.Compute/containers", Priority: 10, Constraints: Constraints{Region: "eu-west-1"}, Preferences: Preferences{Cost: "minimize"}}},
	}

	result, err := engine.Place(PlacementRequest{ResourceType: "Ryobi.Compute/containers"}, rules, testEnvs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// High priority rule (eu, cheapest) should win
	if result.EnvironmentName != "eu-cheap" {
		t.Errorf("expected eu-cheap (high priority rule), got %s", result.EnvironmentName)
	}
	if result.RuleName != "high-priority" {
		t.Errorf("expected rule high-priority, got %s", result.RuleName)
	}
}

func TestPlace_NoRulesFallback(t *testing.T) {
	engine := NewEngine()

	result, err := engine.Place(PlacementRequest{ResourceType: "Ryobi.Compute/containers"}, nil, testEnvs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should pick any connected env — no error
	if result.EnvironmentName == "" {
		t.Error("expected a result")
	}
}

func TestPlace_NoMatchingRule(t *testing.T) {
	engine := NewEngine()
	rules := []PlacementRule{
		{Name: "asia-only", Properties: PlacementRuleProperties{ResourceType: "Ryobi.Compute/containers", Constraints: Constraints{Region: "ap-southeast-1"}}},
	}

	_, err := engine.Place(PlacementRequest{ResourceType: "Ryobi.Compute/containers"}, rules, testEnvs)
	if err == nil {
		t.Error("expected error for no matching environment")
	}
}

func TestPlace_DisconnectedSkipped(t *testing.T) {
	engine := NewEngine()
	envs := []EnvironmentInfo{
		{Name: "offline", Connected: false, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
		{Name: "online", Connected: true, RecipeTypes: map[string]string{"Ryobi.Compute/containers": "k8s"}},
	}

	result, err := engine.Place(PlacementRequest{ResourceType: "Ryobi.Compute/containers"}, nil, envs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EnvironmentName != "online" {
		t.Errorf("expected online, got %s", result.EnvironmentName)
	}
}

func TestPlace_UnsupportedResourceType(t *testing.T) {
	engine := NewEngine()

	_, err := engine.Place(PlacementRequest{ResourceType: "Ryobi.Compute/virtualMachines"}, nil, testEnvs)
	if err == nil {
		t.Error("expected error for unsupported resource type")
	}
}
