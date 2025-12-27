# Conductor Architecture

:::note[Prerequisites]
This guide explains Conductor's internal architecture and design decisions. It's intended for:

- Contributors wanting to understand the codebase structure
- Users embedding Conductor in their applications
- Advanced users configuring distributed deployments
:::


## Overview

Conductor is a workflow orchestration tool for AI tasks. It provides a daemon-first architecture where all operations go through `conductord`, enabling consistent execution, checkpointing, and an API for community tools.

> **Tagline:** AI workflows as simple as shell scripts.

For vision and positioning, see [vision.md](../vision.md).

## Package Structure

```
conductor/
├── cmd/
│   ├── conductor/       # CLI client
│   └── conductord/      # Daemon binary
├── pkg/                 # Public packages (embeddable)
│   ├── workflow/        # Parser, executor
│   ├── llm/             # LLM provider abstraction
│   ├── tools/           # Tool registry and execution
│   ├── agent/           # Agent configuration
│   ├── security/        # Security profiles, sandboxing
│   └── errors/          # Typed error handling
├── internal/
│   ├── daemon/          # Daemon-specific code
│   │   ├── api/         # HTTP/socket API handlers
│   │   ├── scheduler/   # Cron scheduling
│   │   ├── webhook/     # Webhook routing
│   │   └── backend/     # Persistence, checkpointing
│   ├── cli/             # CLI commands
│   ├── connector/       # Integration connectors
│   └── mcp/             # MCP server integration
└── examples/            # Example workflows
```

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────┐
│  Community Tools / Integrations                              │
│  - Desktop UI               - VS Code extension             │
│  - CI/CD integrations       - Custom dashboards             │
└──────────────────────────┬──────────────────────────────────┘
                           │ Daemon API (recommended)
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  conductord                                                  │
│  - Unix socket + HTTP API   - Webhook routing               │
│  - Scheduling (cron)        - State & checkpointing         │
│  - Provider management      - Job queue                     │
└──────────────────────────┬──────────────────────────────────┘
                           │ imports
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  conductor/pkg                                               │
│  - Workflow parser          - Step executor                 │
│  - Tool system              - LLM provider abstraction      │
│  - (Embeddable for advanced use cases)                      │
└─────────────────────────────────────────────────────────────┘
```

:::note[About Historical References]
Some diagrams and documentation may reference companion desktop applications as examples of community tools. Conductor works independently without any additional applications installed. These references are for architectural context only - the CLI and daemon are the primary interfaces for Conductor.
:::


## Daemon-First Architecture

All CLI commands go through `conductord`. The daemon is required, not optional.

**Why daemon-first:**
- Consistent execution model (checkpointing, state, recovery)
- Provider credentials managed centrally
- Same behavior whether running one-off or production workflows
- API enables community tools to build on Conductor

### CLI → Daemon Communication

```
conductor run workflow.yaml
     │
     │ Unix socket (default) or HTTP
     ▼
conductord (daemon)
     │
     │ Execute workflow
     ▼
LLM Providers / Tools
```

### Daemon API

The daemon exposes a versioned REST API:

```
POST   /v1/runs                    # Start workflow
GET    /v1/runs                    # List runs
GET    /v1/runs/{id}               # Get run status
GET    /v1/runs/{id}/output        # Get run output
GET    /v1/runs/{id}/logs          # Stream logs (SSE)
DELETE /v1/runs/{id}               # Cancel run

GET    /v1/workflows               # List available workflows
POST   /v1/workflows/validate      # Validate workflow YAML

GET    /v1/providers               # List configured providers
GET    /v1/health                  # Health check
```

**Communication options:**
- Unix socket: `~/.conductor/conductor.sock` (default, local)
- HTTP: `http://localhost:9000` (for remote/networked access)

## Core Components

### Workflow Parser & Executor (`pkg/workflow`)

Parses YAML workflow definitions and executes steps.

**Step types:**
- `llm` - LLM completion with optional tools
- `tool` - Direct tool execution
- `condition` - Conditional branching
- `parallel` - Parallel step execution (future)

**Features:**
- Output schemas for structured LLM responses
- Template interpolation (`{{.inputs.name}}`)
- Error handling (fail, ignore, retry)

### LLM Provider Abstraction (`pkg/llm`)

Unified interface for LLM providers.

**PostgreSQL Backend (Implemented):**
- Available via `--backend postgres --postgres-url <connection-string>`
- Supports distributed job queue with `SELECT FOR UPDATE SKIP LOCKED`
- Leader election via PostgreSQL advisory locks
- Schedule state persistence across restarts

**Deferred for Distributed Mode:**
- Config sync across instances (P8-T8) - requires multi-node infrastructure
- Distributed scenario tests (P8-T9) - requires database infrastructure for testing

**Model tiers** (provider-agnostic):
- `fast` - Quick, cost-effective (e.g., claude-3-5-haiku)
- `balanced` - Good balance (e.g., claude-sonnet-4-20250514)
- `powerful` - Maximum capability (e.g., claude-opus-4-5-20251101)

**Built-in providers:**
- Anthropic (Claude)
- OpenAI
- Ollama (local)

### Tool System (`pkg/tools`)

Registry and execution framework for workflow tools.

**Built-in tools:**
- `file` - Read/write files
- `shell` - Execute commands
- `http` - HTTP requests

**Custom tools:**
- Inline HTTP tools (defined in workflow YAML)
- Script tools (stdin/stdout)
- MCP servers (via SPEC-9)

### Scheduler (`internal/daemon/scheduler`)

Cron-based workflow scheduling.

```yaml
triggers:
  - type: schedule
    cron: "0 9 * * 1-5"  # 9am weekdays
```

### Webhooks (`internal/daemon/webhook`)

Receives and routes external events to workflows.

**Supported sources:**
- GitHub (PR events, issues)
- Slack (messages, mentions)
- Discord (messages)
- Generic HTTP webhooks

### State & Checkpointing (`internal/daemon/backend`)

Persists workflow state for crash recovery.

**Storage backends:**
- SQLite (default, local)
- PostgreSQL (future, multi-instance)

**Features:**
- Automatic checkpointing between steps
- Resume interrupted workflows
- Execution history and logs

## Embedding (Advanced)

For advanced use cases, `pkg/workflow` can be embedded directly:

```go
import "github.com/tombee/conductor/pkg/workflow"

executor := workflow.NewExecutor(workflow.ExecutorConfig{
    LLMProvider: myProvider,
})
result, err := executor.Execute(ctx, workflowDef, workflow.RunOptions{
    Inputs: map[string]any{"name": "Alice"},
})
```

**When to embed vs use daemon:**

| Use Case | Recommendation |
|----------|----------------|
| Desktop app | Daemon API |
| VS Code extension | Daemon API |
| CI/CD integration | Daemon API |
| Serverless function | Embed core |
| Unit tests | Embed core |

**Note:** Embedding is supported but not the primary path. You lose checkpointing, centralized provider management, and consistent state.

For detailed embedding guidance, see [Embedding in Go](../extending/embedding.md).

## Security

### Credential Management

- API keys stored in system keychain (macOS Keychain, etc.)
- Environment variable fallback
- Never logged or exposed in error messages

### Tool Sandboxing

- File tool: Configurable allowed paths
- Shell tool: Command timeout, optional allowlist
- HTTP tool: Host allowlist for outbound requests

### Webhook Validation

- Signature verification (HMAC for Slack, Ed25519 for Discord)
- Rate limiting on authentication failures

## Specifications

For detailed requirements, see the specs:

| Spec | Topic |
|------|-------|
| SPEC-1 | Installation & Onboarding |
| SPEC-2 | Workflow Format |
| SPEC-3 | Run Command |
| SPEC-4 | Init & Scaffold |
| SPEC-5 | Smart Input Resolvers |
| SPEC-6 | Output Schemas |
| SPEC-7 | Custom Tools |
| SPEC-8 | Scheduled Workflows |
| SPEC-9 | MCP Integration |
| SPEC-10 | Conductor Daemon |
| SPEC-11 | Basic Examples |
| SPEC-12 | Advanced Examples |

### Context Management

**Token Limits:**
- Track per-message token counts
- Prune at 80% capacity
- Keep system message always
- Truncate individual messages if needed

### Metrics

Exposed at `/metrics` endpoint:
- Request counts (total, success, failure)
- Latency percentiles (p50, p95, p99)
- Cost tracking (total, per-provider, per-model)
- Connection pool stats
- Active workflow count

## Testing Strategy

### Unit Tests

**Coverage targets:**
- `pkg/*` packages: 80%+
- `internal/*` packages: 70%+

**Approach:**
- Mock providers for LLM tests
- In-memory storage for workflow tests
- Table-driven tests for validation logic

### Integration Tests

**Scenarios:**
- Full RPC request/response cycle
- LLM provider with mock API
- Workflow execution end-to-end
- Example workflows

### E2E Tests

**With Feature Flag:**
- Run existing E2E tests
- Toggle between Node.js and Go backend
- Verify identical behavior

## Future Architecture

### Phase 2 Enhancements

**Multi-instance Support (Partially Implemented):**
- ✅ PostgreSQL backend (implemented in SPEC-10)
- ✅ Leader election via PostgreSQL advisory locks (implemented)
- ⏳ Config sync across instances (deferred - requires multi-node infrastructure)
- ⏳ Distributed scenario tests (deferred - requires database infrastructure)
- Redis for distributed locking (optional future enhancement)
- gRPC for inter-instance communication (optional future enhancement)

**Enhanced Workflows:**
- Parallel step execution
- Sub-workflow composition
- Conditional branching (CEL expressions)
- Variable interpolation (JSONPath)

**Additional Providers:**
- OpenAI (GPT, etc.)
- Ollama (local models)
- Custom provider SDK

### Extraction to Conductor Repo

When ready to extract:

1. Move `pkg/*` to `github.com/tombee/conductor`
2. Keep `internal/rpc` in original repo (project-specific)
3. Update import paths
4. Publish Conductor as Go module
5. Original project imports Conductor as dependency

**Conductor repo structure:**
```
github.com/tombee/conductor/
├── pkg/          # Core packages (from original project)
├── cmd/          # CLI runtime
├── examples/     # Standalone examples
└── docs/         # Full documentation
```

## References

For detailed package API documentation, see the package READMEs in the source repository:

- `pkg/llm/README.md` - LLM provider abstraction and model configuration
- `pkg/workflow/README.md` - Workflow parsing and execution engine
- `pkg/agent/README.md` - Agent configuration and state management
- `pkg/tools/README.md` - Tool registry and execution framework

## Next Steps

- **Embedding Guide**: [Embedding in Go](../extending/embedding.md) - Using Conductor as a library
- **Contributing**: [Contributing Guide](../extending/contributing.md) - Development setup and guidelines
- **API Reference**: [API Reference](../reference/api.md) - Detailed package documentation

---
*Last updated: 2025-12-23*
