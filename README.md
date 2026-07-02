# Ryobi

A cloud-native application platform that enables developers and platform engineers to deploy and manage applications using Terraform recipes.

Ryobi provides a simple YAML-based workflow for defining applications and their infrastructure, backed by Terraform for provisioning across any cloud provider.

## Key Features

- **Terraform Recipes** — Define reusable infrastructure templates as Terraform modules. Register once, deploy everywhere.
- **YAML Definitions** — Declare your entire application stack in simple, human-readable YAML.
- **Multi-Cloud** — Deploy to Azure, AWS, or any cloud supported by Terraform providers.
- **Lightweight** — Single binary server with PostgreSQL. No Kubernetes required.
- **Predefined Recipes** — Ships with ready-to-use recipes for Kubernetes Deployments (`Ryobi.Compute/containers`) and KubeVirt VMs (`Ryobi.Compute/virtualMachines`).

## Architecture

```
ryobi CLI ──[HTTP]──▶ ryobid (API server + async worker)
                        ├── Environments (sync CRUD)
                        ├── Applications (sync CRUD)
                        └── Resources (async → Terraform recipes)
                              └── terraform init/apply/destroy
```

| Component | Description |
|-----------|-------------|
| `ryobi`   | CLI — parses YAML, calls API, displays status |
| `ryobid`  | Server — API gateway, resource provider, async worker |

## Quick Start

### Prerequisites

- [Podman](https://podman.io) and `podman-compose`
- [Terraform](https://www.terraform.io/downloads) CLI
- Go 1.24+ (if building from source)

### 1. Start the platform

```bash
git clone https://github.com/ryobi-project/ryobi.git
cd ryobi
podman-compose -f deploy/podman-compose.yaml up -d
```

### 2. Install the CLI

```bash
make build
cp dist/ryobi /usr/local/bin/
```

### 3. Deploy an application

Create a YAML file with your environment and application:

```yaml
apiVersion: ryobi/v1
kind: Environment
metadata:
  name: dev
providers:
  azure:
    scope: /subscriptions/<sub-id>/resourceGroups/<rg>
recipes:
  Applications.Datastores/postgresDatabases:
    default:
      templateKind: terraform
      templatePath: ghcr.io/myorg/recipes/postgres:1.0
---
apiVersion: ryobi/v1
kind: Application
metadata:
  name: my-app
  environment: dev
resources:
  - name: database
    type: Applications.Datastores/postgresDatabases
    recipe: default
```

Deploy:

```bash
ryobi deploy app.yaml
```

Check status:

```bash
ryobi app status my-app
```

## CLI Commands

```
ryobi deploy <file>             Deploy from YAML
ryobi env list|show|delete      Manage environments
ryobi app list|show|status|delete   Manage applications
ryobi resource list|show|delete     Manage resources
ryobi recipe list               List registered recipes
ryobi version                   Show version info
```

## Project Structure

```
cmd/
  ryobi/          CLI entry point
  ryobid/         Server entry point
pkg/
  api/            API framework (controllers, async operations)
  gateway/        HTTP router and server configuration
  resources/      Data models, CRUD controllers, async handlers
  recipes/        Terraform executor, config generation, state backends
  components/     Database, queue, hosting abstractions
  cli/            YAML parser, API client, output formatting
deploy/
  Containerfile           Container image build
  podman-compose.yaml     Platform deployment
  systemd/                Systemd unit file
  config/                 Default server configuration
docs/                     Documentation website (GitHub Pages)
examples/                 Sample YAML definitions
```

## Configuration

### Server (`ryobid`)

| Variable | Default | Description |
|----------|---------|-------------|
| `RYOBI_ADDRESS` | `0.0.0.0:9000` | Listen address |
| `RYOBI_DB_URL` | — | PostgreSQL URL (enables PG state backend) |
| `RYOBI_TF_ROOT_DIR` | `~/.ryobi/terraform` | Terraform working directory |
| `TERRAFORM_PATH` | `terraform` | Path to terraform binary |

### CLI

Config file at `~/.ryobi/config.yaml`:

```yaml
server:
  url: http://localhost:9000
```

Override with `--server`:

```bash
ryobi --server http://ryobi.example.com:9000 env list
```

## Development

```bash
# Build
make build

# Run tests
make test

# Clean
make clean
```

## License

Apache 2.0
