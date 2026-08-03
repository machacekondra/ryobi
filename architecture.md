# Ryobi Platform — Architecture Overview

## System Components

Ryobi has **3 binaries** and **1 database**:

| Component | Role |
|-----------|------|
| `ryobi` (CLI) | Parses YAML app definitions, calls the HTTP API, displays status |
| `ryobid` (Control Plane) | HTTP API server + gRPC event dispatcher — the brain of the system |
| `ryobi-env` (Environment Agent) | Registers an environment, watches for work, executes Terraform locally |
| PostgreSQL | Stores resources, async operation queue, and Terraform state |

---

## Component Diagram

```
+-----------------+          +---------------------+         +-----------------+
|   ryobi CLI     |          |  Self-Service Portal |         | Admin Dashboard |
|  (or Portal UI) |          |   (React UI)         |         |  (React UI)     |
+--------+--------+          +---------+-----------+         +--------+--------+
         | HTTP                        | HTTP                        | HTTP
         v                             v                             v
+----------------------------------------------------------------------------+
|                          ryobid  (Control Plane)                           |
|                                                                            |
|  +----------------------------------------------------------------------+  |
|  |  HTTP API Server  (port 9000, Chi Router)                            |  |
|  |                                                                      |  |
|  |  +---------------------+   +------------------------+               |  |
|  |  | Sync Controllers    |   | Async Controllers      |               |  |
|  |  | - Applications      |   | - Resources (PUT)      |               |  |
|  |  | - Environments      |   |   -> 202 Accepted      |               |  |
|  |  | - Catalog Items     |   | - Resources (DELETE)   |               |  |
|  |  | - Placement Rules   |   |   -> 202 Accepted      |               |  |
|  |  +---------------------+   +-----------+------------+               |  |
|  +----------------------------------------|----------------------------+  |
|                                           |                               |
|                                           v                               |
|  +------------------+     +--------------------------+                    |
|  |  StatusManager   |---->|  Queue (PostgreSQL)      |                    |
|  |  (creates ops)   |     |  SKIP LOCKED dequeue     |                    |
|  +------------------+     +------------+-------------+                    |
|                                        | poll (1s)                        |
|                                        v                                  |
|                           +------------------------+                      |
|                           |   Async Worker         |                      |
|                           +-----------+------------+                      |
|                                       |                                   |
|                                       v                                   |
|                           +------------------------+                      |
|                           | Resource Dispatcher    |                      |
|                           | - Resolves app ->      |                      |
|                           |   environment          |                      |
|                           | - Calls Placement      |                      |
|                           |   Engine if needed      |                      |
|                           +-----------+------------+                      |
|                                       |                                   |
|                                       v                                   |
|                           +------------------------+                      |
|                           |  Placement Engine      |                      |
|                           |  1. Load rules         |                      |
|                           |  2. Filter by          |                      |
|                           |     constraints        |                      |
|                           |  3. Score by           |                      |
|                           |     preferences        |                      |
|                           |  4. Select best env    |                      |
|                           +-----------+------------+                      |
|                                       |                                   |
|  +------------------------------------|---------------------------------+ |
|  |  gRPC Server  (port 9001)          |                                 | |
|  |                                    | ResourceEvent                   | |
|  |  - Register / Unregister           | pushed on stream                | |
|  |  - WatchResources (stream) <-------+                                 | |
|  |  - ReportResult --------------------------+                          | |
|  +--------------------------------------------+------------------------+ |
|                                                |                          |
|              Updates resource & operation       |                          |
|              status in database on result       |                          |
+------------------------------------------------|-------------------------+
                    ^                              |
                    | gRPC (persistent connection)  |
                    v                              v
+--------------------------+    +--------------------------+
|  ryobi-env (Agent A)     |    |  ryobi-env (Agent B)     |
|  Environment: "dev"      |    |  Environment: "prod-aws" |
|                          |    |                          |
|  1. Register on startup  |    |  1. Register on startup  |
|  2. Open watch stream    |    |  2. Open watch stream    |
|  3. Receive events       |    |  3. Receive events       |
|  4. terraform init/apply |    |  4. terraform init/apply |
|  5. ReportResult (gRPC)  |    |  5. ReportResult (gRPC)  |
|  6. Unregister on stop   |    |  6. Unregister on stop   |
|                          |    |                          |
|  Has locally:            |    |  Has locally:            |
|  - Terraform binary      |    |  - Terraform binary      |
|  - Cloud credentials     |    |  - AWS credentials       |
|  - Recipe modules        |    |  - Recipe modules        |
+--------------------------+    +--------------------------+
         |                               |
         v                               v
   +-----------+                  +------------+
   | Kubernetes |                  |  AWS Cloud  |
   | Cluster    |                  |  (RDS, EC2) |
   +-----------+                  +------------+
```

---

## Step-by-Step: How a Deployment Works

### Step 1 — User deploys an app
```bash
ryobi deploy app.yaml
```
The CLI parses the YAML file, which defines an Application with Resources. It sends an **HTTP PUT** to `ryobid` for the application and each resource.

### Step 2 — Control plane accepts the request
- The **Application** is saved synchronously (200 OK).
- Each **Resource** is handled asynchronously:
  - Saved to the database
  - An async operation is queued via **StatusManager**
  - Returns **202 Accepted** immediately with a `Location` header for polling

### Step 3 — Async worker picks up the job
The **Worker** polls the queue every 1 second. When it dequeues an operation, it hands it to the **ResourceDispatcher**.

### Step 4 — Placement decision
The dispatcher checks if the resource already has an explicit environment and recipe. If not, the **Placement Engine** kicks in:
1. Loads admin-defined **PlacementRules** from the database
2. Filters connected environments by **hard constraints** (region, sovereignty, capabilities)
3. Scores remaining candidates by **soft preferences** (minimize cost, maximize available resources)
4. Selects the highest-scoring environment

### Step 5 — Dispatch to environment agent via gRPC
The dispatcher sends a **ResourceEvent** over the gRPC **WatchResources** stream to the selected environment agent. This is a server-streaming RPC — the agent holds a persistent connection and receives events as they are pushed.

### Step 6 — Agent executes Terraform
The `ryobi-env` agent:
1. Receives the ResourceEvent from the stream
2. Resolves the recipe from its local config (maps resource type to Terraform module path)
3. Generates `main.tf.json` with module source, provider blocks, and backend config
4. Runs `terraform init` + `terraform apply` (or `destroy`)
5. Extracts outputs from Terraform state

### Step 7 — Agent reports result
The agent calls **ReportResult** gRPC method with the operation ID, success/failure, outputs, and any errors. `ryobid` updates both the resource status and the operation status in the database.

### Step 8 — CLI sees success
The CLI polls the resource status (or uses `--wait`). Once it sees `Succeeded`, it prints the result with any outputs.

---

## Communication Protocols

| From -> To | Protocol | Purpose |
|-----------|----------|---------|
| CLI -> ryobid | **HTTP** (port 9000) | REST API calls (CRUD on all resources) |
| Portal/Admin UI -> ryobid | **HTTP** (port 9000) | Same REST API |
| ryobi-env -> ryobid | **gRPC** (port 9001) | Register, WatchResources (stream), ReportResult, Unregister |
| ryobid -> PostgreSQL | **SQL** | Resource storage, queue, Terraform state |
| ryobi-env -> Infrastructure | **Terraform CLI** | Provisions actual cloud/K8s resources |

---

## gRPC Protocol

Defined in `proto/environment.proto`:

```protobuf
service EnvironmentService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Unregister(UnregisterRequest) returns (UnregisterResponse);
  rpc WatchResources(WatchRequest) returns (stream ResourceEvent);
  rpc ReportResult(ReportResultRequest) returns (ReportResultResponse);
}
```

- **Register** — called once on startup. Creates the environment in the database with providers, recipes, and terraform config.
- **WatchResources** — server-streaming RPC. The agent opens this stream and blocks. When a resource operation targets this environment, `ryobid` pushes a `ResourceEvent`.
- **ReportResult** — called after Terraform execution completes. Reports success/failure and outputs.
- **Unregister** — called on shutdown. Closes all watch channels and deletes the environment from the database.

---

## Key Design Decisions

1. **Server has zero cloud dependencies** — no Terraform binary, no cloud credentials, no provider plugins. All execution happens on agents.
2. **Agents are live processes**, not static config — they register on start, unregister on stop, and maintain persistent gRPC streams.
3. **gRPC streaming over polling** — events are pushed to agents instantly, no wasted polling.
4. **Per-resource placement** — different resources in the same app can land on different environments (e.g., containers on Kubernetes, databases on AWS).
5. **Single PostgreSQL** — stores everything: resources, queue, and Terraform state (via the `pg` backend with per-resource schema isolation).
