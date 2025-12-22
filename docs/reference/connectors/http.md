# HTTP

The `http` connector provides secure HTTP client capabilities for making API requests from workflows.

## Overview

The HTTP connector is a **builtin connector** - it requires no configuration and is always available. It provides secure HTTP/HTTPS requests with built-in protections against SSRF, DNS rebinding, and other network-based attacks.

**Security Features:**
- SSRF protection (blocks private IP ranges by default)
- DNS rebinding prevention with DNS caching
- Request timeout enforcement (30s default)
- Host allowlist support
- HTTPS enforcement option
- Redirect validation and limits
- Automatic header sanitization

## Operations

### http.get

Make an HTTP GET request.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | URL to request |
| `headers` | object | No | HTTP headers to include |

**Example:**

```yaml
# Simple GET request
- http.get: "https://api.github.com/repos/owner/repo"

# GET with headers
- http.get:
    url: "https://api.example.com/data"
    headers:
      Authorization: "Bearer ${API_TOKEN}"
      Accept: "application/json"
```

**Response:**

```yaml
response:
  success: true
  status_code: 200
  headers:
    content-type: "application/json"
    content-length: "1234"
  body: '{"key": "value"}'
```

---

### http.post

Make an HTTP POST request with a body.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | URL to request |
| `body` | string | Yes | Request body |
| `headers` | object | No | HTTP headers to include |

**Example:**

```yaml
# POST JSON data
- http.post:
    url: "https://api.example.com/items"
    headers:
      Content-Type: "application/json"
      Authorization: "Bearer ${API_TOKEN}"
    body: '{"name": "item1", "value": 42}'

# POST from previous step output
- id: create_item
  http.post:
    url: "https://api.example.com/items"
    headers:
      Content-Type: "application/json"
    body: "{{.steps.generate.response}}"
```

**Response:**

```yaml
response:
  success: true
  status_code: 201
  headers:
    content-type: "application/json"
    location: "/items/123"
  body: '{"id": 123, "name": "item1"}'
```

---

### http.put

Make an HTTP PUT request.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | URL to request |
| `body` | string | Yes | Request body |
| `headers` | object | No | HTTP headers to include |

**Example:**

```yaml
- http.put:
    url: "https://api.example.com/items/123"
    headers:
      Content-Type: "application/json"
    body: '{"name": "updated-item", "value": 100}'
```

---

### http.delete

Make an HTTP DELETE request.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | URL to request |
| `headers` | object | No | HTTP headers to include |

**Example:**

```yaml
- http.delete:
    url: "https://api.example.com/items/123"
    headers:
      Authorization: "Bearer ${API_TOKEN}"
```

---

### http.patch

Make an HTTP PATCH request.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | URL to request |
| `body` | string | Yes | Request body |
| `headers` | object | No | HTTP headers to include |

**Example:**

```yaml
- http.patch:
    url: "https://api.example.com/items/123"
    headers:
      Content-Type: "application/json"
    body: '{"value": 200}'
```

---

### http.request (Generic)

Make an HTTP request with any method.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `method` | string | Yes | HTTP method (GET, POST, PUT, DELETE, PATCH, etc.) |
| `url` | string | Yes | URL to request |
| `body` | string | No | Request body (for POST/PUT/PATCH) |
| `headers` | object | No | HTTP headers to include |

**Example:**

```yaml
- http.request:
    method: HEAD
    url: "https://api.example.com/status"

- http.request:
    method: OPTIONS
    url: "https://api.example.com/resource"
    headers:
      Origin: "https://example.com"
```

---

## Response Format

All HTTP operations return:

```yaml
response:
  success: true              # true if 2xx status code
  status_code: 200           # HTTP status code
  headers:                   # Response headers (lowercase keys)
    content-type: "application/json"
    content-length: "1234"
  body: "response body"      # Response body as string
  error: null                # Error message if request failed
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `success` | boolean | `true` if status code is 2xx, `false` otherwise |
| `status_code` | number | HTTP status code (200, 404, 500, etc.) |
| `headers` | object | Response headers (keys normalized to lowercase) |
| `body` | string | Response body as string |
| `error` | string | Error message if request failed (null on success) |

### Accessing Response Data

```yaml
steps:
  - id: fetch_data
    http.get: "https://api.example.com/data"

  # Check if successful
  - id: check_success
    type: condition
    condition:
      expression: "$.fetch_data.success == true"
      then_steps: [process_data]
      else_steps: [handle_error]

  # Parse JSON response
  - id: process_data
    type: llm
    prompt: "Process this data: {{.steps.fetch_data.body}}"

  # Access specific status code
  - id: handle_404
    type: condition
    condition:
      expression: "$.fetch_data.status_code == 404"
      then_steps: [create_resource]

  # Access headers
  - id: check_rate_limit
    type: condition
    condition:
      expression: "$.fetch_data.headers[\"x-ratelimit-remaining\"] < 10"
      then_steps: [wait_for_reset]
```

---

## Examples

### GitHub API Integration

```yaml
name: github-issue-analyzer
description: "Fetch and analyze GitHub issues"

inputs:
  - name: repo
    type: string
    required: true
    description: "Repository in format owner/repo"

steps:
  # Fetch issues from GitHub API
  - id: fetch_issues
    http.get:
      url: "https://api.github.com/repos/{{.inputs.repo}}/issues"
      headers:
        Authorization: "Bearer ${GITHUB_TOKEN}"
        Accept: "application/vnd.github.v3+json"
        User-Agent: "Conductor-Workflow/1.0"

  # Check if request succeeded
  - id: check_response
    type: condition
    condition:
      expression: "$.fetch_issues.success == true"
      then_steps: [analyze_issues]
      else_steps: [handle_error]

  # Analyze issues with LLM
  - id: analyze_issues
    type: llm
    model: balanced
    prompt: |
      Analyze these GitHub issues and identify patterns:

      {{.steps.fetch_issues.body}}

      Provide a summary of common themes and prioritize critical issues.

  # Handle API errors
  - id: handle_error
    type: llm
    prompt: |
      GitHub API request failed:
      Status: {{.steps.fetch_issues.status_code}}
      Error: {{.steps.fetch_issues.error}}

      Suggest troubleshooting steps.

outputs:
  - name: analysis
    type: string
    value: "$.analyze_issues.response"
```

### REST API CRUD Operations

```yaml
name: api-crud-workflow
description: "Create, read, update, delete operations on REST API"

inputs:
  - name: api_base_url
    type: string
    required: true
  - name: item_name
    type: string
    required: true

steps:
  # Create a new item
  - id: create_item
    http.post:
      url: "{{.inputs.api_base_url}}/items"
      headers:
        Content-Type: "application/json"
        Authorization: "Bearer ${API_TOKEN}"
      body: '{"name": "{{.inputs.item_name}}", "status": "active"}'

  # Extract item ID from response
  - id: parse_create_response
    type: llm
    model: fast
    output_type: extraction
    output_options:
      fields: [id]
    prompt: "Extract the id field from this JSON: {{.steps.create_item.body}}"

  # Read the item back
  - id: read_item
    http.get:
      url: "{{.inputs.api_base_url}}/items/{{.steps.parse_create_response.id}}"
      headers:
        Authorization: "Bearer ${API_TOKEN}"

  # Update the item
  - id: update_item
    http.put:
      url: "{{.inputs.api_base_url}}/items/{{.steps.parse_create_response.id}}"
      headers:
        Content-Type: "application/json"
        Authorization: "Bearer ${API_TOKEN}"
      body: '{"status": "completed"}'

  # Delete the item
  - id: delete_item
    http.delete:
      url: "{{.inputs.api_base_url}}/items/{{.steps.parse_create_response.id}}"
      headers:
        Authorization: "Bearer ${API_TOKEN}"

outputs:
  - name: item_id
    type: string
    value: "$.parse_create_response.id"
  - name: final_status
    type: number
    value: "$.delete_item.status_code"
```

### Polling with Retry

```yaml
name: poll-api-until-ready
description: "Poll API endpoint until resource is ready"

steps:
  # Trigger long-running operation
  - id: start_job
    http.post:
      url: "https://api.example.com/jobs"
      headers:
        Content-Type: "application/json"
      body: '{"task": "process_data"}'

  # Poll for job completion
  - id: check_status
    http.get:
      url: "https://api.example.com/jobs/{{.steps.start_job.body}}"
    retry:
      max_attempts: 10
      backoff_base: 5
      backoff_multiplier: 1.5

  # Verify job completed
  - id: verify_complete
    type: llm
    model: fast
    output_type: decision
    output_options:
      choices: [complete, pending, failed]
    prompt: "Check if this job is complete: {{.steps.check_status.body}}"

  - id: handle_result
    type: condition
    condition:
      expression: "$.verify_complete.decision == 'complete'"
      then_steps: [fetch_result]
      else_steps: [handle_timeout]

  - id: fetch_result
    http.get:
      url: "https://api.example.com/jobs/{{.steps.start_job.body}}/result"
```

### Webhook Response

```yaml
name: process-webhook
description: "Process incoming webhook and send response"

inputs:
  - name: webhook_data
    type: object
    required: true
    description: "Data from webhook trigger"

steps:
  # Process webhook data
  - id: analyze
    type: llm
    prompt: "Analyze this webhook data: {{.inputs.webhook_data}}"

  # Send acknowledgment back to webhook source
  - id: acknowledge
    http.post:
      url: "{{.inputs.webhook_data.callback_url}}"
      headers:
        Content-Type: "application/json"
      body: '{"status": "processed", "result": "{{.steps.analyze.response}}"}'
```

---

## Security

### SSRF Protection

By default, the HTTP connector **blocks requests to private IP ranges** to prevent Server-Side Request Forgery (SSRF) attacks:

**Blocked Ranges:**
- Loopback: `127.0.0.0/8`, `::1`
- Private networks: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`
- Link-local: `169.254.0.0/16`, `fe80::/10`
- Multicast: `224.0.0.0/4`, `ff00::/8`

```yaml
# BLOCKED by default
- http.get: "http://127.0.0.1:8080/admin"
- http.get: "http://192.168.1.1/secrets"
- http.get: "http://169.254.169.254/metadata"  # AWS metadata

# ALLOWED
- http.get: "https://api.github.com/repos"
- http.get: "https://example.com/data"
```

**Disable SSRF Protection (Not Recommended):**

```yaml
# config.yaml (runtime configuration)
builtin_tools:
  http:
    block_private_ips: false  # Allows requests to private IPs
```

### DNS Rebinding Protection

The HTTP connector caches DNS resolutions to prevent DNS rebinding attacks:

1. Resolve hostname to IP address
2. Check IP against security rules
3. Cache resolution for duration of request
4. Use cached IP for actual request

This prevents an attacker from:
1. Returning a public IP during validation
2. Changing DNS to private IP before request
3. Bypassing SSRF protections

**DNS Cache Configuration:**

```yaml
# config.yaml
builtin_tools:
  http:
    dns_cache_timeout: 60s  # How long to cache DNS entries
```

### Host Allowlist

Restrict which hosts can be accessed:

```yaml
# config.yaml
builtin_tools:
  http:
    allowed_hosts:
      - api.github.com
      - api.example.com
      - webhooks.slack.com
    allow_subdomains: false  # Exact match only
```

**With Subdomain Matching:**

```yaml
builtin_tools:
  http:
    allowed_hosts:
      - github.com  # Allows api.github.com, gist.github.com, etc.
    allow_subdomains: true
```

### HTTPS Enforcement

Require HTTPS for all requests:

```yaml
# config.yaml
builtin_tools:
  http:
    require_https: true  # Block http:// URLs
```

```yaml
# BLOCKED when require_https: true
- http.get: "http://example.com/data"

# ALLOWED
- http.get: "https://example.com/data"
```

### Redirect Validation

The HTTP connector limits and validates redirects:

**Default Settings:**
- Maximum redirects: 10
- Redirect URL validation: Disabled by default

**Enable Strict Redirect Validation:**

```yaml
# config.yaml
builtin_tools:
  http:
    max_redirects: 5
    validate_redirects: true  # Apply same security rules to redirect targets
```

This prevents:
- Infinite redirect loops
- Redirects to blocked hosts
- Redirects to private IPs (with SSRF protection enabled)

---

## Authentication

### Bearer Token

```yaml
- http.get:
    url: "https://api.example.com/data"
    headers:
      Authorization: "Bearer ${API_TOKEN}"
```

### API Key

```yaml
- http.get:
    url: "https://api.example.com/data"
    headers:
      X-API-Key: "${API_KEY}"
```

### Basic Auth

```yaml
# Encode username:password in base64
- http.get:
    url: "https://api.example.com/data"
    headers:
      Authorization: "Basic ${BASIC_AUTH_TOKEN}"
```

### OAuth 2.0

```yaml
# Step 1: Get access token
- id: get_token
  http.post:
    url: "https://oauth.example.com/token"
    headers:
      Content-Type: "application/x-www-form-urlencoded"
    body: "grant_type=client_credentials&client_id=${CLIENT_ID}&client_secret=${CLIENT_SECRET}"

# Step 2: Extract token from response
- id: parse_token
  type: llm
  model: fast
  output_type: extraction
  output_options:
    fields: [access_token]
  prompt: "Extract access_token from: {{.steps.get_token.body}}"

# Step 3: Use token in API request
- id: api_call
  http.get:
    url: "https://api.example.com/protected"
    headers:
      Authorization: "Bearer {{.steps.parse_token.access_token}}"
```

---

## Error Handling

### Network Errors

```yaml
- id: try_request
  http.get: "https://api.example.com/data"
  on_error:
    strategy: retry
  retry:
    max_attempts: 3
    backoff_base: 2
    backoff_multiplier: 2.0

# Or handle with fallback
- id: try_primary
  http.get: "https://api.primary.com/data"
  on_error:
    strategy: fallback
    fallback_step: try_backup

- id: try_backup
  http.get: "https://api.backup.com/data"
```

### HTTP Error Status Codes

```yaml
- id: fetch_data
  http.get: "https://api.example.com/items/123"

- id: handle_status
  type: condition
  condition:
    expression: "$.fetch_data.status_code == 404"
    then_steps: [create_item]
    else_steps: [update_item]

# Handle rate limiting (429)
- id: check_rate_limit
  type: condition
  condition:
    expression: "$.fetch_data.status_code == 429"
    then_steps: [wait_and_retry]
```

### Timeout Handling

Default timeout: **30 seconds**

```yaml
# Increase timeout for slow APIs
- http.get:
    url: "https://slow-api.example.com/large-dataset"
  timeout: 120  # 2 minutes
```

**Timeout Response:**

```yaml
response:
  success: false
  status_code: 0
  error: "request timeout after 30s"
  body: ""
```

---

## Configuration

HTTP connector behavior can be customized via runtime configuration:

```yaml
# config.yaml (daemon/runtime config)
builtin_tools:
  http:
    # Timeout settings
    timeout: 30s

    # SSRF Protection
    block_private_ips: true

    # Host restrictions
    allowed_hosts:
      - api.github.com
      - api.example.com
    allow_subdomains: false

    # HTTPS enforcement
    require_https: false

    # Redirect handling
    max_redirects: 10
    validate_redirects: false

    # DNS security
    dns_cache_timeout: 60s
```

---

## Best Practices

### 1. Always Use HTTPS for Sensitive Data

```yaml
# GOOD - Encrypted
- http.get:
    url: "https://api.example.com/secrets"
    headers:
      Authorization: "Bearer ${TOKEN}"

# BAD - Credentials sent in plaintext
- http.get:
    url: "http://api.example.com/secrets"
    headers:
      Authorization: "Bearer ${TOKEN}"
```

### 2. Store Credentials in Environment Variables

```yaml
# GOOD - Credential from environment
- http.get:
    url: "https://api.example.com/data"
    headers:
      Authorization: "Bearer ${API_TOKEN}"

# BAD - Hardcoded credential (security violation)
- http.get:
    url: "https://api.example.com/data"
    headers:
      Authorization: "Bearer abc123..."  # Don't do this!
```

### 3. Handle Errors Gracefully

```yaml
# Check success before using response
- id: fetch
  http.get: "https://api.example.com/data"

- id: verify
  type: condition
  condition:
    expression: "$.fetch.success == true"
    then_steps: [process]
    else_steps: [handle_error]
```

### 4. Set Appropriate Timeouts

```yaml
# Quick health checks
- http.get: "https://api.example.com/health"
  timeout: 5

# Large data transfers
- http.get: "https://api.example.com/download/dataset"
  timeout: 300
```

### 5. Use Retry for Transient Failures

```yaml
- http.get: "https://api.example.com/data"
  retry:
    max_attempts: 3
    backoff_base: 1
    backoff_multiplier: 2.0  # 1s, 2s, 4s
```

### 6. Validate Response Content

```yaml
- id: fetch_config
  http.get: "https://api.example.com/config"

# Validate response is JSON
- id: parse_config
  type: llm
  model: fast
  output_type: decision
  output_options:
    choices: [valid_json, invalid]
  prompt: "Is this valid JSON? {{.steps.fetch_config.body}}"

- id: use_config
  type: condition
  condition:
    expression: "$.parse_config.decision == 'valid_json'"
    then_steps: [apply_config]
    else_steps: [use_default_config]
```

---

## Limitations

- **Response size**: Very large responses (>100MB) may cause memory issues
- **Binary data**: Response body is always returned as string; binary data is not fully supported
- **Streaming**: Responses are fully buffered; streaming is not supported
- **Custom protocols**: Only HTTP and HTTPS are supported
- **Client certificates**: Mutual TLS is not currently supported
- **Proxies**: HTTP proxy support is limited to system configuration

---

## Related

- [File Connector](file.md) - Filesystem operations
- [Shell Connector](shell.md) - Execute shell commands
- [Workflow Schema Reference](../workflow-schema.md) - Complete workflow YAML schema
- [Security Documentation](../../operations/security.md) - Security best practices
- [GitHub Connector](github.md) - Specialized GitHub operations
- [Slack Connector](slack.md) - Specialized Slack operations
