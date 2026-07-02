# Design: API Framework

**Status:** Accepted
**Date:** 2026-07-02

## Context

Radius uses a heavily ARM-RPC-based framework (`pkg/armrpc`) with Azure-specific patterns (resource IDs, API versions, planes, subscriptions, OpenAPI validation). Ryobi adapts the useful patterns while removing Azure-specific complexity.

## Decision

### What We Keep from Radius

**Controller interface:**
```go
type Controller interface {
    Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (Response, error)
}
```
Clean separation between HTTP handling and business logic.

**BaseController:**
Provides common utilities (`GetResource`, `SaveResource`, `DatabaseClient`, `StatusManager`).

**Typed resource operations with generics:**
```go
type ResourceOptions[T any] struct {
    RequestConverter  func(body []byte) (*T, error)
    ResponseConverter func(resource *T) (any, error)
    UpdateFilters     []UpdateFilter[T]
    DeleteFilters     []DeleteFilter[T]
}
```

**Filter pipeline:**
`UpdateFilter[T]` and `DeleteFilter[T]` run before mutations, enabling validation logic like "ensure referenced environment exists" or "prevent deletion of in-use environments."

**HandlerForController:**
Wraps a `Controller` in an `http.HandlerFunc`, handling response serialization and error conversion.

**Builder pattern:**
Declarative resource registration generates Chi routes.

### What We Remove

| Radius Pattern | Reason for Removal |
|---------------|-------------------|
| ARM resource ID parsing | Unnecessary outside Azure |
| `api-version` query parameter | Single version API |
| Planes and subscriptions | No multi-tenant routing |
| OpenAPI/Swagger validation middleware | Overkill for simple REST |
| `KubeClient` in controller Options | No Kubernetes |
| `PathBase` in controller Options | No ARM path prefixing |
| ARM error response format | Simple JSON errors |

### Simplified Controller Options

```go
type Options struct {
    Address        string
    DatabaseClient database.Client
    StatusManager  StatusManager
    ResourceType   string
}
```

All fields are required (validated at startup).

### Response Types

Simple JSON responses instead of ARM-formatted responses:

```go
func NewOKResponse(body any) Response
func NewCreatedResponse(body any) Response
func NewAcceptedResponse(operationURL string) Response
func NewNoContentResponse() Response
func NewNotFoundResponse(id string) Response
func NewBadRequestResponse(message string) Response
func NewConflictResponse(message string) Response
func NewInternalErrorResponse(err error) Response
```

Error responses use a consistent format:
```json
{
  "error": {
    "code": "NotFound",
    "message": "resource not found: /api/v1/ryobi/environments/prod"
  }
}
```

### Default Operations

Four generic typed operations handle common CRUD patterns:

| Operation | Mode | Behavior |
|-----------|------|----------|
| `DefaultSyncPut[T]` | Sync | Validate → filter → save → 200/201 |
| `DefaultSyncDelete[T]` | Sync | Get → filter → delete → 204 |
| `DefaultAsyncPut[T]` | Async | Validate → filter → save → queue → 202 |
| `DefaultAsyncDelete[T]` | Async | Get → filter → queue → 202 |

Plus two generic untyped operations for simple resources:
- `GenericGet` — GET by ID
- `GenericList` — LIST by scope and type

### Router

Chi router with standard middleware:
- Request ID
- Real IP
- Request logging
- Panic recovery

Route registration via `gateway.Router.RegisterResourceRoutes()`:
```go
router.RegisterResourceRoutes("/api/v1/environments", "ryobi/environments", ResourceFactories{
    List:   NewGenericListFactory(rootScope),
    Get:    NewGenericGetFactory(rootScope),
    Put:    NewDefaultSyncPutFactory(rootScope, envOpts),
    Delete: NewDefaultSyncDeleteFactory(rootScope, envOpts),
})
```

## Consequences

- Clean, testable controller pattern
- Type-safe request/response handling via generics
- Easy to add new resource types
- Trade-off: no OpenAPI spec validation (manual validation via filters)
- Trade-off: no ARM compatibility
