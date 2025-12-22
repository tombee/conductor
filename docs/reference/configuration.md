# Configuration Reference

Complete reference for all Conductor configuration options.

## Overview

Conductor can be configured via YAML configuration file or environment variables. Environment variables take precedence over file-based configuration.

**Default config location:** `~/.config/conductor/config.yaml`

**Set custom location:** `export CONDUCTOR_CONFIG=/path/to/config.yaml`

## Configuration File Structure

```yaml
# Server configuration
server:
  port_range: [9876, 9899]
  health_check_interval: 500ms
  shutdown_timeout: 5s
  read_timeout: 10s

# Authentication configuration
auth:
  token_length: 32
  rate_limit_max_attempts: 5
  rate_limit_window: 1m
  rate_limit_lockout: 60s

# Logging configuration
log:
  level: info
  format: json
  add_source: false

# LLM provider configuration
llm:
  default_provider: anthropic
  request_timeout: 5s
  max_retries: 3
  retry_backoff_base: 100ms
  connection_pool_size: 10
  connection_idle_timeout: 30s
  trace_retention_days: 7

# Provider configurations
providers:
  claudecode:
    type: claudecode
  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}

# Daemon configuration
daemon:
  auto_start: false
  socket_path: ~/.config/conductor/conductor.sock
```

---

## Server Configuration

Configuration for the Conductor server (when running as a daemon).

### server.port_range

**Type:** `array` of integers
**Default:** `[9876, 9899]`
**Environment:** `SERVER_PORT_MIN`, `SERVER_PORT_MAX`

Port for the daemon server. To use a specific port:

```yaml
server:
  port_range: [8080, 8080]  # Use port 8080
```

The range format allows fallback to alternate ports if the primary is in use (useful during development).

### server.health_check_interval

**Type:** `duration`
**Default:** `500ms`
**Environment:** `SERVER_HEALTH_CHECK_INTERVAL`

Interval for health check polling.

```yaml
server:
  health_check_interval: 500ms
```

### server.shutdown_timeout

**Type:** `duration`
**Default:** `5s`
**Environment:** `SERVER_SHUTDOWN_TIMEOUT`

Maximum duration to wait for graceful shutdown.

```yaml
server:
  shutdown_timeout: 5s
```

### server.read_timeout

**Type:** `duration`
**Default:** `10s`
**Environment:** `SERVER_READ_TIMEOUT`

Maximum duration for reading requests.

```yaml
server:
  read_timeout: 10s
```

---

## Authentication Configuration

Security settings for daemon authentication.

### auth.token_length

**Type:** `integer`
**Default:** `32`
**Environment:** `AUTH_TOKEN_LENGTH`

Length of generated auth tokens in bytes.

```yaml
auth:
  token_length: 32
```

### auth.rate_limit_max_attempts

**Type:** `integer`
**Default:** `5`
**Environment:** `AUTH_RATE_LIMIT_MAX_ATTEMPTS`

Maximum failed authentication attempts before rate limiting kicks in.

```yaml
auth:
  rate_limit_max_attempts: 5
```

### auth.rate_limit_window

**Type:** `duration`
**Default:** `1m`
**Environment:** `AUTH_RATE_LIMIT_WINDOW`

Time window for counting failed attempts.

```yaml
auth:
  rate_limit_window: 1m
```

### auth.rate_limit_lockout

**Type:** `duration`
**Default:** `60s`
**Environment:** `AUTH_RATE_LIMIT_LOCKOUT`

Lockout duration after exceeding rate limit.

```yaml
auth:
  rate_limit_lockout: 60s
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

```yaml
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

```yaml
log:
  format: json
```

### log.add_source

**Type:** `boolean`
**Default:** `false`
**Environment:** `LOG_SOURCE` (set to `1` or `true` to enable)

Add source file and line information to logs.

```yaml
log:
  add_source: false
```

---

## LLM Configuration

Global settings for LLM provider interactions.

### llm.default_provider

**Type:** `string`
**Values:** Provider names (e.g., `anthropic`, `openai`, `ollama`, `claudecode`)
**Default:** `anthropic`
**Environment:** `LLM_DEFAULT_PROVIDER` or `CONDUCTOR_PROVIDER`

Default LLM provider to use when not specified in workflow.

```yaml
llm:
  default_provider: anthropic
```

### llm.request_timeout

**Type:** `duration`
**Default:** `5s`
**Environment:** `LLM_REQUEST_TIMEOUT`

Maximum duration for LLM requests.

```yaml
llm:
  request_timeout: 5s
```

### llm.max_retries

**Type:** `integer`
**Default:** `3`
**Environment:** `LLM_MAX_RETRIES`

Maximum number of retry attempts for failed requests.

```yaml
llm:
  max_retries: 3
```

### llm.retry_backoff_base

**Type:** `duration`
**Default:** `100ms`
**Environment:** `LLM_RETRY_BACKOFF_BASE`

Base duration for exponential backoff retries.

```yaml
llm:
  retry_backoff_base: 100ms
```

With `max_retries: 3` and `retry_backoff_base: 100ms`, retries occur at: 100ms, 200ms, 400ms.

### llm.connection_pool_size

**Type:** `integer`
**Default:** `10`
**Environment:** `LLM_CONNECTION_POOL_SIZE`

Number of HTTP connections per provider.

```yaml
llm:
  connection_pool_size: 10
```

### llm.connection_idle_timeout

**Type:** `duration`
**Default:** `30s`
**Environment:** `LLM_CONNECTION_IDLE_TIMEOUT`

Idle timeout for pooled connections.

```yaml
llm:
  connection_idle_timeout: 30s
```

### llm.trace_retention_days

**Type:** `integer`
**Default:** `7`
**Environment:** `LLM_TRACE_RETENTION_DAYS`

Number of days to retain request traces.

```yaml
llm:
  trace_retention_days: 7
```

---

## Provider Configuration

Individual provider settings.

### providers

**Type:** `map` of provider configurations

Each provider has a unique name and type-specific configuration.

```yaml
providers:
  claudecode:
    type: claudecode

  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}

  openai:
    type: openai
    api_key: ${OPENAI_API_KEY}

  ollama:
    type: ollama
    base_url: http://localhost:11434
```

### Provider Types

#### claudecode

**Description:** Claude Code CLI provider

**Configuration:**
```yaml
providers:
  claudecode:
    type: claudecode
```

No additional configuration required. Uses the `claude` CLI under the hood.

#### anthropic

**Description:** Anthropic API provider

**Configuration:**
```yaml
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
```yaml
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
```yaml
providers:
  ollama:
    type: ollama
    base_url: http://localhost:11434
```

**Optional:**
- `base_url`: Ollama server URL (default: `http://localhost:11434`)

---

## Daemon Configuration

Settings for the Conductor daemon.

### daemon.auto_start

**Type:** `boolean`
**Default:** `false`

Automatically start the daemon when running `conductor run --daemon` if not already running.

```yaml
daemon:
  auto_start: false
```

### daemon.socket_path

**Type:** `string`
**Default:** `~/.config/conductor/conductor.sock`
**Environment:** `CONDUCTOR_DAEMON_SOCKET`

Path to the daemon Unix socket.

```yaml
daemon:
  socket_path: ~/.config/conductor/conductor.sock
```

---

## Environment Variables

All configuration options can be set via environment variables. Environment variables take precedence over file-based configuration.

### General

| Variable | Description |
|----------|-------------|
| `CONDUCTOR_CONFIG` | Path to config file |
| `CONDUCTOR_PROVIDER` | Default provider name (alias for `LLM_DEFAULT_PROVIDER`) |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) |
| `LOG_FORMAT` | Log format (`json`, `text`) |
| `LOG_SOURCE` | Add source info to logs (`1` or `true`) |
| `NO_COLOR` | Disable colored output |

### Server

| Variable | Description |
|----------|-------------|
| `SERVER_PORT_MIN` | Minimum port in range |
| `SERVER_PORT_MAX` | Maximum port in range |
| `SERVER_HEALTH_CHECK_INTERVAL` | Health check interval |
| `SERVER_SHUTDOWN_TIMEOUT` | Shutdown timeout |
| `SERVER_READ_TIMEOUT` | Read timeout |

### Authentication

| Variable | Description |
|----------|-------------|
| `AUTH_TOKEN_LENGTH` | Auth token length in bytes |
| `AUTH_RATE_LIMIT_MAX_ATTEMPTS` | Max auth attempts before lockout |
| `AUTH_RATE_LIMIT_WINDOW` | Time window for counting attempts |
| `AUTH_RATE_LIMIT_LOCKOUT` | Lockout duration |

### LLM

| Variable | Description |
|----------|-------------|
| `LLM_DEFAULT_PROVIDER` | Default provider name |
| `LLM_REQUEST_TIMEOUT` | Request timeout duration |
| `LLM_MAX_RETRIES` | Maximum retry attempts |
| `LLM_RETRY_BACKOFF_BASE` | Backoff base duration |
| `LLM_CONNECTION_POOL_SIZE` | HTTP connection pool size |
| `LLM_CONNECTION_IDLE_TIMEOUT` | Idle connection timeout |
| `LLM_TRACE_RETENTION_DAYS` | Days to retain traces |

### Provider API Keys

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |

### Daemon

| Variable | Description |
|----------|-------------|
| `CONDUCTOR_DAEMON_SOCKET` | Path to daemon socket |

---

## Variable Substitution

Configuration files support environment variable substitution using `${VARIABLE_NAME}` syntax:

```yaml
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

```yaml
llm:
  default_provider: claudecode

providers:
  claudecode:
    type: claudecode
```

### Production Configuration

```yaml
server:
  port_range: [9876, 9899]
  shutdown_timeout: 30s

auth:
  rate_limit_max_attempts: 10
  rate_limit_window: 5m

log:
  level: info
  format: json
  add_source: false

llm:
  default_provider: anthropic
  request_timeout: 30s
  max_retries: 5
  connection_pool_size: 20
  trace_retention_days: 30

providers:
  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}

  openai:
    type: openai
    api_key: ${OPENAI_API_KEY}

daemon:
  auto_start: true
```

### Development Configuration

```yaml
log:
  level: debug
  format: text
  add_source: true

llm:
  default_provider: ollama
  request_timeout: 60s

providers:
  ollama:
    type: ollama
    base_url: http://localhost:11434

  claudecode:
    type: claudecode

daemon:
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
conductor doctor
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

---

## Next Steps

- [CLI Reference](cli.md) - Command-line interface documentation
- [Workflow Schema Reference](workflow-schema.md) - Complete YAML workflow reference
- [API Reference](api.md) - Go package documentation
- [Quick Start](../quick-start.md) - Get started quickly
