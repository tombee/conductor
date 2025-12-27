# Configuration Management

The `config` package provides centralized configuration management for the Conductor server with support for environment variables and YAML file-based configuration.

## Features

- **Environment variable configuration**: All settings can be configured via environment variables
- **File-based configuration**: Optional YAML file for complex configurations
- **Sensible defaults**: Works out-of-the-box with no configuration
- **Validation on startup**: Catches configuration errors early
- **Typed configuration**: Type-safe access to all settings

## Usage

### Basic Usage (Environment Variables Only)

```go
import "github.com/tombee/conductor/internal/config"

// Load configuration from environment variables
cfg, err := config.Load("")
if err != nil {
    log.Fatal(err)
}

// Access configuration
fmt.Printf("Server port: %d\n", cfg.Server.Port)
fmt.Printf("Log level: %s\n", cfg.Log.Level)
```

### With YAML File

```go
// Load from file with environment variable overrides
cfg, err := config.Load("/path/to/config.yaml")
if err != nil {
    log.Fatal(err)
}
```

Or use the `CONDUCTOR_CONFIG` environment variable:

```bash
export CONDUCTOR_CONFIG=/path/to/config.yaml
./conduct
```

### YAML Configuration File

See `config.example.yaml` in the project root for a complete example.

```yaml
server:
  port: 9876
  health_check_interval: 500ms
  shutdown_timeout: 5s

log:
  level: info
  format: json
```

## Configuration Sections

### Server Configuration

Controls RPC server behavior.

| Field | Type | Default | Environment Variable | Description |
|-------|------|---------|---------------------|-------------|
| `port` | `int` | `9876` | - | Port to bind to |
| `health_check_interval` | `time.Duration` | `500ms` | `SERVER_HEALTH_CHECK_INTERVAL` | Health check polling interval |
| `shutdown_timeout` | `time.Duration` | `5s` | `SERVER_SHUTDOWN_TIMEOUT` | Graceful shutdown timeout |
| `read_timeout` | `time.Duration` | `10s` | `SERVER_READ_TIMEOUT` | Request read timeout |

### Auth Configuration

Controls authentication and rate limiting.

| Field | Type | Default | Environment Variable | Description |
|-------|------|---------|---------------------|-------------|
| `token_length` | `int` | `32` | `AUTH_TOKEN_LENGTH` | Auth token length in bytes (min: 16) |
| `rate_limit_max_attempts` | `int` | `5` | `AUTH_RATE_LIMIT_MAX_ATTEMPTS` | Max failed auth attempts |
| `rate_limit_window` | `time.Duration` | `1m` | `AUTH_RATE_LIMIT_WINDOW` | Rate limit time window |
| `rate_limit_lockout` | `time.Duration` | `60s` | `AUTH_RATE_LIMIT_LOCKOUT` | Lockout duration |

### Log Configuration

Controls logging output.

| Field | Type | Default | Environment Variable | Description |
|-------|------|---------|---------------------|-------------|
| `level` | `string` | `info` | `LOG_LEVEL` | Log level (debug, info, warn, error) |
| `format` | `string` | `json` | `LOG_FORMAT` | Log format (json, text) |
| `add_source` | `bool` | `false` | `LOG_SOURCE` | Add source file/line info |

### LLM Configuration

Controls LLM provider settings (Phase 1b).

| Field | Type | Default | Environment Variable | Description |
|-------|------|---------|---------------------|-------------|
| `default_provider` | `string` | `anthropic` | `LLM_DEFAULT_PROVIDER` | Default provider (anthropic, openai, ollama) |
| `request_timeout` | `time.Duration` | `5s` | `LLM_REQUEST_TIMEOUT` | Request timeout |
| `max_retries` | `int` | `3` | `LLM_MAX_RETRIES` | Max retry attempts |
| `retry_backoff_base` | `time.Duration` | `100ms` | `LLM_RETRY_BACKOFF_BASE` | Exponential backoff base |
| `connection_pool_size` | `int` | `10` | `LLM_CONNECTION_POOL_SIZE` | HTTP connection pool size |
| `connection_idle_timeout` | `time.Duration` | `30s` | `LLM_CONNECTION_IDLE_TIMEOUT` | Connection idle timeout |
| `trace_retention_days` | `int` | `7` | `LLM_TRACE_RETENTION_DAYS` | Request trace retention |

## Validation Rules

The configuration is validated on load with the following rules:

- Server ports must be in range 1024-65535
- Port range start must be <= end
- All durations must be positive
- Token length must be >= 16 bytes
- Rate limit values must be positive
- Log level must be one of: debug, info, warn, warning, error
- Log format must be one of: json, text
- LLM default provider must reference a configured provider name

## Environment Variable Priority

Environment variables always take precedence over file-based configuration:

1. Environment variables (highest priority)
2. YAML file values
3. Default values (lowest priority)

## Examples

### Development Setup

```bash
export LOG_LEVEL=debug
export LOG_FORMAT=text
./conduct
```

### Production Setup with YAML

```yaml
# /etc/conductor/config.yaml
server:
  port: 8000
  shutdown_timeout: 30s

log:
  level: warn
  format: json
  add_source: false

llm:
  default_provider: anthropic
  request_timeout: 10s
  max_retries: 5
```

```bash
export CONDUCTOR_CONFIG=/etc/conductor/config.yaml
./conduct
```

### Override Specific Settings

```bash
# Use config file but override log level
export CONDUCTOR_CONFIG=/etc/conductor/config.yaml
export LOG_LEVEL=debug
./conduct
```

## Testing

The config package has comprehensive tests covering:

- Default values
- Environment variable loading
- YAML file loading
- Environment variable overrides
- Validation rules
- Error cases

Run tests with:

```bash
go test ./internal/config/... -v
```

Check coverage:

```bash
go test ./internal/config/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```
