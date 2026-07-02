# Design: Data Storage

**Status:** Accepted
**Date:** 2026-07-02

## Context

Radius uses PostgreSQL for resource storage and queuing, and Kubernetes secrets for Terraform state. Ryobi consolidates all storage into PostgreSQL.

## Decision

### PostgreSQL for Everything

A single PostgreSQL instance handles three concerns:

| Concern | Implementation |
|---------|---------------|
| Resource storage | JSONB documents with ETag-based optimistic concurrency |
| Async operation queue | PostgreSQL table with `SKIP LOCKED` dequeue pattern |
| Terraform state | Native Terraform `pg` backend with per-resource schema isolation |

### Resource Storage

Resources are stored as JSONB documents in a single table:

```
id            TEXT PRIMARY KEY   -- e.g. /api/v1/ryobi/environments/prod
resource_type TEXT               -- e.g. ryobi/environments
root_scope    TEXT               -- e.g. /api/v1
body          JSONB              -- full resource document
etag          TEXT               -- for optimistic concurrency control
created_at    TIMESTAMP
updated_at    TIMESTAMP
```

The `database.Client` interface provides:
- `Get(id)` — retrieve by ID
- `Query(rootScope, resourceType)` — list with filtering
- `Save(object, etag?)` — create or update with optional ETag check
- `Delete(id, etag?)` — delete with optional ETag check

An in-memory implementation is used for development and testing.

### Operation Queue

Async operations (resource deploy/delete) are queued using a PostgreSQL-backed queue:

- `Enqueue` — insert message with visibility timeout
- `Dequeue` — select one message using `FOR UPDATE SKIP LOCKED`
- `Complete` — delete the message after processing
- `Extend` — extend the visibility timeout during long operations

An in-memory implementation is used for development and testing.

### Terraform State Backend

Terraform state is stored using the native `pg` backend:

```hcl
terraform {
  backend "pg" {
    conn_str    = "postgres://ryobi:pass@localhost/ryobi"
    schema_name = "tfstate_a1b2c3d4e5f6"
  }
}
```

Each resource gets a unique schema name derived from a SHA-256 hash of its resource ID. This provides:
- **Isolation** — each resource's state is independent
- **Determinism** — same resource ID always maps to same schema
- **No cleanup needed** — PostgreSQL schemas persist across restarts

A local file backend is available for development when `RYOBI_DB_URL` is not set.

### Concurrency Control

Optimistic concurrency is enforced via ETags:
- Each `Save` generates a new ETag (UUID)
- Passing an ETag to `Save` or `Delete` performs a compare-and-swap
- Stale ETag returns `ErrConcurrency`
- Missing resource returns `ErrNotFound`

## Consequences

- Single database dependency simplifies operations
- No separate message broker needed
- Terraform state lives alongside application state
- In-memory implementations enable fast unit tests
- Trade-off: PostgreSQL becomes a single point of failure
- Trade-off: queue throughput limited by PostgreSQL (sufficient for this use case)
