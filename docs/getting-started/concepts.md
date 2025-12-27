# Core Concepts

Learn the fundamental concepts behind Conductor workflows.

---

## Workflows

A workflow is a YAML file that defines a series of AI-powered operations. Think of it as a recipe: inputs go in, steps execute in order, and outputs come out.

**Minimal workflow:**

```yaml
name: hello-world
steps:
  - id: greet
    type: llm
    prompt: "Say hello to the world"
```

**Complete workflow structure:**

```yaml
name: workflow-name
description: What this workflow does
version: "1.0"

inputs:
  - name: parameter_name
    type: string
    required: true

steps:
  - id: step1
    type: llm
    model: balanced
    prompt: "Do something with {{.inputs.parameter_name}}"

outputs:
  - name: result
    value: "{{.steps.step1.response}}"
```

---

## Inputs and Outputs

### Inputs

Inputs are parameters your workflow accepts. They make workflows reusable with different data.

```yaml
inputs:
  - name: file_path
    type: string
    required: true
    description: Path to the file to analyze

  - name: format
    type: string
    required: false
    default: "markdown"
    description: Output format
```

**Providing inputs:**

```bash
# Interactive prompts
conductor run analyze.yaml

# Command-line flags
conductor run analyze.yaml -i file_path=README.md -i format=json
```

### Outputs

Outputs extract results from your workflow.

```yaml
outputs:
  - name: summary
    type: string
    value: "{{.steps.summarize.response}}"
    description: Brief summary of the analysis
```

---

## Steps

Steps are the building blocks of workflows. Each step performs one operation.

### LLM Steps

Call a language model to process text:

```yaml
- id: review
  type: llm
  model: balanced
  system: "You are a code reviewer."
  prompt: |
    Review this code for issues:
    {{.inputs.code}}
```

### Tool Steps

Use built-in capabilities like file operations or shell commands:

```yaml
# Read a file
- id: read_config
  file.read:
    path: "config.yaml"

# Run a shell command
- id: get_diff
  shell.run:
    command: ["git", "diff", "main...HEAD"]
```

### Parallel Steps

Run multiple steps concurrently:

```yaml
- id: reviews
  type: parallel
  steps:
    - id: security_review
      type: llm
      prompt: "Review for security..."

    - id: performance_review
      type: llm
      prompt: "Review for performance..."
```

---

## Template Variables

Template variables let you insert dynamic values using `{{.variable}}` syntax.

### Reference Inputs

```yaml
inputs:
  - name: user_name
    type: string

steps:
  - id: greet
    type: llm
    prompt: "Say hello to {{.inputs.user_name}}"
```

### Reference Step Outputs

```yaml
steps:
  - id: analyze
    type: llm
    prompt: "Analyze this code..."

  - id: summarize
    type: llm
    prompt: "Summarize: {{.steps.analyze.response}}"
```

### Common Output Fields

- **LLM steps:** `.response` (the model's text response)
- **File tool:** `.content` (file contents)
- **Shell tool:** `.stdout` (command output)
- **HTTP tool:** `.body` (response body)

---

## Model Tiers

Instead of hardcoding model names, use tiers for flexibility:

- **`fast`** — Quick tasks, lower cost (Claude Haiku, GPT-3.5)
- **`balanced`** — Most workflows (Claude Sonnet, GPT-4)
- **`strategic`** — Complex reasoning (Claude Opus, GPT-4 Turbo)

```yaml
- id: quick_summary
  type: llm
  model: fast
  prompt: "Summarize in one sentence..."

- id: deep_analysis
  type: llm
  model: strategic
  prompt: "Perform detailed analysis..."
```

This lets you swap providers without changing workflows.

---

## Execution Flow

Steps execute sequentially by default:

```yaml
steps:
  - id: step1       # Runs first
    type: llm
    prompt: "Analyze..."

  - id: step2       # Runs second (can use step1 output)
    type: llm
    prompt: "Summarize: {{.steps.step1.response}}"
```

Use conditional execution to skip steps:

```yaml
- id: security_scan
  type: llm
  condition:
    expression: 'inputs.scan_type == "full"'
  prompt: "Perform security scan..."
```

---

## Common Patterns

### Read-Process-Write

```yaml
- id: read
  file.read:
    path: "{{.inputs.file_path}}"

- id: process
  type: llm
  prompt: "Process this: {{.steps.read.content}}"

- id: write
  file.write:
    path: "output.txt"
    content: "{{.steps.process.response}}"
```

### Chain of Thought

Break complex tasks into steps:

```yaml
- id: extract
  type: llm
  prompt: "Extract requirements from: {{.inputs.spec}}"

- id: design
  type: llm
  prompt: "Design based on: {{.steps.extract.response}}"

- id: review
  type: llm
  prompt: "Review the design: {{.steps.design.response}}"
```

### Parallel Analysis

Get multiple perspectives simultaneously:

```yaml
- id: reviews
  type: parallel
  steps:
    - id: technical
      type: llm
      prompt: "Technical review..."

    - id: business
      type: llm
      prompt: "Business review..."

- id: synthesize
  type: llm
  prompt: |
    Combine these perspectives:
    Technical: {{.steps.reviews.technical.response}}
    Business: {{.steps.reviews.business.response}}
```

---

## Best Practices

### Descriptive Naming

```yaml
# Good
- id: extract_function_names
- id: analyze_security_risks

# Avoid
- id: step1
- id: do_stuff
```

### Keep Workflows Focused

If a workflow has more than 10 steps, consider splitting it into smaller, focused workflows.

### Use Enums for Fixed Choices

```yaml
inputs:
  - name: priority
    type: string
    enum: ["low", "medium", "high"]
```

### Provide Helpful Descriptions

```yaml
inputs:
  - name: pr_url
    type: string
    description: "GitHub PR URL (e.g., https://github.com/owner/repo/pull/123)"
```

---

## What's Next?

Now that you understand the core concepts:

1. **[Build Your First Workflow](first-workflow.md)** — Hands-on tutorial
2. **[Explore Examples](../examples/)** — See real-world workflows
3. **[Learn Flow Control](../building-workflows/flow-control.md)** — Advanced patterns
4. **[Reference Documentation](../reference/workflow-schema.md)** — Complete YAML specification
