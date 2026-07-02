# Design: Platform Deployment

**Status:** Accepted
**Date:** 2026-07-02

## Context

Radius is deployed as Kubernetes pods via a Helm chart. Ryobi takes a simpler approach, running as Podman containers or native binaries.

## Decision

### Podman Compose (Primary)

The recommended deployment uses `podman-compose` with two containers:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: ryobi
      POSTGRES_PASSWORD: ${RYOBI_DB_PASSWORD}
      POSTGRES_DB: ryobi
    volumes:
      - ryobi-pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ryobi"]

  ryobid:
    build:
      context: .
      dockerfile: deploy/Containerfile
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      RYOBI_DB_URL: postgres://ryobi:${RYOBI_DB_PASSWORD}@postgres:5432/ryobi
    volumes:
      - ryobi-data:/var/lib/ryobi
    ports:
      - "9000:9000"
```

### Container Image

The `ryobid` container image uses a multi-stage build:

1. **Builder stage** (golang:1.24-alpine): compiles the Go binary with CGO disabled
2. **Runtime stage** (alpine:3.21): minimal image with `ca-certificates` and `terraform`

The runtime image:
- Runs as non-root user (UID 10001)
- Includes Terraform CLI for recipe execution
- Exposes port 9000

### Systemd (Alternative)

For bare-metal deployments:

```ini
[Service]
Type=simple
ExecStart=/usr/local/bin/ryobid
EnvironmentFile=/etc/ryobi/ryobid.env
Restart=always
```

Requires a separately managed PostgreSQL instance.

### Why Not Kubernetes?

| Concern | Kubernetes | Podman/Systemd |
|---------|-----------|----------------|
| Setup complexity | High (cluster, Helm, RBAC) | Low (single command) |
| Resource overhead | Significant (control plane) | Minimal |
| Target audience | Teams already on K8s | Any team |
| HA/Scaling | Built-in | Manual (out of scope for v1) |

Ryobi targets simplicity. Teams that need Kubernetes can still use Ryobi to provision K8s-hosted resources via Terraform.

### Configuration

All server configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `RYOBI_ADDRESS` | `0.0.0.0:9000` | Listen address |
| `RYOBI_DB_URL` | — | PostgreSQL URL |
| `RYOBI_TF_ROOT_DIR` | `/var/lib/ryobi/terraform` | Terraform working directory |
| `TERRAFORM_PATH` | `terraform` | Path to terraform binary |

No YAML config file is needed for the server. This keeps containerized deployment simple.

## Consequences

- One-command setup with `podman-compose up`
- No cluster infrastructure required
- Terraform CLI must be available in the container/host
- Trade-off: no built-in HA (acceptable for v1)
- Trade-off: no auto-scaling of async workers
