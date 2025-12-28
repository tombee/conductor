# Your First Workflow

Build a complete workflow from scratch in 5 minutes.

## What You'll Build

A workflow that generates personalized greetings and optionally translates them.

## Create the File

```bash
mkdir my-workflows
cd my-workflows
touch greeting.yaml
```

## Write the Workflow

```yaml
name: greeting
description: Generate personalized greetings

inputs:
  - name: user_name
    type: string
    required: true
    description: Name of the person to greet

  - name: language
    type: string
    required: false
    default: "English"
    description: Language for the greeting

steps:
  - id: generate
    type: llm
    model: fast
    prompt: |
      Generate a warm, friendly greeting for someone named {{.inputs.user_name}}.
      Keep it to 2-3 sentences. Be encouraging and welcoming.

  - id: translate
    type: llm
    model: fast
    condition: 'inputs.language != "English"'
    prompt: |
      Translate this greeting to {{.inputs.language}}:
      {{.steps.generate.response}}

  - id: save
    file.write:
      path: "greeting.txt"
      content: |
        {{if .steps.translate.response}}
        {{.steps.translate.response}}
        {{else}}
        {{.steps.generate.response}}
        {{end}}

outputs:
  - name: greeting
    value: |
      {{if .steps.translate.response}}
      {{.steps.translate.response}}
      {{else}}
      {{.steps.generate.response}}
      {{end}}
```

## Run It

```bash
conductor run greeting.yaml
```

You'll be prompted for inputs:
```
user_name: Alex
language: (press enter for English)
```

**Output:**
```
Hey Alex! It's wonderful to meet you. I hope you're having a great day
and are ready for something exciting. Welcome!

[workflow complete]
```

## Understanding the Workflow

### Metadata
```yaml
name: greeting
description: Generate personalized greetings
```

Identifies the workflow and explains what it does.

### Inputs
```yaml
inputs:
  - name: user_name
    type: string
    required: true
```

Parameters the workflow accepts. Use `{{.inputs.user_name}}` to reference them.

### Steps

**LLM Step:**
```yaml
- id: generate
  type: llm
  model: fast
  prompt: "Generate a greeting for {{.inputs.user_name}}"
```

Sends a prompt to an AI model. Access the response with `{{.steps.generate.response}}`.

**Conditional Step:**
```yaml
- id: translate
  condition: 'inputs.language != "English"'
  type: llm
  prompt: "Translate..."
```

Only runs if the condition is true.

**Tool Step:**
```yaml
- id: save
  file.write:
    path: "greeting.txt"
    content: "{{.steps.generate.response}}"
```

Writes the greeting to a file.

### Outputs
```yaml
outputs:
  - name: greeting
    value: "{{.steps.generate.response}}"
```

Values returned from the workflow.

## Key Concepts

**Template Variables:**
- `{{.inputs.name}}` — Input values
- `{{.steps.id.response}}` — LLM step responses
- `{{.steps.id.output}}` — Tool step outputs

**Model Tiers:**
- `fast` — Quick, low cost
- `balanced` — Default for most tasks
- `powerful` — Complex reasoning

**Step Types:**
- `llm` — Call AI models
- `parallel` — Run steps concurrently
- Tool operations — `file.read`, `shell.run`, `http.get`, etc.

## Try These Variations

**Add another input:**
```yaml
inputs:
  - name: tone
    type: string
    enum: ["professional", "casual", "enthusiastic"]
    default: "casual"
```

**Add parallel steps:**
```yaml
- id: greetings
  type: parallel
  steps:
    - id: formal
      type: llm
      prompt: "Generate a formal greeting..."
    - id: casual
      type: llm
      prompt: "Generate a casual greeting..."
```

**Add error handling:**
```yaml
- id: translate
  type: llm
  prompt: "Translate..."
  on_error:
    strategy: ignore
```

## What's Next?

- [Building Workflows](../building-workflows/) — Patterns and best practices
- [Error Handling](../building-workflows/error-handling.md) — Build resilient workflows
- [Examples](../examples/) — Real-world workflows to learn from
- [Workflow Schema](../reference/workflow-schema.md) — Complete reference

## Common Issues

**"workflow validation failed"**

Check YAML indentation (use spaces, not tabs):
```bash
conductor validate greeting.yaml
```

**"provider not configured"**

Ensure Claude Code is installed:
```bash
claude --version
```

Or configure another provider in your config file.

**Template not working**

Use correct syntax:
- Correct: `{{.inputs.user_name}}`
- Wrong: `{{user_name}}` or `${inputs.user_name}`
