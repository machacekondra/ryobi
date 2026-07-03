# Design: Placement Engine

**Status:** Accepted
**Date:** 2026-07-03

## Context

In the initial design, deploying a resource required explicitly specifying both the target environment and the recipe name. This tightly couples applications to specific infrastructure, making it hard to:

- Deploy the same app across multiple regions
- Optimize for cost or capacity automatically
- Enforce data sovereignty requirements
- Move workloads between environments without changing YAML

The placement engine decouples the "what" from the "where" — users declare what they need (resource type + constraints), and the engine decides where it runs.

## Decision

### Per-Resource Placement

Placement decisions are made **per resource**, not per application. Different resources in the same application can land on different environments:

```yaml
resources:
  - name: eu-api
    type: Ryobi.Compute/containers
    placement:
      constraints:
        region: eu-west-1       # → prod-eu environment

  - name: worker
    type: Ryobi.Compute/containers
    placement:
      preferences:
        cost: minimize          # → cheapest environment
```

### Backward Compatibility

- If `metadata.environment` is set on the application, all resources go there (no placement)
- If `recipe` is set on a resource, that recipe is used directly
- Placement only activates when environment or recipe is omitted

### Constraints vs Preferences

**Constraints** (hard requirements) — environment must match all:

| Constraint | Description | Example |
|------------|-------------|---------|
| `region` | Geographic region | `eu-west-1`, `us-east-1` |
| `sovereignty` | Data sovereignty zone | `eu`, `us` |
| `capabilities` | Required features | `["gpu", "high-memory"]` |

**Preferences** (soft scoring) — used to rank matching environments:

| Preference | Value | Behavior |
|------------|-------|----------|
| `cost` | `minimize` | Prefer environments with lower `costPerHour` |
| `availableResources` | `maximize` | Prefer environments with more free CPU/memory |

When no preferences are specified, the engine scores by running resource count (prefer less busy environments).

### Environment Capabilities

#### Static Capabilities

Set in the environment agent config and sent during gRPC registration:

```yaml
capabilities:
  region: eu-west-1
  sovereignty: eu
  capabilities: [gpu, high-memory]
  costPerHour: 0.50
  maxReplicas: 50
```

Stored on the server in `EnvironmentServer.environments` map.

#### Dynamic Capabilities

Reported periodically (every 30s) via the `Heartbeat` gRPC RPC:

```protobuf
message HeartbeatRequest {
  string environment_name = 1;
  int64 available_cpu_millicores = 2;
  int64 available_memory_mb = 3;
  int32 running_resources = 4;
}
```

Dynamic data is used for `availableResources: maximize` preference scoring.

### Algorithm

```
Input: PlacementRequest{ResourceType, Constraints, Preferences}
       + list of all registered EnvironmentInfo

1. FILTER: Remove disconnected environments
2. FILTER: Remove environments that don't support the resource type
3. FILTER: Remove environments that don't match region constraint
4. FILTER: Remove environments that don't match sovereignty constraint
5. FILTER: Remove environments missing required capabilities
6. If no candidates remain → error

7. SCORE each candidate:
   - cost=minimize    → score = 1 / (1 + costPerHour)
   - resources=maximize → score = normalized(CPU + memory)
   - no preferences   → score = 1 - (running / maxReplicas)

8. SELECT highest scoring candidate
9. RESOLVE recipe name from candidate's RecipeTypes map

Output: PlacementResult{EnvironmentName, RecipeName}
```

### Integration Point

The placement engine is called in `ResourceDispatcher.Run()` (file `pkg/grpcapi/dispatcher.go`):

```go
if environmentName == "" || recipeName == "" {
    result, err := d.placementEngine.Place(request, d.server.GetEnvironments())
    environmentName = result.EnvironmentName
    recipeName = result.RecipeName
}
```

This happens after the resource is saved to the database but before the gRPC event is dispatched to the environment agent.

## Consequences

- Users can deploy without knowing infrastructure details
- Same YAML works across different infrastructure setups
- Cost optimization and capacity-aware scheduling are built in
- Data sovereignty can be enforced via constraints
- Trade-off: placement decisions are best-effort based on reported capabilities
- Trade-off: dynamic capabilities depend on heartbeat accuracy and freshness
- Trade-off: no re-placement of already-deployed resources (placement is at deploy time only)
