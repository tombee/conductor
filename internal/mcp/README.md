# internal/mcp

Model Context Protocol (MCP) implementation for Conductor.

## Overview

MCP enables workflows to integrate with external tool servers. This package manages server processes, handles JSON-RPC communication, and adapts MCP tools to Conductor's tool interface.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Registry                              │
│  (daemon-level server management + auto-start support)       │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                        Manager                               │
│  (server lifecycle: start, stop, restart, health monitoring) │
└────────┬────────────────┬────────────────┬──────────────────┘
         │                │                │
    ┌────▼────┐      ┌────▼────┐      ┌────▼────┐
    │ Server  │      │ Server  │      │ Server  │
    │  State  │      │  State  │      │  State  │
    └────┬────┘      └────┬────┘      └────┬────┘
         │                │                │
    ┌────▼────┐      ┌────▼────┐      ┌────▼────┐
    │ Client  │      │ Client  │      │ Client  │
    │ (stdio) │      │ (stdio) │      │ (stdio) │
    └────┬────┘      └────┬────┘      └────┬────┘
         │                │                │
    ┌────▼────┐      ┌────▼────┐      ┌────▼────┐
    │ Server  │      │ Server  │      │ Server  │
    │ Process │      │ Process │      │ Process │
    └─────────┘      └─────────┘      └─────────┘
```

## Key Components

### Manager

Handles lifecycle for individual MCP server processes:

```go
mgr := mcp.NewManager(mcp.ManagerConfig{Logger: logger})

// Start a server
err := mgr.Start(mcp.ServerConfig{
    Name:    "filesystem",
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
})

// Get status
status, _ := mgr.GetStatus("filesystem")

// Restart
mgr.Restart("filesystem")

// Stop
mgr.Stop("filesystem")
```

### Client

JSON-RPC client for communication with MCP servers:

```go
client, _ := mgr.GetClient("filesystem")

// List tools
tools, _ := client.ListTools(ctx)

// Call a tool
result, _ := client.CallTool(ctx, "read_file", map[string]any{
    "path": "/etc/hosts",
})
```

### Registry

Daemon-level management combining configuration with runtime state:

```go
registry, _ := mcp.NewRegistry(mcp.RegistryConfig{Logger: logger})

// Start auto-start servers
registry.Start(ctx)

// Get summary
summary := registry.GetSummary()
// summary.Total, summary.Running, summary.Stopped, summary.Error
```

### Tool Adapter

Adapts MCP tools to Conductor's `tools.Tool` interface:

```go
mcpTool := mcp.NewMCPTool(serverName, toolDef, client)
toolRegistry.Register(mcpTool)
```

## Server States

| State | Description |
|-------|-------------|
| `stopped` | Server not running |
| `starting` | Process spawning, waiting for ping |
| `running` | Healthy, accepting requests |
| `restarting` | Restarting after failure or explicit request |
| `error` | Failed to start or crashed |

## Failure Handling

The Manager implements exponential backoff for failed servers:

- 1st failure: 1s retry
- 2nd failure: 2s retry
- 3rd failure: 4s retry
- Max backoff: 30s

## Files

| File | Purpose |
|------|---------|
| `manager.go` | Server lifecycle management |
| `client.go` | JSON-RPC client implementation |
| `registry.go` | Daemon-level server registry |
| `config.go` | Configuration loading |
| `tool_adapter.go` | MCP → Conductor tool adaptation |
| `types.go` | Protocol type definitions |
| `events.go` | Server lifecycle events |
| `errors.go` | Error types |
| `logs.go` | Log capture (ring buffer) |
| `state.go` | State persistence |
| `watcher.go` | Configuration file watching |
| `lockfile.go` | Version lockfile management |
| `provider.go` | Provider interfaces for testing |

## Configuration

Daemon-level MCP servers in `~/.config/conductor/mcp-servers.yaml`:

```yaml
servers:
  - name: filesystem
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem"]
    env:
      - HOME=/home/user
    timeout: 30  # seconds
    auto_start: true
```

Workflow-level MCP servers in workflow YAML:

```yaml
name: my-workflow
mcp_servers:
  - name: db
    command: mcp-postgres
    args: ["--connection-string", "$POSTGRES_URL"]
steps:
  - id: query
    llm:
      prompt: "Query the users table"
      functions:
        - mcp:db:*
```
