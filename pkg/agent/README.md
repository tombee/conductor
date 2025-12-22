# Agent Package

The `agent` package provides an LLM-powered agent that can use tools to accomplish tasks.

## Overview

The agent implements the **ReAct (Reasoning + Acting)** pattern:

1. **Reason**: Agent receives a task and analyzes what needs to be done
2. **Act**: Agent calls tools to gather information or perform actions
3. **Observe**: Agent receives tool results and incorporates them
4. **Repeat**: Agent continues reasoning and acting until task is complete

## Components

### Agent (`agent.go`)

Core agent loop that orchestrates LLM calls and tool execution.

**Features:**
- **Max Iterations**: Prevents infinite loops (default: 20)
- **Tool Execution**: Automatic tool discovery and execution
- **Token Tracking**: Monitors token usage across iterations
- **Streaming Support**: Optional streaming handler for real-time updates
- **Error Handling**: Graceful handling of tool and LLM failures

**Example:**

```go
// Create agent
agent := agent.NewAgent(llmProvider, toolRegistry).
    WithMaxIterations(10).
    WithStreamHandler(func(event agent.StreamEvent) {
        fmt.Printf("Event: %s\n", event.Type)
    })

// Run agent
result, err := agent.Run(ctx,
    "You are a helpful assistant that can read files and execute shell commands.",
    "Read the file at /workspace/data.txt and count the lines")

if result.Success {
    fmt.Printf("Task completed: %s\n", result.FinalResponse)
    fmt.Printf("Tools used: %d\n", len(result.ToolExecutions))
    fmt.Printf("Iterations: %d\n", result.Iterations)
    fmt.Printf("Tokens: %d\n", result.TokensUsed.TotalTokens)
}
```

### Context Manager (`context.go`)

Manages the conversation context window and token limits.

**Features:**
- **Token Estimation**: Rough estimate using 4 chars/token heuristic
- **Context Pruning**: Automatically prunes old messages at 80% capacity
- **System Message Preservation**: Always keeps system prompt
- **Content Truncation**: Truncate individual messages to fit budget

**Example:**

```go
manager := agent.NewContextManager(100000) // 100k token limit

// Check if pruning needed
if manager.ShouldPrune(messages) {
    messages = manager.Prune(messages)
}

// Get context statistics
stats := manager.GetStats(messages)
fmt.Printf("Using %d/%d tokens (%.1f%%)\n",
    stats.EstimatedTokens, stats.MaxTokens, stats.UtilizationPct)
```

## LLM Provider Interface

Agents require an LLM provider that implements:

```go
type LLMProvider interface {
    Complete(ctx context.Context, messages []Message) (*Response, error)
    Stream(ctx context.Context, messages []Message) (<-chan StreamEvent, error)
}
```

### Message Format

```go
type Message struct {
    Role       string     // "system", "user", "assistant", "tool"
    Content    string     // Message text
    ToolCalls  []ToolCall // Tools requested by assistant
    ToolCallID string     // Links tool result to its call
}
```

### Tool Call Format

```go
type ToolCall struct {
    ID        string      // Unique call identifier
    Name      string      // Tool name
    Arguments interface{} // Tool inputs (map or JSON string)
}
```

## Agent Result

The agent returns a comprehensive result with execution details:

```go
type Result struct {
    Success        bool             // Task completed successfully
    FinalResponse  string           // Agent's final text response
    ToolExecutions []ToolExecution  // Log of all tool calls
    Iterations     int              // Number of loop iterations
    TokensUsed     TokenUsage       // Total token consumption
    Duration       time.Duration    // Total execution time
    Error          string           // Error if failed
}
```

## Tool Execution Log

Each tool execution is recorded:

```go
type ToolExecution struct {
    ToolName  string                 // Tool that was executed
    Inputs    map[string]interface{} // Tool inputs
    Outputs   map[string]interface{} // Tool outputs
    Success   bool                   // Whether tool succeeded
    Error     string                 // Error if failed
    Duration  time.Duration          // Tool execution time
}
```

## Phase 1 Status

**Implemented:**
- ✅ Basic agent loop with LLM and tool integration
- ✅ Max iterations limit to prevent infinite loops
- ✅ Tool execution with error handling
- ✅ Token usage tracking across iterations
- ✅ Context manager with pruning
- ✅ Comprehensive result tracking

**Testing:**
- ✅ Agent loop: 63.8% coverage
- ✅ Core agent flow tested
- ✅ Tool execution tested
- ✅ Max iterations tested

**Phase 1 Limitations:**
- Streaming support is defined but not implemented
- Context pruning uses simple heuristic (not tiktoken)
- Tool argument parsing is basic (Phase 1 passthrough)
- No support for parallel tool calls
- No conversation history persistence

**Future Enhancements:**
- Streaming LLM responses with tool execution
- Accurate tokenization (tiktoken or equivalent)
- Structured tool argument parsing (JSON Schema)
- Parallel tool execution
- Conversation persistence and resumption
- Memory system (short-term, long-term)
- Multi-agent coordination
- Planning and reflection capabilities
