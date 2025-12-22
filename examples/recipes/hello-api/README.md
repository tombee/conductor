# Hello API Example Workflow

A simple workflow for testing API calls and automation recipes.

## Description

This workflow generates a friendly greeting using an LLM and returns a structured JSON response. It's designed to be used as a test workflow for the integration recipes, demonstrating:

- Basic workflow structure
- Input handling with defaults
- LLM step usage
- Structured output generation

## Inputs

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | string | No | "World" | Name to greet |
| `include_timestamp` | boolean | No | true | Include current timestamp in response |

## Outputs

| Name | Description |
|------|-------------|
| `message` | Formatted greeting message |
| `recipient` | Name that was greeted |

## Usage

### Via CLI

```bash
conductor run examples/recipes/hello-api/workflow.yaml

conductor run examples/recipes/hello-api/workflow.yaml \
  --input name="Alice" \
  --input include_timestamp=true
```

### Via API

```bash
curl -X POST \
  -H "X-API-Key: ${CONDUCTOR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": "hello-api",
    "inputs": {
      "name": "Alice",
      "include_timestamp": true
    }
  }' \
  http://localhost:9000/api/v1/runs
```

### Via Wrapper Script

```bash
./run-workflow.sh hello-api '{"name":"Alice"}'
```

## Expected Output

```json
{
  "message": "Hello Alice! Welcome to Conductor workflows.",
  "recipient": "Alice",
  "timestamp": "2025-12-25 10:30:00"
}
```

## Use Cases

- Testing API integration recipes
- Validating authentication and rate limiting
- Demonstrating basic workflow structure
- Quick smoke test for Conductor daemon
- Example for CLI scripting recipes

## See Also

- [CLI Scripting Recipe](../../../docs/recipes/automation/cli-scripting.md)
- [nginx Reverse Proxy Recipe](../../../docs/recipes/api-gateway/nginx.md)
- [Scheduled Execution Recipe](../../../docs/recipes/automation/scheduling.md)
