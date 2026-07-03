package placement

// PlacementRequest is the input to the placement engine.
type PlacementRequest struct {
	ResourceType string
	Constraints  Constraints
	Preferences  Preferences
}

// Constraints are hard requirements — an environment must match all of them.
type Constraints struct {
	Region       string   `json:"region,omitempty" yaml:"region,omitempty"`
	Sovereignty  string   `json:"sovereignty,omitempty" yaml:"sovereignty,omitempty"`
	Capabilities []string `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
}

// Preferences are soft scoring criteria — used to rank matching environments.
type Preferences struct {
	Cost               string `json:"cost,omitempty" yaml:"cost,omitempty"`                               // "minimize"
	Latency            string `json:"latency,omitempty" yaml:"latency,omitempty"`                         // "minimize"
	AvailableResources string `json:"availableResources,omitempty" yaml:"availableResources,omitempty"`   // "maximize"
}

// PlacementResult is the output of the placement engine.
type PlacementResult struct {
	EnvironmentName string
	RecipeName      string
}

// EnvironmentInfo holds everything the placement engine knows about an environment.
type EnvironmentInfo struct {
	Name         string
	Static       StaticCapabilities
	Dynamic      DynamicCapabilities
	RecipeTypes  map[string]string // resource type → recipe name
	Connected    bool
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
	LastHeartbeat          int64 // unix timestamp
}

// Placement is the YAML structure for per-resource placement policy.
type Placement struct {
	Constraints Constraints `json:"constraints,omitempty" yaml:"constraints,omitempty"`
	Preferences Preferences `json:"preferences,omitempty" yaml:"preferences,omitempty"`
}
