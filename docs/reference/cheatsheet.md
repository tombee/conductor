# Quick Reference Cheatsheet

Essential syntax and commands for working with Conductor workflows.

## Workflow Structure

### Minimal Workflow

```yaml
name: hello-world
steps:
  - id: greet
    type: llm
    prompt: "Say hello"
```

### Complete Workflow

```yaml
name: my-workflow
description: "What this workflow does"
version: "1.0"

inputs:
  - name: input_name
    type: string
    required: true
    default: "value"

steps:
  - id: step1
    type: llm
    model: balanced
    prompt: "Process {{.inputs.input_name}}"

outputs:
  - name: result
    type: string
    value: "{{$.step1.response}}"
```

## Template Variables

### Referencing Inputs

```yaml
# Access workflow input
{{.inputs.input_name}}

# With default value
{{.inputs.optional | default "fallback"}}

# Array input
{{index .inputs.file_list 0}}
```

### Referencing Step Outputs

```yaml
# Access step response
{{$.step_id.response}}

# Access step field
{{$.step_id.field_name}}

# Access nested field
{{$.step_id.nested.field}}
```

### Conditional Logic

```yaml
# If statement
{{if .inputs.enabled}}
  Content when true
{{end}}

# If-else
{{if eq .inputs.mode "prod"}}
  Production
{{else}}
  Development
{{end}}

# Contains check
{{if contains .inputs.tags "urgent"}}
  Priority handling
{{end}}
```

### Iteration

```yaml
# Range over array
{{range .inputs.items}}
  - {{.}}
{{end}}

# Range with index
{{range $i, $item := .inputs.items}}
  {{$i}}: {{$item}}
{{end}}
```

## Step Types

### LLM Step

```yaml
- id: analyze
  type: llm
  model: balanced  # fast, balanced, strategic
  system: "You are a helpful assistant"
  prompt: "Analyze this: {{.inputs.data}}"
  max_tokens: 1000
  temperature: 0.7
```

### Action Step

```yaml
- id: read_file
  type: action
  action: file.read
  inputs:
    path: "data.txt"
```

### Parallel Step

```yaml
- id: parallel_tasks
  type: parallel
  max_concurrency: 3
  steps:
    - id: task1
      type: llm
    - id: task2
      type: llm
```

### Foreach Step

```yaml
- id: process_items
  type: foreach
  items: "{{.inputs.file_list}}"
  steps:
    - id: process
      type: llm
      prompt: "Process {{.item}}"
```

## Connectors (Shorthand)

### File Operations

```yaml
# Read file
- id: read
  file.read: "path/to/file.txt"

# Write file
- id: write
  file.write:
    path: "output.txt"
    content: "{{$.step.response}}"

# Append to file
- id: append
  file.append:
    path: "log.txt"
    content: "Log entry"
```

### Shell Commands

```yaml
# Run shell command
- id: run
  shell.run: "ls -la"

# With array syntax
- id: run
  shell.run:
    command: ["git", "status"]
```

### HTTP Requests

```yaml
# GET request
- id: get
  http:
    url: "https://api.example.com/data"
    method: "GET"

# POST request
- id: post
  http:
    url: "https://api.example.com/create"
    method: "POST"
    body:
      key: "value"
    headers:
      Authorization: "Bearer {{.inputs.token}}"
```

### GitHub

```yaml
# Get file from repository
- id: get_file
  github.get_file:
    repo: "owner/repo"
    path: "README.md"

# Create issue
- id: create_issue
  github.create_issue:
    repo: "owner/repo"
    title: "Issue title"
    body: "Issue description"
    labels: ["bug", "high-priority"]
```

### Slack

```yaml
# Post message
- id: notify
  slack.post_message:
    channel: "#general"
    text: "Workflow completed: {{$.analyze.response}}"
```

## Error Handling

### Retry Configuration

```yaml
- id: api_call
  type: action
  action: http
  inputs:
    url: "https://api.example.com"
  retry:
    max_attempts: 3
    backoff_base: 2
    backoff_multiplier: 2.0
    max_backoff: 60
```

### Error Behavior

```yaml
# Stop workflow on error (default)
- id: critical
  on_error: fail

# Continue to next step
- id: optional
  on_error: continue

# Ignore errors silently
- id: best_effort
  on_error: ignore
```

## Conditions

### Step Conditions

```yaml
# Simple condition
- id: production_only
  condition:
    expression: 'inputs.env == "prod"'

# Contains check
- id: if_bug
  condition:
    expression: '"bug" in steps.classify.response'

# Multiple conditions (AND)
- id: complex
  condition:
    expression: 'inputs.enabled && steps.check.status == "ok"'
```

## CLI Commands

### Running Workflows

```bash
# Run workflow
conductor run workflow.yaml

# With inputs
conductor run workflow.yaml -i name="value" -i count=5

# With input file
conductor run workflow.yaml -f inputs.json

# Dry run (validate only)
conductor run workflow.yaml --dry-run
```

### Validation

```bash
# Validate workflow syntax
conductor validate workflow.yaml

# Verbose validation
conductor validate workflow.yaml --verbose
```

### Providers

```bash
# List configured providers
conductor providers list

# Test provider
conductor providers test anthropic

# Set default provider
conductor providers set-default anthropic
```

### Daemon Mode

```bash
# Start daemon
conductor daemon

# With specific config
conductor daemon --config daemon.yaml

# Background mode
conductor daemon --detach
```

### Tools

```bash
# List available tools/connectors
conductor tools list

# Get tool details
conductor tools info file.read
```

### Initialization

```bash
# Setup wizard
conductor init

# Create new workflow
conductor init my-workflow

# Create single file
conductor init --file workflow.yaml

# List templates
conductor init --list
```

## Model Tiers

| Tier | Speed | Cost | Use Case |
|------|-------|------|----------|
| `fast` | Fastest | Lowest | Simple tasks, extraction, classification |
| `balanced` | Moderate | Medium | General analysis, content generation |
| `strategic` | Slowest | Highest | Complex reasoning, critical analysis |

### Provider Mapping

| Tier | Anthropic | OpenAI |
|------|-----------|--------|
| `fast` | Claude 3 Haiku | GPT-3.5 Turbo |
| `balanced` | Claude 3.5 Sonnet | GPT-4 |
| `strategic` | Claude 3.5 Opus | GPT-4 Turbo |

## Common Patterns

### Read-Process-Write

```yaml
steps:
  - id: read
    file.read: "input.txt"
  - id: process
    type: llm
    prompt: "Process: {{$.read.content}}"
  - id: write
    file.write:
      path: "output.txt"
      content: "{{$.process.response}}"
```

### Conditional Routing

```yaml
- id: classify
  type: llm
  prompt: "Classify: {{.inputs.text}}"

- id: route_a
  condition:
    expression: '"urgent" in steps.classify.response'
  type: llm
  prompt: "Handle urgent case"

- id: route_b
  condition:
    expression: '"normal" in steps.classify.response'
  type: llm
  prompt: "Handle normal case"
```

### Map-Reduce

```yaml
# Map: Process in parallel
- id: map
  type: parallel
  items: "{{.inputs.files}}"
  steps:
    - id: process
      type: llm
      prompt: "Process {{.item}}"

# Reduce: Combine results
- id: reduce
  type: llm
  prompt: |
    Combine results:
    {{range .steps.map.results}}
    - {{.process.response}}
    {{end}}
```

## Configuration File

Location: `~/.config/conductor/config.yaml`

```yaml
# Provider configuration
providers:
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
  openai:
    api_key: "${OPENAI_API_KEY}"

# Default provider
default_provider: anthropic

# Logging
log_level: info  # debug, info, warn, error

# Execution
max_retries: 3
timeout: 120  # seconds
```

## Environment Variables

```bash
# Provider API keys
export ANTHROPIC_API_KEY="your-key"
export OPENAI_API_KEY="your-key"

# Config file location
export CONDUCTOR_CONFIG="/path/to/config.yaml"

# Log level
export CONDUCTOR_LOG_LEVEL="debug"

# Disable TLS verification (dev only)
export CONDUCTOR_SKIP_TLS_VERIFY="true"
```

## Response Formats

### Structured JSON Output

```yaml
- id: extract
  type: llm
  prompt: "Extract user data from: {{.inputs.text}}"
  response_format:
    type: json_schema
    schema:
      type: object
      properties:
        name: { type: string }
        email: { type: string }
        age: { type: integer }
      required: ["name", "email"]
```

## Common Issues

### Template Variable Not Found

```yaml
# Wrong: Missing $ prefix
prompt: "{{.step1.response}}"

# Right: Use $ for step references
prompt: "{{$.step1.response}}"
```

### Indentation Errors

```yaml
# Wrong: Inconsistent spaces
steps:
  - id: step1
      type: llm  # Extra spaces

# Right: Consistent 2-space indentation
steps:
  - id: step1
    type: llm
```

### Missing Required Fields

```yaml
# Wrong: Missing required 'name'
steps:
  - id: greet
    type: llm

# Right: Include 'name' at top level
name: my-workflow
steps:
  - id: greet
    type: llm
```

## Further Reading

- [Workflow Schema Reference](workflow-schema.md) - Complete field documentation
- [CLI Reference](cli.md) - All CLI commands and flags
- [Workflow Patterns](../building-workflows/patterns.md) - Workflow development guide
- [Examples](../examples/index.md) - Real-world workflow examples
