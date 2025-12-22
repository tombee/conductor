# Template Variables

Learn how to use `{{}}` syntax to reference inputs and step outputs in your workflows.

---

## What are Template Variables?

Template variables let you insert dynamic values into your workflow steps. They use Go template syntax with double curly braces: `{{.variable}}`.

**Basic example:**

```yaml
inputs:
  - name: user_name
    type: string

steps:
  - id: greet
    type: llm
    prompt: "Say hello to {{.inputs.user_name}}"
```

When you run this with `user_name: Alice`, the prompt becomes: `"Say hello to Alice"`

---

## Syntax Basics

### The Dot Prefix

All variable references start with a dot (`.`):

```yaml
{{.inputs.name}}        # Correct
{{inputs.name}}         # ❌ Wrong - missing dot
```

### Accessing Inputs

Use `.inputs` to reference workflow inputs:

```yaml
{{.inputs.file_path}}
{{.inputs.api_key}}
{{.inputs.enable_debug}}
```

### Accessing Step Outputs

Use `.steps.step_id.field` to reference step results:

```yaml
{{.steps.analyze.response}}          # LLM response
{{.steps.read_file.content}}          # File contents
{{.steps.get_branch.stdout}}          # Shell output
{{.steps.fetch_data.body}}            # HTTP response body
```

---

## Referencing Inputs

Inputs are accessed via `.inputs.name`:

```yaml
inputs:
  - name: source_file
    type: string
  - name: target_language
    type: string
  - name: verbose
    type: boolean

steps:
  - id: translate
    type: llm
    prompt: |
      Translate the file {{.inputs.source_file}} to {{.inputs.target_language}}.
      Verbose mode: {{.inputs.verbose}}
```

### Nested Object Access

For object-type inputs, use dot notation:

```yaml
inputs:
  - name: config
    type: object
    # Example value: {"api": {"url": "https://api.example.com", "key": "xyz"}}

steps:
  - id: call_api
    http.get:
      url: "{{.inputs.config.api.url}}"
      headers:
        Authorization: "Bearer {{.inputs.config.api.key}}"
```

### Array Access

Access array elements by index:

```yaml
inputs:
  - name: reviewers
    type: array
    # Example value: ["alice", "bob", "charlie"]

steps:
  - id: assign
    type: llm
    prompt: "Primary reviewer: {{.inputs.reviewers.0}}, backup: {{.inputs.reviewers.1}}"
```

---

## Referencing Step Outputs

Each step type produces different outputs.

### LLM Step Outputs

| Field | Description | Example |
|-------|-------------|---------|
| `.response` | Full text response from the model | `{{.steps.analyze.response}}` |
| `.model` | Model name that generated the response | `{{.steps.analyze.model}}` |
| `.tokens` | Token usage information | `{{.steps.analyze.tokens}}` |

**Example:**

```yaml
steps:
  - id: analyze
    type: llm
    prompt: "Analyze this code..."

  - id: summarize
    type: llm
    prompt: |
      Summarize this analysis:
      {{.steps.analyze.response}}
```

### File Tool Outputs

| Field | Description |
|-------|-------------|
| `.content` | File contents (for read operations) |
| `.path` | File path |
| `.size` | File size in bytes |

**Example:**

```yaml
steps:
  - id: read_config
    file.read:
      path: "config.yaml"

  - id: process
    type: llm
    prompt: "Process this config: {{.steps.read_config.content}}"
```

### Shell Tool Outputs

| Field | Description |
|-------|-------------|
| `.stdout` | Standard output from command |
| `.stderr` | Standard error output |
| `.exit_code` | Command exit code |

**Example:**

```yaml
steps:
  - id: get_diff
    shell.run:
      command: ["git", "diff", "main...HEAD"]

  - id: review
    type: llm
    prompt: |
      Review these changes:
      {{.steps.get_diff.stdout}}
```

### HTTP Tool Outputs

| Field | Description |
|-------|-------------|
| `.body` | Response body |
| `.status_code` | HTTP status code |
| `.headers` | Response headers |

**Example:**

```yaml
steps:
  - id: fetch_pr
    http.get:
      url: "https://api.github.com/repos/owner/repo/pulls/123"

  - id: analyze
    type: llm
    prompt: |
      Analyze this PR:
      {{.steps.fetch_pr.body}}
```

### Parallel Step Outputs

Access nested step outputs using path notation:

```yaml
steps:
  - id: reviews
    type: parallel
    steps:
      - id: security_review
        type: llm
        prompt: "Security review..."

      - id: performance_review
        type: llm
        prompt: "Performance review..."

  - id: combine
    type: llm
    prompt: |
      Security findings: {{.steps.reviews.security_review.response}}
      Performance findings: {{.steps.reviews.performance_review.response}}
```

---

## Template Functions

### Conditional Logic

Use `if/else` for conditional content:

```yaml
steps:
  - id: notify
    type: llm
    prompt: |
      {{if .inputs.urgent}}
      URGENT: This requires immediate attention!
      {{else}}
      Normal priority task.
      {{end}}

      Task: {{.inputs.task_description}}
```

### String Operations

Common string functions:

```yaml
# Convert to uppercase
{{.inputs.name | upper}}

# Convert to lowercase
{{.inputs.name | lower}}

# Trim whitespace
{{.inputs.text | trim}}

# Contains check
{{if contains .inputs.text "error"}}Found an error{{end}}
```

### Default Values

Provide fallback values:

```yaml
{{.inputs.optional_value | default "default_value"}}
```

**Example:**

```yaml
steps:
  - id: greet
    type: llm
    prompt: "Hello {{.inputs.name | default \"stranger\"}}"
```

---

## Common Patterns

### Multi-Line Prompts

Use the pipe `|` for multi-line strings:

```yaml
steps:
  - id: analyze
    type: llm
    prompt: |
      You are a code reviewer. Analyze the following:

      File: {{.inputs.file_path}}

      Content:
      {{.steps.read_file.content}}

      Focus areas:
      - Security vulnerabilities
      - Performance issues
      - Code style
```

### Combining Multiple Outputs

Reference multiple steps in one prompt:

```yaml
steps:
  - id: get_diff
    shell.run:
      command: ["git", "diff"]

  - id: get_commits
    shell.run:
      command: ["git", "log", "--oneline", "-5"]

  - id: analyze
    type: llm
    prompt: |
      Review this branch:

      Recent commits:
      {{.steps.get_commits.stdout}}

      Changes:
      {{.steps.get_diff.stdout}}
```

### Escaping Special Characters

If your data contains template delimiters, quote them:

```yaml
# This works even if content has {{}} in it
content: "{{.steps.read_file.content}}"
```

---

## Best Practices

### Always Use Quotes

Quote template variables in YAML to avoid parsing issues:

```yaml
# Good
path: "{{.inputs.file_path}}"
content: "{{.steps.read.content}}"

# Can cause issues
path: {{.inputs.file_path}}
```

### Check for Empty Values

Handle cases where inputs might be empty:

```yaml
prompt: |
  {{if .inputs.optional_context}}
  Context: {{.inputs.optional_context}}
  {{end}}

  Main task: {{.inputs.task}}
```

### Use Descriptive Variable Names

Make templates readable:

```yaml
# Good
{{.inputs.source_file_path}}
{{.steps.security_analysis.response}}

# Harder to understand
{{.inputs.src}}
{{.steps.step1.response}}
```

---

## Debugging Templates

### View Variable Values

Add a debug step to see what's in your variables:

```yaml
steps:
  - id: debug
    type: llm
    prompt: |
      Debug info:
      Input name: {{.inputs.name}}
      Previous step output: {{.steps.previous.response}}
```

### Common Errors

**"template: invalid syntax"**

- Check for unmatched `{{` and `}}`
- Ensure all variable paths start with `.`
- Verify step IDs are correct

**"field not found"**

- The step ID or field name doesn't exist
- Check spelling: `.response` not `.result`
- Verify the step has completed before referencing it

**Empty output**

- Variable exists but is empty
- Use `{{.variable | default "fallback"}}` to provide defaults

---

## Advanced Examples

### Dynamic Prompts Based on Input

```yaml
inputs:
  - name: review_type
    type: string
    enum: ["security", "performance", "style"]

steps:
  - id: review
    type: llm
    system: |
      {{if eq .inputs.review_type "security"}}
      You are a security engineer focused on vulnerabilities.
      {{else if eq .inputs.review_type "performance"}}
      You are a performance engineer focused on efficiency.
      {{else}}
      You are a code reviewer focused on maintainability.
      {{end}}
    prompt: "Review this code: {{.inputs.code}}"
```

### Iterating Over Arrays

```yaml
inputs:
  - name: files
    type: array

steps:
  - id: process_files
    type: llm
    prompt: |
      Process these files:
      {{range .inputs.files}}
      - {{.}}
      {{end}}
```

### Building JSON Payloads

```yaml
steps:
  - id: create_issue
    http.post:
      url: "https://api.github.com/repos/owner/repo/issues"
      headers:
        Authorization: "Bearer ${GITHUB_TOKEN}"
      body: |
        {
          "title": "{{.inputs.issue_title}}",
          "body": "{{.steps.generate_description.response}}",
          "labels": ["bug", "{{.inputs.priority}}"]
        }
```

---

## What's Next?

- **[Inputs and Outputs](inputs-outputs.md)** — Understand what variables you can reference
- **[Workflows and Steps](workflows-steps.md)** — See templates in context
- **[Error Handling](error-handling.md)** — Handle template errors gracefully
