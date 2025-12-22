# Webhook Handler Example Workflow

A workflow for processing and analyzing webhook events with validation, insights extraction, and notification generation.

## Description

This workflow demonstrates a complete webhook event processing pipeline:

1. **Validation** - Validates event structure and required fields
2. **Processing** - Extracts insights and entities from event data
3. **Notification** - Generates formatted notifications for various channels

It's designed to be used with the webhook receiver recipes, showing how to:

- Handle arbitrary webhook payloads
- Validate event structure
- Extract actionable insights with LLM
- Generate multi-channel notifications
- Use conditional execution

## Inputs

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `event_type` | string | Yes | Type of webhook event (created/updated/deleted) |
| `event_id` | string | Yes | Unique event identifier |
| `timestamp` | string | No | Event timestamp |
| `data` | object | Yes | Event payload data |

## Outputs

| Name | Description |
|------|-------------|
| `validation_result` | Event validation result (JSON) |
| `processing_result` | Processed event insights (JSON) |
| `notification` | Generated notification messages for Slack/email/alerts |
| `event_id` | Original event ID for correlation |

## Usage

### Via CLI

```bash
conductor run examples/recipes/webhook-handler/workflow.yaml \
  --input event_type="created" \
  --input event_id="evt_123abc" \
  --input timestamp="2025-12-25T10:00:00Z" \
  --input data='{"resource":"user","action":"signup","user_id":"user_456"}'
```

### Via API

```bash
curl -X POST \
  -H "X-API-Key: ${CONDUCTOR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": "webhook-handler",
    "inputs": {
      "event_type": "created",
      "event_id": "evt_123abc",
      "timestamp": "2025-12-25T10:00:00Z",
      "data": {
        "resource": "user",
        "action": "signup",
        "user_id": "user_456",
        "email": "alice@example.com"
      }
    }
  }' \
  http://localhost:9000/api/v1/runs
```

### Via Webhook Receiver

With the Python webhook receiver from the recipes:

```bash
# Compute signature
payload='{"type":"created","id":"evt_123","timestamp":"2025-12-25T10:00:00Z","data":{"resource":"user","action":"signup"}}'
signature=$(echo -n "$payload" | openssl dgst -sha256 -hmac "your-secret" | sed 's/^.* //')

# Send webhook
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: sha256=$signature" \
  -d "$payload" \
  http://localhost:8080/webhook
```

## Example Event Payloads

### User Signup Event

```json
{
  "event_type": "created",
  "event_id": "evt_user_signup_123",
  "timestamp": "2025-12-25T10:00:00Z",
  "data": {
    "resource": "user",
    "action": "signup",
    "user_id": "user_456",
    "email": "alice@example.com",
    "plan": "pro",
    "referrer": "organic"
  }
}
```

### Pull Request Event

```json
{
  "event_type": "updated",
  "event_id": "evt_pr_123",
  "timestamp": "2025-12-25T11:30:00Z",
  "data": {
    "resource": "pull_request",
    "action": "synchronized",
    "pr_number": 42,
    "repo": "org/repo",
    "author": "alice",
    "commits": 3
  }
}
```

### Payment Event

```json
{
  "event_type": "created",
  "event_id": "evt_payment_789",
  "timestamp": "2025-12-25T14:20:00Z",
  "data": {
    "resource": "payment",
    "action": "succeeded",
    "amount": 9900,
    "currency": "USD",
    "customer_id": "cus_abc123",
    "subscription_id": "sub_xyz789"
  }
}
```

## Expected Output

```json
{
  "validation_result": {
    "valid": true,
    "errors": []
  },
  "processing_result": {
    "summary": "User signed up for pro plan via organic referrer",
    "entities": ["user_456", "alice@example.com"],
    "actions": ["Send welcome email", "Activate pro features"],
    "priority": "medium"
  },
  "notification": {
    "slack": "🎉 New user signup: alice@example.com (Pro plan)",
    "email_subject": "New Pro Plan Signup - user_456",
    "alert": "User signup: pro plan"
  },
  "event_id": "evt_user_signup_123"
}
```

## Use Cases

- Testing webhook receiver recipes
- Demonstrating event-driven automation
- Validating webhook payload mapping
- Example for LLM-based event processing
- Multi-channel notification patterns

## Customization

### Custom Validation Logic

Modify the `validate_event` step to add your validation rules:

```yaml
- id: validate_event
  type: llm
  inputs:
    prompt: |
      Validate webhook event for MY_SERVICE:
      - Must have event_type in [created, updated, deleted]
      - Must have valid event_id format: evt_[a-z0-9]+
      - Data must contain MY_REQUIRED_FIELD
```

### Custom Processing Logic

Adapt the `process_event` step for your use case:

```yaml
- id: process_event
  type: llm
  inputs:
    system: "You are a payment event analyzer"
    prompt: |
      Analyze this payment event:
      {{ .data | toJson }}

      Extract:
      - Payment status and amount
      - Customer information
      - Risk indicators
```

## See Also

- [Webhook Receivers Recipe](../../../docs/recipes/automation/webhooks.md)
- [CLI Scripting Recipe](../../../docs/recipes/automation/cli-scripting.md)
- [Daemon Mode - Webhooks](../../../docs/guides/daemon-mode.md#webhooks)
