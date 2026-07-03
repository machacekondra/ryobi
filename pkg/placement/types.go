package placement

// PlacementRule is an administrator-defined API object that maps a resource type
// to placement constraints and preferences. Stored via the REST API.
type PlacementRule struct {
	ID         string              `json:"id,omitempty"`
	Name       string              `json:"name"`
	Type       string              `json:"type,omitempty"`
	Properties PlacementRuleProperties `json:"properties"`
}

// PlacementRuleProperties defines what a placement rule matches and how it ranks.
type PlacementRuleProperties struct {
	// ResourceType this rule applies to (e.g. "Ryobi.Compute/containers").
	ResourceType string `json:"resourceType"`

	// Constraints are hard requirements — environment must match all.
	Constraints Constraints `json:"constraints,omitempty"`

	// Preferences are soft scoring criteria for ranking matching environments.
	Preferences Preferences `json:"preferences,omitempty"`

	// Priority determines rule precedence when multiple rules match the same resource type.
	// Higher priority wins. Default is 0.
	Priority int `json:"priority,omitempty"`
}

// Constraints are hard requirements — an environment must match all of them.
type Constraints struct {
	Region       string   `json:"region,omitempty"`
	Sovereignty  string   `json:"sovereignty,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// Preferences are soft scoring criteria — used to rank matching environments.
type Preferences struct {
	Cost               string `json:"cost,omitempty"`               // "minimize"
	AvailableResources string `json:"availableResources,omitempty"` // "maximize"
}

// PlacementRequest is the input to the placement engine.
type PlacementRequest struct {
	ResourceType string
}

// PlacementResult is the output of the placement engine.
type PlacementResult struct {
	EnvironmentName string
	RecipeName      string
	RuleName        string
}

// EnvironmentInfo holds everything the placement engine knows about an environment.
type EnvironmentInfo struct {
	Name        string
	Static      StaticCapabilities
	Dynamic     DynamicCapabilities
	RecipeTypes map[string]string // resource type → recipe name
	Connected   bool
}

// StaticCapabilities are set at registration and don't change.
type StaticCapabilities struct {
	Region       string
	Sovereignty  string
	Capabilities []string
	CostPerHour  float64
	MaxReplicas  int32
}

// DynamicCapabilities are updated via heartbeat.
type DynamicCapabilities struct {
	AvailableCPUMillicores int64
	AvailableMemoryMB      int64
	RunningResources       int32
	LastHeartbeat          int64
}

// PlacementRuleResourceType is the API resource type for placement rules.
const PlacementRuleResourceType = "ryobi/placements"
