# --- Core ---

variable "name" {
  description = "Name of the deployment and related resources"
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace"
  type        = string
  default     = "default"
}

variable "create_namespace" {
  description = "Create the namespace if it doesn't exist"
  type        = bool
  default     = false
}

variable "image" {
  description = "Container image (e.g. 'nginx:1.25', 'myregistry.io/app:latest')"
  type        = string
}

variable "replicas" {
  description = "Number of pod replicas"
  type        = number
  default     = 1
}

variable "labels" {
  description = "Additional labels for all resources"
  type        = map(string)
  default     = {}
}

variable "image_pull_policy" {
  description = "Image pull policy (Always, IfNotPresent, Never)"
  type        = string
  default     = "IfNotPresent"
}

variable "image_pull_secret" {
  description = "Name of the image pull secret"
  type        = string
  default     = ""
}

variable "restart_policy" {
  description = "Pod restart policy"
  type        = string
  default     = "Always"
}

variable "service_account_name" {
  description = "Service account to use for the pod"
  type        = string
  default     = ""
}

# --- Ports ---

variable "ports" {
  description = "Container ports to expose"
  type = list(object({
    container_port = number
    service_port   = optional(number)
    protocol       = optional(string, "TCP")
    name           = optional(string)
  }))
  default = []
}

# --- Environment ---

variable "env" {
  description = "Environment variables"
  type = list(object({
    name  = string
    value = optional(string)
    secret_key_ref = optional(object({
      name = string
      key  = string
    }))
  }))
  default = []
}

variable "config_data" {
  description = "Data for a ConfigMap that will be mounted as env vars"
  type        = map(string)
  default     = {}
}

variable "secret_data" {
  description = "Data for a Secret that will be mounted as env vars"
  type        = map(string)
  default     = {}
  sensitive   = true
}

# --- Resources ---

variable "cpu_request" {
  description = "CPU request (e.g. '100m', '0.5', '1')"
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

# --- Volumes ---

variable "volumes" {
  description = "Volumes to mount into the container"
  type = list(object({
    name          = string
    mount_path    = string
    size          = optional(string, "")
    access_mode   = optional(string, "ReadWriteOnce")
    storage_class = optional(string, "")
    read_only     = optional(bool, false)
  }))
  default = []
}

# --- Health Checks ---

variable "health_check_path" {
  description = "HTTP path for liveness and readiness probes (empty to disable)"
  type        = string
  default     = ""
}

variable "health_check_port" {
  description = "Port for health check probes"
  type        = number
  default     = 8080
}

variable "health_check_initial_delay" {
  description = "Initial delay seconds for liveness probe"
  type        = number
  default     = 15
}

variable "health_check_period" {
  description = "Period seconds for liveness probe"
  type        = number
  default     = 10
}

# --- Init Containers ---

variable "init_containers" {
  description = "Init containers to run before the main container"
  type = list(object({
    name    = string
    image   = string
    command = optional(list(string))
    args    = optional(list(string))
  }))
  default = []
}

# --- Service ---

variable "service_type" {
  description = "Kubernetes Service type (ClusterIP, NodePort, LoadBalancer)"
  type        = string
  default     = "ClusterIP"
}

# --- Ingress ---

variable "ingress_host" {
  description = "Hostname for Ingress (empty to skip Ingress creation)"
  type        = string
  default     = ""
}

variable "ingress_path" {
  description = "Ingress path"
  type        = string
  default     = "/"
}

variable "ingress_class" {
  description = "Ingress class name"
  type        = string
  default     = "nginx"
}

variable "ingress_tls_secret" {
  description = "TLS secret name for Ingress (empty for no TLS)"
  type        = string
  default     = ""
}

variable "ingress_annotations" {
  description = "Additional annotations for the Ingress resource"
  type        = map(string)
  default     = {}
}
