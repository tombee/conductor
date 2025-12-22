# Integration Testing Infrastructure

This document describes the integration testing infrastructure implemented for testing real components.

## Overview

The integration test infrastructure provides:
- Cost tracking with per-test and suite budget enforcement
- Test configuration from environment variables  
- Cleanup management for resource tracking
- Retry helpers with exponential backoff
- Common test fixtures and utilities

## Build Tags

Integration tests use build tags for selective execution:

```bash
# Basic integration tests (memory backend, LLM tests with API keys)
go test -tags=integration ./...

# Postgres tests via testcontainers (nightly)
go test -tags=integration,postgres ./...

# Full API coverage tests (nightly)
go test -tags=integration,nightly ./...
```

## LLM Provider Tests

Location: `pkg/llm/providers/*_integration_test.go`

Tests real API calls with cost tracking:
- **Anthropic**: Completion, streaming, tool calling, error handling
- **OpenAI**: Completion, streaming, tool calling (when implemented)
- **Ollama**: Local provider testing (no API key needed)

All tests skip automatically when API keys are not available.

## Database Tests

Location: `internal/controller/backend/memory/integration_test.go`

Tests real backend operations:
- Full run lifecycle (CRUD)
- Checkpoint persistence
- Concurrent access
- Run filtering

## Cost Tracking

Cost tracking prevents runaway expenses during integration tests:

```go
tracker := integration.NewCostTracker()
tracker.SetTestBudget(0.50)  // $0.50 per test
tracker.SetSuiteBudget(25.0) // $25 total

// After each API call
if err := tracker.Record(usage, modelInfo); err != nil {
    t.Fatal(err) // Budget exceeded
}
```

## Environment Configuration

Tests load configuration from environment variables:

- `ANTHROPIC_API_KEY` - For Anthropic provider tests
- `OPENAI_API_KEY` - For OpenAI provider tests
- `OLLAMA_URL` - For Ollama tests (defaults to http://localhost:11434)
- `POSTGRES_URL` - For Postgres integration tests

## Test Fixtures

Common fixtures in `internal/testing/integration/fixtures.go`:

```go
// Simple completion request
req := integration.SimpleCompletionRequest("fast", "Hello")

// Streaming request
req := integration.StreamingCompletionRequest("balanced", "Count to 3")

// Tool calling request
tools := []llm.Tool{integration.CalculatorTool()}
req := integration.ToolCallingRequest("strategic", "What is 15 * 7?", tools)
```

## Retry Logic

Integration tests use retry logic for transient failures:

```go
err := integration.Retry(ctx, func() error {
    return makeAPICall()
}, integration.DefaultRetryConfig())
```

Automatically retries on:
- Network timeouts
- HTTP 429 (rate limit)
- HTTP 503 (service unavailable)
- HTTP 500 (server error)

Does NOT retry on:
- HTTP 401/403 (authentication)
- HTTP 404 (not found)
- Context cancellation

## Cleanup Management

Automatic cleanup of test resources:

```go
cleanup := integration.NewCleanupManager(t)
cleanup.Add("database connection", dbConn.Close)
cleanup.Add("temp file", func() error { return os.Remove(tmpFile) })
// Cleanup runs automatically via t.Cleanup()
```

## Future Work

Phases 4 and 5 from the spec remain to be implemented:

### Phase 4: HTTP Pipeline Integration Tests
- Real HTTP server on random port
- Full request/response cycle testing
- JSON serialization validation
- SSE/streaming endpoint tests
- Concurrent request handling

### Phase 5: MCP Server Integration Tests
- Real MCP server process spawning
- Tool listing and calling with real IPC
- Crash recovery tests
- Resource cleanup verification

## CI Integration

Integration tests should be run in CI with secrets configured:

```yaml
# .github/workflows/test.yml
- name: Run integration tests
  env:
    ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
  run: go test -tags=integration ./...
```

Nightly builds should run full suite including Postgres and Tier 3 LLM tests.
