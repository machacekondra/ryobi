# KubeVirt VM Recipe

Deploys a virtual machine on a Kubernetes cluster using [KubeVirt](https://kubevirt.io).

## Resource Type

`Ryobi.Compute/virtualMachines`

## Prerequisites

- Kubernetes cluster with KubeVirt installed
- CDI (Containerized Data Importer) for disk image support
- `kubeconfig` accessible to the Ryobi server

## Parameters

### Core

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `name` | string | (required) | VM name |
| `namespace` | string | `default` | Target namespace |
| `create_namespace` | bool | `false` | Create namespace if missing |
| `running` | bool | `true` | Start the VM immediately |
| `labels` | map | `{}` | Additional labels |

### CPU & Memory

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `cpu_cores` | number | `1` | CPU cores |
| `cpu_sockets` | number | `1` | CPU sockets |
| `cpu_threads` | number | `1` | Threads per core |
| `memory` | string | `1Gi` | Memory request |
| `memory_limit` | string | `""` | Memory limit |

### Disk

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `disk_image` | string | `""` | Container registry URL for boot disk |
| `disk_size` | string | `10Gi` | Boot disk size |
| `disk_bus` | string | `virtio` | Disk bus type |
| `storage_class` | string | `""` | Storage class for PVC |

### Network

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `network_type` | string | `masquerade` | `masquerade` or `bridge` |

### Cloud-Init

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `cloud_init_enabled` | bool | `false` | Enable cloud-init |
| `cloud_init_user_data` | string | `""` | Cloud-init YAML |

### Service

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `service_ports` | list | `[]` | Ports to expose |
| `service_type` | string | `ClusterIP` | Service type |

## Outputs

| Output | Description |
|--------|-------------|
| `vm_name` | Name of the VirtualMachine |
| `vm_namespace` | Namespace of the VirtualMachine |
| `service_name` | Name of the Service (if created) |
| `service_cluster_ip` | ClusterIP of the Service (if created) |

## Example

```yaml
resources:
  - name: dev-vm
    type: Ryobi.Compute/virtualMachines
    recipe: kubevirt
    parameters:
      name: dev-vm
      cpu_cores: 2
      memory: 4Gi
      disk_image: "docker://quay.io/containerdisks/fedora:latest"
      disk_size: 30Gi
      cloud_init_enabled: true
      cloud_init_user_data: |
        #cloud-config
        hostname: dev-vm
        packages: [vim, git]
      service_ports:
        - name: ssh
          port: 22
          target_port: 22
```
