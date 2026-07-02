output "deployment_name" {
  description = "Name of the Kubernetes Deployment"
  value       = kubernetes_deployment.app.metadata[0].name
}

output "namespace" {
  description = "Namespace of the deployed resources"
  value       = var.namespace
}

output "service_name" {
  description = "Name of the Kubernetes Service (if created)"
  value       = length(var.ports) > 0 ? kubernetes_service.app[0].metadata[0].name : ""
}

output "service_cluster_ip" {
  description = "ClusterIP of the Service (if created)"
  value       = length(var.ports) > 0 ? kubernetes_service.app[0].spec[0].cluster_ip : ""
}

output "ingress_host" {
  description = "Ingress hostname (if created)"
  value       = var.ingress_host
}

output "endpoint" {
  description = "Primary endpoint for connecting to this container"
  value       = length(var.ports) > 0 ? "${kubernetes_service.app[0].metadata[0].name}.${var.namespace}.svc.cluster.local:${var.ports[0].container_port}" : ""
}
