# Design: Terraform State Management

**Status:** Accepted
**Date:** 2026-07-02

## Context

Radius stores Terraform state in Kubernetes secrets using the `kubernetes` backend. Each recipe deployment creates a K8s secret in the `radius-system` namespace. Ryobi replaces this with PostgreSQL.

## Decision

### PostgreSQL Backend

Terraform natively supports a `pg` backend that stores state in a PostgreSQL database. Ryobi uses this as the default state backend.

```hcl
terraform {
  backend "pg" {
    conn_str    = "postgres://ryobi:pass@localhost:5432/ryobi?sslmode=disable"
    schema_name = "tfstate_a1b2c3d4e5f67890"
  }
}
```

### Schema Isolation

Each resource gets a unique, deterministic schema name:

```go
func generateSchemaName(resourceID string) string {
    hash := sha256.Sum256([]byte(resourceID))
    return fmt.Sprintf("tfstate_%x", hash[:8])
}
```

This ensures:
- **Isolation:** one resource's state cannot affect another's
- **Determinism:** redeploying the same resource uses the same schema
- **Uniqueness:** different resource IDs produce different schemas
- **Safety:** no risk of state collision

### Backend Interface

```go
type Backend interface {
    BuildBackend(resourceRecipe *recipes.ResourceMetadata) (backendType string, config map[string]any, error error)
}
```

Two implementations:

| Backend | Use Case | State Location |
|---------|----------|---------------|
| `PostgresBackend` | Production | PostgreSQL schema per resource |
| `LocalBackend` | Development | Local file per resource |

Selection is based on the `RYOBI_DB_URL` environment variable:
- Set → PostgreSQL backend
- Not set → local file backend

### Comparison with Radius

| Aspect | Radius | Ryobi |
|--------|--------|-------|
| Backend type | `kubernetes` | `pg` |
| State location | K8s secrets in `radius-system` | PostgreSQL schemas |
| Requires | Kubernetes cluster + RBAC | PostgreSQL instance |
| Cleanup | Delete K8s secret after destroy | Terraform handles schema cleanup |
| Validation | Check secret exists via K8s API | Not needed (PG backend handles it) |

### State Lifecycle

1. **Deploy:** Terraform creates the schema if it doesn't exist, stores state
2. **Update:** Terraform reads existing state from schema, applies diff, writes new state
3. **Destroy:** Terraform reads state, destroys resources, removes state from schema
4. **Schema cleanup:** Left to PostgreSQL admin (schemas are small, can be dropped periodically)

## Consequences

- No Kubernetes dependency for state storage
- Same PostgreSQL instance used for app data and TF state
- Native Terraform backend — no custom code for state read/write
- Trade-off: PostgreSQL must be highly available in production
- Trade-off: schemas accumulate over time (minor concern, can be cleaned up)
