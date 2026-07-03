# Ryobi

A cloud-native application platform that enables developers and platform engineers to deploy and manage applications using Terraform recipes.

Ryobi provides a simple YAML-based workflow for defining applications and their infrastructure, backed by Terraform for provisioning across any cloud provider.

## Key Features

- **Terraform Recipes** — Define reusable infrastructure templates as Terraform modules. Register once, deploy everywhere.
- **YAML Definitions** — Declare your entire application stack in simple, human-readable YAML.
- **Multi-Cloud** — Deploy to Azure, AWS, or any cloud supported by Terraform providers.
- **Lightweight** — Single binary server with PostgreSQL. No Kubernetes required.
- **Predefined Recipes** — Ships with ready-to-use recipes for Kubernetes Deployments (`Ryobi.Compute/containers`) and KubeVirt VMs (`Ryobi.Compute/virtualMachines`).
- **Smart Placement** — Automatically select the best environment per resource based on constraints (region, sovereignty, capabilities) and preferences (cost, capacity).

## Architecture

```
ryobi CLI ──[HTTP]──▶ ryobid (HTTP :9000 + gRPC :9001)
                        ├── Applications (sync CRUD)
                        ├── Resources (async → dispatches via gRPC)
                        └── gRPC watch stream
                              ↕
                        ryobi-env (environment agent)
                          ├── Registers environment on startup
                          ├── Watches for resource events
                          ├── Executes terraform locally
                          └── Reports results back via gRPC
```

| Component | Description |
|-----------|-------------|
| `ryobi`     | CLI — parses YAML, calls API, displays status |
| `ryobid`    | Server — HTTP API, gRPC event dispatcher |
| `ryobi-env` | Environment agent — registers environment, executes Terraform recipes |

## Quick Start

### Prerequisites

- [Terraform](https://www.terraform.io/downloads) CLI
- Go 1.24+ (if building from source)
- A Kubernetes cluster (for the built-in container/VM recipes)

### 1. Build

```bash
git clone https://github.com/ryobi-project/ryobi.git
cd ryobi
make build
```

### 2. Start the server

```bash
./dist/ryobid
```

### 3. Start an environment agent

```bash
./dist/ryobi-env examples/env-kubernetes.yaml
```

### 4. Deploy an application

```yaml
# app.yaml
apiVersion: ryobi/v1
kind: Application
metadata:
  name: my-app
  environment: dev
resources:
  - name: nginx
    type: Ryobi.Compute/containers
    recipe: kubernetes
    parameters:
      name: nginx
      image: nginx:1.25-alpine
      replicas: 2
      ports:
        - container_port: 80
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
ryobi deploy <file>                     Deploy from YAML
ryobi env list|show|delete              Manage environments
ryobi app list|show|status|delete       Manage applications (delete removes resources first)
ryobi resource list|show|delete         Manage resources
ryobi recipe list                       List registered recipes
ryobi placement create|list|show|delete Manage placement rules (admin)
ryobi version                           Show version info

ryobi-env <config.yaml>                 Start an environment agent
```

## Project Structure

```
cmd/
  ryobi/          CLI entry point
  ryobid/         Server entry point (HTTP + gRPC)
  ryobi-env/      Environment agent entry point
pkg/
  api/            API framework (controllers, async operations)
  grpcapi/        gRPC server, dispatcher, protobuf generated code
  gateway/        HTTP router and server configuration
  resources/      Data models, CRUD controllers
  recipes/        Terraform executor, config generation, state backends
  components/     Database, queue, hosting abstractions
  cli/            YAML parser, API client, output formatting
proto/            Protobuf service definitions
recipes/          Predefined Terraform recipe modules
deploy/
  Containerfile           Container image build
  podman-compose.yaml     Platform deployment
  systemd/                Systemd unit file
  config/                 Default server configuration
docs/                     Documentation website (GitHub Pages)
examples/                 Sample YAML definitions + environment agent configs
```

## Configuration

### Server (`ryobid`)

| Variable | Default | Description |
|----------|---------|-------------|
| `RYOBI_ADDRESS` | `0.0.0.0:9000` | HTTP API listen address |
| `RYOBI_GRPC_ADDRESS` | `0.0.0.0:9001` | gRPC listen address (for env agents) |

### Environment Agent (`ryobi-env`)

Configured via a YAML file (see `examples/env-kubernetes.yaml`):

```yaml
name: dev
server:
  grpcAddress: localhost:9001
providers:
  kubernetes:
    scope: default
terraformProviders:
  kubernetes:
    config_path: "~/.kube/config"
recipes:
  - resourceType: Ryobi.Compute/containers
    recipeName: kubernetes
    templatePath: ../recipes/kubernetes-pod
terraform:
  binaryPath: terraform
  workDir: ~/.ryobi/terraform
```

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
