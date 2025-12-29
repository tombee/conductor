# Custom

This guide shows you how to create custom integrations for any REST API in your workflows.

## Quick Example

```conductor
connectors:
  my_api:
    base_url: https://api.example.com
    auth:
      type: bearer
      token: ${API_TOKEN}
    operations:
      get_user:
        method: GET
        path: /users/{user_id}
        request_schema:
          type: object
          properties:
            user_id: { type: string }
          required: [user_id]
        response_transform: "{id: .id, name: .name, email: .email}"

steps:
  - id: fetch_user
    type: integration
    integration: my_api.get_user
    inputs:
      user_id: "12345"
```

## Anatomy of a Integration

### Base Configuration

```conductor
connectors:
  my_api:
    base_url: https://api.example.com  # Required for inline integrations
    auth:                              # Optional
      type: bearer
      token: ${API_TOKEN}
    rate_limit:                        # Optional
      requests_per_second: 10
      requests_per_minute: 100
    headers:                           # Optional, applied to all operations
      Content-Type: application/json
      User-Agent: Conductor/1.0
    operations:                        # Required for inline integrations
      # ... operation definitions
```

### Operation Definition

```conductor
operations:
  operation_name:
    method: POST                       # Required: GET, POST, PUT, PATCH, DELETE
    path: /api/resource/{id}          # Required: URL path with {param} placeholders
    request_schema:                    # Optional but recommended: JSON Schema for inputs
      type: object
      properties:
        id: { type: string }
        name: { type: string }
      required: [id]
    response_transform: ".data"        # Optional: jq expression to extract data
    headers:                           # Optional: operation-specific headers
      X-Custom-Header: value
    timeout: 30                        # Optional: operation timeout in seconds
```

## Authentication

### Bearer Token (Default)

Most APIs use bearer tokens:

```conductor
auth:
  type: bearer  # Optional, inferred if only 'token' is present
  token: ${API_TOKEN}

# Shorthand (bearer assumed):
auth:
  token: ${API_TOKEN}
```

Sets header: `Authorization: Bearer ${API_TOKEN}`

### Basic Auth

For APIs using HTTP Basic Authentication:

```conductor
auth:
  type: basic
  username: ${API_USER}
  password: ${API_PASSWORD}
```

Sets header: `Authorization: Basic base64(username:password)`

### API Key

For custom API key headers:

```conductor
auth:
  type: api_key
  header: X-API-Key      # Header name
  value: ${API_KEY}      # API key value
```

Sets header: `X-API-Key: ${API_KEY}`

### No Auth

Omit the `auth` section for public APIs.

## Path Templates

Use `{param}` placeholders in paths:

```conductor
operations:
  get_user:
    method: GET
    path: /users/{user_id}
    request_schema:
      type: object
      properties:
        user_id: { type: string }
      required: [user_id]
```

When called:
```conductor
inputs:
  user_id: "alice"
```

Becomes: `GET /users/alice`

### Path Parameter Encoding

Path parameters are automatically URL-encoded:

| Input | Encoded |
|-------|---------|
| `alice` | `alice` |
| `user@example.com` | `user%40example.com` |
| `test/value` | `test%2Fvalue` |

### Security

Path parameters are validated to prevent path traversal:

- `../etc/passwd` → Rejected
- `%2e%2e/admin` → Rejected
- `\0` (null byte) → Rejected

## Request Body

For POST, PUT, and PATCH operations, non-path parameters become the request body:

```conductor
operations:
  create_user:
    method: POST
    path: /users
    request_schema:
      type: object
      properties:
        name: { type: string }
        email: { type: string }
      required: [name, email]
```

Called with:
```conductor
inputs:
  name: "Alice"
  email: "alice@example.com"
```

Sends:
```json
POST /users
{"name": "Alice", "email": "alice@example.com"}
```

## Response Transforms

Extract only needed data from API responses using jq syntax.

### Basic Transforms

```conductor
# Extract single field
response_transform: ".id"

# Extract nested field
response_transform: ".user.email"

# Extract multiple fields
response_transform: "{id, name, email}"

# Rename field
response_transform: "{user_id: .id, user_name: .name}"
```

### Array Transforms

```conductor
# Get first element
response_transform: ".[0]"

# Map array
response_transform: "[.[] | {id, name}]"

# Filter and map
response_transform: "[.[] | select(.active) | {id, name}]"

# Extract field from all elements
response_transform: "[.[].name]"
```

### Common Patterns

```conductor
# Unwrap data envelope
response_transform: ".data"

# Get list of IDs
response_transform: "[.[].id]"

# Count results
response_transform: "length"

# Join strings
response_transform: ".tags | join(\", \")"
```

## jq Transform Cheat Sheet

| Goal | Expression | Example |
|------|------------|---------|
| Extract field | `.field` | `{"name": "alice"}` → `"alice"` |
| Nested field | `.user.email` | `{"user": {"email": "a@b.com"}}` → `"a@b.com"` |
| Array first | `.[0]` | `[1, 2, 3]` → `1` |
| Array all | `.[]` | `[1, 2, 3]` → `1`, `2`, `3` (multiple outputs) |
| Select fields | `{name, email}` | `{"name": "a", "email": "b", "age": 30}` → `{"name": "a", "email": "b"}` |
| Rename | `{user_id: .id}` | `{"id": 123}` → `{"user_id": 123}` |
| Map array | `[.[] \| {name}]` | `[{"name": "a", "x": 1}]` → `[{"name": "a"}]` |
| Filter | `[.[] \| select(.active)]` | `[{"active": true}, {"active": false}]` → `[{"active": true}]` |
| Length | `length` | `[1, 2, 3]` → `3` |
| Join | `.items \| join(", ")` | `{"items": ["a", "b"]}` → `"a, b"` |
| Has key | `has("field")` | `{"field": 1}` → `true` |
| Type check | `type` | `123` → `"number"` |
| String ops | `\| tostring` | `123` → `"123"` |
| Math | `\| . + 1` | `5` → `6` |

## Request Schema Validation

Define JSON Schema to validate inputs before execution:

```conductor
operations:
  create_user:
    method: POST
    path: /users
    request_schema:
      type: object
      properties:
        name:
          type: string
          minLength: 1
          maxLength: 100
        email:
          type: string
          format: email
        age:
          type: integer
          minimum: 18
        role:
          type: string
          enum: [admin, user, guest]
        preferences:
          type: object
          properties:
            newsletter: { type: boolean }
      required: [name, email]
```

Benefits:
- Catches errors before API call
- Documents expected inputs
- Provides clear error messages

## Rate Limiting

Protect against API quota exhaustion:

```conductor
connectors:
  my_api:
    base_url: https://api.example.com
    rate_limit:
      requests_per_second: 10    # Max 10 requests per second
      requests_per_minute: 100   # Max 100 requests per minute
      timeout: 30                # Wait up to 30s for rate limit slot
```

Uses token bucket algorithm for smooth rate limiting. Requests exceeding the limit wait up to `timeout` seconds before failing.

## Headers

### Global Headers

Apply to all operations:

```conductor
connectors:
  my_api:
    base_url: https://api.example.com
    headers:
      Content-Type: application/json
      User-Agent: MyApp/1.0
      X-API-Version: "2023-01"
```

### Operation Headers

Override or add to global headers:

```conductor
operations:
  upload_file:
    method: POST
    path: /upload
    headers:
      Content-Type: multipart/form-data  # Overrides global
      X-Upload-Source: workflow
```

### Environment Variables

Reference environment variables in any header or auth value:

```conductor
headers:
  X-API-Key: ${API_KEY}
  X-Tenant-ID: ${TENANT_ID}
```

## Complete Example

Here's a full example for an internal API:

```conductor
name: process-feedback
description: Analyze user feedback and create tickets

connectors:
  helpdesk:
    base_url: https://api.helpdesk.internal
    auth:
      type: bearer
      token: ${HELPDESK_TOKEN}
    rate_limit:
      requests_per_second: 5
    headers:
      X-API-Version: "2024-01"
    operations:
      get_user:
        method: GET
        path: /users/{user_id}
        request_schema:
          type: object
          properties:
            user_id: { type: string }
          required: [user_id]
        response_transform: "{id, name, email, tier}"

      create_ticket:
        method: POST
        path: /tickets
        request_schema:
          type: object
          properties:
            title: { type: string, minLength: 1 }
            description: { type: string }
            priority: { type: string, enum: [low, medium, high, urgent] }
            user_id: { type: string }
          required: [title, priority]
        response_transform: "{id, ticket_number: .number, url: .html_url}"

steps:
  # Get user context
  - id: get_user
    type: integration
    integration: helpdesk.get_user
    inputs:
      user_id: "{{.inputs.user_id}}"

  # LLM analyzes feedback with user context
  - id: analyze
    type: llm
    model: balanced
    prompt: |
      Analyze this feedback and determine priority:

      User: {{.steps.get_user.name}} ({{.steps.get_user.tier}} tier)
      Feedback: {{.inputs.feedback}}

      Consider:
      - Issue severity
      - User tier (enterprise = higher priority)
      - Issue type (bug vs feature request)

      Respond with:
      - title: Clear, actionable title
      - priority: low, medium, high, or urgent
      - description: Detailed analysis for support team

  # Create ticket
  - id: create_ticket
    type: integration
    integration: helpdesk.create_ticket
    inputs:
      title: "{{.steps.analyze.title}}"
      description: "{{.steps.analyze.description}}"
      priority: "{{.steps.analyze.priority}}"
      user_id: "{{.inputs.user_id}}"

outputs:
  - name: ticket_url
    value: "{{.steps.create_ticket.url}}"
```

## Testing and Debugging

### Dry Run

See what requests would be made without executing:

```bash
conductor run --dry-run workflow.yaml
```

### Debug Logging

Enable verbose logging to see requests and responses:

```bash
conductor run --log-level debug workflow.yaml
```

### Test Single Operation

Test an operation in isolation:

```bash
conductor integration invoke my_api.get_user --inputs '{"user_id":"123"}'
```

### Validate Workflow

Catch schema errors before running:

```bash
conductor validate workflow.yaml
```

## Security Best Practices

### Never Hardcode Secrets

Always use environment variables:

```conductor
# Good
auth:
  token: ${API_TOKEN}

# Bad - secret in file!
auth:
  token: "secret-token-12345"
```

### Use SSRF Protection

Limit which hosts can be accessed:

```conductor
security:
  allowed_hosts:
    - api.example.com
    - "*.trusted-domain.com"
  blocked_hosts:
    - "*.internal.corp"  # Block internal domains
```

### Validate Inputs

Always define `request_schema` to validate inputs:

```conductor
request_schema:
  type: object
  properties:
    id: { type: string, pattern: "^[0-9]+$" }  # Numbers only
    email: { type: string, format: email }      # Valid email
  required: [id]
```

## Troubleshooting

### Operation Not Found

**Error**: `operation "get_user" not found in integration "my_api"`

**Fix**: Check operation name in integration definition matches usage:
```conductor
operations:
  get_user:  # Must match integration.get_user in step
```

### Invalid Request Schema

**Error**: `request validation failed: missing required field "name"`

**Fix**: Ensure all required fields are provided:
```conductor
inputs:
  name: "value"  # Add missing field
```

### Transform Error

**Error**: `response transform failed: cannot index number with string`

**Fix**: Check response structure matches transform:
```bash
# Debug: see raw response
conductor run --log-level debug workflow.yaml
```

### Rate Limit Exceeded

**Error**: `rate limit exceeded, waited 30s`

**Fix**: Reduce request rate:
```conductor
rate_limit:
  requests_per_second: 5  # Lower from 10
```

## Next Steps

- Review [bundled integration examples](./github.md)
- Learn about [MCP vs Integrations](./index.md#when-to-use-connectors-vs-mcp)
- Read [operational runbooks](./runbooks.md)
