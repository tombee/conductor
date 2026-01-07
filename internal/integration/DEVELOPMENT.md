# Integration Development Guide

This document describes the required process for developing new external integrations in Conductor. Following this process ensures integrations work correctly in real workflows before being released.

## Integration Architecture

All integrations follow a common pattern:

```
internal/integration/{name}/
├── integration.go       # Factory + Execute() dispatch
├── {resource}.go        # Operations grouped by resource (pages.go, issues.go)
├── errors.go            # Integration-specific error types
├── types.go             # API request/response types
└── integration_test.go  # Unit tests with mock HTTP server
```

Integrations embed `api.BaseProvider` to get common functionality:
- HTTP request execution with auth
- JSON request/response handling
- URL building with path parameters
- Required parameter validation

## Development Checklist

**Every new integration MUST complete these steps before being merged:**

### 1. Implementation

- [ ] Create integration directory under `internal/integration/{name}/`
- [ ] Implement `New{Name}Integration(config *api.ProviderConfig) (operation.Provider, error)`
- [ ] Embed `*api.BaseProvider` in your integration struct
- [ ] Implement `Execute(ctx, operation, inputs)` with switch dispatch
- [ ] Implement `Operations() []api.OperationInfo` returning all operation metadata
- [ ] Implement `OperationSchema(operation string) *api.OperationSchema` for each operation
- [ ] Add error types in `errors.go` with proper categorization
- [ ] Register in `internal/integration/registry.go`

### 2. Testing - Mock Server (Required)

- [ ] Create `integration_test.go` with mock HTTP server
- [ ] Test each operation returns expected response structure
- [ ] Test validation errors for missing required parameters
- [ ] Test error handling for API errors (401, 403, 404, 429, 500)
- [ ] Test unknown operation handling

### 3. Testing - Real API (Required)

Real API tests should be added to the integration-specific test file (not smoke_test.go) so that each integration remains a self-contained module.

- [ ] Add real API test in `internal/integration/{name}/integration_test.go`:
  ```go
  func TestRealAPI_{Name}Operations(t *testing.T) {
      token := os.Getenv("{NAME}_TOKEN")
      if token == "" {
          t.Skip("{NAME}_TOKEN not set, skipping real API test")
      }
      // Test read operations that don't modify data
      // Test write operations with cleanup or idempotent operations
  }
  ```
- [ ] Run real API tests locally with actual credentials
- [ ] Document required environment variables for testing

### 4. Testing - Workflow E2E (Required)

- [ ] Create a simple test workflow in `test/e2e/workflows/{name}-test.yaml`
- [ ] Run the workflow with `conductor run test/e2e/workflows/{name}-test.yaml`
- [ ] Verify workflow completes without errors
- [ ] Verify step outputs contain expected data

### 5. Registration Verification

- [ ] Run smoke tests: `go test ./internal/integration/... -run "TestAll" -v`
- [ ] Verify your integration appears in test output
- [ ] Run: `conductor integrations add {name} --token test` (should not error)

## Operation Naming Convention

Operations MUST follow these conventions:

- **snake_case**: `create_issue`, not `createIssue` or `CreateIssue`
- **Verb first**: `create_`, `get_`, `list_`, `update_`, `delete_`
- **Singular for single item**: `get_page`, not `get_pages`
- **Plural for collections**: `list_issues`, not `list_issue`
- **Resource last**: `create_issue`, not `issue_create`

## Operation Metadata

Every operation MUST have complete metadata:

```go
func (c *MyIntegration) Operations() []api.OperationInfo {
    return []api.OperationInfo{
        {
            Name:        "create_issue",
            Description: "Create a new issue in a project",
            Category:    "issues",
            Tags:        []string{"write"},
        },
        {
            Name:        "list_issues",
            Description: "List issues with optional filters",
            Category:    "issues",
            Tags:        []string{"read", "paginated"},
        },
    }
}
```

**Tags:**
- `read` - Operation only reads data
- `write` - Operation creates or modifies data
- `delete` - Operation deletes data (subset of write)
- `paginated` - Operation supports pagination
- `destructive` - Operation cannot be undone

## Input Validation

Validate inputs early with clear error messages:

```go
func (c *MyIntegration) createThing(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
    // 1. Validate required parameters
    if err := c.ValidateRequired(inputs, []string{"name", "project_id"}); err != nil {
        return nil, err
    }

    // 2. Extract and type-check values
    name, _ := inputs["name"].(string)
    projectID, _ := inputs["project_id"].(string)

    // 3. Validate format/constraints
    if len(name) < 1 || len(name) > 255 {
        return nil, &MyError{
            Code:    "validation_error",
            Message: "name must be between 1 and 255 characters",
        }
    }

    // 4. Continue with API call...
}
```

## Error Handling

Create integration-specific error types:

```go
// errors.go
type MyError struct {
    Code     string
    Message  string
    Category ErrorCategory
}

type ErrorCategory int

const (
    ErrorCategoryValidation ErrorCategory = iota
    ErrorCategoryAuth
    ErrorCategoryNotFound
    ErrorCategoryRateLimit
    ErrorCategoryServer
)

func (e *MyError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ParseError converts HTTP response to typed error
func ParseError(resp *transport.Response) error {
    if resp.StatusCode < 400 {
        return nil
    }
    // Parse error body and return typed error
}
```

## Running Tests

```bash
# Mock-based smoke tests (always run)
go test ./internal/integration/... -run "TestAll" -v

# Real API tests (run with credentials)
NOTION_TOKEN=secret_xxx NOTION_TEST_PAGE_ID=xxx go test ./internal/integration/... -run "TestRealAPI" -v

# Individual integration tests
go test ./internal/integration/{name}/... -v
```

## Example: Adding a New Integration

```go
// internal/integration/example/integration.go
package example

import (
    "context"
    "github.com/tombee/conductor/internal/operation"
    "github.com/tombee/conductor/internal/operation/api"
)

type ExampleIntegration struct {
    *api.BaseProvider
}

func NewExampleIntegration(config *api.ProviderConfig) (operation.Provider, error) {
    baseURL := config.BaseURL
    if baseURL == "" {
        baseURL = "https://api.example.com/v1"
    }

    return &ExampleIntegration{
        BaseProvider: api.NewBaseProvider("example", &api.ProviderConfig{
            BaseURL:   baseURL,
            Token:     config.Token,
            Transport: config.Transport,
        }),
    }, nil
}

func (c *ExampleIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
    switch operation {
    case "get_thing":
        return c.getThing(ctx, inputs)
    case "create_thing":
        return c.createThing(ctx, inputs)
    default:
        return nil, fmt.Errorf("unknown operation: %s", operation)
    }
}

func (c *ExampleIntegration) Operations() []api.OperationInfo {
    return []api.OperationInfo{
        {Name: "get_thing", Description: "Get a thing by ID", Category: "things", Tags: []string{"read"}},
        {Name: "create_thing", Description: "Create a new thing", Category: "things", Tags: []string{"write"}},
    }
}

func (c *ExampleIntegration) OperationSchema(operation string) *api.OperationSchema {
    switch operation {
    case "get_thing":
        return &api.OperationSchema{
            Description: "Get a thing by ID",
            Parameters: []api.ParameterInfo{
                {Name: "thing_id", Type: "string", Required: true, Description: "The thing ID"},
            },
        }
    case "create_thing":
        return &api.OperationSchema{
            Description: "Create a new thing",
            Parameters: []api.ParameterInfo{
                {Name: "name", Type: "string", Required: true, Description: "Thing name"},
                {Name: "description", Type: "string", Required: false, Description: "Optional description"},
            },
        }
    }
    return nil
}
```

Then register in `internal/integration/registry.go`:

```go
var BuiltinRegistry = map[string]Factory{
    // ... existing integrations
    "example": example.NewExampleIntegration,
}
```

## Common Mistakes to Avoid

1. **Not running real API tests** - Mock tests can't catch API behavior changes
2. **Skipping workflow testing** - Integration code can work but fail in workflow context
3. **Missing operation metadata** - Operations without descriptions break tooling
4. **Incomplete error handling** - All error paths should return typed, actionable errors
5. **Not registering in registry.go** - Integration exists but can't be used
6. **Hardcoding URLs** - Always allow BaseURL override for testing
