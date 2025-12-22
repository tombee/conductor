# Creating Custom Tools

:::note[Prerequisites]
This guide is for developers who want to extend Conductor with custom tools. If you're just getting started, begin with the [Getting Started](../getting-started/) guide.

**Required knowledge:**

- Go programming (intermediate level)
- Understanding of Go interfaces
- Familiarity with JSON Schema
- Basic understanding of Conductor workflows

**Before creating custom tools:**

- Have worked with Conductor's built-in actions (file, shell, http)
- Read [Actions and Integrations Reference](../reference/integrations/index.md)
:::


Conductor provides two ways to create custom tools: **declarative tools** defined in workflow YAML and **programmatic tools** written in Go. This guide covers both approaches.

## Overview

Tools in Conductor are discrete functions that can be:

- Called by LLM agents during workflow execution
- Invoked directly in tool steps
- Shared across multiple workflows
- Validated against security policies

### When to Create Custom Tools

Create custom tools when you need to:

- Integrate with internal APIs not covered by built-in integrations
- Implement reusable logic across workflows
- Execute complex operations with structured inputs/outputs
- Provide LLM agents with custom capabilities

### Tool Types

| Type | Definition | Use Case |
|------|------------|----------|
| **Declarative HTTP** | YAML-defined HTTP request template | API calls, webhooks, simple integrations |
| **Declarative Script** | YAML-defined script execution | Shell scripts, custom commands, data processing |
| **Programmatic** | Go code implementing Tool interface | Complex logic, database access, performance-critical operations |

## Declarative Tools

Declarative tools are defined directly in your workflow YAML using the `functions` section. They're perfect for simple integrations and don't require Go programming.

### HTTP Tools

HTTP tools make HTTP requests with templated inputs.

**Example: Slack Notification Tool**

```conductor
name: slack-notifier
functions:
  send_slack_message:
    type: http
    description: Send a message to a Slack channel
    inputs:
      channel:
        type: string
        description: The Slack channel to post to
        required: true
      message:
        type: string
        description: The message text to send
        required: true
    http:
      method: POST
      url: https://slack.com/api/chat.postMessage
      headers:
        Authorization: "Bearer ${SLACK_TOKEN}"
        Content-Type: "application/json"
      body:
        channel: "{{.channel}}"
        text: "{{.message}}"
    outputs:
      response_type: object
      jq: '{ok: .ok, ts: .ts, channel: .channel}'

steps:
  - id: notify
    type: llm
    model: fast
    prompt: |
      Draft a brief status update about the deployment.
    tools:
      - send_slack_message
```

**HTTP Tool Fields:**

- `method` - HTTP method (GET, POST, PUT, DELETE, PATCH)
- `url` - Target URL (supports environment variable expansion)
- `headers` - Request headers as key-value pairs
- `body` - Request body (templated with input variables)
- `outputs.jq` - jq expression to transform the HTTP response

**Environment Variables:**

Use `${VAR_NAME}` syntax to reference environment variables:

```conductor
http:
  url: "${API_BASE_URL}/users"
  headers:
    Authorization: "Bearer ${API_TOKEN}"
```

:::caution[Security: Credentials in Declarative Tools]
Never hardcode credentials in workflow files. Always use environment variables:

- ✓ Good: `Authorization: "Bearer ${API_TOKEN}"`
- ✗ Bad: `Authorization: "Bearer sk-abc123..."`

Store credentials securely and pass them via environment variables when running Conductor.
:::


### Script Tools

Script tools execute shell scripts or commands with structured inputs.

**Example: Git Status Tool**

```conductor
functions:
  git_status:
    type: script
    description: Get the status of a git repository
    inputs:
      repo_path:
        type: string
        description: Path to the git repository
        required: true
    script:
      command: git
      args:
        - -C
        - "{{.repo_path}}"
        - status
        - --short
      env:
        GIT_CONFIG_GLOBAL: /dev/null  # Use repo config only
    outputs:
      response_type: string  # Captures stdout
```

**Script Tool Fields:**

- `command` - Command or script to execute
- `args` - Command arguments (templated with input variables)
- `script` - Full script content (alternative to command+args)
- `env` - Environment variables to set for the script
- `timeout` - Execution timeout (default: 30s)
- `outputs.response_type` - Output type (string, object, array)

**Using Script Content:**

```conductor
functions:
  process_data:
    type: script
    description: Process data with a custom script
    script:
      content: |
        #!/bin/bash
        set -euo pipefail

        # Access inputs via environment variables
        input_file="${INPUT_FILE}"

        # Process the file
        jq '.[] | select(.status == "active")' "$input_file"
      env:
        INPUT_FILE: "{{.file_path}}"
```

:::danger[Security: Script Execution Risks]
Script tools execute arbitrary commands. Follow these security practices:

- **Never execute untrusted input** - Validate and sanitize all inputs
- **Use absolute paths** - Avoid PATH manipulation attacks
- **Set restrictive env** - Minimize environment variable exposure
- **Enable sandboxing** - Use tool sandboxing profiles (see [Tool Sandboxing](../architecture/tool-sandboxing.md))

Example of unsafe pattern:
```conductor
# DANGEROUS - Don't do this!
script:
  command: bash
  args:
    - -c
    - "{{.user_input}}"  # User input executed directly
```
:::


### Input and Output Schemas

Define clear schemas for tool inputs and outputs using JSON Schema conventions.

**Input Schema Fields:**

```conductor
inputs:
  field_name:
    type: string|number|boolean|object|array
    description: Human-readable explanation
    required: true|false
    default: default_value
    enum: [value1, value2]  # Allowed values
```

**Output Transformations:**

Use jq expressions to extract and transform tool outputs:

```conductor
outputs:
  response_type: object
  jq: |
    {
      success: .ok,
      id: .data.id,
      created_at: .data.created_at,
      errors: [.errors[]? | {code: .code, message: .message}]
    }
```

**Common jq Patterns:**

```jq
# Extract single field
.data.id

# Transform array
[.items[] | {name: .name, value: .value}]

# Filter and map
[.results[] | select(.status == "active") | .id]

# Conditional extraction
if .success then .data else .error end

# Handle optional fields
{id: .id, tags: (.tags // [])}
```

## Programmatic Tools

For complex operations, create tools in Go by implementing the `tools.Tool` interface.

### Tool Interface

```go
package tools

type Tool interface {
    Name() string
    Description() string
    Schema() *Schema
    Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error)
}
```

### Example: Database Query Tool

```go
package mytools

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/tombee/conductor/pkg/tools"
)

type DatabaseTool struct {
    db *sql.DB
}

func NewDatabaseTool(db *sql.DB) *DatabaseTool {
    return &DatabaseTool{db: db}
}

func (t *DatabaseTool) Name() string {
    return "database.query"
}

func (t *DatabaseTool) Description() string {
    return "Execute a read-only SQL query against the database"
}

func (t *DatabaseTool) Schema() *tools.Schema {
    return &tools.Schema{
        Inputs: &tools.ParameterSchema{
            Type: "object",
            Properties: map[string]*tools.Property{
                "query": {
                    Type:        "string",
                    Description: "The SQL query to execute (SELECT only)",
                },
                "limit": {
                    Type:        "number",
                    Description: "Maximum number of rows to return",
                    Default:     100,
                },
            },
            Required: []string{"query"},
        },
        Outputs: &tools.ParameterSchema{
            Type:        "object",
            Description: "Query results",
            Properties: map[string]*tools.Property{
                "rows": {
                    Type:        "array",
                    Description: "Result rows as objects",
                },
                "count": {
                    Type:        "number",
                    Description: "Number of rows returned",
                },
            },
        },
    }
}

func (t *DatabaseTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
    // Extract and validate inputs
    query, ok := inputs["query"].(string)
    if !ok {
        return nil, fmt.Errorf("query must be a string")
    }

    // Security: Only allow SELECT queries
    if !isSelectQuery(query) {
        return nil, fmt.Errorf("only SELECT queries are allowed")
    }

    limit := 100
    if limitVal, ok := inputs["limit"].(float64); ok {
        limit = int(limitVal)
    }

    // Execute query with context for cancellation
    rows, err := t.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }
    defer rows.Close()

    // Convert results to map
    results, err := scanResults(rows, limit)
    if err != nil {
        return nil, fmt.Errorf("failed to scan results: %w", err)
    }

    return map[string]interface{}{
        "rows":  results,
        "count": len(results),
    }, nil
}

func isSelectQuery(query string) bool {
    // Implement query validation logic
    // This is a simplified example
    return len(query) >= 6 && query[:6] == "SELECT"
}

func scanResults(rows *sql.Rows, limit int) ([]map[string]interface{}, error) {
    // Implementation to convert sql.Rows to []map[string]interface{}
    // Truncated for brevity
    return nil, nil
}
```

### Registering Programmatic Tools

Register your custom tool with Conductor's tool registry:

```go
package main

import (
    "database/sql"
    "log"

    "github.com/tombee/conductor/pkg/tools"
    "github.com/tombee/conductor/pkg/workflow"
    mytools "myapp/tools"
)

func main() {
    // Create tool registry
    registry := tools.NewRegistry()

    // Initialize your custom tool
    db, err := sql.Open("postgres", "connection_string")
    if err != nil {
        log.Fatal(err)
    }

    dbTool := mytools.NewDatabaseTool(db)

    // Register the tool
    if err := registry.Register(dbTool); err != nil {
        log.Fatal(err)
    }

    // Create workflow executor with custom registry
    executor := workflow.NewExecutor(
        workflow.WithToolRegistry(registry),
    )

    // Run workflows that use your custom tool
    // ...
}
```

### Best Practices for Programmatic Tools

**1. Validate Inputs**

Always validate input types and values:

```go
func (t *MyTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
    // Type assertion with check
    name, ok := inputs["name"].(string)
    if !ok {
        return nil, fmt.Errorf("name must be a string")
    }

    // Range validation
    count, ok := inputs["count"].(float64)  // JSON numbers are float64
    if !ok || count < 1 || count > 1000 {
        return nil, fmt.Errorf("count must be a number between 1 and 1000")
    }

    // Enum validation
    mode, ok := inputs["mode"].(string)
    if !ok || (mode != "fast" && mode != "thorough") {
        return nil, fmt.Errorf("mode must be 'fast' or 'thorough'")
    }

    // ...
}
```

**2. Respect Context Cancellation**

Always check context for cancellation in long-running operations:

```go
func (t *MyTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
    results := []string{}

    for _, item := range items {
        // Check for cancellation
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
            // Continue processing
        }

        result, err := processItem(ctx, item)
        if err != nil {
            return nil, err
        }
        results = append(results, result)
    }

    return map[string]interface{}{"results": results}, nil
}
```

**3. Return Structured Outputs**

Return well-structured outputs that LLMs can easily parse:

```go
// Good: Structured output
return map[string]interface{}{
    "success": true,
    "user": map[string]interface{}{
        "id":    user.ID,
        "name":  user.Name,
        "email": user.Email,
    },
    "created_at": user.CreatedAt.Format(time.RFC3339),
}, nil

// Avoid: Unstructured string
return map[string]interface{}{
    "result": fmt.Sprintf("User %s created with ID %d", user.Name, user.ID),
}, nil
```

**4. Handle Errors Gracefully**

Provide actionable error messages:

```go
// Good: Specific, actionable error
if err != nil {
    return nil, fmt.Errorf("failed to connect to API at %s: %w (check network and credentials)", apiURL, err)
}

// Avoid: Generic error
if err != nil {
    return nil, err
}
```

**5. Implement Idempotency**

Make tools safe to retry:

```go
func (t *CreateResourceTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
    name := inputs["name"].(string)

    // Check if resource already exists
    existing, err := t.client.Get(ctx, name)
    if err == nil {
        // Resource exists, return it instead of creating duplicate
        return map[string]interface{}{
            "id":      existing.ID,
            "created": false,
            "message": "Resource already exists",
        }, nil
    }

    // Create new resource
    resource, err := t.client.Create(ctx, name)
    if err != nil {
        return nil, err
    }

    return map[string]interface{}{
        "id":      resource.ID,
        "created": true,
    }, nil
}
```

## Tool Security

### Security Interceptor

Implement security checks by registering an interceptor:

```go
type SecurityInterceptor struct {
    allowedTools map[string]bool
}

func (s *SecurityInterceptor) Intercept(ctx context.Context, tool tools.Tool, inputs map[string]interface{}) error {
    // Check if tool is allowed
    if !s.allowedTools[tool.Name()] {
        return fmt.Errorf("tool %s is not allowed in this context", tool.Name())
    }

    // Validate inputs don't contain malicious content
    if err := s.validateInputs(inputs); err != nil {
        return fmt.Errorf("security validation failed: %w", err)
    }

    return nil
}

func (s *SecurityInterceptor) PostExecute(ctx context.Context, tool tools.Tool, outputs map[string]interface{}, err error) {
    // Log tool execution for audit
    log.Printf("Tool %s executed with result: %v, error: %v", tool.Name(), outputs, err)
}

// Register interceptor
registry.SetInterceptor(&SecurityInterceptor{
    allowedTools: map[string]bool{
        "database.query": true,
        "api.call":       true,
    },
})
```

### Sandboxing

For script and shell tools, use sandboxing profiles to restrict operations:

```conductor
# In conductor config
sandbox:
  profile: strict  # Options: strict, air-gapped, permissive
  allowed_commands:
    - git
    - jq
    - curl
  blocked_paths:
    - /etc
    - /var
    - ~/.ssh
```

See [Tool Sandboxing](../architecture/tool-sandboxing.md) for detailed security configuration.

## Testing Custom Tools

### Unit Testing

Test your tools independently:

```go
func TestDatabaseTool_Execute(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer db.Close()

    tool := NewDatabaseTool(db)

    tests := []struct {
        name    string
        inputs  map[string]interface{}
        want    map[string]interface{}
        wantErr bool
    }{
        {
            name: "valid query",
            inputs: map[string]interface{}{
                "query": "SELECT id, name FROM users WHERE active = true",
                "limit": 10,
            },
            want: map[string]interface{}{
                "count": 2,
                "rows": []map[string]interface{}{
                    {"id": 1, "name": "Alice"},
                    {"id": 2, "name": "Bob"},
                },
            },
            wantErr: false,
        },
        {
            name: "invalid query type",
            inputs: map[string]interface{}{
                "query": "DELETE FROM users",  // Not a SELECT
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := tool.Execute(context.Background(), tt.inputs)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Execute() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Testing

Test tools in actual workflows:

```conductor
# test-workflow.yaml
name: test-database-tool
steps:
  - id: query
    type: llm
    model: fast
    prompt: List all active users
    tools:
      - database.query
```

```go
func TestDatabaseToolInWorkflow(t *testing.T) {
    registry := tools.NewRegistry()
    db := setupTestDB(t)
    defer db.Close()

    registry.Register(NewDatabaseTool(db))

    executor := workflow.NewExecutor(
        workflow.WithToolRegistry(registry),
    )

    result, err := executor.Run(context.Background(), "test-workflow.yaml", nil)
    if err != nil {
        t.Fatalf("workflow failed: %v", err)
    }

    // Assert workflow output
    // ...
}
```

## Examples

### Example: Weather API Tool

```conductor
functions:
  get_weather:
    type: http
    description: Get current weather for a location
    inputs:
      location:
        type: string
        description: City name or coordinates
        required: true
      units:
        type: string
        description: Temperature units (metric or imperial)
        default: metric
        enum: [metric, imperial]
    http:
      method: GET
      url: "https://api.openweathermap.org/data/2.5/weather"
      params:
        q: "{{.location}}"
        units: "{{.units}}"
        appid: "${OPENWEATHER_API_KEY}"
    outputs:
      response_type: object
      jq: |
        {
          location: .name,
          temperature: .main.temp,
          conditions: .weather[0].description,
          humidity: .main.humidity
        }
```

### Example: File Validation Tool

```go
type FileValidatorTool struct{}

func (t *FileValidatorTool) Name() string {
    return "file.validate"
}

func (t *FileValidatorTool) Description() string {
    return "Validate file format and content"
}

func (t *FileValidatorTool) Schema() *tools.Schema {
    return &tools.Schema{
        Inputs: &tools.ParameterSchema{
            Type: "object",
            Properties: map[string]*tools.Property{
                "path": {
                    Type:        "string",
                    Description: "Path to file to validate",
                },
                "format": {
                    Type:        "string",
                    Description: "Expected format",
                    Enum:        []interface{}{"json", "yaml", "xml", "csv"},
                },
            },
            Required: []string{"path", "format"},
        },
        Outputs: &tools.ParameterSchema{
            Type: "object",
            Properties: map[string]*tools.Property{
                "valid": {
                    Type:        "boolean",
                    Description: "Whether file is valid",
                },
                "errors": {
                    Type:        "array",
                    Description: "Validation errors if any",
                },
            },
        },
    }
}

func (t *FileValidatorTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
    path := inputs["path"].(string)
    format := inputs["format"].(string)

    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %w", err)
    }

    var errors []string

    switch format {
    case "json":
        var v interface{}
        if err := json.Unmarshal(data, &v); err != nil {
            errors = append(errors, fmt.Sprintf("invalid JSON: %v", err))
        }
    case "yaml":
        var v interface{}
        if err := yaml.Unmarshal(data, &v); err != nil {
            errors = append(errors, fmt.Sprintf("invalid YAML: %v", err))
        }
    default:
        return nil, fmt.Errorf("unsupported format: %s", format)
    }

    return map[string]interface{}{
        "valid":  len(errors) == 0,
        "errors": errors,
    }, nil
}
```

## See Also

- [Actions and Integrations Reference](../reference/integrations/index.md)
- [Workflow Schema Reference](../reference/workflow-schema.md)
- [Testing](../guides/testing.md)
