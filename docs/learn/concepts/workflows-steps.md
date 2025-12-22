# Workflows and Steps

Learn how workflows are structured and how steps execute.

---

## What is a Workflow?

A workflow is a YAML file that defines a series of operations. Think of it as a recipe: inputs go in, steps execute in order, outputs come out.

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

## Workflow Metadata

### Required Fields

- **`name`** — Unique identifier (lowercase, hyphens)
- **`steps`** — Array of operations to perform

### Optional Fields

- **`description`** — Human-readable summary
- **`version`** — Semantic version (e.g., "1.0", "2.1.3")
- **`inputs`** — Parameters the workflow accepts
- **`outputs`** — Results to return

**Example:**

```yaml
name: code-analyzer
description: Analyze code quality and suggest improvements
version: "1.2.0"
```

---

## Steps: The Building Blocks

Each step is an individual operation. Steps execute sequentially (unless wrapped in a parallel step).

### Step Types

Conductor supports three step types:

1. **`llm`** — Call a language model
2. **`tool`** — Use built-in tools (file, http, shell)
3. **`parallel`** — Run multiple steps concurrently

### Common Step Fields

Every step has:

- **`id`** — Unique identifier within the workflow
- **`type`** — Step type (`llm`, `tool`, or `parallel`)
- **`name`** (optional) — Human-readable label

**Example:**

```yaml
steps:
  - id: analyze
    name: Analyze Code Quality
    type: llm
    prompt: "Review this code..."
```

---

## LLM Steps

LLM steps call your configured language model provider (Claude, GPT, Gemini, etc.).

**Basic LLM step:**

```yaml
- id: summarize
  type: llm
  prompt: "Summarize this text: {{.inputs.text}}"
```

**With all options:**

```yaml
- id: review
  type: llm
  model: strategic              # Model tier (fast, balanced, strategic)
  system: "You are a code reviewer. Focus on security and performance."
  prompt: |
    Review this code for issues:
    {{.inputs.code}}
  temperature: 0.7              # Creativity (0.0-1.0)
  max_tokens: 2000              # Maximum response length
```

### LLM Step Fields

- **`model`** — Tier: `fast`, `balanced`, or `strategic` (see [Model Tiers](model-tiers.md))
- **`system`** — System prompt (sets behavior/persona)
- **`prompt`** — User prompt (your request)
- **`temperature`** — Randomness (0.0 = deterministic, 1.0 = creative)
- **`max_tokens`** — Maximum response length

:::tip[System vs Prompt]
Use `system` for behavior instructions ("You are a...") and `prompt` for the task ("Review this code...").
:::


---

## Tool Steps

Tool steps use built-in capabilities like file operations, HTTP requests, or shell commands.

**File tool:**

```yaml
- id: read_config
  file.read:
    path: "config.yaml"
```

**Shell tool:**

```yaml
- id: get_diff
  shell.run:
    command: ["git", "diff", "main...HEAD"]
```

**HTTP tool:**

```yaml
- id: fetch_data
  http.get:
    url: "https://api.example.com/data"
    headers:
      Authorization: "Bearer ${API_TOKEN}"
```

:::note[Connector Shorthand]
Instead of `type: tool` + `tool: file`, use the shorthand `file.read:` syntax shown above.
:::


See [Connectors](../../reference/connectors/index.md) for complete documentation.

---

## Parallel Steps

Parallel steps run multiple sub-steps concurrently. Great for running multiple AI personas or independent API calls.

**Example: Multi-persona code review**

```yaml
- id: reviews
  type: parallel
  max_concurrency: 3
  steps:
    - id: security_review
      type: llm
      model: strategic
      system: "You are a security engineer..."
      prompt: "Review for security issues: {{.inputs.code}}"

    - id: performance_review
      type: llm
      model: balanced
      system: "You are a performance engineer..."
      prompt: "Review for performance issues: {{.inputs.code}}"

    - id: style_review
      type: llm
      model: fast
      system: "You are a code quality reviewer..."
      prompt: "Review for style issues: {{.inputs.code}}"
```

### Parallel Step Fields

- **`type: parallel`** — Required
- **`steps`** — Array of steps to run concurrently
- **`max_concurrency`** — Maximum parallel steps (default: unlimited)

:::caution[Cost Awareness]
Parallel LLM steps make multiple API calls simultaneously. This speeds up workflows but increases costs proportionally.
:::


---

## Step Execution Order

Steps execute in the order defined in your YAML:

```yaml
steps:
  - id: step1       # Runs first
    type: llm
    prompt: "Analyze: {{.inputs.text}}"

  - id: step2       # Runs second (can reference step1)
    type: llm
    prompt: "Summarize: {{.steps.step1.response}}"

  - id: step3       # Runs third (can reference step1 and step2)
    type: llm
    prompt: "Combine: {{.steps.step1.response}} and {{.steps.step2.response}}"
```

**Key points:**

1. Each step waits for the previous step to complete
2. Later steps can reference outputs from earlier steps
3. Use `type: parallel` to run steps concurrently

---

## Conditional Execution

Skip steps based on runtime conditions:

```yaml
- id: security_scan
  type: llm
  condition:
    expression: 'inputs.scan_type == "full" or inputs.priority == "high"'
  prompt: "Perform security scan on {{.inputs.code}}"
```

The step runs only if the condition evaluates to `true`.

See [Flow Control](../../guides/flow-control.md) for more details on conditional execution.

---

## Best Practices

### Naming Steps

Use descriptive IDs that explain what the step does:

```yaml
# Good
- id: extract_function_names
- id: analyze_security_risks
- id: generate_summary_report

# Avoid
- id: step1
- id: llm_call
- id: do_stuff
```

### Breaking Down Complex Workflows

Keep workflows focused. If a workflow has >10 steps, consider splitting it:

```yaml
# Instead of one giant workflow:
# analyze-and-deploy.yaml (20 steps)

# Break into focused workflows:
# analyze.yaml (5 steps)
# deploy.yaml (4 steps)
```

### Reusing Workflows

Reference another workflow as a step (composition):

```yaml
- id: run_analysis
  workflow: "./analyze.yaml"
  inputs:
    code: "{{.inputs.code}}"
```

---

## Common Patterns

### Chain of Thought

Break complex tasks into multiple LLM steps:

```yaml
- id: extract_requirements
  type: llm
  prompt: "Extract requirements from: {{.inputs.spec}}"

- id: design_architecture
  type: llm
  prompt: "Design architecture for: {{.steps.extract_requirements.response}}"

- id: identify_risks
  type: llm
  prompt: "Identify risks in: {{.steps.design_architecture.response}}"
```

### Read-Process-Write

Common data processing pattern:

```yaml
- id: read
  file.read:
    path: "{{.inputs.file_path}}"

- id: process
  type: llm
  prompt: "Process this data: {{.steps.read.content}}"

- id: write
  file.write:
    path: "output.txt"
    content: "{{.steps.process.response}}"
```

### Parallel Analysis

Get multiple perspectives simultaneously:

```yaml
- id: parallel_analysis
  type: parallel
  steps:
    - id: technical_analysis
      type: llm
      prompt: "Technical review..."

    - id: business_analysis
      type: llm
      prompt: "Business review..."

    - id: security_analysis
      type: llm
      prompt: "Security review..."

- id: synthesize
  type: llm
  prompt: |
    Synthesize these analyses:
    Technical: {{.steps.parallel_analysis.technical_analysis.response}}
    Business: {{.steps.parallel_analysis.business_analysis.response}}
    Security: {{.steps.parallel_analysis.security_analysis.response}}
```

---

## What's Next?

- **[Inputs and Outputs](inputs-outputs.md)** — Pass data into and out of workflows
- **[Template Variables](template-variables.md)** — Reference inputs and step outputs
- **[Model Tiers](model-tiers.md)** — Choose the right model for each step
- **[Error Handling](error-handling.md)** — Handle failures gracefully
