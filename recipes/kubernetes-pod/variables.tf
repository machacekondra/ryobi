variable "name" {
  description = "Name of the deployment"
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace"
  type        = string
  default     = "default"
}

variable "image" {
  description = "Container image (e.g. 'nginx:1.25')"
  type        = string
}

variable "replicas" {
  description = "Number of pod replicas"
  type        = number
  default     = 1
}

variable "labels" {
  description = "Additional labels"
  type        = map(string)
  default     = {}
}

variable "image_pull_policy" {
  description = "Image pull policy (Always, IfNotPresent, Never)"
  type        = string
  default     = "IfNotPresent"
}

variable "ports" {
  description = "Container ports to expose"
  type = list(object({
    container_port = number
    protocol       = optional(string, "TCP")
  }))
  default = []
}

variable "env" {
  description = "Environment variables"
  type = list(object({
    name  = string
    value = string
  }))
  default = []
}

variable "cpu_request" {
  description = "CPU request (e.g. '100m', '0.5')"
  type        = string
  default     = "100m"
}

variable "cpu_limit" {
  description = "CPU limit (empty = same as request)"
  type        = string
  default     = ""
}

variable "memory_request" {
  description = "Memory request (e.g. '128Mi', '1Gi')"
  type        = string
  default     = "128Mi"
}

variable "memory_limit" {
  description = "Memory limit (empty = same as request)"
  type        = string
  default     = ""
}
