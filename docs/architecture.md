# Conductor Architecture

## Overview

Conductor is a workflow orchestration library and runtime for building LLM-powered automation. It provides a modular architecture designed for embedding in Go applications while also functioning as a standalone runtime.

## Design Principles

### Embeddable First

Every package in Conductor is designed to work standalone or embedded:

- **No global state**: All components accept dependencies via constructors
- **Clean interfaces**: Public APIs are well-defined and stable
- **Zero foreman dependencies**: Core packages have no knowledge of foreman
- **Pluggable components**: Providers, storage, tools are all swappable

### Observable by Default

Transparency is built into the architecture:

- **Correlation IDs**: Every operation has a unique identifier
- **Event streams**: Components emit structured events
- **Cost tracking**: LLM usage is tracked per-request
- **Request tracing**: Full request/response logging with configurable retention

### Production Ready

Designed for real-world deployment:

- **Error handling**: Graceful degradation with retry and failover
- **Resource limits**: Timeouts, connection pools, context management
- **Testing**: High test coverage with mock providers
- **Performance**: Connection pooling, efficient state management

## Package Structure

```
conduct/
├── cmd/                    # CLI entrypoint
│   └── conduct/           # Main binary
├── pkg/                   # Public API (embeddable)
│   ├── llm/              # LLM provider abstraction
│   ├── workflow/         # Workflow engine
│   ├── agent/            # Agent execution loop
│   └── tools/            # Tool registry and execution
├── internal/             # Private implementation
│   ├── rpc/             # WebSocket RPC server
│   ├── config/          # Configuration management
│   ├── log/             # Structured logging
│   └── db/              # SQLite persistence
├── api/                  # API definitions (shared types)
├── examples/             # Example workflows
└── docs/                 # Documentation
```

### Package Philosophy

**`pkg/*` packages:**
- Designed for embedding in other Go projects
- Minimal dependencies (standard library + well-maintained external libs)
- Stable public APIs
- Comprehensive documentation and examples

**`internal/*` packages:**
- Implementation details
- Can change without breaking external consumers
- Foreman-specific integrations live here

**`cmd/*` packages:**
- CLI and server entrypoints
- Thin wrappers around pkg/ functionality

## Core Components

### 1. LLM Provider Abstraction (`pkg/llm`)

**Purpose:** Provider-agnostic interface for LLM interactions.

**Architecture:**

```
Provider Interface
    |
    +-- Registry (manages providers)
    |
    +-- Anthropic Provider
    +-- OpenAI Provider (placeholder)
    +-- Ollama Provider (placeholder)
    |
    +-- Retry Wrapper (exponential backoff)
    +-- Failover Wrapper (circuit breaker)
    +-- Cost Tracker (token usage)
```

**Key features:**
- Model tiers (fast/balanced/strategic) abstract provider details
- Connection pooling for HTTP efficiency
- Streaming support with channel-based API
- Tool calling abstraction

**Design decisions:**

**Why a provider interface?**
- Allows swapping providers without changing application code
- Enables testing with mock providers
- Supports failover between providers

**Why model tiers?**
- Applications don't need to know specific model names
- Model mapping can change without code updates
- Simplifies multi-provider support

**Why cost tracking?**
- LLM costs can be significant at scale
- Per-request tracking enables budgeting
- Correlation IDs link costs to operations

### 2. Workflow Engine (`pkg/workflow`)

**Purpose:** State machine-based workflow orchestration.

**Architecture:**

```
Workflow Definition (YAML)
    |
    v
Parser (validates and loads)
    |
    v
State Machine Engine
    |
    +-- State Management (current state, history)
    +-- Transition Rules (guards and actions)
    +-- Event System (pub/sub)
    |
    v
Step Executor
    |
    +-- Tool Execution
    +-- LLM Calls
    +-- Condition Evaluation
    +-- Parallel Execution (Phase 1 placeholder)
```

**Key features:**
- YAML-based workflow definitions
- Type-validated inputs (string, number, boolean, object, array)
- Multiple step types (action, llm, condition, parallel)
- Configurable error handling (fail, ignore, retry, fallback)
- Event-driven architecture for observability

**Design decisions:**

**Why YAML definitions?**
- Human-readable and editable
- Language-agnostic (works with any language)
- Standard for workflow definitions

**Why state machine?**
- Clear state transitions prevent invalid states
- Easy to reason about workflow progress
- Supports pause/resume naturally

**Why event system?**
- Decouples workflow engine from consumers
- Enables real-time UI updates
- Facilitates logging and monitoring

### 3. Agent Loop (`pkg/agent`)

**Purpose:** ReAct-style agent that uses tools to accomplish tasks.

**Architecture:**

```
Agent
    |
    +-- LLM Provider (reasoning)
    +-- Tool Registry (actions)
    +-- Context Manager (token limits)
    |
    v
ReAct Loop
    1. LLM reasons about task
    2. LLM requests tool calls
    3. Execute tools
    4. LLM observes results
    5. Repeat until task complete
```

**Key features:**
- Max iteration limit prevents infinite loops
- Token tracking across iterations
- Context pruning at 80% capacity
- Streaming support for real-time updates

**Design decisions:**

**Why ReAct pattern?**
- Industry-proven for LLM agents
- Balances reasoning and action
- Natural fit for tool use

**Why context management?**
- Token limits are finite
- Pruning enables long-running tasks
- System message always preserved

**Why max iterations?**
- Prevents runaway costs
- Fails fast on impossible tasks
- Configurable per use case

### 4. Tool System (`pkg/tools`)

**Purpose:** Registry and execution framework for agent tools.

**Architecture:**

```
Tool Interface
    |
    +-- Registry (discovery and execution)
    |
    +-- Schema (JSON Schema validation)
    |
    +-- Builtin Tools
        +-- File (read/write with safety)
        +-- Shell (exec with timeout)
        +-- HTTP (API calls with allowlist)
```

**Key features:**
- JSON Schema-based input validation
- Configurable safety limits (timeouts, allowlists, max sizes)
- Tool descriptors for LLM function calling
- Extensible via interface implementation

**Design decisions:**

**Why JSON Schema?**
- Standard for API validation
- LLM providers support it natively
- Clear documentation of expected inputs

**Why safety limits?**
- Prevent runaway operations
- Sandbox untrusted tool execution
- Configurable per deployment environment

### 5. RPC Server (`internal/rpc`)

**Purpose:** WebSocket-based RPC for foreman integration.

**Architecture:**

```
WebSocket Server
    |
    +-- Authentication (token-based)
    +-- Message Protocol (JSON-RPC style)
    +-- Streaming Support (SSE-like over WebSocket)
    |
    v
Request Handlers
    +-- Health Check
    +-- LLM Complete/Stream
    +-- Workflow Operations
    +-- Metrics
```

**Key features:**
- Correlation IDs for request/response matching
- Session resumption on reconnection
- Version negotiation in handshake
- Rate limiting on auth failures

**Design decisions:**

**Why WebSocket?**
- Bi-directional streaming
- Lower latency than HTTP polling
- Single connection for all operations

**Why token auth?**
- Simple and secure for localhost
- No persistent credential storage
- Single-session lifetime

**Why correlation IDs?**
- Async request/response correlation
- Enables tracing across components
- Links errors to originating requests

## Data Flow

### Example: LLM Request via RPC

```
Electron Main Process
    |
    | WebSocket RPC Request
    | {"id": "req-123", "method": "llm.complete", "params": {...}}
    v
RPC Server
    |
    | Route to LLM Handler
    v
LLM Provider (with retry/failover)
    |
    | HTTP Request to Anthropic API
    v
Anthropic API
    |
    | HTTP Response
    v
LLM Provider
    |
    | Track cost, correlation ID
    v
RPC Server
    |
    | WebSocket RPC Response
    | {"id": "req-123", "result": {...}}
    v
Electron Main Process
```

### Example: Workflow Execution

```
User Triggers Workflow
    |
    v
Workflow Engine
    |
    | Load definition from YAML
    | Validate inputs
    | Initialize state machine
    v
Execute Steps
    |
    +-- Action Step → Tool Registry → Execute Tool
    +-- LLM Step → LLM Provider → API Call
    +-- Condition Step → Evaluate Expression
    |
    | Each step:
    | - Emits events (start, complete, error)
    | - Updates workflow state
    | - Stores result in context
    v
Workflow Complete
    |
    | Final state: Completed or Failed
    | Output computed from step results
```

### Example: Agent Loop

```
Agent.Run(task)
    |
    | Initialize conversation with system prompt + task
    v
Iteration 1
    |
    | LLM reasons: "Need to read file X"
    | LLM requests: Tool("file", {"operation": "read", "path": "X"})
    v
Execute Tool("file")
    |
    | Read file X
    | Return content
    v
Iteration 2
    |
    | LLM observes: File content
    | LLM reasons: "Need to analyze content"
    | LLM requests: Tool("shell", {"command": "wc", "args": ["-l"]})
    v
Execute Tool("shell")
    |
    | Count lines
    | Return count
    v
Iteration 3
    |
    | LLM observes: Line count
    | LLM reasons: "Task complete"
    | LLM responds: "The file has 42 lines"
    | No tool calls requested
    v
Agent Done
    |
    | Return result with:
    | - Final response
    | - Tool executions log
    | - Token usage
    | - Iteration count
```

## Persistence

### SQLite Storage (`internal/db`)

**Schema:**
- `workflows`: Workflow instances with state
- `events`: Event log for observability
- `cost_records`: LLM usage tracking
- `tool_executions`: Tool call history

**Features:**
- Auto-upgrade on startup with backup
- Configurable retention (default: 7 days)
- Thread-safe access via database/sql

**Why SQLite?**
- Zero configuration
- Embedded (no separate process)
- ACID transactions
- Sufficient for single-instance deployment

**Future:** PostgreSQL backend for multi-instance deployments.

## Error Handling

### Retry Strategy

**LLM Provider Retry:**
- Exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms
- Max retries: 5
- Retry on: HTTP 5xx, HTTP 429, network timeout
- No retry on: HTTP 4xx (except 429), invalid request

**Workflow Step Retry:**
- Configurable per step
- Max attempts: 3
- Backoff: 1s, 2s, 4s
- Retry on: Tool execution error, LLM error

### Failover Strategy

**Provider Failover:**
- Circuit breaker with 5 failure threshold
- Circuit timeout: 30s
- Failover order: Primary → Secondary → Tertiary
- Logged events for monitoring

**Graceful Degradation:**
- Backend crash → Auto-restart (3 attempts)
- All providers fail → Return error to client
- Database locked → Retry with backoff

## Security

### Authentication

**Backend-to-Electron:**
- 32-byte random token (base64url encoded)
- Passed via WebSocket header
- Single session lifetime (memory only)
- Rate limiting: 5 failed attempts/min → 60s lockout

### Credential Management

**LLM API Keys:**
- macOS Keychain (preferred)
- Encrypted file fallback (`~/.config/foreman/credentials.enc`)
- AES-256-GCM encryption
- Machine-specific key derivation
- Zero in memory after use

### Tool Sandboxing

**File Tool:**
- Allowed paths restriction
- Max file size limit
- No symlink following

**Shell Tool:**
- Command allowlist
- 30s timeout with SIGTERM/SIGKILL
- No interactive commands

**HTTP Tool:**
- Host allowlist
- Timeout limits
- No credential leakage in logs

## Performance

### Connection Pooling

**HTTP Client:**
- Max idle connections: 10 per provider
- Idle timeout: 30s
- Connection reuse enabled
- Metrics: active/idle counts

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
- Run existing foreman E2E tests
- Toggle between Node.js and Go backend
- Verify identical behavior

## Future Architecture

### Phase 2 Enhancements

**Multi-instance Support:**
- PostgreSQL backend
- Redis for distributed locking
- gRPC for inter-instance communication

**Enhanced Workflows:**
- Parallel step execution
- Sub-workflow composition
- Conditional branching (CEL expressions)
- Variable interpolation (JSONPath)

**Additional Providers:**
- OpenAI (GPT-4, etc.)
- Ollama (local models)
- Custom provider SDK

### Extraction to Conductor Repo

When ready to extract:

1. Move `pkg/*` to `github.com/tombee/conductor`
2. Keep `internal/rpc` in foreman (foreman-specific)
3. Update import paths
4. Publish Conductor as Go module
5. Foreman imports Conductor as dependency

**Conductor repo structure:**
```
github.com/tombee/conductor/
├── pkg/          # Core packages (from foreman)
├── cmd/          # CLI runtime
├── examples/     # Standalone examples
└── docs/         # Full documentation
```

**Foreman continues as:**
```
github.com/tombee/foreman/
├── src/          # Electron app
├── internal/     # RPC layer to Conduct
└── docs/         # Foreman-specific docs
```

## References

- [LLM Package README](../pkg/llm/README.md)
- [Workflow Package README](../pkg/workflow/README.md)
- [Agent Package README](../pkg/agent/README.md)
- [Tools Package README](../pkg/tools/README.md)
- [Startup Runbook](./runbooks/startup.md)
- [Troubleshooting Runbook](./runbooks/troubleshooting.md)
