output "vm_name" {
  description = "Name of the created VirtualMachine"
  value       = var.name
}

output "vm_namespace" {
  description = "Namespace of the VirtualMachine"
  value       = var.namespace
}

output "service_name" {
  description = "Name of the Kubernetes Service (if created)"
  value       = length(var.service_ports) > 0 ? kubernetes_service.vm[0].metadata[0].name : ""
}

output "service_cluster_ip" {
  description = "ClusterIP of the Service (if created)"
  value       = length(var.service_ports) > 0 ? kubernetes_service.vm[0].spec[0].cluster_ip : ""
}
