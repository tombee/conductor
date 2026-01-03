# Actions

Actions are local operations that workflows can perform.

## LLM

Call an LLM with a prompt:

```yaml
steps:
  - id: generate
    type: llm
    prompt: Generate a haiku about coding
```

### Model Tiers

Use model tiers to select capability level without specifying provider-specific model names:

- `fast` - Quick responses, lower cost
- `balanced` - Good balance of speed and quality (default)
- `strategic` - Maximum capability for complex tasks

```yaml
steps:
  - id: analyze
    type: llm
    model: strategic
    prompt: Analyze this complex codebase...
```

See [Model Tiers](./model-tiers.md) for more details.

## Shell

Execute shell commands:

```yaml
steps:
  - id: build
    shell.run: npm install && npm run build
```

With options:

```yaml
steps:
  - id: build
    shell.run:
      command: npm run build
      working_dir: ./frontend
      env:
        NODE_ENV: production
```

## File

Read and write files:

```yaml
steps:
  - id: read_config
    file.read: config.json

  - id: save_output
    file.write:
      path: output.txt
      content: "{{.steps.process.response}}"
```

### File Operations

- `file.read` - Read file contents
- `file.write` - Write content to file
- `file.append` - Append to existing file
- `file.delete` - Remove file
- `file.exists` - Check if file exists
- `file.list` - List directory contents

## HTTP

Make HTTP requests:

```yaml
steps:
  - id: api_call
    http.post:
      url: https://api.example.com/endpoint
      headers:
        Authorization: "Bearer {{env.API_TOKEN}}"
        Content-Type: application/json
      body:
        key: value
```

### HTTP Methods

- `http.get` - Retrieve data
- `http.post` - Create resources
- `http.put` - Update resources
- `http.patch` - Partial update
- `http.delete` - Remove resources

### Response Handling

```yaml
steps:
  - id: fetch
    http.get: https://api.example.com/data

  - id: process
    type: llm
    prompt: "Analyze this data: {{.steps.fetch.body}}"
```

## Transform

Transform data between formats:

```yaml
steps:
  - id: parse
    transform.json_parse: "{{.steps.read.content}}"

  - id: extract
    transform.jq:
      input: "{{.steps.parse.result}}"
      query: ".items[] | select(.active == true)"
```

### Transform Operations

- `transform.json_parse` - Parse JSON string
- `transform.json_stringify` - Convert to JSON string
- `transform.yaml_parse` - Parse YAML string
- `transform.jq` - Query with jq syntax

## Utility

Utility operations:

```yaml
steps:
  - id: wait
    utility.sleep: 5s

  - id: random_id
    utility.uuid: {}
```

### Utility Operations

- `utility.sleep` - Wait for duration
- `utility.random_int` - Generate random integer
- `utility.uuid` - Generate UUID
- `utility.timestamp` - Get current timestamp
