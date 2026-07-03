# Design Decisions

This directory contains architectural design documents (ADRs) for the Ryobi project. Each document captures a significant decision, its context, rationale, and consequences.

## Index

| # | Title | Scope |
|---|-------|-------|
| [001](001-architecture-overview.md) | Architecture Overview | Three binaries (CLI, server, env agent), gRPC dispatch, flat REST API |
| [002](002-terraform-only-recipes.md) | Terraform-Only Recipes | All infrastructure via Terraform, no Bicep |
| [003](003-yaml-application-format.md) | YAML Application Format | YAML instead of Bicep for app definitions |
| [004](004-data-storage.md) | Data Storage | PostgreSQL for resources, queue, and TF state |
| [005](005-async-operations.md) | Async Operations | Queue + gRPC dispatch to environment agents |
| [006](006-platform-deployment.md) | Platform Deployment | Podman containers or systemd, no Helm |
| [007](007-api-framework.md) | API Framework | Adapted from Radius armrpc, simplified |
| [008](008-terraform-state-management.md) | Terraform State Management | PostgreSQL pg backend, schema isolation |
| [009](009-environment-agent.md) | Environment Agent | Separate binary, gRPC watch stream, auto-register/unregister |

## Format

Each document follows this structure:

- **Status:** Accepted / Proposed / Deprecated
- **Date:** When the decision was made
- **Context:** Why the decision was needed
- **Decision:** What was decided and how it works
- **Consequences:** Trade-offs and implications
