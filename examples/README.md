# Conductor Workflow Examples

This directory contains example workflows demonstrating how to use Conductor for various automation scenarios.

## Available Examples

### [Write Song](./write-song/)

Generate songs with lyrics and chord symbols. The LLM picks an authentic song structure for the genre and uses chords diatonic to the specified key.

**Use Cases:**
- Creative writing with musical structure
- Learning chord progressions by genre
- Quick songwriting inspiration

**Key Features:**
- Genre-authentic structures (12-bar blues, verse-chorus, AABA, etc.)
- Configurable musical key (defaults to C Major)
- Proper chord progressions for each genre

[View Example →](./write-song/)

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

### [IaC Review](./iac-review/)

Analyze Infrastructure as Code changes (Terraform, Pulumi, CDK) to produce risk assessments and operator-friendly summaries.

**Use Cases:**
- Pre-deployment risk assessment
- Change approval workflows
- On-call documentation for infrastructure changes

**Key Features:**
- Multi-tool support (Terraform, Pulumi, CDK)
- Risk scoring with GO/NO-GO recommendations
- Anomaly detection for unexpected changes
- Operator-friendly summaries (no IaC jargon)
- Environment-aware risk thresholds

[View Example →](./iac-review/)

### [Security Audit](./security-audit/)

Security analysis workflow that reviews code for common vulnerabilities.

**Use Cases:**
- Pre-commit security validation
- Security review for pull requests
- OWASP vulnerability detection

**Key Features:**
- Common vulnerability pattern detection
- Severity-based findings
- Remediation suggestions

[View Example →](./security-audit/)

### [Slack Integration](./slack-integration/)

Demonstrate Slack-style workflow outputs with formatted messages.

**Use Cases:**
- Notification workflows
- Alert formatting
- Team communication automation

**Key Features:**
- Structured message formatting
- Status-based messaging
- Integration with notification channels

[View Example →](./slack-integration/)

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
    engine := workflow.NewExecutor()

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

inputs:
  - name: input_param
    type: string
    required: true
    description: What this input is for

steps:
  - id: step_1
    name: First Step
    model: fast
    system: "You are an expert at..."
    prompt: "Analyze: {{.inputs.input_param}}"
    timeout: 30s

outputs:
  - name: result
    value: "{{.steps.step_1.response}}"
    description: The workflow output
```

### Step Types

- **llm**: Make LLM API calls (default step type)
  ```yaml
  - id: analyze
    model: fast|balanced|powerful
    system: "System prompt"
    prompt: "User prompt with {{.inputs.variable}}"
  ```

- **shell**: Execute shell commands
  ```yaml
  - id: run_command
    shell.run: echo "Hello World"
  ```

- **file**: Read/write files
  ```yaml
  - id: read_config
    file.read: ./config.json
  ```

- **parallel**: Concurrent execution
  ```yaml
  - id: reviews
    steps:
      - id: security
        prompt: "Review for security issues..."
      - id: performance
        prompt: "Review for performance..."
    max_concurrency: 3
  ```

- **condition**: Skip steps conditionally using `when`
  ```yaml
  - id: optional_step
    when: '"feature" in inputs.features'
    prompt: "..."
  ```

### Template Variables

Access inputs and previous step outputs using Go template syntax:

```yaml
prompt: |
  Original input: {{.inputs.input_name}}
  Previous step result: {{.steps.step_id.response}}
  Conditional: {{if .inputs.context}}Context: {{.inputs.context}}{{end}}
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

| Tier | Use Case | Cost |
|------|----------|------|
| fast | Quick classification, extraction | $ |
| balanced | Most workflows, analysis | $$ |
| powerful | Complex reasoning, synthesis | $$$ |

## Best Practices

### 1. Use Fast Models for Parallel Steps

When running multiple steps in parallel (like code review personas), use `model: fast` to keep costs reasonable:

```yaml
- id: security_review
  model: fast  # Parallel step, use fast model
  prompt: "..."
```

### 2. Use Balanced/Powerful for Synthesis

Use more powerful models for final consolidation steps that require nuanced understanding:

```yaml
- id: consolidate
  model: balanced  # Or powerful for critical decisions
  prompt: "..."
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
