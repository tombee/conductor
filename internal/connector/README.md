# internal/connector

Runtime execution framework for declarative connectors.

## Overview

Connectors provide deterministic, schema-validated operations that execute without LLM involvement. They enable workflows to interact with external services through HTTP-based integrations.

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                         Registry                              │
│  (stores connectors, provides lookup by name)                 │
└─────────────────────────────┬────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
    ┌──────────┐        ┌──────────┐        ┌──────────┐
    │  GitHub  │        │  Slack   │        │   Jira   │  ...
    │Connector │        │Connector │        │Connector │
    └────┬─────┘        └────┬─────┘        └────┬─────┘
         │                   │                   │
         └───────────────────┼───────────────────┘
                             ▼
                      ┌────────────┐
                      │  Executor  │◄─── Rate Limiting
                      └──────┬─────┘     Security
                             │           Transforms
                             ▼
                      ┌────────────┐
                      │  HTTP      │
                      │ Transport  │◄─── SSRF Protection
                      └────────────┘     Auth Headers
```

## Key Components

| File | Purpose |
|------|---------|
| `connector.go` | Connector interface and Result type |
| `registry.go` | Connector storage and lookup |
| `executor.go` | Operation execution with retries |
| `ratelimit.go` | Token bucket rate limiting |
| `security.go` | SSRF protection, host validation |
| `auth.go` | Authentication header generation |
| `transform.go` | Response transformation (jq) |
| `metrics.go` | Prometheus metrics collection |
| `errors.go` | Error types and classification |

## Built-in Connectors

| Connector | Operations | Auth Types |
|-----------|------------|------------|
| GitHub | 15 | Bearer token |
| Slack | 10 | Bearer token |
| Jira | 11 | Basic auth |
| Discord | 12 | Bot token |
| Jenkins | 15 | Basic auth + CRUMB |

### Subpackage Connectors

| Package | Purpose |
|---------|---------|
| `file/` | File system read/write operations |
| `shell/` | Shell command execution |
| `utility/` | Random, ID generation, math |
| `transform/` | Data transformation connector |

## Usage Example

```go
// Create registry with built-in connectors
registry := connector.NewBuiltinRegistry()

// Get a connector
github, _ := registry.Get("github")

// Execute an operation
result, err := github.Execute(ctx, "create_issue", map[string]any{
    "owner":  "myorg",
    "repo":   "myrepo",
    "title":  "Bug report",
    "body":   "Description",
})

// Access response
fmt.Printf("Issue #%v created\n", result.Response.(map[string]any)["number"])
```

## Security

### SSRF Protection

```go
cfg := connector.DefaultConfig()
cfg.AllowedHosts = []string{"api.github.com", "api.slack.com"}
cfg.BlockedHosts = []string{"169.254.169.254"}  // AWS metadata
```

### Rate Limiting

```go
// Configured per-connector, persisted across restarts
cfg.StateFilePath = "/var/lib/conductor/ratelimit.json"
```

## Workflow Integration

Connectors are invoked in workflow steps:

```yaml
steps:
  - id: create-issue
    connector: github.create_issue
    inputs:
      owner: "{{.inputs.repo_owner}}"
      repo: "{{.inputs.repo_name}}"
      title: "{{.steps.analyze.title}}"
```

Or using shorthand syntax:

```yaml
steps:
  - id: post-message
    slack.post_message:
      channel: "#alerts"
      text: "Deployment complete"
```

## Extension

To add a new connector:

1. Create a new subpackage (e.g., `internal/connector/myservice/`)
2. Implement the `Connector` interface
3. Register in `builtin.go`

```go
type MyConnector struct {
    config Config
}

func (c *MyConnector) Name() string {
    return "myservice"
}

func (c *MyConnector) Execute(ctx context.Context, op string, inputs map[string]any) (*Result, error) {
    // Implementation
}
```
