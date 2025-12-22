# internal/controller

The controller package provides Conductor's persistent server process.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Controller                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │   API    │  │ Webhooks │  │Scheduler │  │  Leader Elector  │ │
│  │ (REST)   │  │          │  │ (cron)   │  │  (distributed)   │ │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘ │
│       │             │             │                  │           │
│       └─────────────┴─────────────┼──────────────────┘           │
│                                   │                              │
│                           ┌───────▼───────┐                      │
│                           │    Runner     │◄──── Concurrency     │
│                           │  (executor)   │      Control         │
│                           └───────┬───────┘                      │
│                                   │                              │
│       ┌───────────────────────────┼───────────────────────────┐  │
│       │                           │                           │  │
│  ┌────▼────┐  ┌──────────┐  ┌─────▼─────┐  ┌────────────────┐ │  │
│  │ Backend │  │Checkpoint│  │    MCP    │  │ Auth Middleware│ │  │
│  │(storage)│  │ Manager  │  │ Registry  │  │   (API keys)   │ │  │
│  └─────────┘  └──────────┘  └───────────┘  └────────────────┘ │  │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### Core Components

| Package | Purpose |
|---------|---------|
| `controller.go` | Main Controller struct - lifecycle and component wiring |
| `runner/` | Workflow executor with checkpointing |
| `backend/` | Run state persistence (memory, postgres) |
| `api/` | REST API handlers for runs, triggers, schedules |

### Input Sources

| Package | Purpose |
|---------|---------|
| `webhook/` | Process webhooks from GitHub, Slack, etc. |
| `scheduler/` | Cron-based workflow scheduling |
| `trigger/` | Workflow trigger scanning and validation |

### Infrastructure

| Package | Purpose |
|---------|---------|
| `auth/` | API key validation middleware |
| `listener/` | Unix socket and TCP listener setup |
| `leader/` | Leader election for distributed mode |
| `checkpoint/` | Run state checkpointing for recovery |
| `config/` | Controller-specific configuration |

### Integrations

| Package | Purpose |
|---------|---------|
| `github/` | GitHub API integration and token resolution |
| `remote/` | Remote workflow fetching (github:user/repo) |
| `cache/` | Caching for remote workflows |
| `queue/` | Internal work queue management |

## Data Flow

1. **API Request -> Runner**
   - Client sends POST to `/v1/trigger/{workflow}`
   - API handler loads workflow YAML
   - Runner.Submit() creates run with pending status
   - Semaphore controls concurrency

2. **Webhook -> Runner**
   - External service sends POST to `/webhooks/{path}`
   - Webhook router matches route, verifies signature
   - Maps payload to workflow inputs
   - Runner.Submit() creates run

3. **Scheduler -> Runner**
   - Cron job fires based on schedule
   - Scheduler loads workflow
   - Runner.Submit() creates run

## Extension Points

- **Custom Backends**: Implement `backend.Backend` interface
- **Webhook Sources**: Add routes to webhook configuration
- **Metrics**: Use `SetMetrics()` to wire up collectors

## Configuration

Key configuration options in `config.Config.Controller`:

```yaml
controller:
  listen:
    socket_path: /var/run/conductor.sock
    # OR
    address: ":8080"

  backend:
    type: postgres  # or "memory"
    postgres:
      connection_string: postgres://...

  max_concurrent_runs: 10
  default_timeout: 30m

  webhooks:
    routes:
      - path: /github
        source: github
        workflow: on-push.yaml

  schedules:
    enabled: true
    schedules:
      - name: nightly
        cron: "0 0 * * *"
        workflow: nightly-build.yaml
```
