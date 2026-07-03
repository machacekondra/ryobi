# Design: YAML Application Format

**Status:** Accepted
**Date:** 2026-07-02

## Context

Radius uses Bicep (a domain-specific language) for application definitions. Bicep requires a compiler, has a learning curve, and is primarily associated with Azure. Ryobi replaces this with plain YAML.

## Decision

### Two Document Types

Ryobi YAML files support two kinds of documents:

**Environment** — defines the deployment target:
```yaml
apiVersion: ryobi/v1
kind: Environment
metadata:
  name: prod
providers:
  azure:
    scope: /subscriptions/{sub}/resourceGroups/{rg}
recipes:
  Applications.Datastores/postgresDatabases:
    default:
      templateKind: terraform
      templatePath: ghcr.io/myorg/recipes/postgres:1.0
recipeConfig:
  terraform:
    providers:
      azurerm:
        subscription_id: "{sub}"
```

**Application** — defines resources to deploy:
```yaml
apiVersion: ryobi/v1
kind: Application
metadata:
  name: my-app
  environment: prod
resources:
  - name: database
    type: Applications.Datastores/postgresDatabases
    recipe: default
    parameters:
      sku: Standard_B1ms
    connections:
      - name: cache
        source: redis
        key: connectionString
```

### Multi-Document Support

A single YAML file can contain both environments and applications, separated by `---`. The CLI processes environments first, then applications, ensuring dependencies are met.

### Validation

The CLI validates documents before sending API calls:
- `apiVersion` must be `ryobi/v1`
- `kind` must be `Application` or `Environment`
- `metadata.name` is required on all documents
- `metadata.environment` is optional on Applications (placement engine decides if omitted)
- Each resource must have `name` and `type`; `recipe` is optional (placement engine selects if omitted)
- Resources can include a `placement` section with constraints and preferences for automatic environment selection

### Design Principles

- **No proprietary language.** YAML is universally understood.
- **Declarative.** The file describes the desired state, not the steps to get there.
- **Composable.** Environments and applications can be in separate files or combined.
- **Simple.** No templating, no conditionals, no loops. For complex logic, use Terraform modules.

## Alternatives Considered

| Format | Rejected Because |
|--------|-----------------|
| Bicep | Azure-specific, requires compiler, learning curve |
| HCL | Would be confusing alongside Terraform modules |
| JSON | Too verbose for human authoring |
| CUE | Niche adoption, learning curve |

## Consequences

- Easy to learn and write
- Works with any text editor (no IDE plugin needed)
- Version controllable in Git
- Trade-off: no computed values or templating in the YAML itself
- Trade-off: complex conditional logic must live in Terraform modules
