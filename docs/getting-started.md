# Getting Started with Conduct

Conductor is a workflow orchestration library and runtime for building AI-powered automation. This guide will help you get started with Conductor as a standalone tool.

## Installation

### From Source

```bash
git clone https://github.com/tombee/conductor.git
cd conduct
go install ./cmd/conduct
```

### Using go install

```bash
go install github.com/tombee/conductor/cmd/conduct@latest
```

Verify installation:

```bash
conductor --version
```

## Configuration

Create a configuration file at `~/.config/conductor/config.yaml`:

```yaml
# LLM provider configuration
providers:
  anthropic:
    api_key: your-api-key-here
    default_model: claude-3-5-sonnet-20241022

# Storage configuration
storage:
  type: sqlite
  path: ~/.config/conductor/conduct.db

# Logging configuration
logging:
  level: info
  format: json
```

Alternatively, use environment variables:

```bash
export ANTHROPIC_API_KEY=your-api-key-here
```

## Your First Workflow

Create a simple workflow file `hello.yaml`:

```yaml
name: hello-world
description: A simple greeting workflow
version: "1.0"

inputs:
  - name: name
    type: string
    required: true
    description: Name to greet

steps:
  - id: greet
    name: Generate Greeting
    type: llm
    action: anthropic.complete
    inputs:
      model: fast
      system: "You are a friendly assistant that creates personalized greetings."
      prompt: "Create a warm, personalized greeting for {{.name}}"
    timeout: 30

outputs:
  - name: greeting
    type: string
    value: $.greet.content
    description: The personalized greeting
```

Run the workflow:

```bash
conductor run hello.yaml --input name="Alice"
```

You should see output like:

```
[INFO] Starting workflow: hello-world
[INFO] Step: greet (llm)
Hello Alice! It's wonderful to meet you. I hope you're having a fantastic day!
[INFO] Workflow completed successfully
[INFO] Outputs:
  greeting: Hello Alice! It's wonderful to meet you. I hope you're having a fantastic day!
```

## Understanding Workflows

A Conductor workflow consists of:

- **Inputs**: Parameters the workflow accepts
- **Steps**: Sequential actions to perform
- **Outputs**: Results extracted from step outputs

### Step Types

**LLM Steps** - Make LLM API calls:

```yaml
- id: analyze
  type: llm
  action: anthropic.complete
  inputs:
    model: balanced
    system: "You are an expert analyst."
    prompt: "Analyze: {{.input_data}}"
```

**Action Steps** - Execute tools:

```yaml
- id: read_file
  type: action
  action: file.read
  inputs:
    path: "{{.file_path}}"
```

**Condition Steps** - Branch based on conditions:

```yaml
- id: check_result
  type: condition
  condition:
    expression: $.analyze.success == true
    then_steps: ["success_handler"]
    else_steps: ["error_handler"]
```

### Model Tiers

Conductor uses model tiers to abstract away provider-specific model names:

| Tier | Use Case | Claude Model | Cost |
|------|----------|--------------|------|
| `fast` | Quick classification, simple tasks | Claude 3.5 Haiku | $ |
| `balanced` | Most workflows, analysis | Claude 3.5 Sonnet | $$ |
| `strategic` | Complex reasoning, synthesis | Claude 3 Opus | $$$ |

### Template Variables

Access inputs and step outputs using Go template syntax:

```yaml
prompt: |
  User input: {{.user_input}}
  Previous result: {{$.previous_step.content}}
  {{if .optional_context}}Context: {{.optional_context}}{{end}}
```

## Example: File Analysis Workflow

Create `analyze-file.yaml`:

```yaml
name: analyze-file
description: Analyze a text file and extract key insights
version: "1.0"

inputs:
  - name: file_path
    type: string
    required: true
    description: Path to file to analyze

steps:
  - id: read_file
    name: Read File
    type: action
    action: file.read
    inputs:
      path: "{{.file_path}}"
    timeout: 10

  - id: analyze
    name: Analyze Content
    type: llm
    action: anthropic.complete
    inputs:
      model: balanced
      system: |
        You are a content analyst. Analyze the provided text and extract:
        - Main topics (3-5 bullet points)
        - Key insights
        - Suggested actions
      prompt: |
        File: {{.file_path}}

        Content:
        {{$.read_file.content}}
    timeout: 45

outputs:
  - name: analysis
    type: string
    value: $.analyze.content
    description: Analysis results
```

Run it:

```bash
conductor run analyze-file.yaml --input file_path="README.md"
```

## Error Handling

Add retry logic and error handling:

```yaml
steps:
  - id: api_call
    type: action
    action: http.get
    inputs:
      url: "https://api.example.com/data"
    on_error:
      strategy: retry
    retry:
      max_attempts: 3
      backoff_base: 2
      backoff_multiplier: 2.0
    timeout: 30
```

Error strategies:

- `fail` - Stop workflow immediately (default)
- `ignore` - Continue despite errors
- `retry` - Retry with exponential backoff
- `fallback` - Execute a fallback step

## Workflow Output

Get JSON output for automation:

```bash
conductor run workflow.yaml --output-json > result.json
```

Get specific output values:

```bash
conductor run workflow.yaml --output-format=value --output-key=greeting
```

## Next Steps

Now that you understand the basics:

1. **Explore Examples**: See [examples/](../examples/) for real-world workflows
   - [code-review](../examples/code-review/) - Multi-agent code review
   - [issue-triage](../examples/issue-triage/) - Automatic issue classification

2. **Learn the API**: Read [API Reference](./api-reference.md) to embed Conductor in your Go application

3. **Embed in Your Project**: Follow [Embedding Guide](./embedding.md) to integrate Conductor into your codebase

4. **Understand Architecture**: Review [Architecture](./architecture.md) to learn how Conductor works internally

## Common Tasks

### List Available Tools

```bash
conductor tools list
```

### Validate a Workflow

```bash
conductor validate workflow.yaml
```

### View Workflow History

```bash
conductor history
conductor history --workflow hello-world
```

### Check Token Usage and Costs

```bash
conductor costs --workflow hello-world
conductor costs --date 2024-01-15
```

## Troubleshooting

**"Provider not configured"**
- Ensure your API key is set in config.yaml or environment variables
- Check that the provider name matches (e.g., "anthropic", not "claude")

**"Workflow validation failed"**
- Verify YAML syntax is correct
- Ensure all required inputs are defined
- Check that step IDs are unique

**"Max iterations reached"**
- Agent workflows may hit the iteration limit (default: 20)
- Increase max_iterations in agent configuration
- Simplify the task or break it into smaller workflows

**High token usage**
- Use `fast` model tier for simple tasks
- Review system prompts for unnecessary verbosity
- Consider caching common responses

## Getting Help

- **Documentation**: [docs/](../docs/)
- **Examples**: [examples/](../examples/)
- **Issues**: [GitHub Issues](https://github.com/tombee/conductor/issues)
- **Discussions**: [GitHub Discussions](https://github.com/tombee/conductor/discussions)
