# Configuration Reference

Complete reference for all Conductor configuration options.

## Overview

Conductor can be configured via YAML configuration file or environment variables. Environment variables take precedence over file-based configuration.

**Default config location:** `~/.config/conductor/config.yaml`

**Set custom location:** `export CONDUCTOR_CONFIG=/path/to/config.yaml`

## Configuration File Structure

```conductor
# Server configuration
server:
  port: 9876
  shutdown_timeout: 5s

# Authentication configuration
auth:
  token_length: 32

# Logging configuration
log:
  level: info
  format: json
  add_source: false

# LLM provider configuration
llm:
  default_provider: claude-code
  request_timeout: 5s
  max_retries: 3
  retry_backoff_base: 100ms

# Provider configurations
providers:
  claude-code:
    type: claude-code

# Controller configuration
controller:
  auto_start: false
  socket_path: ~/.config/conductor/conductor.sock
```

---

## Server Configuration

Configuration for the Conductor server (when running as a controller).

### server.port

**Type:** `integer`
**Default:** `9876`

Port for the controller server. To use a different port:

```conductor
server:
  port: 8080
```

If the port is already in use, the controller will fail immediately with a clear error message indicating the port number and suggesting diagnostic commands (`lsof -i :PORT` or `ss -tlnp`) to identify the conflicting service.

### server.shutdown_timeout

**Type:** `duration`
**Default:** `5s`
**Environment:** `SERVER_SHUTDOWN_TIMEOUT`

Maximum duration to wait for graceful shutdown.

```conductor
server:
  shutdown_timeout: 5s
```

---

## Authentication Configuration

Security settings for controller authentication.

### auth.token_length

**Type:** `integer`
**Default:** `32`
**Environment:** `AUTH_TOKEN_LENGTH`

Length of generated auth tokens in bytes.

```conductor
auth:
  token_length: 32
```

---

## Logging Configuration

Controls logging behavior and output format.

### log.level

**Type:** `string`
**Values:** `debug`, `info`, `warn`, `warning`, `error`
**Default:** `info`
**Environment:** `LOG_LEVEL`

Minimum log level to output.

- `debug`: Detailed diagnostic information
- `info`: General informational messages
- `warn`/`warning`: Warning messages
- `error`: Error conditions

```conductor
log:
  level: info
```

### log.format

**Type:** `string`
**Values:** `json`, `text`
**Default:** `json`
**Environment:** `LOG_FORMAT`

Output format for log messages.

- `json`: Structured JSON logs (recommended for production)
- `text`: Human-readable text logs

```conductor
log:
  format: json
```

### log.add_source

**Type:** `boolean`
**Default:** `false`
**Environment:** `LOG_SOURCE` (set to `1` or `true` to enable)

Add source file and line information to logs.

```conductor
log:
  add_source: false
```

---

## LLM Configuration

Global settings for LLM provider interactions.

### llm.default_provider

**Type:** `string`
**Values:** Provider names (e.g., `claude-code`, `anthropic`, `openai`, `ollama`)
**Default:** `claude-code`
**Environment:** `LLM_DEFAULT_PROVIDER` or `CONDUCTOR_PROVIDER`

Default LLM provider to use when not specified in workflow.

```conductor
llm:
  default_provider: claude-code
```

### llm.request_timeout

**Type:** `duration`
**Default:** `5s`
**Environment:** `LLM_REQUEST_TIMEOUT`

Maximum duration for LLM requests.

```conductor
llm:
  request_timeout: 5s
```

### llm.max_retries

**Type:** `integer`
**Default:** `3`
**Environment:** `LLM_MAX_RETRIES`

Maximum number of retry attempts for failed requests.

```conductor
llm:
  max_retries: 3
```

### llm.retry_backoff_base

**Type:** `duration`
**Default:** `100ms`
**Environment:** `LLM_RETRY_BACKOFF_BASE`

Base duration for exponential backoff retries.

```conductor
llm:
  retry_backoff_base: 100ms
```

With `max_retries: 3` and `retry_backoff_base: 100ms`, retries occur at: 100ms, 200ms, 400ms.

---

## Provider Configuration

Individual provider settings.

### providers

**Type:** `map` of provider configurations

Each provider has a unique name and type-specific configuration.

**Supported vs Experimental Providers:**

Conductor officially supports **Claude Code CLI** (`claude-code`) in this release. Other provider types are experimental and may work but are not officially tested or supported.

To enable experimental providers in interactive mode, set the environment variable:
```bash
export CONDUCTOR_ALL_PROVIDERS=1
```

When using experimental providers via the `--type` flag or manual configuration, you will see a warning message. The workflow will still execute, but use experimental providers at your own risk.

```conductor
providers:
  claudecode:
    type: claudecode  # Officially supported

  anthropic:
    type: anthropic   # Experimental
    api_key: ${ANTHROPIC_API_KEY}

  openai:
    type: openai      # Experimental
    api_key: ${OPENAI_API_KEY}

  ollama:
    type: ollama      # Experimental
    base_url: http://localhost:11434
```

### Provider Types

#### claudecode

**Description:** Claude Code CLI provider

**Configuration:**
```conductor
providers:
  claudecode:
    type: claudecode
```

No additional configuration required. Uses the `claude` CLI under the hood.

#### anthropic

**Description:** Anthropic API provider

**Configuration:**
```conductor
providers:
  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
```

**Required:**
- `api_key`: Anthropic API key (can reference environment variable)

**Environment variable:** `ANTHROPIC_API_KEY`

#### openai

**Description:** OpenAI API provider

**Configuration:**
```conductor
providers:
  openai:
    type: openai
    api_key: ${OPENAI_API_KEY}
```

**Required:**
- `api_key`: OpenAI API key (can reference environment variable)

**Environment variable:** `OPENAI_API_KEY`

#### ollama

**Description:** Ollama local models provider

**Configuration:**
```conductor
providers:
  ollama:
    type: ollama
    base_url: http://localhost:11434
```

**Optional:**
- `base_url`: Ollama server URL (default: `http://localhost:11434`)

---

## Controller Configuration

Settings for the Conductor controller.

### controller.auto_start

**Type:** `boolean`
**Default:** `false`

Automatically start the controller when running `conductor run` if not already running.

```conductor
controller:
  auto_start: false
```

### controller.socket_path

**Type:** `string`
**Default:** `~/.config/conductor/conductor.sock`
**Environment:** `CONDUCTOR_CONTROLLER_SOCKET`

Path to the controller Unix socket.

```conductor
controller:
  socket_path: ~/.config/conductor/conductor.sock
```

### controller.force_insecure

**Type:** `boolean`
**Default:** `false`
**CLI Flag:** `--force-insecure`

Explicitly acknowledge running with insecure configuration. When set, security warnings about disabled authentication or TLS are suppressed. **For development/testing environments only.**

```conductor
controller:
  force_insecure: false  # Not recommended for production
```

> **Warning:** Never use `force_insecure: true` in production. This flag is intended only for local development or testing scenarios where you understand the security implications.

---

## Controller Authentication

Security settings for controller API authentication. **Authentication is enabled by default** as a secure default.

### controller_auth.enabled

**Type:** `boolean`
**Default:** `true`

Enable API authentication for the controller. When enabled, all API requests must include a valid authentication token.

```conductor
controller_auth:
  enabled: true  # Secure by default
```

When authentication is disabled and the controller is accessible over the network, a security warning is logged at startup.

### controller_auth.allow_unix_socket

**Type:** `boolean`
**Default:** `true`

Allow unauthenticated access via Unix socket. This is convenient for local development while maintaining security for network access.

```conductor
controller_auth:
  allow_unix_socket: true
```

---

## Observability Storage Retention

Settings for how long observability data is retained.

### controller.observability.storage.retention.trace_days

**Type:** `integer`
**Default:** `7`

Number of days to retain trace data. Must be a positive integer when observability is enabled.

### controller.observability.storage.retention.event_days

**Type:** `integer`
**Default:** `30`

Number of days to retain event data. Must be a positive integer when observability is enabled.

### controller.observability.storage.retention.aggregate_days

**Type:** `integer`
**Default:** `90`

Number of days to retain aggregate metrics. Must be a positive integer when observability is enabled.

```conductor
controller:
  observability:
    enabled: true
    storage:
      retention:
        trace_days: 7
        event_days: 30
        aggregate_days: 90
```

---

## Environment Variables

All configuration options can be set via environment variables. Environment variables take precedence over file-based configuration.

### General

| Variable | Description |
|----------|-------------|
| `CONDUCTOR_CONFIG` | Path to config file |
| `CONDUCTOR_PROVIDER` | Default provider name (alias for `LLM_DEFAULT_PROVIDER`) |
| `CONDUCTOR_ALL_PROVIDERS` | Enable all providers in interactive mode (set to `1`) |
| `CONDUCTOR_WORKSPACE` | Default workspace path |
| `CONDUCTOR_PROFILE` | Profile selection for workspace |
| `CONDUCTOR_DEBUG` | Enable debug mode with source info (`1` or `true`) |
| `CONDUCTOR_LOG_LEVEL` | Log level (takes precedence over `LOG_LEVEL`) |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) |
| `LOG_FORMAT` | Log format (`json`, `text`) |
| `LOG_SOURCE` | Add source info to logs (`1` or `true`) |
| `NO_COLOR` | Disable colored output |
| `CONDUCTOR_NON_INTERACTIVE` | Disable interactive prompts (`true`) |
| `CONDUCTOR_ACCESSIBLE` | Enable accessibility mode (`1`) |

### Server

| Variable | Description |
|----------|-------------|
| `SERVER_SHUTDOWN_TIMEOUT` | Server shutdown timeout |

### Authentication

| Variable | Description |
|----------|-------------|
| `AUTH_TOKEN_LENGTH` | Auth token length in bytes |
| `CONDUCTOR_API_KEY` | API key for controller authentication |
| `CONDUCTOR_API_TOKEN` | API token for CLI authentication |

### LLM

| Variable | Description |
|----------|-------------|
| `LLM_DEFAULT_PROVIDER` | Default provider name |
| `LLM_REQUEST_TIMEOUT` | Request timeout duration |
| `LLM_MAX_RETRIES` | Maximum retry attempts |
| `LLM_RETRY_BACKOFF_BASE` | Backoff base duration |
| `LLM_FAILOVER_PROVIDERS` | Comma-separated list of failover providers |
| `LLM_CIRCUIT_BREAKER_THRESHOLD` | Number of failures before circuit opens |
| `LLM_CIRCUIT_BREAKER_TIMEOUT` | Duration before circuit breaker resets |

### Provider API Keys

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `GITHUB_TOKEN` | GitHub token (standard) |
| `CONDUCTOR_GITHUB_TOKEN` | GitHub token (Conductor-specific alternative) |

### Controller

| Variable | Description |
|----------|-------------|
| `CONDUCTOR_SOCKET` | Path to controller socket (CLI side) |
| `CONDUCTOR_LISTEN_SOCKET` | Path to controller listen socket (controller side) |
| `CONDUCTOR_TCP_ADDR` | Controller TCP address |
| `CONDUCTOR_DAEMON_URL` | Controller URL for CLI connections |
| `CONDUCTOR_DAEMON_AUTO_START` | Auto-start controller (`1` or `true`) |
| `CONDUCTOR_PID_FILE` | Controller PID file location |
| `CONDUCTOR_DATA_DIR` | Controller data directory |
| `CONDUCTOR_WORKFLOWS_DIR` | Workflows directory |
| `CONDUCTOR_DAEMON_LOG_LEVEL` | Controller log level |
| `CONDUCTOR_DAEMON_LOG_FORMAT` | Controller log format (`json`, `text`) |
| `CONDUCTOR_MAX_CONCURRENT_RUNS` | Maximum concurrent workflow runs |
| `CONDUCTOR_DEFAULT_TIMEOUT` | Default workflow timeout |
| `CONDUCTOR_SHUTDOWN_TIMEOUT` | Controller shutdown timeout |
| `CONDUCTOR_DRAIN_TIMEOUT` | Controller drain timeout |
| `CONDUCTOR_CHECKPOINTS_ENABLED` | Enable workflow checkpoints (`1` or `true`) |

### Public API

| Variable | Description |
|----------|-------------|
| `CONDUCTOR_PUBLIC_API_ENABLED` | Enable public API (`1` or `true`) |
| `CONDUCTOR_PUBLIC_API_TCP` | Public API TCP address |

### Security

| Variable | Description |
|----------|-------------|
| `CONDUCTOR_MASTER_KEY` | Master key for secrets encryption |
| `CONDUCTOR_TRACE_KEY` | Encryption key for trace storage (32-byte base64 or passphrase) |
| `CONDUCTOR_ALLOWED_PATHS` | Colon-separated list of allowed paths for MCP server file access |

### Integrations

| Variable | Description |
|----------|-------------|
| `SLACK_BOT_TOKEN` | Slack bot token for Slack integration |
| `PAGERDUTY_TOKEN` | PagerDuty token for incident management |
| `DATADOG_API_KEY` | Datadog API key |
| `DATADOG_APP_KEY` | Datadog application key |
| `DATADOG_SITE` | Datadog site (defaults to `datadoghq.com`) |
| `JIRA_EMAIL` | Jira account email |
| `JIRA_API_TOKEN` | Jira API token |
| `JIRA_BASE_URL` | Jira instance base URL |

---

## Variable Substitution

Configuration files support environment variable substitution using `${VARIABLE_NAME}` syntax:

```conductor
providers:
  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
```

At runtime, `${ANTHROPIC_API_KEY}` will be replaced with the value of the `ANTHROPIC_API_KEY` environment variable.

---

## Duration Format

Duration values use Go's duration format:

- `ns` - nanoseconds
- `us` or `µs` - microseconds
- `ms` - milliseconds
- `s` - seconds
- `m` - minutes
- `h` - hours

Examples:
- `100ms` - 100 milliseconds
- `5s` - 5 seconds
- `1m30s` - 1 minute 30 seconds
- `2h` - 2 hours

---

## Example Configurations

### Minimal Configuration

```conductor
llm:
  default_provider: claude-code

providers:
  claude-code:
    type: claude-code
```

### Production Configuration

```conductor
server:
  port: 9876
  shutdown_timeout: 30s

log:
  level: info
  format: json
  add_source: false

llm:
  default_provider: anthropic
  request_timeout: 30s
  max_retries: 5

providers:
  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}

  openai:
    type: openai
    api_key: ${OPENAI_API_KEY}

controller:
  auto_start: true
  # Authentication is enabled by default - no need to specify

controller_auth:
  enabled: true  # Secure by default
  allow_unix_socket: true
```

### Development Configuration

```conductor
log:
  level: debug
  format: text
  add_source: true

llm:
  default_provider: claude-code
  request_timeout: 60s

providers:
  claude-code:
    type: claude-code

controller:
  auto_start: true
```

---

## Configuration Management

### View Current Configuration

```bash
conductor config show
```

### Show Config File Path

```bash
conductor config path
```

### Edit Configuration

```bash
conductor config edit
```

### Validate Configuration

```bash
conductor health
```

---

## Security Best Practices

1. **Never commit API keys** to version control
2. **Use environment variables** for sensitive values
3. **Restrict file permissions** on config file:
   ```bash
   chmod 600 ~/.config/conductor/config.yaml
   ```
4. **Use credential managers** for API keys when possible
5. **Rotate API keys** regularly
6. **Enable rate limiting** in production environments
7. **Keep authentication enabled** - the controller enables auth by default; only disable for local development
8. **Use TLS for remote access** - if exposing the controller over TCP, configure TLS
9. **Review security warnings** - the controller logs warnings at startup for insecure configurations
10. **Never use `--force-insecure` in production** - this flag suppresses security warnings and should only be used for testing

---

## Next Steps

- [CLI Reference](cli.md) - Command-line interface documentation
- [Workflow Schema Reference](workflow-schema.md) - Complete YAML workflow reference
- [API Reference](api.md) - Go package documentation
- [Getting Started](../getting-started/) - Installation and first workflow
