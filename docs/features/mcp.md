# MCP Server Integration

Extend LLM capabilities with Model Context Protocol (MCP) servers.

## Overview

MCP servers provide additional tools that LLMs can call during execution. Connect to MCP servers to expose domain-specific capabilities without defining them in workflows.

## Configuration

Configure MCP servers in `~/.config/conductor/config.yaml`:

```yaml
mcp:
  servers:
    filesystem:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/dir"]
    github:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_PERSONAL_ACCESS_TOKEN: ${GITHUB_TOKEN}
```

## Using MCP Tools

LLM steps automatically access registered MCP tools:

```yaml
steps:
  - id: analyze
    llm:
      model: balanced
      prompt: "Read main.go and summarize the key functions"
```

The LLM can call `filesystem:read_file` if the filesystem server is configured.

## Tool Namespacing

Tools are namespaced by server name:

- `filesystem:read_file`
- `filesystem:write_file`
- `github:create_issue`
- `github:list_pull_requests`

## Server Lifecycle

### Startup

Conductor starts MCP servers on-demand when workflows need them:

1. Server starts when first LLM step requests MCP tools
2. Health check verifies server is responding (10s timeout)
3. Server's available tools are registered

### Health Checks

Conductor monitors servers every 30 seconds. After 3 consecutive failures, the server restarts automatically.

### Shutdown

On workflow completion:

1. Wait up to 30 seconds for in-flight requests
2. Send SIGTERM
3. After 10 seconds, send SIGKILL

## Common Servers

### Filesystem

Access files from a specific directory:

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

Only paths under the specified directory are accessible.

### GitHub

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

Tools include: `create_issue`, `list_issues`, `create_pull_request`, and more.

### Custom Servers

Build your own MCP server:

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

## Security

### Source Verification

Only run servers from trusted sources:
- Official `@modelcontextprotocol/server-*` packages
- Your organization's verified code
- Third-party code you've reviewed

MCP servers execute with the same permissions as Conductor. A malicious server could access files, network, or credentials.

### Tool Scoping

Limit what tools can access:

```yaml
mcp:
  servers:
    filesystem:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp/sandbox"]
```

## Troubleshooting

### Server Fails to Start

Check command exists:

```bash
which npx
npx -y @modelcontextprotocol/server-filesystem --help
```

Check Conductor logs:

```bash
grep "MCP" /var/log/conductor/conductor.log
```

### Tool Not Available

List available tools:

```bash
conductor mcp list-tools
```

Verify the server name and tool namespace.

### Connection Timeout

Increase timeout:

```yaml
mcp:
  servers:
    slow-server:
      command: ...
      timeout: 120s
```

## Monitoring

Monitor MCP server health via:

- `/health` endpoint includes MCP server status
- Metrics: `conductor_mcp_servers_status{server="name",status="healthy|unhealthy"}`
- Logs: MCP lifecycle events logged at INFO level

Restart a server:

```bash
conductor mcp restart filesystem
```
