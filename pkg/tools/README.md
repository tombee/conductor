# Tools Package

The `tools` package provides a registry system for workflow tools that can be used by LLM agents and workflow steps.

## Components

### Tool Registry (`registry.go`)

Central registry for tool discovery and execution:
- **Registration**: Register tools by name with validation
- **Discovery**: List available tools and get their schemas
- **Execution**: Execute tools with input validation
- **Tool Descriptors**: Export tool information for LLM function calling

### Tool Interface

All tools implement this interface:

```go
type Tool interface {
    Name() string
    Description() string
    Schema() *Schema
    Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error)
}
```

### Schema System

JSON Schema-based input/output validation:
- **Parameter Schema**: Define expected input parameters
- **Property Types**: string, number, boolean, object, array
- **Required Fields**: Mark required vs optional inputs
- **Validation**: Automatic input validation before execution

## Built-in Tools

### File Tool (`builtin/file.go`)

Read and write files on the local filesystem.

**Operations:**
- `read`: Read file contents
- `write`: Write content to file (creates parent directories)

**Configuration:**
- `WithMaxFileSize(bytes)`: Limit maximum file size
- `WithAllowedPaths(paths)`: Restrict to specific directories

**Example:**

```go
tool := builtin.NewFileTool().
    WithMaxFileSize(10 * 1024 * 1024).
    WithAllowedPaths([]string{"/workspace"})

result, err := tool.Execute(ctx, map[string]interface{}{
    "operation": "read",
    "path": "/workspace/file.txt",
})
```

### Shell Tool (`builtin/shell.go`)

Execute shell commands in a sandboxed environment.

**Features:**
- 30-second default timeout
- SIGTERM/SIGKILL process cleanup
- Captures stdout, stderr, exit code
- Optional command allowlist

**Configuration:**
- `WithTimeout(duration)`: Set command timeout
- `WithWorkingDir(dir)`: Set working directory
- `WithAllowedCommands(commands)`: Restrict allowed commands

**Example:**

```go
tool := builtin.NewShellTool().
    WithTimeout(5 * time.Second).
    WithAllowedCommands([]string{"ls", "cat", "grep"})

result, err := tool.Execute(ctx, map[string]interface{}{
    "command": "ls",
    "args": []interface{}{"-la"},
})
```

### HTTP Tool (`builtin/http.go`)

Make HTTP requests to external APIs.

**Features:**
- Supports GET, POST, PUT, DELETE, etc.
- Custom headers
- Request/response body handling
- Optional host allowlist

**Configuration:**
- `WithTimeout(duration)`: Set request timeout
- `WithAllowedHosts(hosts)`: Restrict allowed hosts

**Example:**

```go
tool := builtin.NewHTTPTool().
    WithTimeout(10 * time.Second).
    WithAllowedHosts([]string{"api.example.com"})

result, err := tool.Execute(ctx, map[string]interface{}{
    "method": "POST",
    "url": "https://api.example.com/data",
    "headers": map[string]interface{}{
        "Content-Type": "application/json",
    },
    "body": `{"key": "value"}`,
})
```

## Usage Example

```go
// Create registry
registry := tools.NewRegistry()

// Register builtin tools
registry.Register(builtin.NewFileTool())
registry.Register(builtin.NewShellTool())
registry.Register(builtin.NewHTTPTool())

// Execute a tool
outputs, err := registry.Execute(ctx, "file", map[string]interface{}{
    "operation": "read",
    "path": "/workspace/data.txt",
})

// Get tool descriptors for LLM function calling
descriptors := registry.GetToolDescriptors()
// Pass descriptors to LLM to enable tool use
```

## Phase 1 Status

**Implemented:**
- ✅ Tool registry with validation
- ✅ JSON Schema-based tool schemas
- ✅ Tool execution with input validation
- ✅ File tool (read/write with safety checks)
- ✅ Shell tool (command execution with timeout)
- ✅ HTTP tool (API requests with safety)

**Testing:**
- ✅ Registry: 78.9% coverage
- ✅ File tool: Comprehensive tests
- ⚠️ Shell/HTTP tools: Untested (22.4% overall builtin coverage)

**Future Enhancements:**
- Additional builtin tools (glob, grep, git, etc.)
- Tool input schema validation (full JSON Schema)
- Tool output transformation
- Tool composition (tool chains)
- Tool sandboxing (containers, VMs)
