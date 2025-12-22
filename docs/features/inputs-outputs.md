# Inputs and Outputs

Workflows accept inputs and produce outputs for reusability and composition.

## Inputs

Define parameters your workflow accepts:

```yaml
name: greet
inputs:
  name:
    type: string
    description: Person to greet
    required: true
  enthusiasm:
    type: number
    default: 5
steps:
  - id: greet
    llm:
      prompt: "Greet ${inputs.name} with enthusiasm level ${inputs.enthusiasm}"
```

Pass inputs when running:

```bash
conductor run greet.yaml -i name="Alice" -i enthusiasm=10
```

### Input Types

- `string` - Text values
- `number` - Numeric values
- `boolean` - true/false
- `object` - Structured JSON data
- `array` - List of values

### Required vs Optional

Mark inputs as required or provide defaults:

```yaml
inputs:
  required_param:
    type: string
    required: true
  optional_param:
    type: string
    default: "default value"
```

## Outputs

Return data from workflows:

```yaml
name: analyze
steps:
  - id: process
    llm:
      prompt: "Analyze this data"
outputs:
  result: ${steps.process.output}
  metadata:
    timestamp: ${steps.process.startTime}
    model: ${steps.process.model}
```

Access outputs when composing workflows or using them programmatically.

### Output Formats

Specify how outputs should be formatted and validated using the `format` field:

```yaml
outputs:
  - name: recipe
    type: string
    value: "{{.steps.generate.response}}"
    description: Generated recipe with formatting
    format: markdown

  - name: config
    type: string
    value: "{{.steps.extract.response}}"
    description: JSON configuration
    format: json

  - name: count
    type: string
    value: "{{.steps.calculate.response}}"
    description: Numeric count
    format: number

  - name: script
    type: string
    value: "{{.steps.generate_code.response}}"
    description: Python code with syntax highlighting
    format: code:python
```

**Supported formats:**

- `string` - Plain text (default, no validation)
- `number` - Validates numeric values (integers, floats, scientific notation)
- `markdown` - Markdown text with CLI rendering and formatting
- `json` - Validates JSON and pretty-prints with 2-space indentation
- `code` - Code without syntax highlighting
- `code:<language>` - Code with syntax highlighting (e.g., `code:python`, `code:javascript`, `code:go`)

**Format validation:**

Output format validation occurs after the output value expression is evaluated. If validation fails, the workflow fails with an `OutputValidationError`. Error messages are generic to avoid exposing sensitive data, but full details are logged with the run ID for authorized debugging.

**CLI display formatting:**

When outputs are displayed via `conductor run` or `conductor runs show`:
- Formatting is applied only when stdout is an interactive TTY
- Markdown renders with headers, lists, and emphasis formatting
- JSON is pretty-printed with 2-space indentation
- Code is syntax-highlighted when a language is specified
- Piped output contains no ANSI codes for clean machine processing

## Variable Syntax

Reference inputs, step outputs, and environment variables:

```yaml
# Input reference
${inputs.paramName}

# Step output reference
${steps.stepId.output}

# Environment variable
${ENV_VAR_NAME}

# Nested object access
${steps.stepId.output.field.nested}

# Array element
${steps.stepId.output[0]}
```

## Step Outputs

Each step produces an `output` field. For LLM steps, this is the generated text. For other actions, it's the result of the operation:

```yaml
steps:
  - id: read
    file:
      action: read
      path: data.txt
  - id: process
    llm:
      prompt: "Summarize: ${steps.read.output}"
```

## Complex Outputs

Structure outputs with nested data:

```yaml
outputs:
  summary: ${steps.analyze.output}
  details:
    processedAt: ${steps.analyze.timestamp}
    items: ${steps.collect.outputs}
    status: "complete"
```

## Environment Variables

Reference environment variables for secrets and configuration:

```yaml
steps:
  - id: api_call
    http:
      url: https://api.example.com
      headers:
        Authorization: "Bearer ${API_TOKEN}"
```

Set before running:

```bash
export API_TOKEN="your-token"
conductor run workflow.yaml
```
