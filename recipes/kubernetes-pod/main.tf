terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">= 2.20.0"
    }
  }
}

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
        container {
          name              = var.name
          image             = var.image
          image_pull_policy  = var.image_pull_policy

          dynamic "port" {
            for_each = var.ports
            content {
              container_port = port.value.container_port
              protocol       = lookup(port.value, "protocol", "TCP")
            }
          }

          dynamic "env" {
            for_each = var.env
            content {
              name  = env.value.name
              value = env.value.value
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
        }
      }
    }
  }
}
