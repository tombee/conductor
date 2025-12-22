# Error Codes Reference

This reference documents all error codes used by Conductor with descriptions and resolution steps.

## CLI Exit Codes

Conductor uses specific exit codes to indicate different failure modes. These codes help scripts and automation tools determine the type of failure.

| Exit Code | Name | Description |
|-----------|------|-------------|
| 0 | `ExitSuccess` | Command completed successfully |
| 1 | `ExitExecutionFailed` | Workflow execution failed |
| 2 | `ExitInvalidWorkflow` | Invalid workflow file |
| 3 | `ExitMissingInput` | Required input missing |
| 4 | `ExitProviderError` | LLM provider error |
| 70 | `ExitMissingInputNonInteractive` | Missing inputs in non-interactive mode |

### Exit Code Examples

**Check exit code in shell scripts:**

```bash
conductor run workflow.yaml
case $? in
  0) echo "Success" ;;
  1) echo "Execution failed" ;;
  2) echo "Invalid workflow" ;;
  3) echo "Missing input" ;;
  4) echo "Provider error" ;;
  *) echo "Unknown error" ;;
esac
```

## JSON Error Codes

When using `--json` output format, Conductor returns structured error codes for programmatic error handling.

### Validation Errors (E001-E099)

| Code | Name | Description | Resolution |
|------|------|-------------|------------|
| E001 | `ErrorCodeMissingField` | Missing required field in workflow | Add the required field to your workflow YAML |
| E002 | `ErrorCodeInvalidYAML` | Invalid YAML syntax | Check YAML syntax with `conductor validate` |
| E003 | `ErrorCodeSchemaViolation` | Schema constraint violation | Review the [workflow schema reference](workflow-schema.md) |
| E004 | `ErrorCodeInvalidReference` | Invalid reference to unknown step ID | Ensure all step IDs referenced exist in the workflow |

**Example validation error:**

```json
{
  "error": {
    "code": "E003",
    "message": "validation failed at $.steps[0].type: type must be one of [llm, tool, workflow]"
  }
}
```

**Resolution steps:**

1. Run `conductor validate workflow.yaml` to see detailed validation errors
2. Check the workflow schema reference for correct field types
3. Verify all required fields are present

### Execution Errors (E100-E199)

| Code | Name | Description | Resolution |
|------|------|-------------|------------|
| E101 | `ErrorCodeProviderNotFound` | LLM provider not found | Configure the provider in `~/.conductor/config.yaml` |
| E102 | `ErrorCodeProviderTimeout` | LLM provider timeout | Increase timeout or check provider status |
| E103 | `ErrorCodeStepFailed` | Step execution failed | Check step logs for specific error details |
| E104 | `ErrorCodeWorkflowTimeout` | Workflow timeout exceeded | Increase workflow timeout or optimize steps |

**Example execution error:**

```json
{
  "error": {
    "code": "E101",
    "message": "Provider 'openai' not found",
    "suggestion": "Configure provider in config.yaml"
  }
}
```

**Resolution steps:**

1. Check provider configuration with `conductor config list`
2. Verify API keys are set correctly
3. Test provider with `conductor providers list`

### Configuration Errors (E200-E299)

| Code | Name | Description | Resolution |
|------|------|-------------|------------|
| E201 | `ErrorCodeConfigNotFound` | Configuration file not found | Run `conductor init` to create default config |
| E202 | `ErrorCodeInvalidConfig` | Invalid provider configuration | Check config syntax and required fields |
| E203 | `ErrorCodeMissingAPIKey` | Missing API key | Set environment variable or add to config |

**Example configuration error:**

```json
{
  "error": {
    "code": "E203",
    "message": "Missing API key for provider 'openai'",
    "suggestion": "Set OPENAI_API_KEY environment variable"
  }
}
```

**Resolution steps:**

1. Set API key: `export OPENAI_API_KEY=your-key-here`
2. Or add to config: `conductor config set providers.openai.api_key ${OPENAI_API_KEY}`
3. Verify with `conductor config show`

### Input Errors (E300-E399)

| Code | Name | Description | Resolution |
|------|------|-------------|------------|
| E301 | `ErrorCodeMissingInput` | Required input missing | Provide the input via CLI flag or prompt |
| E302 | `ErrorCodeInvalidInput` | Invalid input format | Check input type matches workflow schema |
| E303 | `ErrorCodeFileNotFound` | Input file not found | Verify file path exists |

**Example input error:**

```json
{
  "error": {
    "code": "E301",
    "message": "Required input 'branch' missing"
  }
}
```

**Resolution steps:**

1. Run with input flag: `conductor run workflow.yaml --input branch=main`
2. Use input file: `conductor run workflow.yaml --inputs inputs.yaml`
3. Run interactively to be prompted for inputs

### Resource Errors (E400-E499)

| Code | Name | Description | Resolution |
|------|------|-------------|------------|
| E401 | `ErrorCodeNotFound` | Resource not found | Verify the resource exists |
| E402 | `ErrorCodeInternal` | Internal error | Check logs and report issue |
| E403 | `ErrorCodeExecutionFailed` | Execution failed | Review error details and retry |

## Connector Errors

Connector operations use typed errors for consistent handling across all connectors.

### Error Types

| Type | Description | Retryable | Resolution |
|------|-------------|-----------|------------|
| `auth_error` | Authentication or authorization failure (401, 403) | No | Check authentication credentials and permissions |
| `not_found` | Resource not found (404) | No | Verify the resource exists and the path is correct |
| `validation_error` | Invalid request data (400, 422) | No | Check request inputs against operation schema |
| `rate_limited` | Rate limit exceeded (429) | Yes | Wait for rate limit window or configure rate_limit in connector |
| `server_error` | Server-side error (500+) | Yes | Retry or contact the service provider |
| `timeout` | Operation timeout | Yes | Increase timeout or check service responsiveness |
| `connection_error` | Network or DNS error | Yes | Check network connectivity and DNS resolution |
| `transform_error` | Response transform failed | No | Check jq expression syntax and response structure |
| `ssrf_blocked` | SSRF protection blocked request | No | Add host to allowed_hosts if access is intentional |
| `path_injection` | Path traversal attempt blocked | No | Remove path traversal sequences (../, %2e%2e) |

**Example connector error:**

```
ConnectorError: 429 Too Many Requests (type: rate_limited) [HTTP 429]
Suggestion: Wait for rate limit window or configure rate_limit in connector
```

**Resolution by error type:**

**Authentication Errors (`auth_error`):**

1. Verify API credentials are correct
2. Check token hasn't expired
3. Ensure account has required permissions
4. Review connector auth configuration

**Rate Limit Errors (`rate_limited`):**

1. Wait for rate limit window to reset
2. Configure rate limiting in connector:
   ```yaml
   connectors:
     github:
       rate_limit:
         requests_per_second: 1
   ```
3. Use burst limits if available
4. Implement exponential backoff

**Server Errors (`server_error`):**

1. Check service status page
2. Retry with exponential backoff
3. Contact service provider support
4. Implement circuit breaker pattern

**Transform Errors (`transform_error`):**

1. Validate jq expression syntax
2. Test expression against sample response
3. Check response structure matches expectations
4. Use simpler transformations

**SSRF Protection (`ssrf_blocked`):**

1. Verify target host is safe to access
2. Add to allowed hosts:
   ```yaml
   connectors:
     custom:
       allowed_hosts:
         - api.trusted-service.com
   ```
3. Use DNS instead of IP addresses where possible

## MCP (Model Context Protocol) Errors

MCP server management operations use specific error codes with actionable suggestions.

### Error Codes

| Code | Description | Resolution |
|------|-------------|------------|
| `NOT_FOUND` | MCP server not found | Check server name with `conductor mcp list` |
| `ALREADY_EXISTS` | Server already exists | Use different name or remove existing server |
| `ALREADY_RUNNING` | Server is already running | Check status or restart if needed |
| `NOT_RUNNING` | Server is not running | Start the server first |
| `COMMAND_NOT_FOUND` | Command executable not found | Install required runtime or use absolute path |
| `START_FAILED` | Server failed to start | Check logs and validate configuration |
| `PING_FAILED` | Server failed to respond | Verify server implements MCP protocol |
| `CONNECTION_CLOSED` | Server connection closed | Check if server crashed, review logs |
| `VALIDATION` | Invalid configuration | Fix configuration syntax |
| `CONFIG` | Configuration error | Review configuration format |
| `TIMEOUT` | Operation timeout | Increase timeout or check server responsiveness |
| `INTERNAL` | Internal error | Report issue with details |

**Example MCP error:**

```
Error: MCP server 'my-server' not found

  Suggestions:
  - Check the server name: conductor mcp list
  - Register the server: conductor mcp add my-server --command <cmd>
```

**Common MCP error resolutions:**

**Command Not Found:**

```bash
# Check command is in PATH
which npx

# Use absolute path
conductor mcp add my-server --command /usr/local/bin/npx mcp-server

# Install Node.js if missing
brew install node  # macOS
```

**Start Failed:**

```bash
# Check server logs
conductor mcp logs my-server

# Validate configuration
conductor mcp validate my-server

# Test command manually
npx @modelcontextprotocol/server-example
```

**Ping Failed:**

```bash
# Check logs for protocol errors
conductor mcp logs my-server

# Increase timeout
conductor mcp add my-server --command npx mcp-server --timeout 10

# Verify server implements MCP protocol correctly
```

## Schema Validation Errors

Schema validation errors occur when workflow outputs don't match expected schemas.

### Validation Error Structure

```json
{
  "error_code": "SCHEMA_VALIDATION_FAILED",
  "path": "$.items[0].name",
  "keyword": "type",
  "message": "expected string, got number",
  "expected_schema": {...},
  "actual_output": {...}
}
```

**Fields:**

- `path`: JSON path to the failing field (e.g., `$.category`, `$.items[0].name`)
- `keyword`: Schema keyword that failed (`type`, `required`, `enum`, etc.)
- `message`: Human-readable error description
- `expected_schema`: The schema that was expected
- `actual_output`: The actual output that failed validation

**Example validation error:**

```
validation failed at $.steps[0].output.category (type): expected string, got number
```

**Resolution steps:**

1. Review the schema definition in your workflow
2. Check the output format from the LLM or tool
3. Adjust prompt to match expected schema format
4. Use validation constraints (type, enum, pattern)
5. Test with `conductor validate workflow.yaml`

## Security Errors

Security errors occur when operations are blocked by security policies.

### Access Denied Error

```
security: access denied - tool=file, resource_type=file, resource=/etc/passwd,
action=read, profile=restricted, reason=path not in allowed list
```

**Fields:**

- `tool`: Tool name that was blocked
- `resource_type`: Type of resource (`file`, `command`, `network`)
- `resource`: Specific resource (path, command, host)
- `action`: Attempted action (`read`, `write`, `execute`, `connect`)
- `profile`: Active security profile
- `reason`: Why access was denied

**Resolution steps:**

1. Review security profile configuration
2. Add resource to allowed list if safe
3. Use less restrictive profile for development
4. Create custom security profile

**Example security profile:**

```yaml
security:
  profiles:
    custom:
      file:
        allowed_paths:
          - /home/user/project
          - /tmp
      shell:
        allowed_commands:
          - git
          - npm
      network:
        allowed_hosts:
          - api.github.com
```

:::caution[Security Best Practices]
Only add resources to allowed lists if you trust them. Security restrictions protect against malicious workflows and accidental data exposure.
:::


## Cost Limit Errors

Cost limit errors occur when workflow execution exceeds configured spending limits.

### Cost Limit Exceeded Error

```json
{
  "error": "CostLimitExceededError",
  "scope": "workflow",
  "reason": "Workflow cost $5.23 exceeds limit $5.00",
  "actual": 5.23,
  "limit": 5.00
}
```

**Resolution steps:**

1. Review cost breakdown: `conductor costs show --workflow-id <id>`
2. Increase limit in workflow or config:
   ```yaml
   limits:
     max_cost_per_workflow: 10.00
   ```
3. Optimize prompts to reduce token usage
4. Use cheaper model tiers for non-critical steps
5. Implement cost controls per step

## Troubleshooting Tips

### Enable Debug Logging

```bash
# Set log level to debug
conductor run workflow.yaml --log-level debug

# Or via environment variable
export CONDUCTOR_LOG_LEVEL=debug
conductor run workflow.yaml
```

### Check Configuration

```bash
# Show current configuration
conductor config show

# List providers
conductor providers list

# Validate workflow
conductor validate workflow.yaml
```

### Test Components Individually

```bash
# Test LLM provider
conductor providers test openai

# Test connector
conductor connectors test github

# Validate inputs
conductor run workflow.yaml --dry-run
```

### Review Logs

```bash
# Show recent workflow runs
conductor runs list

# Show specific run details
conductor runs show <run-id>

# Show MCP server logs
conductor mcp logs <server-name>
```

## See Also

- [CLI Reference](cli.md) - Complete CLI command reference
- [Workflow Schema](workflow-schema.md) - Workflow file format
- [Troubleshooting Guide](../operations/troubleshooting.md) - Common issues and solutions
- [Security Guide](../operations/security.md) - Security configuration
