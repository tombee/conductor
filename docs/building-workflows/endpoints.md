# Workflow as API (Endpoints)

Conductor endpoints allow you to expose workflows as RESTful API endpoints, turning your automation into accessible web services that can be called from any HTTP client.

## Overview

The endpoints feature provides:

- **RESTful API** for workflows with configurable inputs
- **Authentication and authorization** using API keys with scopes
- **Rate limiting** to protect against abuse
- **Input validation** against workflow schemas
- **Synchronous and asynchronous execution modes**
- **Server-Sent Events (SSE)** for real-time streaming
- **CORS support** for browser-based applications (optional)

## Quick Start

### 1. Define an Endpoint

Add endpoints to your controller configuration (`config.yaml`):

```conductor
controller:
  endpoints:
    enabled: true
    endpoints:
      - name: review-pr
        description: Review GitHub pull requests
        workflow: examples/code-review/workflow.yaml
        inputs:
          repository: "default/repo"
        scopes:
          - review-pr
          - review-*
        rate_limit: "10/hour"
        timeout: 300s
```

### 2. Create an API Key

Generate an API key with appropriate scopes:

```bash
conductor key create review-bot --scopes review-pr
```

### 3. Call the Endpoint

Make an HTTP request to execute the workflow:

```bash
curl -X POST http://localhost:8080/v1/endpoints/review-pr/runs \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": {
      "pr_url": "https://github.com/org/repo/pull/123",
      "repository": "org/repo"
    }
  }'
```

Response (async mode):

```json
{
  "id": "run_abc123",
  "status": "pending",
  "created_at": "2025-01-15T10:30:00Z"
}
```

## Configuration

### Endpoint Definition

Each endpoint has the following configuration options:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique identifier for the endpoint |
| `description` | string | No | Human-readable description |
| `workflow` | string | Yes | Path to workflow file (relative or absolute) |
| `inputs` | object | No | Default inputs for the workflow |
| `scopes` | []string | No | Required scopes to access this endpoint |
| `rate_limit` | string | No | Rate limit (e.g., "100/hour", "10/minute") |
| `timeout` | duration | No | Maximum execution time for synchronous requests |
| `public` | boolean | No | Allow unauthenticated access (default: false) |

### Example Configuration

```conductor
controller:
  endpoints:
    enabled: true
    endpoints:
      # Simple endpoint with defaults
      - name: hello
        workflow: hello.yaml

      # Production endpoint with full configuration
      - name: analyze-security
        description: Analyze code for security vulnerabilities
        workflow: workflows/security-scan.yaml
        inputs:
          scan_level: "standard"
          report_format: "json"
        scopes:
          - security-scan
        rate_limit: "50/hour"
        timeout: 600s
```

## Authentication and Authorization

### API Key Scopes

Endpoints use API key scopes for fine-grained access control:

```conductor
controller:
  api_keys:
    - id: bot-key
      name: "Automation Bot"
      scopes:
        - review-*      # Wildcard: access all review endpoints
        - deploy-staging # Exact match: only deploy-staging endpoint
```

**Scope Matching Rules:**

- Empty scopes (`[]`) = admin key (full access)
- Exact match: `review-pr` grants access to `review-pr` endpoint
- Wildcard suffix: `review-*` grants access to `review-pr`, `review-code`, etc.
- No match = 404 Not Found (security by obscurity)

### Creating Scoped API Keys

```bash
# Admin key (full access)
conductor key create admin-key

# Scoped key for specific endpoints
conductor key create review-bot --scopes review-pr,review-code

# Wildcard scope
conductor key create deploy-bot --scopes "deploy-*"
```

## Execution Modes

### Asynchronous Mode (Default)

Returns immediately with a run ID:

```bash
curl -X POST http://localhost:8080/v1/endpoints/review-pr/runs \
  -H "Authorization: Bearer your-api-key" \
  -d '{"inputs": {"pr_url": "..."}}'
```

Response:

```json
{
  "id": "run_abc123",
  "status": "pending",
  "created_at": "2025-01-15T10:30:00Z"
}
```

Check status later:

```bash
curl http://localhost:8080/v1/runs/run_abc123 \
  -H "Authorization: Bearer your-api-key"
```

### Synchronous Mode

Wait for completion before returning (use `?wait=true`):

```bash
curl -X POST "http://localhost:8080/v1/endpoints/review-pr/runs?wait=true&timeout=60s" \
  -H "Authorization: Bearer your-api-key" \
  -d '{"inputs": {"pr_url": "..."}}'
```

Response (after workflow completes):

```json
{
  "status": "completed",
  "output": {
    "message": "PR looks good! ✓",
    "score": 8.5
  }
}
```

**Timeout Handling:**

If execution exceeds the timeout:
- Returns HTTP 408 Request Timeout
- Workflow continues running in the background
- Response includes run ID for polling

### Streaming Mode (SSE)

Stream real-time execution updates using Server-Sent Events:

```bash
curl -X POST "http://localhost:8080/v1/endpoints/review-pr/runs?wait=true&stream=true" \
  -H "Authorization: Bearer your-api-key" \
  -d '{"inputs": {"pr_url": "..."}}'
```

SSE Event Stream:

```
event: start
data: {"run_id":"run_abc123","status":"running"}

event: log
data: {"step":"analyze","message":"Analyzing code..."}

event: log
data: {"step":"review","message":"Generating review..."}

event: done
data: {"status":"completed","output":{"message":"PR looks good!"}}
```

## Input Validation

Endpoints validate request inputs against the workflow's input schema:

**Workflow Definition:**

```conductor
name: review-pr
inputs:
  - name: pr_url
    type: string
    required: true
    description: GitHub PR URL

  - name: severity
    type: enum
    enum: ["low", "medium", "high"]
    default: "medium"

  - name: max_comments
    type: number
    default: 10
```

**Valid Request:**

```json
{
  "inputs": {
    "pr_url": "https://github.com/org/repo/pull/123",
    "severity": "high",
    "max_comments": 5
  }
}
```

**Invalid Request (missing required field):**

```bash
curl -X POST http://localhost:8080/v1/endpoints/review-pr/runs \
  -H "Authorization: Bearer key" \
  -d '{"inputs": {}}'
```

Response:

```json
{
  "error": "input validation failed: required input \"pr_url\" is missing"
}
```

**Invalid Request (wrong type):**

```json
{
  "inputs": {
    "pr_url": "https://github.com/org/repo/pull/123",
    "max_comments": "five"  // Should be number
  }
}
```

Response:

```json
{
  "error": "input validation failed: input \"max_comments\" must be a number, got string"
}
```

## Rate Limiting

Protect endpoints from abuse with configurable rate limits:

```conductor
endpoints:
  - name: expensive-analysis
    workflow: analysis.yaml
    rate_limit: "10/hour"  # 10 requests per hour per endpoint
```

**Rate Limit Formats:**

- `N/second` - N requests per second
- `N/minute` - N requests per minute
- `N/hour` - N requests per hour
- `N/day` - N requests per day

**Rate Limit Headers:**

All responses include rate limit information:

```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 7
X-RateLimit-Reset: 1642252800
```

**Rate Limit Exceeded:**

HTTP 429 Too Many Requests:

```json
{
  "error": "rate limit exceeded",
  "limit": 10,
  "remaining": 0,
  "reset_at": 1642252800,
  "retry_after": 3456
}
```

Headers:

```
Retry-After: 3456
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1642252800
```

## CORS Configuration

Enable CORS for browser-based applications (disabled by default):

```conductor
controller:
  http:
    cors:
      enabled: true
      allowed_origins:
        - "https://app.example.com"
        - "https://*.myapp.com"  # Wildcard subdomain
      allowed_methods:
        - GET
        - POST
        - OPTIONS
      allowed_headers:
        - Content-Type
        - Authorization
      max_age: 86400  # 24 hours
      allow_credentials: true
```

**Security Note:** Admin endpoints (`/v1/admin/*`) are automatically excluded from CORS.

## Monitoring and Metrics

When observability is enabled, endpoint metrics are exposed at `/metrics`:

**Endpoint Metrics:**

- `conductor_endpoint_requests_total{endpoint, status}` - Total requests per endpoint
- `conductor_endpoint_request_duration_seconds{endpoint}` - Request latency histogram
- `conductor_endpoint_rate_limit_exceeded_total{endpoint}` - Rate limit violations

Example query (Prometheus):

```promql
# Request rate per endpoint
rate(conductor_endpoint_requests_total[5m])

# P95 latency per endpoint
histogram_quantile(0.95, conductor_endpoint_request_duration_seconds)

# Rate limit hit rate
rate(conductor_endpoint_rate_limit_exceeded_total[1h])
```

## API Reference

### List Endpoints

```http
GET /v1/endpoints
Authorization: Bearer <api-key>
```

Response:

```json
{
  "endpoints": [
    {
      "name": "review-pr",
      "description": "Review GitHub pull requests",
      "inputs": {
        "repository": "default/repo"
      }
    }
  ]
}
```

**Note:** Only returns endpoints accessible to the authenticated key's scopes.

### Get Endpoint Details

```http
GET /v1/endpoints/{name}
Authorization: Bearer <api-key>
```

Response:

```json
{
  "name": "review-pr",
  "description": "Review GitHub pull requests",
  "inputs": {
    "repository": "default/repo",
    "severity": "medium"
  }
}
```

### Create Endpoint Run

```http
POST /v1/endpoints/{name}/runs
Authorization: Bearer <api-key>
Content-Type: application/json

{
  "inputs": {
    "key": "value"
  },
  "workspace": "optional-workspace",
  "profile": "optional-profile"
}
```

**Query Parameters:**

- `wait=true` - Synchronous mode (wait for completion)
- `timeout=60s` - Timeout for synchronous requests (max: 5m)
- `stream=true` - Enable SSE streaming (requires `wait=true`)

### List Endpoint Runs

```http
GET /v1/endpoints/{name}/runs
Authorization: Bearer <api-key>
```

Response:

```json
{
  "runs": [
    {
      "id": "run_abc123",
      "status": "completed",
      "created_at": "2025-01-15T10:30:00Z",
      "completed_at": "2025-01-15T10:31:30Z"
    }
  ]
}
```

## Best Practices

### Security

1. **Use scoped API keys** - Grant minimum necessary permissions
2. **Enable rate limiting** - Protect against abuse
3. **Validate inputs** - Define strict input schemas in workflows
4. **Keep CORS disabled** - Only enable for trusted browser applications
5. **Use HTTPS** - Always use TLS in production

### Performance

1. **Use async mode** - For long-running workflows (>30s)
2. **Set appropriate timeouts** - Match workflow execution time
3. **Monitor metrics** - Track latency and rate limits
4. **Cache workflow definitions** - Endpoints load workflows once at startup

### Workflow Design

1. **Define clear inputs** - Use type validation and defaults
2. **Return structured output** - Use JSON for programmatic consumption
3. **Handle errors gracefully** - Return meaningful error messages
4. **Keep workflows focused** - One endpoint = one responsibility

## Troubleshooting

### 404 Not Found

**Cause:** Endpoint doesn't exist OR API key lacks required scopes

**Solution:**
- Verify endpoint name: `curl http://localhost:8080/v1/endpoints`
- Check API key scopes: `conductor key list`
- Verify endpoint scopes in config match key scopes

### 400 Bad Request (Input Validation Failed)

**Cause:** Request inputs don't match workflow schema

**Solution:**
- Check error message for specific validation failure
- Review workflow input definitions
- Ensure input types match (string, number, boolean, etc.)
- Verify required inputs are provided

### 429 Too Many Requests

**Cause:** Rate limit exceeded

**Solution:**
- Check `Retry-After` header for wait time
- Increase rate limit in endpoint configuration
- Implement exponential backoff in client

### 408 Request Timeout (Synchronous Mode)

**Cause:** Workflow execution exceeded timeout

**Solution:**
- Use async mode for long-running workflows
- Increase `timeout` query parameter (max 5m)
- Poll run status using returned run ID

## Examples

### CI/CD Integration

```conductor
# GitHub Actions workflow
name: Review PR
on: pull_request

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - name: Trigger Conductor Review
        run: |
          curl -X POST "${{ secrets.CONDUCTOR_URL }}/v1/endpoints/review-pr/runs?wait=true" \
            -H "Authorization: Bearer ${{ secrets.CONDUCTOR_API_KEY }}" \
            -H "Content-Type: application/json" \
            -d "{\"inputs\":{\"pr_url\":\"${{ github.event.pull_request.html_url }}\"}}"
```

### Slack Integration

```javascript
// Slack Bolt app handler
app.command('/review-pr', async ({ command, ack, say }) => {
  await ack();

  const response = await fetch('http://conductor:8080/v1/endpoints/review-pr/runs', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${process.env.CONDUCTOR_API_KEY}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      inputs: {
        pr_url: command.text
      }
    })
  });

  const run = await response.json();
  await say(`Review started: ${run.id}`);
});
```

### Webhook Handler

```conductor
# Endpoint that processes webhooks
endpoints:
  - name: github-webhook
    workflow: webhooks/github.yaml
    scopes:
      - webhooks
    rate_limit: "1000/hour"
```

```bash
# GitHub webhook configuration
curl -X POST https://api.github.com/repos/org/repo/hooks \
  -H "Authorization: token $GITHUB_TOKEN" \
  -d '{
    "config": {
      "url": "https://conductor.example.com/v1/endpoints/github-webhook/runs",
      "content_type": "json",
      "secret": "your-webhook-secret"
    },
    "events": ["pull_request"]
  }'
```

## See Also

- [API Reference](../reference/api.md) - Complete API documentation
- [Daemon Mode](controller.md) - Running Conductor as a service
- [Profiles](profiles.md) - Runtime configuration with profiles
- [Workflow Schema](../reference/workflow-schema.md) - Workflow input/output definitions
