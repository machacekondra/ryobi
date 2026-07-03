# Design: Environment Agent

**Status:** Accepted
**Date:** 2026-07-03

## Context

In the initial architecture, `ryobid` executed Terraform directly via an in-process async worker. This required the server to have Terraform installed, access to cloud credentials, and visibility into recipe module sources. This created operational concerns:

- Server needs all cloud credentials for all environments
- Server needs Terraform and all provider plugins
- No isolation between environments
- Single point of failure for recipe execution

## Decision

### Separate Environment Agent Binary

The `ryobi-env` binary is a generic environment agent that:

1. Reads a `config.yaml` defining the environment name, providers, credentials, and recipes
2. Connects to `ryobid` via gRPC
3. Registers the environment (auto-creates it in the database)
4. Opens a server-streaming watch for resource events
5. Executes Terraform locally when events arrive
6. Reports results back via gRPC
7. Unregisters the environment on shutdown (SIGINT/SIGTERM)

### Configuration Format

```yaml
name: dev

server:
  grpcAddress: localhost:9001

providers:
  kubernetes:
    scope: default

terraformProviders:
  kubernetes:
    config_path: "~/.kube/config"

recipes:
  - resourceType: Ryobi.Compute/containers
    recipeName: kubernetes
    templatePath: ../recipes/kubernetes-pod

terraform:
  binaryPath: terraform
  workDir: ~/.ryobi/terraform
  stateBackend: local
```

Key config properties:
- `name` — environment name, must be unique across connected agents
- `server.grpcAddress` — ryobid gRPC endpoint
- `providers` — cloud provider scopes (passed to ryobid for storage)
- `terraformProviders` — Terraform provider blocks injected into `main.tf.json`
- `recipes` — maps resource types to local Terraform modules
- `terraform` — execution settings (binary path, working directory, state backend)

Template paths are resolved relative to the config file's directory and converted to absolute paths.

### gRPC Protocol

Defined in `proto/environment.proto`:

```protobuf
service EnvironmentService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Unregister(UnregisterRequest) returns (UnregisterResponse);
  rpc WatchResources(WatchRequest) returns (stream ResourceEvent);
  rpc ReportResult(ReportResultRequest) returns (ReportResultResponse);
}
```

**Register** — called once on startup. Creates the environment in ryobid's database with providers, recipes, and terraform config.

**WatchResources** — server-streaming RPC. The agent opens this stream and blocks. When a resource operation targets this environment, `ryobid` pushes a `ResourceEvent` containing the operation ID, resource details, recipe name, and parameters.

**ReportResult** — called after Terraform execution completes. Reports success/failure and outputs. `ryobid` updates the resource and operation status.

**Unregister** — called on shutdown. Closes all watch channels and deletes the environment from the database.

### Lifecycle

```
Start:
  ryobi-env config.yaml
    → gRPC Register(name, providers, recipes)
    → gRPC WatchResources(name) — blocks, receives events
    → For each event:
        → Resolve recipe from local config
        → Generate main.tf.json (module + backend + providers)
        → terraform init + apply/destroy
        → gRPC ReportResult(operationId, success, outputs)

Shutdown (SIGINT/SIGTERM):
    → defer: gRPC Unregister(name)
    → Closes gRPC connection
    → Environment removed from ryobid database
```

### Server-Side Dispatch

When a resource operation arrives at `ryobid`:

1. `DefaultAsyncPut` saves the resource and queues an async operation
2. The async worker dequeues and calls `ResourceDispatcher`
3. The dispatcher resolves: resource → application → environment name
4. It finds the gRPC watch channel for that environment
5. It sends a `ResourceEvent` on the channel
6. If no agent is connected, the resource is marked as failed immediately

### Terraform Execution on the Agent

The agent executes Terraform identically to the old in-process executor:

1. Creates a working directory under `terraform.workDir`
2. Generates `main.tf.json` with module source, backend config, and provider blocks
3. Runs `terraform init` then `terraform apply` (or `destroy`)
4. Reads state via `terraform show` to extract outputs
5. Reports results back via `ReportResult`

State backend is configurable: local files (default) or PostgreSQL `pg` backend.

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|-----------------|
| Server executes Terraform directly | Server needs all credentials, no environment isolation |
| Polling-based (agent polls server for work) | Higher latency, wasteful polling, no push notification |
| WebSocket instead of gRPC | gRPC provides typed contracts, streaming, and code generation |
| Agent as HTTP server (server pushes to agent) | Requires agent to be network-reachable from server, NAT issues |

## Consequences

- Server is lightweight — no Terraform, no cloud SDKs
- Each environment runs its own credentials and recipes locally
- Multiple environments can connect to a single server simultaneously
- Clean lifecycle: register on start, unregister on stop
- gRPC streaming provides immediate event delivery (no polling)
- Trade-off: requires a running agent per environment
- Trade-off: agent must maintain a persistent gRPC connection
- Trade-off: if the agent crashes without clean shutdown, the environment record persists until manually cleaned up
