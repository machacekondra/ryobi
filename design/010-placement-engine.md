# Design: Placement Engine

**Status:** Accepted
**Date:** 2026-07-03 (updated)

## Context

Deploying a resource requires knowing which environment and recipe to use. Rather than forcing application developers to make this decision, placement is managed by administrators who define rules as API objects. The placement engine evaluates these rules at deploy time.

## Decision

### Administrator-Defined Placement Rules

Placement rules are **API objects** created by administrators, not inline YAML on resources. This separates the concern:

- **Developers** define what they need: resource type + parameters
- **Administrators** define where things run: placement rules with constraints and preferences

```bash
# Admin creates rules via CLI
ryobi placement create eu-containers \
  --resource-type Ryobi.Compute/containers \
  --region eu-west-1 --sovereignty eu --priority 10

ryobi placement create cost-optimized \
  --resource-type Ryobi.Compute/containers \
  --cost minimize --priority 1
```

### PlacementRule API Object

Stored at `/api/v1/placements/{name}`:

```json
{
  "name": "eu-containers",
  "properties": {
    "resourceType": "Ryobi.Compute/containers",
    "constraints": {
      "region": "eu-west-1",
      "sovereignty": "eu"
    },
    "preferences": {
      "cost": "minimize"
    },
    "priority": 10
  }
}
```

### Per-Resource Placement

Placement decisions are made **per resource**. Different resources in the same application can land on different environments based on which rules match their resource type.

### Application YAML Stays Simple

Developers don't specify placement — just resource type and parameters:

```yaml
apiVersion: ryobi/v1
kind: Application
metadata:
  name: my-app
resources:
  - name: api
    type: Ryobi.Compute/containers
    parameters:
      name: api
      image: myapp:latest
```

### Backward Compatibility

- If `metadata.environment` is set on the application, all resources go there (no placement)
- If `recipe` is set on a resource, that recipe is used directly
- Placement only activates when environment or recipe is omitted

### Rule Matching and Priority

When multiple rules match the same resource type, they are evaluated in **priority order** (highest first). The first rule that finds a matching environment wins. This allows layered policies:

| Rule | Resource Type | Constraints | Preferences | Priority |
|------|--------------|-------------|-------------|----------|
| `eu-sovereign` | `Ryobi.Compute/containers` | region=eu-west-1, sovereignty=eu | | 10 |
| `cost-optimized` | `Ryobi.Compute/containers` | | cost=minimize | 1 |

With these rules: containers go to EU if an EU environment is connected. Otherwise, fall back to cheapest.

### Constraints vs Preferences

**Constraints** (hard requirements) — environment must match all:

| Constraint | Description |
|------------|-------------|
| `region` | Geographic region (e.g. `eu-west-1`) |
| `sovereignty` | Data sovereignty zone (e.g. `eu`) |
| `capabilities` | Required features (e.g. `["gpu"]`) |

**Preferences** (soft scoring) — used to rank matching environments:

| Preference | Value | Behavior |
|------------|-------|----------|
| `cost` | `minimize` | Prefer lower `costPerHour` |
| `availableResources` | `maximize` | Prefer more free CPU/memory |

### Algorithm

```
Input: PlacementRequest{ResourceType}
       + PlacementRules (from database, sorted by priority desc)
       + EnvironmentInfo (from connected agents)

1. Find all rules matching the resource type
2. Sort by priority (highest first)
3. For each rule:
   a. FILTER environments by rule's constraints
   b. FILTER by resource type support
   c. FILTER disconnected environments
   d. If candidates remain:
      - SCORE by rule's preferences
      - SELECT highest scoring
      - RESOLVE recipe from environment
      - RETURN result
4. If no rules matched or all rules failed:
   - FALLBACK: pick any connected environment supporting the type

Output: PlacementResult{EnvironmentName, RecipeName, RuleName}
```

### Integration Point

The placement engine is called in `ResourceDispatcher.Run()` (`pkg/grpcapi/dispatcher.go`):

```go
if environmentName == "" || recipeName == "" {
    rules := d.loadPlacementRules(ctx)  // from database
    envs := d.server.GetEnvironments()
    result, err := d.placementEngine.Place(request, rules, envs)
}
```

### CLI Commands

```bash
ryobi placement create <name> [flags]   # Create a rule
ryobi placement list                     # List all rules
ryobi placement show <name>              # Show rule details
ryobi placement delete <name>            # Delete a rule
```

## Consequences

- Clean separation: developers define apps, admins define placement policy
- Rules are API objects — can be managed, versioned, audited
- Priority-based evaluation allows layered policies (strict → fallback)
- No placement config in application YAML — apps are portable
- Trade-off: requires admin setup before placement works
- Trade-off: no re-placement of already-deployed resources
