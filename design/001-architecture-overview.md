# Design: Architecture Overview

**Status:** Accepted
**Date:** 2026-07-02

## Context

Ryobi is modeled after the [Radius](https://github.com/radius-project/radius) project but with significant architectural simplifications. This document captures the high-level architecture decisions and how they differ from Radius.

## Decision

### Single Server Binary

Radius runs 5+ binaries (ucpd, applications-rp, dynamic-rp, controller, rad). Ryobi consolidates into **two binaries**:

| Binary | Replaces | Responsibility |
|--------|----------|---------------|
| `ryobi` | `rad` | CLI — parses YAML, calls API, displays status |
| `ryobid` | `ucpd` + `applications-rp` + `dynamic-rp` + `controller` | API server + async worker |

**Rationale:** A single server binary reduces operational complexity. The async worker runs in-process using goroutines and a shared database queue, eliminating the need for inter-service communication.

### No Kubernetes Dependency

Radius is deeply integrated with Kubernetes (CRDs, controller-runtime, Helm charts, K8s secrets for TF state). Ryobi removes all Kubernetes dependencies:

- No CRDs or reconcilers
- No controller-runtime or client-go
- No Helm chart
- No K8s secrets for state storage
- Platform runs as Podman containers or native binaries via systemd

**Rationale:** Many teams deploy infrastructure tooling outside of Kubernetes. Removing this dependency makes Ryobi usable in any environment with Podman or a Linux server.

### Flat REST API

Radius uses ARM-RPC style APIs with complex resource IDs (`/subscriptions/{sub}/resourceGroups/{rg}/providers/Applications.Core/...`), planes, and API version query parameters. Ryobi uses flat REST:

```
/api/v1/environments/{name}
/api/v1/applications/{name}
/api/v1/applications/{app}/resources/{name}
/api/v1/operations/{id}
```

**Rationale:** ARM compatibility is unnecessary outside Azure. Flat URLs are easier to understand, debug, and integrate with.

## Component Interaction

```
ryobi CLI ──[HTTP]──▶ ryobid
                        ├── Chi Router
                        │     ├── Sync controllers (environments, applications)
                        │     └── Async controllers (resources → 202 Accepted)
                        ├── StatusManager → PostgreSQL queue
                        └── Async Worker (goroutine)
                              └── Terraform executor
                                    ├── Config generation (main.tf.json)
                                    ├── terraform init/apply/destroy
                                    └── State stored in PostgreSQL
```

## Consequences

- Simpler deployment: one server process + PostgreSQL
- No Kubernetes cluster required
- Easier to develop and test locally
- Trade-off: no built-in HA or horizontal scaling (single server)
- Trade-off: no CRD-based GitOps workflow
