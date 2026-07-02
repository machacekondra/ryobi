# Design: Terraform-Only Recipes

**Status:** Accepted
**Date:** 2026-07-02

## Context

Radius supports two recipe drivers: Terraform and Bicep. Application workloads can also be deployed directly as Kubernetes containers. Ryobi simplifies this by making Terraform the sole mechanism for provisioning all infrastructure.

## Decision

### All Infrastructure via Terraform

- **No Bicep support.** Bicep is Azure-specific and adds complexity.
- **No direct container management.** Ryobi does not run containers via Podman/Docker API. All workloads (VMs, databases, web apps, containers) are provisioned through Terraform recipes.
- **Podman is only for platform deployment** — running `ryobid` itself, not user workloads.

### Recipe Lifecycle

1. **Register:** Define recipes in environment YAML under the `recipes` field, mapping resource types to Terraform module sources.
2. **Reference:** Application resources reference a recipe by name and resource type.
3. **Execute:** The async worker resolves the recipe, generates `main.tf.json` (module source, backend config, provider config), and runs `terraform init` + `terraform apply`.
4. **Output:** Terraform outputs are extracted from state and stored on the resource status.
5. **Delete:** `terraform destroy` is run, then the resource record is deleted.

### Config Generation

For each recipe execution, Ryobi generates a `main.tf.json` containing:

```json
{
  "terraform": {
    "backend": {
      "pg": {
        "conn_str": "postgres://...",
        "schema_name": "tfstate_<hash>"
      }
    }
  },
  "provider": {
    "azurerm": [{"features": {}, "subscription_id": "..."}]
  },
  "module": {
    "recipe": {
      "source": "ghcr.io/myorg/recipes/postgres:1.0",
      "sku": "Standard_B1ms"
    }
  },
  "output": {
    "result": {"value": "${module.recipe}"}
  }
}
```

### Parameter Merging

Recipe parameters come from two sources with resource-level parameters taking precedence:

1. Environment recipe definition (`recipes.<type>.<name>.parameters`)
2. Application resource definition (`resources[].parameters`)

### Provider Configuration

The executor auto-configures Terraform providers based on the environment:

| Environment Provider | Terraform Provider | Configuration Source |
|---------------------|-------------------|---------------------|
| `azure` | `azurerm` | Subscription ID from scope + `recipeConfig.terraform.providers.azurerm` |
| `aws` | `aws` | Region from scope + `recipeConfig.terraform.providers.aws` |
| `kubernetes` | `kubernetes` | `recipeConfig.terraform.providers.kubernetes` (e.g. `config_path`) |

### Template Path Resolution

The `templatePath` in a recipe definition can be:
- **Local path** (`./recipes/kubernetes-pod`) — resolved to absolute path at execution time
- **Registry** (`ghcr.io/myorg/recipes/postgres:1.0`) — used as-is
- **Git URL** — used as-is
- **HTTP URL** — used as-is

### Predefined Recipes

Ryobi ships with built-in recipes in the `recipes/` directory:

| Recipe | Resource Type | Description |
|--------|--------------|-------------|
| `kubernetes-pod` | `Ryobi.Compute/containers` | Kubernetes Deployment |
| `kubevirt-vm` | `Ryobi.Compute/virtualMachines` | KubeVirt VirtualMachine |

## Consequences

- Simpler codebase: one driver instead of two
- Full Terraform provider ecosystem available (Azure, AWS, Kubernetes, etc.)
- Any infrastructure that Terraform can manage is deployable
- Predefined recipes provide out-of-the-box support for common resource types
- Trade-off: no Bicep support for Azure-native teams
- Trade-off: no direct container runtime (everything goes through Terraform)
