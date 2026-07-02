terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">= 2.20.0"
    }
  }
}

# --- Namespace (optional) ---

resource "kubernetes_namespace" "app" {
  count = var.create_namespace ? 1 : 0

  metadata {
    name = var.namespace
    labels = {
      "managed-by" = "ryobi"
    }
  }
}

# --- ConfigMap (optional, from env_from_configmap) ---

resource "kubernetes_config_map" "app" {
  count = length(var.config_data) > 0 ? 1 : 0

  metadata {
    name      = "${var.name}-config"
    namespace = var.namespace
    labels = {
      "managed-by"   = "ryobi"
      "ryobi/recipe" = "kubernetes-pod"
      "app"          = var.name
    }
  }

  data = var.config_data

  depends_on = [kubernetes_namespace.app]
}

# --- Secret (optional, from secret_data) ---

resource "kubernetes_secret" "app" {
  count = length(var.secret_data) > 0 ? 1 : 0

  metadata {
    name      = "${var.name}-secret"
    namespace = var.namespace
    labels = {
      "managed-by"   = "ryobi"
      "ryobi/recipe" = "kubernetes-pod"
      "app"          = var.name
    }
  }

  data = var.secret_data

  depends_on = [kubernetes_namespace.app]
}

# --- PersistentVolumeClaim (optional) ---

resource "kubernetes_persistent_volume_claim" "app" {
  for_each = { for v in var.volumes : v.name => v if v.size != "" }

  metadata {
    name      = "${var.name}-${each.key}"
    namespace = var.namespace
    labels = {
      "managed-by" = "ryobi"
      "app"        = var.name
    }
  }

  spec {
    access_modes = [lookup(each.value, "access_mode", "ReadWriteOnce")]
    resources {
      requests = {
        storage = each.value.size
      }
    }
    storage_class_name = lookup(each.value, "storage_class", null)
  }

  depends_on = [kubernetes_namespace.app]
}

# --- Deployment ---

resource "kubernetes_deployment" "app" {
  metadata {
    name      = var.name
    namespace = var.namespace
    labels = merge(var.labels, {
      "managed-by"   = "ryobi"
      "ryobi/recipe" = "kubernetes-pod"
      "app"          = var.name
    })
  }

  spec {
    replicas = var.replicas

    selector {
      match_labels = {
        "app" = var.name
      }
    }

    template {
      metadata {
        labels = merge(var.labels, {
          "app" = var.name
        })
      }

      spec {
        dynamic "init_container" {
          for_each = var.init_containers
          content {
            name    = init_container.value.name
            image   = init_container.value.image
            command = lookup(init_container.value, "command", null)
            args    = lookup(init_container.value, "args", null)
          }
        }

        container {
          name  = var.name
          image = var.image

          dynamic "port" {
            for_each = var.ports
            content {
              container_port = port.value.container_port
              protocol       = lookup(port.value, "protocol", "TCP")
              name           = lookup(port.value, "name", "port-${port.value.container_port}")
            }
          }

          dynamic "env" {
            for_each = var.env
            content {
              name  = env.value.name
              value = lookup(env.value, "value", null)

              dynamic "value_from" {
                for_each = lookup(env.value, "secret_key_ref", null) != null ? [1] : []
                content {
                  secret_key_ref {
                    name = env.value.secret_key_ref.name
                    key  = env.value.secret_key_ref.key
                  }
                }
              }
            }
          }

          dynamic "env_from" {
            for_each = length(var.config_data) > 0 ? [1] : []
            content {
              config_map_ref {
                name = kubernetes_config_map.app[0].metadata[0].name
              }
            }
          }

          dynamic "env_from" {
            for_each = length(var.secret_data) > 0 ? [1] : []
            content {
              secret_ref {
                name = kubernetes_secret.app[0].metadata[0].name
              }
            }
          }

          resources {
            requests = {
              cpu    = var.cpu_request
              memory = var.memory_request
            }
            limits = {
              cpu    = var.cpu_limit != "" ? var.cpu_limit : var.cpu_request
              memory = var.memory_limit != "" ? var.memory_limit : var.memory_request
            }
          }

          dynamic "volume_mount" {
            for_each = var.volumes
            content {
              name       = volume_mount.value.name
              mount_path = volume_mount.value.mount_path
              read_only  = lookup(volume_mount.value, "read_only", false)
            }
          }

          dynamic "liveness_probe" {
            for_each = var.health_check_path != "" ? [1] : []
            content {
              http_get {
                path = var.health_check_path
                port = var.health_check_port
              }
              initial_delay_seconds = var.health_check_initial_delay
              period_seconds        = var.health_check_period
            }
          }

          dynamic "readiness_probe" {
            for_each = var.health_check_path != "" ? [1] : []
            content {
              http_get {
                path = var.health_check_path
                port = var.health_check_port
              }
              initial_delay_seconds = 5
              period_seconds        = 5
            }
          }

          image_pull_policy = var.image_pull_policy
        }

        dynamic "volume" {
          for_each = var.volumes
          content {
            name = volume.value.name

            dynamic "persistent_volume_claim" {
              for_each = volume.value.size != "" ? [1] : []
              content {
                claim_name = "${var.name}-${volume.value.name}"
              }
            }

            dynamic "empty_dir" {
              for_each = volume.value.size == "" ? [1] : []
              content {}
            }
          }
        }

        restart_policy                  = var.restart_policy
        service_account_name            = var.service_account_name != "" ? var.service_account_name : null
        automount_service_account_token = var.service_account_name != "" ? true : null

        dynamic "image_pull_secrets" {
          for_each = var.image_pull_secret != "" ? [1] : []
          content {
            name = var.image_pull_secret
          }
        }
      }
    }
  }

  depends_on = [
    kubernetes_namespace.app,
    kubernetes_config_map.app,
    kubernetes_secret.app,
    kubernetes_persistent_volume_claim.app,
  ]
}

# --- Service ---

resource "kubernetes_service" "app" {
  count = length(var.ports) > 0 ? 1 : 0

  metadata {
    name      = var.name
    namespace = var.namespace
    labels = {
      "managed-by" = "ryobi"
      "app"        = var.name
    }
  }

  spec {
    selector = {
      "app" = var.name
    }

    type = var.service_type

    dynamic "port" {
      for_each = var.ports
      content {
        name        = lookup(port.value, "name", "port-${port.value.container_port}")
        port        = lookup(port.value, "service_port", port.value.container_port)
        target_port = port.value.container_port
        protocol    = lookup(port.value, "protocol", "TCP")
      }
    }
  }

  depends_on = [kubernetes_deployment.app]
}

# --- Ingress (optional) ---

resource "kubernetes_ingress_v1" "app" {
  count = var.ingress_host != "" ? 1 : 0

  metadata {
    name      = var.name
    namespace = var.namespace
    labels = {
      "managed-by" = "ryobi"
      "app"        = var.name
    }
    annotations = var.ingress_annotations
  }

  spec {
    ingress_class_name = var.ingress_class

    rule {
      host = var.ingress_host

      http {
        path {
          path      = var.ingress_path
          path_type = "Prefix"

          backend {
            service {
              name = kubernetes_service.app[0].metadata[0].name
              port {
                number = var.ports[0].container_port
              }
            }
          }
        }
      }
    }

    dynamic "tls" {
      for_each = var.ingress_tls_secret != "" ? [1] : []
      content {
        hosts       = [var.ingress_host]
        secret_name = var.ingress_tls_secret
      }
    }
  }

  depends_on = [kubernetes_service.app]
}
