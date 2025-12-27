# Structured Logging

The `internal/log` package provides structured logging using Go's standard library `log/slog` package (Go 1.22+).

## Features

- **JSON and Text formats**: Machine-parseable JSON or human-readable text output
- **Configurable log levels**: debug, info, warn, error
- **Environment-based configuration**: LOG_LEVEL, LOG_FORMAT, LOG_SOURCE
- **Correlation ID support**: For cross-process tracing
- **Request/Response logging**: Middleware for RPC operations
- **Zero external dependencies**: Uses only Go stdlib

## Usage

### Basic Usage

```go
import "github.com/tombee/conductor/internal/log"

// Create logger from environment variables
logger := log.New(log.FromEnv())

// Log at different levels
logger.Debug("debug message", "key", "value")
logger.Info("info message", "key", "value")
logger.Warn("warning message", "key", "value")
logger.Error("error message", "key", "value")
```

### Custom Configuration

```go
cfg := &log.Config{
    Level:     "debug",
    Format:    log.FormatJSON,
    Output:    os.Stdout,
    AddSource: true,
}

logger := log.New(cfg)
```

### Correlation IDs

```go
logger := log.New(log.FromEnv())

// Add correlation ID for tracing
loggerWithID := log.WithCorrelationID(logger, "correlation-123")
loggerWithID.Info("processing request")
```

### Component Logging

```go
logger := log.New(log.FromEnv())

// Add component identifier
rpcLogger := log.WithComponent(logger, "rpc")
rpcLogger.Info("server started", "port", 9876)
```

### RPC Middleware

```go
import "github.com/tombee/conductor/internal/log"

logger := log.New(log.FromEnv())
middleware := log.NewRPCMiddleware(logger)

req := &log.RPCRequest{
    MessageType:   "execute_tool",
    CorrelationID: "correlation-123",
    RequestID:     "request-456",
    RemoteAddr:    "127.0.0.1:54321",
}

// Automatically logs request and response
err := middleware.Handler(req, func() error {
    // Process request
    return nil
})
```

## Environment Variables

- **LOG_LEVEL**: Set log level (debug, info, warn, error). Default: `info`
- **LOG_FORMAT**: Set output format (json, text). Default: `json`
- **LOG_SOURCE**: Set to `1` to include source file/line information. Default: `0`

## Examples

### JSON Output (default)

```bash
LOG_LEVEL=debug go run main.go
```

Output:
```json
{"time":"2025-12-22T10:30:00.000Z","level":"INFO","msg":"server started","port":9876}
{"time":"2025-12-22T10:30:01.000Z","level":"DEBUG","msg":"received request","correlation_id":"abc-123"}
```

### Text Output

```bash
LOG_FORMAT=text go run main.go
```

Output:
```
time=2025-12-22T10:30:00.000Z level=INFO msg="server started" port=9876
time=2025-12-22T10:30:01.000Z level=DEBUG msg="received request" correlation_id=abc-123
```

## Testing

Run tests:
```bash
go test ./internal/log/...
```

Run tests with coverage:
```bash
go test -cover ./internal/log/...
```

Run benchmarks:
```bash
go test -bench=. -benchmem ./internal/log/
```

## Performance

Benchmarks on Apple M1:
- JSON format: ~650 ns/op, 308 B/op, 0 allocs/op
- Text format: ~730 ns/op, 328 B/op, 0 allocs/op

## Design Decisions

### Why slog?

- **Standard library**: No external dependencies, guaranteed compatibility
- **Structured by design**: Built for JSON output and structured attributes
- **Performance**: Zero allocations for structured logging
- **Context support**: Built-in support for context propagation

### Why JSON as default?

- **Machine parsing**: Easy to parse and aggregate in log management systems
- **Structured data**: Preserves types and nested data
- **Query-friendly**: Can be queried with jq, grep, and log aggregators

### Correlation IDs

Correlation IDs enable tracing across:
- RPC requests and responses
- LLM provider calls
- Workflow state transitions
- Tool executions

This makes debugging distributed operations significantly easier.

## Integration

The logging package is used throughout Conduct:

- **RPC Server**: Logs connection lifecycle, requests, responses
- **LLM Providers**: Logs API calls, failures, latencies
- **Workflow Executor**: Logs state transitions, events
- **Tools**: Logs tool execution, errors

All logs include correlation IDs when available for end-to-end tracing.
