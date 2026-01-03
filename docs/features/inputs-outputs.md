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
