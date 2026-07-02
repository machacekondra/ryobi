# Kubernetes Pod Recipe

Deploys a containerized application on Kubernetes as a Deployment with optional Service, Ingress, ConfigMap, Secret, and PersistentVolumeClaims.

## Resource Type

`Ryobi.Compute/containers`

## Prerequisites

- Kubernetes cluster
- `kubeconfig` accessible to the Ryobi server

## What Gets Created

| Resource | Condition |
|----------|-----------|
| Namespace | `create_namespace = true` |
| Deployment | Always |
| Service | `ports` is non-empty |
| Ingress | `ingress_host` is set |
| ConfigMap | `config_data` is non-empty |
| Secret | `secret_data` is non-empty |
| PVCs | `volumes` with `size` set |

## Parameters

### Core

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `name` | string | (required) | Deployment name |
| `image` | string | (required) | Container image |
| `namespace` | string | `default` | Target namespace |
| `replicas` | number | `1` | Pod replica count |
| `create_namespace` | bool | `false` | Create namespace if missing |
| `image_pull_policy` | string | `IfNotPresent` | Pull policy |
| `image_pull_secret` | string | `""` | Image pull secret name |
| `service_account_name` | string | `""` | Service account |

### Ports

```yaml
ports:
  - container_port: 8080
    service_port: 80       # optional, defaults to container_port
    protocol: TCP          # optional
    name: http             # optional
```

### Environment

```yaml
env:
  - name: DATABASE_URL
    value: "postgres://..."
  - name: API_KEY
    secret_key_ref:
      name: my-secret
      key: api-key
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `config_data` | map | `{}` | Key-value pairs injected as ConfigMap env vars |
| `secret_data` | map | `{}` | Key-value pairs injected as Secret env vars |

### Resources

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `cpu_request` | string | `100m` | CPU request |
| `cpu_limit` | string | `""` | CPU limit (defaults to request) |
| `memory_request` | string | `128Mi` | Memory request |
| `memory_limit` | string | `""` | Memory limit (defaults to request) |

### Volumes

```yaml
volumes:
  - name: data
    mount_path: /app/data
    size: 5Gi                # creates a PVC; empty = emptyDir
    storage_class: fast-ssd  # optional
    read_only: false         # optional
```

### Health Checks

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `health_check_path` | string | `""` | HTTP path (empty disables probes) |
| `health_check_port` | number | `8080` | Probe port |

### Service

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `service_type` | string | `ClusterIP` | `ClusterIP`, `NodePort`, `LoadBalancer` |

### Ingress

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `ingress_host` | string | `""` | Hostname (empty skips Ingress) |
| `ingress_path` | string | `/` | URL path |
| `ingress_class` | string | `nginx` | Ingress class |
| `ingress_tls_secret` | string | `""` | TLS secret name |

## Outputs

| Output | Description |
|--------|-------------|
| `deployment_name` | Deployment name |
| `namespace` | Resource namespace |
| `service_name` | Service name (if created) |
| `service_cluster_ip` | ClusterIP (if created) |
| `ingress_host` | Ingress hostname (if created) |
| `endpoint` | In-cluster endpoint (`svc.cluster.local`) |

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
      health_check_path: /healthz
      ingress_host: api.example.com
```
