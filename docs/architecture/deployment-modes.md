# Deployment Modes

Conductor supports multiple deployment topologies for different use cases.

## Mode Comparison

| Mode | Use Case | State Backend | Scaling |
|------|----------|---------------|---------|
| Local CLI | Development, one-off runs | SQLite | Single process |
| Single-node Daemon | Small teams, CI/CD | SQLite | Single process |
| [exe.dev](/deploy/exe.dev/) | Quick demos, experiments | SQLite | Single VM |
| Distributed | Production, high availability | PostgreSQL | Horizontal |
| Kubernetes | Cloud-native deployment | PostgreSQL | Auto-scaling |

## Local CLI Mode

Direct execution for development and testing.

```mermaid
graph LR
    subgraph Local["Local Machine"]
        CLI["conductor CLI"]
        Daemon["conductord"]
        SQLite["SQLite"]

        CLI -->|socket| Daemon
        Daemon --> SQLite
    end

    subgraph External["External Services"]
        LLM["LLM API"]
    end

    Daemon --> LLM
```

**Setup:**
```bash
# Start daemon
conductord &

# Run workflows
conductor run workflow.yaml
```

**Characteristics:**
- Default mode, zero configuration
- State persisted in `~/.conductor/conductor.db`
- Socket at `~/.conductor/conductor.sock`

## Single-node Daemon Mode

For CI/CD and small team deployments.

```mermaid
graph TB
    subgraph Clients["Clients"]
        CLI["CLI Users"]
        CICD["CI/CD Pipelines"]
        Webhooks["Webhook Sources"]
    end

    subgraph Server["Server"]
        Daemon["conductord<br/>:9000"]
        SQLite["SQLite"]
    end

    CLI -->|HTTP| Daemon
    CICD -->|HTTP| Daemon
    Webhooks -->|HTTP| Daemon
    Daemon --> SQLite
```

**Setup:**
```bash
# Start with HTTP listener
conductord --listen :9000

# Configure CLI to use remote
export CONDUCTOR_HOST=http://server:9000
conductor run workflow.yaml
```

**Characteristics:**
- Single instance handles all requests
- HTTP API for remote access
- Suitable for < 100 concurrent workflows

## Distributed Mode

High-availability production deployment.

```mermaid
graph TB
    subgraph LB["Load Balancer"]
        HAProxy["HAProxy / nginx"]
    end

    subgraph Instances["Conductor Instances"]
        D1["conductord-1"]
        D2["conductord-2"]
        D3["conductord-3"]
    end

    subgraph Storage["Shared Storage"]
        PG["PostgreSQL"]
    end

    HAProxy --> D1
    HAProxy --> D2
    HAProxy --> D3

    D1 --> PG
    D2 --> PG
    D3 --> PG

    D1 -.->|leader election| PG
    D2 -.->|leader election| PG
    D3 -.->|leader election| PG
```

**Setup:**
```bash
# Start each instance with PostgreSQL backend
conductord \
  --listen :9000 \
  --backend postgres \
  --postgres-url "postgres://user:pass@db:5432/conductor"
```

**Characteristics:**
- PostgreSQL for shared state
- Leader election via advisory locks
- Job queue with `SELECT FOR UPDATE SKIP LOCKED`
- Any instance can handle any request

### Leader Election

```mermaid
graph LR
    subgraph Instances
        D1["conductord-1<br/>(Leader)"]
        D2["conductord-2<br/>(Follower)"]
        D3["conductord-3<br/>(Follower)"]
    end

    subgraph Leader_Duties["Leader Responsibilities"]
        Scheduler["Cron Scheduler"]
        Cleanup["State Cleanup"]
    end

    D1 --> Scheduler
    D1 --> Cleanup
```

Only the leader runs:
- Cron schedule evaluation
- Expired state cleanup
- Background maintenance tasks

All instances handle:
- API requests
- Workflow execution
- Webhook processing

## Kubernetes Deployment

Cloud-native deployment with auto-scaling.

```mermaid
graph TB
    subgraph Ingress["Ingress"]
        IG["Ingress Controller"]
    end

    subgraph Conductor["Conductor Deployment"]
        subgraph Pods["Pods (3 replicas)"]
            P1["conductor-0"]
            P2["conductor-1"]
            P3["conductor-2"]
        end
        SVC["Service<br/>conductor:9000"]
    end

    subgraph Database["Database"]
        PG["PostgreSQL<br/>(StatefulSet or RDS)"]
    end

    subgraph Storage["Persistent Storage"]
        PVC["PersistentVolumeClaim<br/>workflow-files"]
    end

    IG --> SVC
    SVC --> P1
    SVC --> P2
    SVC --> P3

    P1 --> PG
    P2 --> PG
    P3 --> PG

    P1 --> PVC
    P2 --> PVC
    P3 --> PVC
```

**Example Deployment:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: conductor
spec:
  replicas: 3
  selector:
    matchLabels:
      app: conductor
  template:
    spec:
      containers:
      - name: conductor
        image: ghcr.io/tombee/conductor:latest
        args:
        - --backend=postgres
        - --postgres-url=$(DATABASE_URL)
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: conductor-secrets
              key: database-url
        ports:
        - containerPort: 9000
        livenessProbe:
          httpGet:
            path: /v1/health
            port: 9000
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
```

**Characteristics:**
- Horizontal Pod Autoscaler based on queue depth
- Health checks for automatic recovery
- Secrets via Kubernetes Secrets
- Prometheus metrics at `/metrics`

## exe.dev Deployment

Lightweight VM hosting with minimal setup.

```mermaid
graph LR
    subgraph Local["Local Machine"]
        CLI["conductor CLI"]
    end

    subgraph ExeDev["exe.dev VM"]
        Daemon["conductord<br/>:9000"]
        SQLite["SQLite"]
        Daemon --> SQLite
    end

    subgraph External["External Services"]
        LLM["LLM API"]
    end

    CLI -->|HTTPS| Daemon
    Daemon --> LLM
```

**Setup:**
```bash
# Create VM
ssh exe.dev new --name=conductor

# Install (see deploy/exe.dev/README.md for full instructions)
ssh exe.dev ssh conductor
curl -fsSL .../install-conductor.sh | bash

# Configure local CLI
export CONDUCTOR_HOST=https://conductor-xxx.exe.dev
export CONDUCTOR_API_KEY=<key-from-install>
```

**Characteristics:**
- Deploy in under 5 minutes
- Persistent disk for SQLite storage
- API key authentication required
- Best for demos, experiments, and small teams

See [deploy/exe.dev/](/deploy/exe.dev/) for complete setup guide.

## Choosing a Mode

```mermaid
graph TD
    Start["Start Here"]

    Start --> Q1{"Production<br/>deployment?"}
    Q1 -->|No| Q1a{"Quick demo<br/>or experiment?"}
    Q1a -->|Yes| ExeDev["exe.dev"]
    Q1a -->|No| Local["Local CLI Mode"]
    Q1 -->|Yes| Q2{"Multiple<br/>instances needed?"}

    Q2 -->|No| Single["Single-node Daemon"]
    Q2 -->|Yes| Q3{"Kubernetes?"}

    Q3 -->|No| Distributed["Distributed Mode"]
    Q3 -->|Yes| K8s["Kubernetes Deployment"]
```

---
*See [Architecture Overview](overview.md) for detailed component documentation.*
