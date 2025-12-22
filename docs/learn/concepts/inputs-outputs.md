# Inputs and Outputs

Learn how to pass data into workflows and extract results.

---

## Workflow Inputs

Inputs are parameters your workflow accepts. They make workflows reusable with different data.

### Defining Inputs

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
    description: Output format (markdown or json)

  - name: verbose
    type: boolean
    default: false
    description: Include detailed analysis
```

### Input Fields

- **`name`** — Identifier (use snake_case)
- **`type`** — Data type (see below)
- **`required`** — Whether input must be provided (default: false)
- **`default`** — Default value if not provided
- **`description`** — Help text shown to users
- **`enum`** — Array of allowed values (optional)

### Input Types

| Type | Example | Description |
|------|---------|-------------|
| `string` | `"hello"` | Text value |
| `number` | `42` | Integer or decimal |
| `boolean` | `true` | True or false |
| `array` | `["a", "b"]` | List of values |
| `object` | `{"key": "value"}` | JSON object |

**Example with enum:**

```yaml
inputs:
  - name: priority
    type: string
    required: true
    enum: ["low", "medium", "high", "critical"]
    description: Issue priority level
```

---

## Providing Input Values

### Interactive Prompts

When you run a workflow, Conductor prompts for required inputs:

```bash
$ conductor run analyze.yaml

file_path: README.md
format: (leave blank for default)
verbose: false
```

### Command-Line Flags

Provide inputs directly to skip prompts:

```bash
conductor run analyze.yaml \
  -i file_path=README.md \
  -i format=json \
  -i verbose=true
```

### Environment Variables

Reference environment variables in default values:

```yaml
inputs:
  - name: api_key
    type: string
    default: "${API_KEY}"
    description: API authentication key
```

### Input Files

Load inputs from a JSON file:

```bash
conductor run workflow.yaml --input-file inputs.json
```

**inputs.json:**

```json
{
  "file_path": "README.md",
  "format": "json",
  "verbose": true
}
```

---

## Using Inputs in Steps

Reference inputs using template syntax: `{{.inputs.name}}`

```yaml
inputs:
  - name: user_name
    type: string
    required: true

  - name: language
    type: string
    default: "English"

steps:
  - id: greet
    type: llm
    prompt: |
      Generate a friendly greeting for {{.inputs.user_name}} in {{.inputs.language}}.
```

### Accessing Nested Values

For array and object inputs:

```yaml
inputs:
  - name: config
    type: object

steps:
  - id: process
    type: llm
    prompt: "The API endpoint is {{.inputs.config.api_url}}"
```

**Array access:**

```yaml
inputs:
  - name: reviewers
    type: array

steps:
  - id: assign
    type: llm
    prompt: "Assign to {{.inputs.reviewers.0}}"  # First element
```

---

## Workflow Outputs

Outputs extract results from your workflow. They define what data is returned when the workflow completes.

### Defining Outputs

```yaml
outputs:
  - name: summary
    type: string
    value: "{{.steps.summarize.response}}"
    description: Brief summary of the analysis

  - name: score
    type: number
    value: "{{.steps.calculate_score.response}}"
    description: Numerical quality score (0-100)

  - name: report_path
    type: string
    value: "{{.inputs.output_file}}"
    description: Path where report was saved
```

### Output Fields

- **`name`** — Identifier for the output value
- **`type`** — Data type (string, number, boolean, array, object)
- **`value`** — Template expression to extract the value
- **`description`** — What this output represents

---

## Referencing Step Outputs

Each step stores its results. Access them using `{{.steps.step_id.field}}`:

### LLM Step Outputs

LLM steps provide:

- **`.response`** — The full text response from the model
- **`.model`** — Model that generated the response
- **`.tokens`** — Token usage information

```yaml
steps:
  - id: analyze
    type: llm
    prompt: "Analyze this code..."

  - id: summarize
    type: llm
    prompt: "Summarize: {{.steps.analyze.response}}"

outputs:
  - name: full_analysis
    value: "{{.steps.analyze.response}}"
```

### Tool Step Outputs

Tool outputs vary by tool type:

**File tool:**

```yaml
- id: read_file
  file.read:
    path: "data.txt"

- id: process
  type: llm
  prompt: "Process: {{.steps.read_file.content}}"
```

**Shell tool:**

```yaml
- id: get_branch
  shell.run:
    command: ["git", "rev-parse", "--abbrev-ref", "HEAD"]

- id: analyze
  type: llm
  prompt: "Analyzing branch: {{.steps.get_branch.stdout}}"
```

**HTTP tool:**

```yaml
- id: fetch_data
  http.get:
    url: "https://api.example.com/data"

- id: process
  type: llm
  prompt: "Process this data: {{.steps.fetch_data.body}}"
```

### Parallel Step Outputs

Access nested step outputs using dot notation:

```yaml
- id: reviews
  type: parallel
  steps:
    - id: security_review
      type: llm
      prompt: "Security review..."

    - id: performance_review
      type: llm
      prompt: "Performance review..."

- id: consolidate
  type: llm
  prompt: |
    Combine reviews:

    Security: {{.steps.reviews.security_review.response}}
    Performance: {{.steps.reviews.performance_review.response}}
```

---

## Output Formats

### Simple String Output

Most common use case:

```yaml
outputs:
  - name: result
    type: string
    value: "{{.steps.process.response}}"
```

### Multiple Outputs

Return several values:

```yaml
outputs:
  - name: summary
    type: string
    value: "{{.steps.summarize.response}}"

  - name: details
    type: string
    value: "{{.steps.analyze.response}}"

  - name: timestamp
    type: string
    value: "{{.steps.get_timestamp.stdout}}"
```

### Structured Output

Return complex data:

```yaml
outputs:
  - name: report
    type: object
    value: |
      {
        "summary": "{{.steps.summarize.response}}",
        "score": {{.steps.score.response}},
        "recommendations": "{{.steps.recommend.response}}"
      }
```

---

## Best Practices

### Input Naming

Use descriptive, lowercase names with underscores:

```yaml
# Good
inputs:
  - name: source_file_path
  - name: max_retries
  - name: enable_debug_mode

# Avoid
inputs:
  - name: file       # Too vague
  - name: srcPath    # Inconsistent casing
  - name: x          # Not descriptive
```

### Required vs Optional

Make inputs required only if the workflow cannot function without them:

```yaml
inputs:
  # Required: workflow fails without this
  - name: code_to_review
    type: string
    required: true

  # Optional: workflow has sensible default
  - name: output_format
    type: string
    default: "markdown"
```

### Provide Helpful Descriptions

Users see descriptions when prompted for input:

```yaml
inputs:
  - name: pr_url
    type: string
    required: true
    description: "GitHub PR URL (e.g., https://github.com/owner/repo/pull/123)"
```

### Use Enums for Fixed Choices

Constrain inputs to valid values:

```yaml
inputs:
  - name: review_type
    type: string
    required: true
    enum: ["security", "performance", "style", "all"]
    description: Type of code review to perform
```

### Validate Input in Prompts

Reference inputs in prompts to validate they're used correctly:

```yaml
inputs:
  - name: code
    type: string
    required: true

steps:
  - id: validate
    type: llm
    prompt: |
      First, verify this is valid code: {{.inputs.code}}

      If it's not code, respond with "ERROR: Not valid code"
      Otherwise, proceed with analysis...
```

---

## Common Patterns

### Pass-Through Outputs

Return an input as an output (useful for chaining workflows):

```yaml
inputs:
  - name: file_path
    type: string
    required: true

outputs:
  - name: processed_file
    type: string
    value: "{{.inputs.file_path}}"
```

### Computed Outputs

Combine multiple step outputs:

```yaml
outputs:
  - name: full_report
    type: string
    value: |
      # Code Review Report

      ## Summary
      {{.steps.summary.response}}

      ## Security Analysis
      {{.steps.security.response}}

      ## Performance Analysis
      {{.steps.performance.response}}
```

### Conditional Outputs

Use conditionals to customize output:

```yaml
outputs:
  - name: status
    type: string
    value: "{{if .steps.validate.response contains 'ERROR'}}Failed{{else}}Success{{end}}"
```

---

## Example: Complete Workflow

Here's a workflow showing inputs and outputs working together:

```yaml
name: code-analyzer
description: Analyze code quality and generate a report

inputs:
  - name: code_file
    type: string
    required: true
    description: Path to code file to analyze

  - name: focus_areas
    type: array
    default: ["security", "performance", "style"]
    description: Which aspects to analyze

  - name: output_file
    type: string
    default: "analysis.md"
    description: Where to save the report

steps:
  - id: read_code
    file.read:
      path: "{{.inputs.code_file}}"

  - id: analyze
    type: llm
    model: balanced
    prompt: |
      Analyze this code focusing on: {{.inputs.focus_areas}}

      Code:
      {{.steps.read_code.content}}

  - id: save_report
    file.write:
      path: "{{.inputs.output_file}}"
      content: "{{.steps.analyze.response}}"

outputs:
  - name: analysis
    type: string
    value: "{{.steps.analyze.response}}"
    description: Full code analysis

  - name: report_location
    type: string
    value: "{{.inputs.output_file}}"
    description: Where the report was saved

  - name: analyzed_file
    type: string
    value: "{{.inputs.code_file}}"
    description: Which file was analyzed
```

---

## What's Next?

- **[Template Variables](template-variables.md)** — Master the `{{}}` syntax
- **[Workflows and Steps](workflows-steps.md)** — Understand workflow structure
- **[Error Handling](error-handling.md)** — Handle missing inputs gracefully
