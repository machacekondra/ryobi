# Design: Async Operations

**Status:** Accepted
**Date:** 2026-07-02

## Context

Terraform operations (apply/destroy) are long-running. The API must not block while waiting for Terraform to complete. This design describes how Ryobi handles asynchronous operations, adapted from the Radius ARM-RPC async pattern.

## Decision

### Sync vs Async Resources

| Resource Type | Operation | Mode |
|--------------|-----------|------|
| Environments | PUT/DELETE | Synchronous — returns 200/201/204 |
| Applications | PUT/DELETE | Synchronous — returns 200/201/204 |
| Resources | PUT | Asynchronous — returns 202 Accepted |
| Resources | DELETE | Asynchronous — returns 202 Accepted |
| Credentials | PUT/DELETE | Synchronous — returns 200/201/204 |

### Async Flow

```
1. Client: PUT /api/v1/applications/myapp/resources/db
     │
2. Frontend Controller (DefaultAsyncPut):
     ├── Validate request body (RequestConverter)
     ├── Run UpdateFilters
     ├── Save resource to database
     ├── Queue async operation (StatusManager)
     └── Return 202 Accepted + Location header
     │
3. Queue: PostgreSQL message with {operationId, resourceId, operationType}
     │
4. Async Worker (polling loop):
     ├── Dequeue message
     ├── Update operation status → Provisioning
     ├── Look up AsyncController from registry
     ├── Call controller.Run()
     │     ├── Resolve environment → recipe definition
     │     ├── Generate main.tf.json
     │     ├── Run terraform init + apply
     │     ├── Extract outputs from state
     │     └── Update resource status in database
     ├── Update operation status → Succeeded/Failed
     └── Complete (remove message from queue)
     │
5. Client: GET /api/v1/operations/{operationId}
     └── Returns current status (Accepted → Provisioning → Succeeded/Failed)
```

### Components

**StatusManager** — creates operation records and enqueues work:
- `QueueAsyncOperation(operationID, resourceID, operationType, timeout)` → returns operation URL
- `Get(operationID)` → returns current status
- `Update(operationID, state, result)` → transitions state
- `Delete(operationID)` → cleanup

**ControllerRegistry** — maps `(resourceType, method)` to async controllers:
- `Register(resourceType, method, controller)`
- `Get(resourceType, method)` → returns controller

**Worker** — polls the queue and dispatches to controllers:
- Runs as a `hosting.Service` alongside the API server
- 1-second poll interval
- Updates operation status on success/failure
- Completes the queue message after processing

### Operation Status

```json
{
  "id": "op-uuid",
  "resourceId": "/api/v1/ryobi/applications/myapp/ryobi/resources/db",
  "status": "Succeeded",
  "startTime": "2026-07-02T10:00:00Z",
  "endTime": "2026-07-02T10:02:30Z",
  "error": null
}
```

States: `Accepted` → `Provisioning` → `Succeeded` | `Failed` | `Canceled`

### Typed Controllers

Frontend controllers use Go generics for type safety:

```go
type DefaultAsyncPut[T any] struct {
    opts ResourceOptions[T]  // converters, filters, timeouts
}

type ResourceOptions[T any] struct {
    RequestConverter  func(body []byte) (*T, error)
    ResponseConverter func(resource *T) (any, error)
    UpdateFilters     []UpdateFilter[T]
    DeleteFilters     []DeleteFilter[T]
}
```

This ensures request bodies are validated and typed before being saved or queued.

## Consequences

- Long-running Terraform operations don't block the API
- Clients can poll for status or use `--wait` flag
- Operation history is persisted in the database
- Trade-off: more complex than synchronous-only API
- Trade-off: single worker means sequential processing (can be extended to concurrent workers)
