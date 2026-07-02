terraform {
  required_providers {
    kubevirt = {
      source  = "kubevirt/kubevirt"
      version = ">= 0.2.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">= 2.20.0"
    }
  }
}

# --- Namespace (optional) ---

resource "kubernetes_namespace" "vm" {
  count = var.create_namespace ? 1 : 0

  metadata {
    name = var.namespace
    labels = {
      "managed-by" = "ryobi"
    }
  }
}

# --- Data Volume (optional boot disk from container image) ---

resource "kubernetes_manifest" "data_volume" {
  count = var.disk_image != "" ? 1 : 0

  manifest = {
    apiVersion = "cdi.kubevirt.io/v1beta1"
    kind       = "DataVolume"
    metadata = {
      name      = "${var.name}-boot"
      namespace = var.namespace
    }
    spec = {
      source = {
        registry = {
          url = var.disk_image
        }
      }
      pvc = {
        accessModes = [var.disk_access_mode]
        resources = {
          requests = {
            storage = var.disk_size
          }
        }
        storageClassName = var.storage_class
      }
    }
  }

  depends_on = [kubernetes_namespace.vm]
}

# --- VirtualMachine ---

resource "kubernetes_manifest" "vm" {
  manifest = {
    apiVersion = "kubevirt.io/v1"
    kind       = "VirtualMachine"
    metadata = {
      name      = var.name
      namespace = var.namespace
      labels    = merge(var.labels, {
        "managed-by"   = "ryobi"
        "ryobi/recipe" = "kubevirt-vm"
      })
    }
    spec = {
      running = var.running

      template = {
        metadata = {
          labels = merge(var.labels, {
            "kubevirt.io/vm" = var.name
          })
        }
        spec = {
          domain = {
            cpu = {
              cores   = var.cpu_cores
              sockets = var.cpu_sockets
              threads = var.cpu_threads
            }
            resources = {
              requests = {
                memory = var.memory
              }
              limits = var.memory_limit != "" ? {
                memory = var.memory_limit
              } : {}
            }
            devices = {
              disks = concat(
                var.disk_image != "" ? [{
                  name = "boot"
                  disk = {
                    bus = var.disk_bus
                  }
                }] : [],
                var.cloud_init_enabled ? [{
                  name = "cloudinit"
                  disk = {
                    bus = "virtio"
                  }
                }] : []
              )
              interfaces = [{
                name                 = "default"
                masquerade           = var.network_type == "masquerade" ? {} : null
                bridge               = var.network_type == "bridge" ? {} : null
              }]
            }
          }

          networks = [{
            name = "default"
            pod  = {}
          }]

          volumes = concat(
            var.disk_image != "" ? [{
              name = "boot"
              dataVolume = {
                name = "${var.name}-boot"
              }
            }] : [],
            var.cloud_init_enabled ? [{
              name = "cloudinit"
              cloudInitNoCloud = {
                userData = var.cloud_init_user_data
              }
            }] : []
          )
        }
      }
    }
  }

  depends_on = [
    kubernetes_namespace.vm,
    kubernetes_manifest.data_volume,
  ]
}

# --- Service (optional, exposes VM via NodePort or ClusterIP) ---

resource "kubernetes_service" "vm" {
  count = length(var.service_ports) > 0 ? 1 : 0

  metadata {
    name      = var.name
    namespace = var.namespace
    labels = {
      "managed-by" = "ryobi"
    }
  }

  spec {
    selector = {
      "kubevirt.io/vm" = var.name
    }

    type = var.service_type

    dynamic "port" {
      for_each = var.service_ports
      content {
        name        = port.value.name
        port        = port.value.port
        target_port = port.value.target_port
        protocol    = lookup(port.value, "protocol", "TCP")
      }
    }
  }

  depends_on = [kubernetes_manifest.vm]
}
