# Ryobi Platform — System Overview

## 1. End-to-End Flow: From Catalog Definition to Deployment

```mermaid
flowchart TB
    %% ── Actors ──
    Admin["🔧 Admin"]
    User["👤 Self-Service User"]

    %% ── Admin side ──
    subgraph AdminDashboard["Admin Dashboard"]
        DefineCatalog["Define Catalog Item\n&laquo;Three-Tier App&raquo;"]
        DefineResources["Declare Resources"]
        ExposeParams["Expose Parameters"]
        DefineRules["Define Placement Rules"]
    end

    Admin --> DefineCatalog
    DefineCatalog --> DefineResources
    DefineCatalog --> ExposeParams
    Admin --> DefineRules

    DefineResources --> R1["Container Backend\ntype: Ryobi.Compute/containers"]
    DefineResources --> R2["SQL Database\ntype: Ryobi.Data/sqlDatabases"]
    DefineResources --> R3["Redis Cache\ntype: Ryobi.Data/redisCaches"]

    ExposeParams --> P1["backendReplicas\ntype: number, default: 2"]
    ExposeParams --> P2["dbStorageGB\ntype: number, default: 20"]

    %% ── User side ──
    subgraph Portal["Self-Service Portal"]
        Browse["Browse Catalog"]
        SelectItem["Select\n&laquo;Three-Tier App&raquo;"]
        FillParams["Fill Parameters\nbackendReplicas = 4\ndbStorageGB = 50"]
        ClickDeploy["Click Deploy"]
    end

    User --> Browse
    Browse --> SelectItem
    SelectItem --> FillParams
    FillParams --> ClickDeploy

    %% ── Platform processing ──
    subgraph Platform["Ryobi Control Plane"]
        CreateApp["Create Application"]
        CreateResources["Create 3 Resources\n(with user parameters merged)"]
        Dispatch["Resource Dispatcher"]
        PlacementEngine["Placement Engine"]
    end

    ClickDeploy --> CreateApp --> CreateResources
    CreateResources --> Dispatch

    Dispatch -- "For each resource" --> PlacementEngine

    %% ── Placement to environments ──
    PlacementEngine -- "Container Backend" --> EnvK8s["Kubernetes Env\n(Terraform recipe)"]
    PlacementEngine -- "SQL Database" --> EnvAWS["AWS Env\n(RDS recipe)"]
    PlacementEngine -- "Redis Cache" --> EnvAWS2["AWS Env\n(ElastiCache recipe)"]

    %% ── Results ──
    EnvK8s -- "Terraform apply" --> Deployed["Deployed\nInfrastructure"]
    EnvAWS -- "Terraform apply" --> Deployed
    EnvAWS2 -- "Terraform apply" --> Deployed

    Deployed -- "Status + Health" --> Portal

    %% ── Styling ──
    style Admin fill:#f4a261,stroke:#e76f51,color:#000
    style User fill:#2a9d8f,stroke:#264653,color:#fff
    style AdminDashboard fill:#fdf0d5,stroke:#e76f51
    style Portal fill:#d4f1f4,stroke:#2a9d8f
    style Platform fill:#e9ecef,stroke:#495057
    style PlacementEngine fill:#ffb703,stroke:#fb8500,color:#000
```

## 2. Provider Selection: How the Placement Engine Picks a SQL Database Provider

```mermaid
flowchart TB
    %% ── Trigger ──
    Dispatch["Resource Dispatcher\nresource type: Ryobi.Data/sqlDatabases"]

    %% ── Registered environments ──
    subgraph Environments["Registered Environment Agents"]
        direction LR
        E1["☁️ AWS Env\nregion: us-east-1\nsovereignty: us\ncost: $0.80/hr\nrecipe: aws-rds"]
        E2["☁️ Azure Env\nregion: eu-west-1\nsovereignty: eu\ncost: $0.95/hr\nrecipe: azure-sql"]
        E3["🏢 On-Prem Env\nregion: us-east-1\nsovereignty: us\ncost: $0.40/hr\nrecipe: postgres-vm"]
    end

    %% ── Placement rules ──
    subgraph Rules["Admin-Defined Placement Rules (sorted by priority)"]
        Rule1["Rule: sql-us-prod\npriority: 100\nresourceType: Ryobi.Data/sqlDatabases\nconstraints:\n  region: us-east-1\n  sovereignty: us\npreferences:\n  cost: minimize"]
    end

    %% ── Step 1: Load ──
    Dispatch --> LoadRules["1. Load Placement Rules\nfor Ryobi.Data/sqlDatabases"]
    LoadRules --> Rules

    %% ── Step 2: Hard constraints ──
    Rules --> Filter["2. Filter by Hard Constraints\nregion = us-east-1\nsovereignty = us"]

    Filter -- "✅ matches" --> E1
    Filter -- "❌ eu-west-1, eu" --> E2Fail["Azure Env — EXCLUDED"]
    Filter -- "✅ matches" --> E3

    %% ── Step 3: Soft scoring ──
    subgraph Scoring["3. Score Candidates (soft preferences)"]
        direction TB
        ScoreE1["AWS Env\ncost: $0.80/hr → score 0.50"]
        ScoreE3["On-Prem Env\ncost: $0.40/hr → score 1.00"]
    end

    E1 --> ScoreE1
    E3 --> ScoreE3

    %% ── Step 4: Select ──
    ScoreE1 --> Select["4. Select Highest Score"]
    ScoreE3 --> Select

    Select --> Winner["✅ Winner: On-Prem Env\nrecipe: postgres-vm\nrule: sql-us-prod"]

    %% ── Step 5: Dispatch ──
    Winner --> Agent["5. Dispatch ResourceEvent\nto On-Prem Environment Agent"]
    Agent --> Terraform["6. Agent Runs\nTerraform Apply\n(postgres-vm recipe)"]
    Terraform --> Result["7. Report Result\nstate: Succeeded\noutputs: host, port, dbName"]
    Result --> ResourceStatus["Resource Status Updated\nin Control Plane"]

    %% ── Styling ──
    style Dispatch fill:#ffb703,stroke:#fb8500,color:#000
    style E2Fail fill:#ef233c,stroke:#d90429,color:#fff
    style Winner fill:#2a9d8f,stroke:#264653,color:#fff
    style Scoring fill:#fdf0d5,stroke:#e76f51
    style Environments fill:#e9ecef,stroke:#495057
    style Rules fill:#d4f1f4,stroke:#2a9d8f
```

## How to Read These Diagrams

### Diagram 1 — End-to-End Flow
1. **Admin** uses the Admin Dashboard to define a **Catalog Item** called "Three-Tier App", declaring three resources (Container Backend, SQL Database, Redis Cache) and exposing user-configurable **parameters** (e.g., `backendReplicas`).
2. **User** opens the Self-Service Portal, browses the catalog, selects the item, fills in parameter values, and clicks **Deploy**.
3. The **Control Plane** creates an Application with three Resources (merging user parameter values into the catalog defaults), then the **Resource Dispatcher** sends each resource to the **Placement Engine**.
4. The Placement Engine selects the best **Environment** (provider) for each resource and dispatches it for Terraform execution.

### Diagram 2 — Provider Selection (SQL Database)
1. Three environments are registered, each advertising support for `Ryobi.Data/sqlDatabases` with different regions, costs, and recipes.
2. The admin has defined a placement rule requiring `region: us-east-1` and `sovereignty: us`, with a preference to **minimize cost**.
3. **Hard constraint filtering** eliminates Azure (wrong region/sovereignty). AWS and On-Prem both pass.
4. **Soft scoring** ranks On-Prem higher because its cost ($0.40/hr) is lower than AWS ($0.80/hr).
5. The On-Prem environment wins. Its agent receives the resource event, runs the `postgres-vm` Terraform recipe, and reports the result back to the control plane.
