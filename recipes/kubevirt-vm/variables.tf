# --- Core ---

variable "name" {
  description = "Name of the virtual machine"
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace for the VM"
  type        = string
  default     = "default"
}

variable "create_namespace" {
  description = "Create the namespace if it doesn't exist"
  type        = bool
  default     = false
}

variable "running" {
  description = "Whether the VM should be started"
  type        = bool
  default     = true
}

variable "labels" {
  description = "Additional labels to apply to the VM"
  type        = map(string)
  default     = {}
}

# --- CPU ---

variable "cpu_cores" {
  description = "Number of CPU cores"
  type        = number
  default     = 1
}

variable "cpu_sockets" {
  description = "Number of CPU sockets"
  type        = number
  default     = 1
}

variable "cpu_threads" {
  description = "Number of CPU threads per core"
  type        = number
  default     = 1
}

# --- Memory ---

variable "memory" {
  description = "Memory request (e.g. '1Gi', '512Mi')"
  type        = string
  default     = "1Gi"
}

variable "memory_limit" {
  description = "Memory limit (empty string means no limit)"
  type        = string
  default     = ""
}

# --- Disk ---

variable "disk_image" {
  description = "Container registry URL for the boot disk image (e.g. 'docker://quay.io/containerdisks/fedora:latest')"
  type        = string
  default     = ""
}

variable "disk_size" {
  description = "Size of the boot disk PVC"
  type        = string
  default     = "10Gi"
}

variable "disk_bus" {
  description = "Disk bus type (virtio, sata, scsi)"
  type        = string
  default     = "virtio"
}

variable "disk_access_mode" {
  description = "PVC access mode for the disk"
  type        = string
  default     = "ReadWriteOnce"
}

variable "storage_class" {
  description = "Storage class for the boot disk PVC"
  type        = string
  default     = ""
}

# --- Network ---

variable "network_type" {
  description = "Network interface type (masquerade or bridge)"
  type        = string
  default     = "masquerade"

  validation {
    condition     = contains(["masquerade", "bridge"], var.network_type)
    error_message = "network_type must be 'masquerade' or 'bridge'"
  }
}

# --- Cloud-Init ---

variable "cloud_init_enabled" {
  description = "Enable cloud-init configuration"
  type        = bool
  default     = false
}

variable "cloud_init_user_data" {
  description = "Cloud-init user data (YAML string)"
  type        = string
  default     = ""
}

# --- Service ---

variable "service_ports" {
  description = "Ports to expose via a Kubernetes Service"
  type = list(object({
    name        = string
    port        = number
    target_port = number
    protocol    = optional(string, "TCP")
  }))
  default = []
}

variable "service_type" {
  description = "Service type (ClusterIP, NodePort, LoadBalancer)"
  type        = string
  default     = "ClusterIP"
}
