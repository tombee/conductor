# Conductor Workflow Examples

This directory contains example workflows demonstrating how to use Conductor for various automation scenarios.

## Available Examples

### [Code Review](./code-review/)

Multi-persona AI code review workflow that analyzes changes from security, performance, and style perspectives.

**Use Cases:**
- Automated PR reviews in CI/CD
- Pre-commit validation
- Architecture decision review

**Key Features:**
- Parallel execution of three review personas
- Consolidated findings with severity levels
- Configurable model tiers for cost/quality tradeoff

[View Example →](./code-review/)

### [Issue Triage](./issue-triage/)

Intelligent issue classification that automatically labels, prioritizes, and assigns GitHub issues to appropriate teams.

**Use Cases:**
- Automatic issue labeling on creation
- Support ticket routing
- Bug/feature/question classification

**Key Features:**
- Type, priority, and sentiment analysis
- Label extraction from issue content
- Team assignment suggestions
- Markdown summary generation

[View Example →](./issue-triage/)

## Running Examples

### Command Line

```bash
# Run code review on a git diff
git diff main..feature | conductor run examples/code-review

# Run issue triage
conductor run examples/issue-triage \
  --input title="App crashes on startup" \
  --input body="Detailed description here..."

# Get JSON output for automation
conductor run examples/code-review --output-json > review.json
```

### Programmatic Usage

```go
package main

import (
    "context"
    "os"

    "github.com/tombee/conductor/pkg/workflow"
)

func main() {
    // Load workflow definition
    data, _ := os.ReadFile("examples/code-review/workflow.yaml")
    def, _ := workflow.ParseDefinition(data)

    // Create engine
    engine := workflow.NewEngine()

    // Execute workflow
    result, err := engine.Execute(context.Background(), def, map[string]interface{}{
        "diff": getDiff(),
    })

    if err != nil {
        panic(err)
    }

    // Use results
    review := result.Outputs["review"].(string)
    println(review)
}
```

## Creating Your Own Workflows

### Workflow Structure

```yaml
name: my-workflow
description: What this workflow does
version: "1.0"

inputs:
  - name: input_param
    type: string
    required: true
    description: What this input is for

steps:
  - id: step_1
    name: First Step
    type: llm
    action: anthropic.complete
    inputs:
      model: fast
      system: "You are an expert at..."
      prompt: "Analyze: {{.input_param}}"
    timeout: 30

outputs:
  - name: result
    type: string
    value: $.step_1.content
    description: The workflow output
```

### Step Types

- **llm**: Make LLM API calls
  ```yaml
  type: llm
  action: anthropic.complete
  inputs:
    model: fast|balanced|strategic
    system: "System prompt"
    prompt: "User prompt with {{.variables}}"
  ```

- **action**: Execute tools (file read/write, bash commands)
  ```yaml
  type: action
  action: read_file
  inputs:
    path: "{{.file_path}}"
  ```

- **condition**: Conditional branching
  ```yaml
  type: condition
  condition:
    expression: $.previous_step.status == "success"
    then_steps: ["success_path"]
    else_steps: ["failure_path"]
  ```

- **parallel**: Concurrent execution (Phase 2)
  ```yaml
  type: parallel
  steps:
    - {parallel step definitions}
  ```

### Template Variables

Access inputs and previous step outputs using Go template syntax:

```yaml
prompt: |
  Original input: {{.input_name}}
  Previous step result: {{$.step_id.content}}
  Conditional: {{if .context}}Context: {{.context}}{{end}}
```

### Error Handling

```yaml
on_error:
  strategy: retry|ignore|fail|fallback
  fallback_step: "error_handler_step_id"

retry:
  max_attempts: 3
  backoff_base: 2
  backoff_multiplier: 2.0
```

### Timeouts

```yaml
timeout: 30  # seconds
```

## Model Tiers

Choose model tier based on task complexity and cost requirements:

| Tier | Claude Model | Use Case | Cost |
|------|-------------|----------|------|
| fast | Haiku | Quick classification, extraction | $ |
| balanced | Sonnet | Most workflows, analysis | $$ |
| strategic | Opus | Complex reasoning, synthesis | $$$ |

## Best Practices

### 1. Use Fast Models for Parallel Steps

When running multiple steps in parallel (like code review personas), use `model: fast` to keep costs reasonable:

```yaml
- id: security_review
  type: llm
  action: anthropic.complete
  inputs:
    model: fast  # Parallel step, use fast model
```

### 2. Use Balanced/Strategic for Synthesis

Use more powerful models for final consolidation steps that require nuanced understanding:

```yaml
- id: consolidate
  type: llm
  inputs:
    model: balanced  # Or strategic for critical decisions
```

### 3. Set Appropriate Timeouts

- Simple classification: 10-20s
- Analysis tasks: 30-45s
- Complex synthesis: 45-60s

```yaml
timeout: 30  # Adjust based on expected complexity
```

### 4. Add Retry Logic for Reliability

```yaml
retry:
  max_attempts: 2
  backoff_base: 2
  backoff_multiplier: 2.0
```

### 5. Validate Inputs

Use required and type fields to ensure valid inputs:

```yaml
inputs:
  - name: code
    type: string
    required: true
    description: Code to analyze
```

### 6. Document System Prompts

Make system prompts explicit and well-documented:

```yaml
system: |
  You are a security expert. Focus on:
  - SQL injection
  - XSS vulnerabilities
  - Authentication issues

  Format findings as:
  - CRITICAL: Must fix
  - WARNING: Should investigate
```

### 7. Provide Context in Prompts

Include relevant context for better analysis:

```yaml
prompt: |
  {{if .context}}
  Context: {{.context}}
  {{end}}

  {{if .repository}}
  Repository: {{.repository}}
  {{end}}

  Analyze: {{.content}}
```

## Integration Examples

### GitHub Actions

```yaml
- name: Run Workflow
  run: |
    conductor run examples/code-review \
      --input diff="$(git diff main)" \
      --output-json > results.json
```

### Pre-commit Hook

```bash
#!/bin/bash
git diff --cached | conductor run examples/code-review
if [ $? -ne 0 ]; then
  echo "Review found issues"
  exit 1
fi
```

### CI/CD Pipeline

```bash
# In Jenkins, GitLab CI, etc.
conductor run examples/issue-triage \
  --input title="$ISSUE_TITLE" \
  --input body="$ISSUE_BODY" \
  --output-json | jq -r '.labels[]'
```

## Learn More

- [Workflow Definition Reference](../docs/workflow.md)
- [LLM Provider Configuration](../docs/llm-providers.md)
- [Getting Started Guide](../docs/getting-started.md)
- [API Reference](../docs/api-reference.md)

## Contributing Examples

We welcome contributions! To add a new example:

1. Create a directory: `examples/your-example/`
2. Add `workflow.yaml` with the workflow definition
3. Add `README.md` with:
   - Description and use cases
   - Usage examples
   - Expected output
   - Customization options
4. Test the workflow with real inputs
5. Submit a PR

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.
