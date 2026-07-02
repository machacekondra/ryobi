output "deployment_name" {
  description = "Name of the Kubernetes Deployment"
  value       = kubernetes_deployment.app.metadata[0].name
}

output "namespace" {
  description = "Namespace of the Deployment"
  value       = var.namespace
}
