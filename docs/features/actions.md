# Actions

Actions are local operations that workflows can perform.

## LLM

Call an LLM with a prompt:

```yaml
steps:
  - id: generate
    llm:
      model: claude-3-5-sonnet-20241022
      prompt: "Generate a haiku about coding"
      temperature: 0.7
      max_tokens: 1000
```

### Available Models

- `claude-3-5-sonnet-20241022` - Fast, high quality (recommended)
- `claude-3-opus-20240229` - Most capable
- `claude-3-haiku-20240307` - Fastest, cost-effective

See [Model Tiers](./model-tiers.md) for selection guidance.

## Shell

Execute shell commands:

```yaml
steps:
  - id: build
    shell:
      command: |
        npm install
        npm run build
      workingDir: /path/to/project
```

### Environment Variables

```yaml
steps:
  - id: deploy
    shell:
      command: ./deploy.sh
      env:
        ENVIRONMENT: production
        API_KEY: ${API_KEY}
```

## File

Read and write files:

```yaml
steps:
  - id: read
    file:
      action: read
      path: data.json
  - id: write
    file:
      action: write
      path: output.txt
      content: ${steps.process.output}
```

### File Operations

- `read` - Read file contents
- `write` - Write content to file
- `append` - Append to existing file
- `delete` - Remove file

## HTTP

Make HTTP requests:

```yaml
steps:
  - id: api_call
    http:
      method: POST
      url: https://api.example.com/endpoint
      headers:
        Authorization: "Bearer ${API_TOKEN}"
        Content-Type: application/json
      body:
        key: value
```

### HTTP Methods

- `GET` - Retrieve data
- `POST` - Create resources
- `PUT` - Update resources
- `PATCH` - Partial update
- `DELETE` - Remove resources

### Response Handling

```yaml
steps:
  - id: fetch
    http:
      method: GET
      url: https://api.example.com/data
  - id: process
    llm:
      prompt: "Analyze: ${steps.fetch.output}"
```

## Transform

Transform data between formats:

```yaml
steps:
  - id: to_json
    transform:
      operation: yaml_to_json
      input: ${steps.read.output}
  - id: extract
    transform:
      operation: jq
      query: ".items[] | select(.active == true)"
      input: ${steps.to_json.output}
```

### Transform Operations

- `yaml_to_json` - Convert YAML to JSON
- `json_to_yaml` - Convert JSON to YAML
- `jq` - Query JSON with jq syntax
- `template` - Apply Go templates

## Utility

Utility operations:

```yaml
steps:
  - id: wait
    utility:
      operation: sleep
      duration: 5s
  - id: random
    utility:
      operation: random
      min: 1
      max: 100
```

### Utility Operations

- `sleep` - Wait for duration
- `random` - Generate random number
- `uuid` - Generate UUID
- `timestamp` - Get current timestamp
