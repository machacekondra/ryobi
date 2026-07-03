# Design: Async Operations

**Status:** Accepted
**Date:** 2026-07-03 (updated from 2026-07-02)

## Context

Terraform operations (apply/destroy) are long-running. The API must not block while waiting for Terraform to complete. This design describes how Ryobi handles asynchronous operations using a combination of an internal queue and gRPC dispatch to environment agents.

## Decision

### Sync vs Async Resources

| Resource Type | Operation | Mode |
|--------------|-----------|------|
| Applications | PUT/DELETE | Synchronous — returns 200/201/204 |
| Resources | PUT | Asynchronous — returns 202 Accepted |
| Resources | DELETE | Asynchronous — returns 202 Accepted |
| Credentials | PUT/DELETE | Synchronous — returns 200/201/204 |

Environments are managed by `ryobi-env` agents, not by direct API calls.

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
3. Queue: message with {operationId, resourceId, operationType}
     │
4. Async Worker → ResourceDispatcher:
     ├── Dequeue message
     ├── Update operation status → Provisioning
     ├── Look up application → environment name
     ├── Update resource status → Deploying
     ├── Dispatch ResourceEvent over gRPC watch stream
     └── Complete queue message
     │
5. Environment Agent (ryobi-env):
     ├── Receive ResourceEvent from watch stream
     ├── Resolve recipe from local config
     ├── Generate main.tf.json
     ├── Run terraform init + apply
     ├── Extract outputs from state
     └── Call ReportResult gRPC
     │
6. ryobid gRPC Server (ReportResult):
     ├── Update resource status → Succeeded/Failed + outputs
     └── Update operation status → Succeeded/Failed
     │
7. Client: polls resource status → sees Succeeded
```

### Components

**StatusManager** — creates operation records and enqueues work:
- `QueueAsyncOperation(operationID, resourceID, operationType, timeout)` → returns operation URL
- `Get(operationID)` → returns current status
- `Update(operationID, state, result)` → transitions state

**ResourceDispatcher** — async controller that dispatches events to environment agents:
- Replaces the old `DeployResource`/`DeleteResource` controllers that executed Terraform directly
- Resolves the application → environment name
- Sends a `ResourceEvent` over the gRPC watch stream to the correct environment agent
- If no agent is connected, marks the resource as failed immediately

**Worker** — polls the queue and calls the dispatcher:
- Runs as a `hosting.Service` alongside the API and gRPC servers
- 1-second poll interval
- Completes the queue message after dispatching (result comes back asynchronously via gRPC)

**EnvironmentServer** (gRPC) — manages connected environment agents:
- `Register` — creates environment in database, stores agent config
- `WatchResources` — server-streaming RPC, pushes `ResourceEvent` to agents
- `ReportResult` — receives operation results, updates resource + operation status
- `Unregister` — removes environment from database, closes watch channels

### Operation Status

```json
{
  "id": "op-uuid",
  "resourceId": "/api/v1/ryobi/applications/myapp/ryobi/resources/db",
  "status": "Succeeded",
  "startTime": "2026-07-03T10:00:00Z",
  "endTime": "2026-07-03T10:02:30Z",
  "error": null
}
```

States: `Accepted` → `Provisioning` → `Deploying` → `Succeeded` | `Failed`

### Resource Status

Resources now also track error messages:

```json
{
  "state": "Failed",
  "error": "terraform apply failed: exit status 1...",
  "outputs": {},
  "outputResources": []
}
```

### Typed Controllers

Frontend controllers use Go generics for type safety:

```go
type DefaultAsyncPut[T any] struct {
    opts ResourceOptions[T]
}

type ResourceOptions[T any] struct {
    RequestConverter  func(body []byte) (*T, error)
    ResponseConverter func(resource *T) (any, error)
    UpdateFilters     []UpdateFilter[T]
    DeleteFilters     []DeleteFilter[T]
}
```

## Consequences

- Long-running Terraform operations don't block the API
- Terraform executes on the environment agent, not the server
- Server has no Terraform or cloud provider dependencies
- Multiple environments can process operations concurrently
- Clients can poll for status or use `--wait` flag
- Trade-off: requires a running `ryobi-env` agent per environment
- Trade-off: if agent disconnects, operations fail until it reconnects
