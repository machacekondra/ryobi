# Design: Architecture Overview

**Status:** Accepted
**Date:** 2026-07-03 (updated from 2026-07-02)

## Context

Ryobi is modeled after the [Radius](https://github.com/radius-project/radius) project but with significant architectural simplifications. This document captures the high-level architecture decisions and how they differ from Radius.

## Decision

### Three Binaries

Radius runs 5+ binaries (ucpd, applications-rp, dynamic-rp, controller, rad). Ryobi uses three:

| Binary | Role | Communication |
|--------|------|---------------|
| `ryobi` | CLI — parses YAML, calls API, displays status | HTTP → ryobid |
| `ryobid` | API server + gRPC event dispatcher | HTTP (port 9000) + gRPC (port 9001) |
| `ryobi-env` | Environment agent — registers environment, watches for resources, executes Terraform | gRPC → ryobid |

**Rationale:** Separating the environment agent from the server allows:
- Multiple environments to connect to a single server
- Each environment runs its own credentials and Terraform locally
- Environments can be started/stopped independently
- No Terraform or cloud credentials needed on the server

### Environment Agent Model

Environments are not static configuration — they are **live agents** that connect to `ryobid`:

1. `ryobi-env` starts with a `config.yaml` specifying the environment name, providers, credentials, and recipes
2. It connects to `ryobid` via gRPC and registers the environment (creates it in the database)
3. It opens a watch stream and waits for resource events
4. When a resource is deployed, `ryobid` dispatches the event over the gRPC stream
5. `ryobi-env` executes `terraform init/apply` locally and reports the result back
6. On shutdown (SIGINT/SIGTERM), it unregisters the environment and removes it from the database

### No Kubernetes Dependency

Radius is deeply integrated with Kubernetes (CRDs, controller-runtime, Helm charts, K8s secrets for TF state). Ryobi removes all Kubernetes dependencies:

- No CRDs or reconcilers
- No controller-runtime or client-go
- No Helm chart
- No K8s secrets for state storage
- Platform runs as Podman containers or native binaries via systemd

**Rationale:** Many teams deploy infrastructure tooling outside of Kubernetes. Removing this dependency makes Ryobi usable in any environment with Podman or a Linux server.

### Flat REST API

Radius uses ARM-RPC style APIs with complex resource IDs. Ryobi uses flat REST:

```
/api/v1/environments/{name}
/api/v1/applications/{name}
/api/v1/applications/{app}/resources/{name}
/api/v1/operations/{id}
```

**Rationale:** ARM compatibility is unnecessary outside Azure. Flat URLs are easier to understand, debug, and integrate with.

## Component Interaction

```
ryobi CLI ──[HTTP]──▶ ryobid (port 9000)
                        ├── Chi Router (HTTP API)
                        │     ├── Sync controllers (applications)
                        │     └── Async controllers (resources → 202 Accepted)
                        ├── StatusManager → Queue
                        ├── Async Worker → ResourceDispatcher
                        └── gRPC Server (port 9001)
                              ├── Register/Unregister environments
                              ├── WatchResources stream
                              └── ReportResult
                                    ↕ gRPC stream
                              ryobi-env (environment agent)
                                ├── Registers environment on startup
                                ├── Watches for resource events
                                ├── Executes terraform init/apply/destroy
                                ├── Reports results back via gRPC
                                └── Unregisters on shutdown
```

### Request Flow: Deploy a Resource

```
1. ryobi deploy app.yaml
     │  HTTP PUT /api/v1/applications/myapp/resources/nginx
2. ryobid
     │  Saves resource to database
     │  Queues async operation → 202 Accepted
     │  Worker dequeues → ResourceDispatcher
     │  Finds application → environment name
     │  Sends ResourceEvent over gRPC watch stream
3. ryobi-env
     │  Receives ResourceEvent
     │  Resolves recipe from local config
     │  Generates main.tf.json
     │  Runs terraform init + terraform apply
     │  Extracts outputs from state
     │  Calls ReportResult gRPC (success + outputs)
4. ryobid
     │  Updates resource status (Succeeded + outputs)
     │  Updates operation status (Succeeded)
5. ryobi CLI
     │  Polls resource status → sees Succeeded
     │  Prints success
```

## Consequences

- Server has no Terraform or cloud provider dependencies
- Multiple environments can connect to a single server
- Environment agents run close to the infrastructure (same network, same credentials)
- Graceful lifecycle: register on start, unregister on stop
- Trade-off: requires a running `ryobi-env` agent per environment
- Trade-off: if the agent disconnects, resource operations will fail until it reconnects
