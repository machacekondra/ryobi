# Kubernetes Pod Recipe

Deploys a containerized application on Kubernetes as a Deployment.

## Resource Type

`Ryobi.Compute/containers`

## Prerequisites

- Kubernetes cluster
- `kubeconfig` accessible to the Ryobi server

## What Gets Created

- Kubernetes Deployment with configurable replicas, ports, env vars, and resource limits

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `name` | string | (required) | Deployment name |
| `image` | string | (required) | Container image (e.g. `nginx:1.25`) |
| `namespace` | string | `default` | Target namespace |
| `replicas` | number | `1` | Pod replica count |
| `labels` | map | `{}` | Additional labels |
| `image_pull_policy` | string | `IfNotPresent` | `Always`, `IfNotPresent`, `Never` |
| `ports` | list | `[]` | Container ports (see below) |
| `env` | list | `[]` | Environment variables (see below) |
| `cpu_request` | string | `100m` | CPU request |
| `cpu_limit` | string | `""` | CPU limit (defaults to request) |
| `memory_request` | string | `128Mi` | Memory request |
| `memory_limit` | string | `""` | Memory limit (defaults to request) |

### Ports

```yaml
ports:
  - container_port: 8080
    protocol: TCP          # optional, default: TCP
```

### Environment Variables

```yaml
env:
  - name: DATABASE_URL
    value: "postgres://..."
  - name: LOG_LEVEL
    value: info
```

## Outputs

| Output | Description |
|--------|-------------|
| `deployment_name` | Name of the Kubernetes Deployment |
| `namespace` | Namespace of the Deployment |

## Example

```yaml
resources:
  - name: api
    type: Ryobi.Compute/containers
    recipe: kubernetes
    parameters:
      name: api
      image: myregistry.io/api:v1.2
      replicas: 3
      ports:
        - container_port: 8080
      cpu_request: "250m"
      memory_request: "256Mi"
      env:
        - name: LOG_LEVEL
          value: info
```
