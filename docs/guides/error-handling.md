# Error Handling Developer Guide

This guide explains how to handle errors effectively in Conductor, covering both developer-facing error handling patterns and workflow-level error strategies.

## Overview

Conductor provides a comprehensive error handling system with:

- **Typed Errors**: Structured error types for different failure modes
- **User-Friendly Messages**: Errors implement `UserVisibleError` for actionable suggestions
- **Error Wrapping**: Context-preserving error chains using `errors.Is()` and `errors.As()`
- **Workflow Error Strategies**: Runtime error handling (retry, fallback, ignore)

## Typed Error System

### Core Error Types

Conductor provides five core error types in `pkg/errors`:

#### ValidationError

Used for user input validation failures, malformed data, or constraint violations.

```go
import conductorerrors "github.com/tombee/conductor/pkg/errors"

func ValidateWorkflowName(name string) error {
    if name == "" {
        return &conductorerrors.ValidationError{
            Field:      "name",
            Message:    "workflow name cannot be empty",
            Suggestion: "Provide a non-empty name for the workflow",
        }
    }
    return nil
}
```

**Fields:**
- `Field`: Identifies which input field failed validation
- `Message`: Human-readable error description
- `Suggestion`: Actionable guidance for fixing the error

#### NotFoundError

Used when a requested resource does not exist.

```go
func GetWorkflow(id string) (*Workflow, error) {
    wf := store.Find(id)
    if wf == nil {
        return nil, &conductorerrors.NotFoundError{
            Resource: "workflow",
            ID:       id,
        }
    }
    return wf, nil
}
```

**Fields:**
- `Resource`: Type of resource (e.g., "workflow", "tool", "connector")
- `ID`: Identifier that was not found

#### ProviderError

Used for LLM provider failures originating from external services.

```go
func CallProvider(req Request) (*Response, error) {
    resp, err := client.Do(req)
    if err != nil {
        return nil, &conductorerrors.ProviderError{
            Provider:   "anthropic",
            StatusCode: resp.StatusCode,
            Message:    "API request failed",
            RequestID:  resp.Header.Get("X-Request-ID"),
            Suggestion: "Check API key and rate limits",
            Cause:      err,
        }
    }
    return resp, nil
}
```

**Fields:**
- `Provider`: Name of the LLM provider (e.g., "anthropic", "openai")
- `Code`: Provider-specific error code
- `StatusCode`: HTTP status code (if applicable)
- `Message`: Human-readable error message
- `Suggestion`: Actionable guidance for resolution
- `RequestID`: Correlates error with provider logs
- `Cause`: Underlying error (supports `Unwrap()`)

#### ConfigError

Used for configuration file errors, missing settings, or invalid config values.

```go
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, &conductorerrors.ConfigError{
            Key:    "config_file",
            Reason: fmt.Sprintf("cannot read config file: %s", path),
            Cause:  err,
        }
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, &conductorerrors.ConfigError{
            Key:    "config_file",
            Reason: "invalid YAML syntax",
            Cause:  err,
        }
    }

    if cfg.APIKey == "" {
        return nil, &conductorerrors.ConfigError{
            Key:    "api_key",
            Reason: "API key is required",
            Cause:  nil,
        }
    }

    return &cfg, nil
}
```

**Fields:**
- `Key`: Configuration key with the problem (e.g., "api_key", "database.host")
- `Reason`: Explains what's wrong with the configuration
- `Cause`: Underlying error (supports `Unwrap()`)

#### TimeoutError

Used when an operation exceeds its configured timeout.

```go
import "time"

func ExecuteWithTimeout(ctx context.Context, op string) error {
    start := time.Now()
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    if err := doWork(ctx); err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return &conductorerrors.TimeoutError{
                Operation: op,
                Duration:  time.Since(start),
                Cause:     err,
            }
        }
        return err
    }
    return nil
}
```

**Fields:**
- `Operation`: Describes what timed out (e.g., "LLM request", "workflow step")
- `Duration`: How long the operation ran before timing out
- `Cause`: Underlying error (supports `Unwrap()`)

### Error Helper Functions

The `pkg/errors` package provides convenience wrappers for standard library functions:

```go
import conductorerrors "github.com/tombee/conductor/pkg/errors"

// Wrap adds context to an error
err := conductorerrors.Wrap(err, "loading workflow")

// Wrapf adds formatted context
err := conductorerrors.Wrapf(err, "loading workflow %s", id)

// Is checks if an error matches a type
if conductorerrors.Is(err, &conductorerrors.NotFoundError{}) {
    // handle not found
}

// As extracts typed error from error chain
var notFoundErr *conductorerrors.NotFoundError
if conductorerrors.As(err, &notFoundErr) {
    log.Printf("Resource not found: %s/%s", notFoundErr.Resource, notFoundErr.ID)
}
```

### UserVisibleError Interface

Errors that should be displayed to end users with helpful messages implement the `UserVisibleError` interface:

```go
type UserVisibleError interface {
    error
    IsUserVisible() bool
    UserMessage() string
    Suggestion() string
}
```

**Example implementation** (from `internal/connector/errors.go`):

```go
func (e *Error) IsUserVisible() bool {
    return true
}

func (e *Error) UserMessage() string {
    return e.Message
}

func (e *Error) Suggestion() string {
    return e.SuggestText
}
```

The CLI automatically formats `UserVisibleError` instances to show suggestions:

```
Error: workflow not found: my-workflow

Suggestion: Check the workflow name with 'conductor workflows list'
```

### ErrorClassifier Interface

For programmatic error handling (retry logic, error reporting), implement the `ErrorClassifier` interface:

```go
type ErrorClassifier interface {
    error
    ErrorType() string
    IsRetryable() bool
}
```

**Example** (connector errors are classified for retry logic):

```go
func (e *Error) ErrorType() string {
    return string(e.Type)  // "rate_limited", "timeout", etc.
}

func (e *Error) IsRetryable() bool {
    switch e.Type {
    case ErrorTypeRateLimit, ErrorTypeTimeout, ErrorTypeServer, ErrorTypeConnection:
        return true
    default:
        return false
    }
}
```

## Error Handling Patterns

### Wrapping Errors with Context

Always wrap errors with context as they propagate up the call stack:

```go
func LoadWorkflow(path string) (*Workflow, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, conductorerrors.Wrapf(err, "reading workflow file %s", path)
    }

    wf, err := parseWorkflow(data)
    if err != nil {
        return nil, conductorerrors.Wrapf(err, "parsing workflow file %s", path)
    }

    return wf, nil
}
```

This creates an error chain:
```
parsing workflow file my-workflow.yaml: validation failed at $.steps[0].type: invalid type "invalid"
```

### Checking Error Types

Use `errors.Is()` for sentinel errors and `errors.As()` for typed errors:

```go
import (
    "errors"
    conductorerrors "github.com/tombee/conductor/pkg/errors"
)

func HandleError(err error) {
    // Check for specific error types
    var notFoundErr *conductorerrors.NotFoundError
    if errors.As(err, &notFoundErr) {
        log.Printf("Resource not found: %s/%s", notFoundErr.Resource, notFoundErr.ID)
        return
    }

    var validationErr *conductorerrors.ValidationError
    if errors.As(err, &validationErr) {
        log.Printf("Validation failed on %s: %s", validationErr.Field, validationErr.Message)
        if validationErr.Suggestion != "" {
            log.Printf("Suggestion: %s", validationErr.Suggestion)
        }
        return
    }

    // Handle generic error
    log.Printf("Error: %v", err)
}
```

### Preserving Error Types Through Chains

When wrapping typed errors, preserve the original error for `errors.As()`:

```go
func ProcessWorkflow(id string) error {
    wf, err := GetWorkflow(id)  // Returns *NotFoundError
    if err != nil {
        // Wrap preserves the NotFoundError type
        return conductorerrors.Wrapf(err, "processing workflow %s", id)
    }

    // Later, this can still be checked with errors.As()
    return nil
}
```

### Domain-Specific Errors

Existing domain errors (like `connector.Error` and `MCPError`) implement `UserVisibleError` for integration with CLI formatting:

**Connector Error Example:**

```go
// internal/connector/errors.go
type Error struct {
    Type        ErrorType
    Message     string
    StatusCode  int
    SuggestText string
    Cause       error
}

func (e *Error) IsUserVisible() bool { return true }
func (e *Error) UserMessage() string { return e.Message }
func (e *Error) Suggestion() string  { return e.SuggestText }
```

**MCP Error Example:**

```go
// internal/mcp/errors.go
type MCPError struct {
    Code        MCPErrorCode
    Message     string
    Suggestions []string
    Cause       error
}

func (e *MCPError) IsUserVisible() bool { return true }
func (e *MCPError) UserMessage() string { return e.Message }
func (e *MCPError) Suggestion() string {
    if len(e.Suggestions) > 0 {
        return strings.Join(e.Suggestions, "\n")
    }
    return ""
}
```

## Linting and Validation

### Wrapcheck Linter

Conductor uses the `wrapcheck` linter to ensure errors are wrapped with context:

```yaml
# .golangci.yml
linters:
  enable:
    - wrapcheck

linters-settings:
  wrapcheck:
    ignoreSigs:
      - .Errorf(
      - errors.New(
      - .Wrap(
      - .Wrapf(
    ignorePackageGlobs:
      - github.com/tombee/conductor/pkg/errors
      - github.com/tombee/conductor/internal/connector
      - github.com/tombee/conductor/internal/mcp
```

**What it checks:**
- Errors returned from external packages must be wrapped with context
- Internal errors (from `pkg/errors`, `connector`, `mcp`) can be returned directly

**Example violations:**

```go
// BAD: Returns external error without context
func LoadFile(path string) error {
    _, err := os.ReadFile(path)
    return err  // wrapcheck violation
}

// GOOD: Wraps with context
func LoadFile(path string) error {
    _, err := os.ReadFile(path)
    if err != nil {
        return conductorerrors.Wrapf(err, "loading file %s", path)
    }
    return nil
}
```

### Running Linters

```bash
# Run all linters including wrapcheck
golangci-lint run

# Run only wrapcheck
golangci-lint run --disable-all --enable wrapcheck

# Fix auto-fixable issues
golangci-lint run --fix
```

## Debug Mode

Set `CONDUCTOR_DEBUG=true` to enable enhanced error logging with file, line, and function context:

```bash
export CONDUCTOR_DEBUG=true
conductor run workflow.yaml
```

**Debug output example:**

```
ERROR: workflow execution failed
  File: pkg/workflow/executor.go:145
  Function: (*Executor).executeStep
  Error: step failed: api_call: provider anthropic error (429) [HTTP 429]: rate limit exceeded (request-id: req_abc123)
  Suggestion: Wait for rate limit window or configure rate_limit in connector
```

This helps diagnose issues by showing:
- Exact file and line number where error occurred
- Function name in the call stack
- Full error chain with context
- Actionable suggestions

## Workflow Error Strategies

Conductor provides runtime error handling strategies for workflow steps.

### Fail (Default)

Stop workflow immediately on error:

```yaml
  - id: critical_step
    http.post: "https://api.example.com/critical"
    on_error:
      strategy: fail
```

### Ignore

Continue despite errors:

```yaml
  - id: optional_notification
    http.post: "https://slack.example.com/webhook"
    on_error:
      strategy: ignore
```

### Retry

Retry with exponential backoff:

```yaml
  - id: flaky_api
    http.get: "https://flaky-api.example.com"
    retry:
      max_attempts: 3
      backoff_base: 2
      backoff_multiplier: 2.0
```

Backoff calculation: `delay = base * (multiplier ^ attempt)`

### Fallback

Execute alternative step:

```yaml
  - id: primary_api
    http.get: "https://primary.example.com/data"
    on_error:
      strategy: fallback
      fallback_step: backup_api

  - id: backup_api
    http.get: "https://backup.example.com/data"
```

### Timeout Configuration

Set maximum execution time:

```yaml
  - id: slow_api
    http.get: "https://slow-api.example.com"
    timeout: 60  # seconds
```

Recommended timeouts:
- LLM calls: 30-120 seconds
- HTTP APIs: 10-60 seconds
- File operations: 5-30 seconds

## Best Practices

### 1. Use Typed Errors for Expected Failures

Return typed errors for expected failure modes:

```go
// GOOD: Typed error for expected failure
func GetWorkflow(id string) (*Workflow, error) {
    wf := store.Find(id)
    if wf == nil {
        return nil, &conductorerrors.NotFoundError{
            Resource: "workflow",
            ID:       id,
        }
    }
    return wf, nil
}

// BAD: Generic error
func GetWorkflow(id string) (*Workflow, error) {
    wf := store.Find(id)
    if wf == nil {
        return nil, errors.New("workflow not found")
    }
    return wf, nil
}
```

### 2. Always Wrap External Errors

Wrap errors from external packages with context:

```go
// GOOD: Wrapped with context
data, err := os.ReadFile(path)
if err != nil {
    return conductorerrors.Wrapf(err, "reading config file %s", path)
}

// BAD: No context
data, err := os.ReadFile(path)
if err != nil {
    return err
}
```

### 3. Provide Actionable Suggestions

Include suggestions that users can act on:

```go
// GOOD: Actionable suggestion
return &conductorerrors.ConfigError{
    Key:    "api_key",
    Reason: "API key is required but not set",
    Cause:  nil,
}
// CLI will show: "Set ANTHROPIC_API_KEY environment variable"

// BAD: No suggestion
return &conductorerrors.ConfigError{
    Key:    "api_key",
    Reason: "missing",
}
```

### 4. Preserve Error Chains

Use typed errors that support `Unwrap()` for error chains:

```go
// GOOD: Preserves cause
return &conductorerrors.ProviderError{
    Provider: "anthropic",
    Message:  "request failed",
    Cause:    originalErr,  // Supports errors.Is/As
}

// BAD: Loses original error
return &conductorerrors.ProviderError{
    Provider: "anthropic",
    Message:  fmt.Sprintf("request failed: %v", originalErr),
}
```

### 5. Check Error Types Before Retrying

Only retry errors that are retryable:

```go
func ExecuteWithRetry(fn func() error) error {
    for attempt := 0; attempt < 3; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }

        // Don't retry validation errors
        var validationErr *conductorerrors.ValidationError
        if errors.As(err, &validationErr) {
            return err
        }

        // Retry timeouts and provider errors
        var timeoutErr *conductorerrors.TimeoutError
        var providerErr *conductorerrors.ProviderError
        if errors.As(err, &timeoutErr) || errors.As(err, &providerErr) {
            time.Sleep(backoff(attempt))
            continue
        }

        return err
    }
    return errors.New("max retries exceeded")
}
```

### 6. Use Exponential Backoff

Avoid overwhelming failing services:

```go
func backoff(attempt int) time.Duration {
    base := 2 * time.Second
    multiplier := 2.0
    return time.Duration(float64(base) * math.Pow(multiplier, float64(attempt)))
}
```

### 7. Limit Retry Attempts

- 3-5 attempts for transient failures (network, timeouts)
- 2-3 attempts for critical operations
- 0-1 attempts for validation errors (fail fast)

### 8. Set Appropriate Timeouts

Match expected operation duration:

```go
// LLM requests: longer timeout
ctx, cancel := context.WithTimeout(ctx, 120*time.Second)

// HTTP APIs: medium timeout
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)

// File operations: short timeout
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
```

## Common Patterns

### Critical Path with Retry

```yaml
  - id: save_data
    http.post: "https://db.example.com/save"
    timeout: 30
    retry:
      max_attempts: 3
      backoff_base: 2
      backoff_multiplier: 2.0
    on_error:
      strategy: fail  # Stop if all retries fail
```

### Optional Best-Effort

```yaml
  - id: send_notification
    http.post: "https://notifications.example.com"
    timeout: 5
    retry:
      max_attempts: 2
      backoff_base: 1
      backoff_multiplier: 1.5
    on_error:
      strategy: ignore
```

### Fallback Chain

```go
func GetData() ([]byte, error) {
    data, err := fetchFromPrimary()
    if err == nil {
        return data, nil
    }

    // Log primary failure
    log.Printf("Primary fetch failed: %v", err)

    // Try fallback
    data, err = fetchFromFallback()
    if err != nil {
        return nil, conductorerrors.Wrap(err, "all data sources failed")
    }

    return data, nil
}
```

## See Also

- [Error Codes Reference](../reference/error-codes.md) - Catalog of error types and codes
- [Debugging Guide](debugging.md) - Diagnose workflow issues
- [Troubleshooting Guide](../operations/troubleshooting.md) - Common problems and solutions
- [CONTRIBUTING.md](../../CONTRIBUTING.md) - Error handling conventions for contributors
