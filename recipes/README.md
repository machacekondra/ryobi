# Predefined Recipes

This directory contains built-in Terraform recipe modules that ship with Ryobi.

## Available Recipes

| Recipe | Resource Type | Description |
|--------|--------------|-------------|
| [kubevirt-vm](kubevirt-vm/) | `Ryobi.Compute/virtualMachines` | KubeVirt virtual machine on Kubernetes |

## Using a Recipe

Register the recipe in your environment YAML, then reference it from application resources:

```yaml
# Environment
recipes:
  Ryobi.Compute/virtualMachines:
    kubevirt:
      templateKind: terraform
      templatePath: ./recipes/kubevirt-vm

# Application resource
resources:
  - name: my-vm
    type: Ryobi.Compute/virtualMachines
    recipe: kubevirt
    parameters:
      name: my-vm
      memory: 2Gi
      disk_image: "docker://quay.io/containerdisks/fedora:latest"
```

## Creating Custom Recipes

Any Terraform module can be used as a recipe. See [Create a Recipe](../docs/guides/create-recipe.html) in the documentation.
