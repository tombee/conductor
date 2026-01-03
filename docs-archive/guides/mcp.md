# MCP Servers

Extend LLM capabilities with Model Context Protocol (MCP) servers.

## Overview

MCP servers provide additional tools that LLMs can call during workflow execution. Instead of defining every capability in your workflow, you can connect to MCP servers that expose domain-specific tools.

**Key concepts:**

- **Server lifecycle** — Conductor manages MCP server processes alongside your workflows
- **Tool registry** — Tools from MCP servers become available in LLM steps
- **Automatic availability** — Once configured, LLM steps can use MCP tools without additional setup

## Server Configuration

Configure MCP servers in your Conductor config:

```yaml
# ~/.config/conductor/config.yaml
mcp:
  servers:
    # Filesystem access
    filesystem:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/dir"]

    # GitHub integration
    github:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_PERSONAL_ACCESS_TOKEN: ${GITHUB_TOKEN}

    # Custom HTTP tool server
    custom-api:
      command: python
      args: ["./mcp-servers/api-server.py"]
      env:
        API_BASE_URL: https://api.example.com
        API_KEY: ${CUSTOM_API_KEY}
```

### Configuration Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `command` | string | Yes | Command to start the server |
| `args` | array | No | Command arguments |
| `env` | object | No | Environment variables |
| `working_dir` | string | No | Working directory for the process |
| `timeout` | duration | No | Startup timeout (default: 30s) |

## Server Lifecycle

### Startup

Conductor starts MCP servers when needed:

1. **On-demand start** — Server starts when first LLM step requests MCP tools
2. **Health check** — Conductor verifies server is responding (timeout: 10s)
3. **Tool discovery** — Server's available tools are registered

```
[MCP] Starting filesystem server...
[MCP] filesystem: Health check passed
[MCP] filesystem: Registered 5 tools
```

### Health Checks

Conductor monitors server health:

- **Interval:** Every 30 seconds
- **Timeout:** 10 seconds per check
- **Failure threshold:** 3 consecutive failures trigger restart

### Restart Behavior

When a server fails:

1. **Graceful restart** — Wait 5 seconds, attempt restart
2. **Retry limit** — Maximum 3 restart attempts
3. **Backoff** — Delay doubles with each attempt (5s, 10s, 20s)
4. **Circuit breaker** — After 3 failures, server marked unhealthy for 5 minutes

### Graceful Shutdown

On workflow completion or Conductor shutdown:

1. **Drain timeout** — Wait up to 30 seconds for in-flight requests
2. **SIGTERM** — Send termination signal
3. **Force kill** — After 10 seconds, send SIGKILL

## Tool Registry

### How Tools Become Available

When an MCP server starts, Conductor:

1. Queries the server's tool list via MCP protocol
2. Registers each tool with a namespaced name: `{server-name}:{tool-name}`
3. Makes tools available to subsequent LLM steps

### Using MCP Tools in Workflows

LLM steps automatically have access to registered MCP tools:

```conductor
steps:
  - id: analyze
    type: llm
    model: balanced
    prompt: |
      Read the contents of main.go and summarize the key functions.
    # LLM can call filesystem:read_file tool if filesystem server is configured
```

### Tool Namespacing

Tools are namespaced by server name to avoid conflicts:

- `filesystem:read_file`
- `filesystem:write_file`
- `github:create_issue`
- `github:list_pull_requests`
- `custom-api:query_users`

## Configuration Examples

### Filesystem Server

Allow LLM to read files from a specific directory:

```yaml
mcp:
  servers:
    filesystem:
      command: npx
      args:
        - "-y"
        - "@modelcontextprotocol/server-filesystem"
        - "/home/user/projects"
```

**Security:** Only paths under `/home/user/projects` are accessible.

### GitHub Server

Enable GitHub operations:

```yaml
mcp:
  servers:
    github:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_PERSONAL_ACCESS_TOKEN: ${GITHUB_TOKEN}
```

**Available tools:**
- `github:create_issue`
- `github:list_issues`
- `github:create_pull_request`
- `github:list_pull_requests`
- And more...

### Custom HTTP Tool Server

Build your own MCP server for internal APIs:

```yaml
mcp:
  servers:
    internal-api:
      command: python
      args: ["./servers/internal-api.py"]
      working_dir: /opt/conductor
      env:
        API_ENDPOINT: https://internal-api.company.com
        API_TOKEN: ${INTERNAL_API_TOKEN}
      timeout: 60s
```

## Security Considerations

### Source Verification

Only run MCP servers from trusted sources:

- **Official servers** — `@modelcontextprotocol/server-*` packages
- **Internal servers** — Your organization's verified code
- **Third-party** — Review source code before use

!!! warning "Untrusted servers"
    MCP servers execute with the same permissions as Conductor. A malicious server could access files, network, or credentials available to the Conductor process.

### Tool Permission Scoping

Limit what tools can access:

```yaml
mcp:
  servers:
    filesystem:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp/sandbox"]
      # Only /tmp/sandbox is accessible
```

### Information Leakage Prevention

Be aware of what data flows through MCP tools:

- Tool inputs may contain sensitive workflow data
- Tool outputs are visible to LLM and may appear in logs
- Avoid passing secrets through MCP tool parameters

### Rate Limiting

Protect against runaway tool usage:

```yaml
mcp:
  rate_limit:
    requests_per_minute: 100
    burst: 20
```

### Audit Logging

MCP tool calls are logged for audit:

```json
{
  "level": "info",
  "msg": "mcp_tool_call",
  "server": "github",
  "tool": "create_issue",
  "duration_ms": 245,
  "workflow_id": "abc123",
  "correlation_id": "xyz789"
}
```

## Troubleshooting

### Server Fails to Start

**Symptoms:** Workflow fails with "MCP server not available"

**Checks:**

1. **Command exists:** Verify the command is in PATH
   ```bash
   which npx
   npx -y @modelcontextprotocol/server-filesystem --help
   ```

2. **Permissions:** Ensure Conductor can execute the command
3. **Dependencies:** Install required packages
   ```bash
   npm install -g @modelcontextprotocol/server-filesystem
   ```

4. **Logs:** Check Conductor logs for startup errors
   ```bash
   grep "MCP" /var/log/conductor/conductor.log
   ```

### Tool Not Available

**Symptoms:** LLM says it can't access a tool, or tool call fails

**Checks:**

1. **Server running:** Verify server started successfully
   ```bash
   conductor status  # Check MCP servers section
   ```

2. **Tool registered:** List available tools
   ```bash
   conductor mcp list-tools
   ```

3. **Namespace:** Use fully qualified name `{server}:{tool}`

### Connection Timeout

**Symptoms:** "MCP server timeout" errors

**Fixes:**

1. **Increase timeout:**
   ```yaml
   mcp:
     servers:
       slow-server:
         command: ...
         timeout: 120s  # Increase from default 30s
   ```

2. **Check server health:** Server may be overloaded or unresponsive
3. **Network issues:** Verify network connectivity if server is remote

### Execution Errors

**Symptoms:** Tool calls return errors

**Debugging:**

1. **Check tool parameters:** Verify inputs match expected schema
2. **Review server logs:** MCP server may log detailed errors
3. **Test directly:** Call the tool outside of Conductor to isolate issue

## Operational Considerations

### Resource Requirements

MCP servers consume system resources:

- **Memory:** Each server is a separate process (typically 50-200MB)
- **CPU:** Minimal when idle, scales with tool usage
- **File descriptors:** Each server uses stdio for communication

### Monitoring

Monitor MCP server health via:

- **Health endpoint:** `/health` includes MCP server status
- **Metrics:** `conductor_mcp_servers_status{server="name",status="healthy|unhealthy"}`
- **Logs:** MCP lifecycle events logged at INFO level

### Recovery Procedures

**Restart a stuck server:**

```bash
conductor mcp restart filesystem
```

**Disable a problematic server:**

```yaml
mcp:
  servers:
    problematic:
      enabled: false  # Temporarily disable
      command: ...
```

## Implementation Status

!!! note "Feature maturity"
    MCP support is functional but some advanced features are planned:

    - **Stable:** Server lifecycle, tool registry, basic configuration
    - **Planned:** Dynamic tool discovery, tool permission policies, server health dashboard

    Track progress: [GitHub Issues](https://github.com/tombee/conductor/issues?q=label%3Amcp)

## See Also

- [Custom Integrations](../reference/integrations/custom.md) — Build integrations without MCP
- [LLM Providers](../architecture/llm-providers.md) — Provider configuration
- [Features](../features.md) — All Conductor capabilities
