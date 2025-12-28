# Error Handling

Build resilient workflows that handle failures gracefully.

## Error Strategies

Control what happens when a step fails:

```conductor
steps:
  - id: risky_operation
    type: llm
    prompt: "Analyze this data..."
    on_error:
      strategy: retry
      max_attempts: 3
      backoff: exponential
```

**Available strategies:**
- **`fail`** (default): Stop workflow execution
- **`retry`**: Retry the step with backoff
- **`fallback`**: Use an alternative step
- **`ignore`**: Continue despite the error

## Retry with Backoff

Retry transient failures automatically:

```conductor
steps:
  - id: api_call
    http.get:
      url: "https://api.example.com/data"
    on_error:
      strategy: retry
      max_attempts: 3
      backoff: exponential
      initial_delay: 1s
      max_delay: 30s
```

**Backoff types:**
- **`fixed`**: Same delay between retries
- **`exponential`**: Doubling delay (1s, 2s, 4s, 8s, ...)
- **`linear`**: Incrementing delay (1s, 2s, 3s, 4s, ...)

## Fallback Steps

Provide an alternative when a step fails:

```conductor
steps:
  - id: primary_llm
    type: llm
    model: powerful
    prompt: "Analyze this complex data..."
    on_error:
      strategy: fallback
      fallback_step: backup_llm

  - id: backup_llm
    type: llm
    model: fast
    prompt: "Provide a basic analysis of this data..."
```

## Ignoring Errors

Continue execution even if a step fails:

```conductor
steps:
  - id: optional_notification
    slack.post_message:
      channel: "#alerts"
      text: "Workflow started"
    on_error:
      strategy: ignore  # Don't fail workflow if Slack is down

  - id: critical_work
    type: llm
    prompt: "Do the important work..."
```

## Conditional Error Handling

Different strategies based on error type:

```conductor
steps:
  - id: llm_call
    type: llm
    prompt: "Process this..."
    on_error:
      - condition: 'error.type == "rate_limit"'
        strategy: retry
        max_attempts: 5
        backoff: exponential

      - condition: 'error.type == "provider_error"'
        strategy: fallback
        fallback_step: backup_provider

      - default:
        strategy: fail
```

## Error Context

Access error information in subsequent steps:

```conductor
steps:
  - id: risky_step
    type: llm
    prompt: "..."
    on_error:
      strategy: ignore

  - id: check_result
    type: llm
    condition: '!steps.risky_step.error'
    prompt: "Process the successful result..."

  - id: handle_failure
    type: llm
    condition: 'steps.risky_step.error'
    prompt: |
      The previous step failed with: {{.steps.risky_step.error.message}}
      Provide an alternative approach.
```

**Available error fields:**
- `{{.steps.id.error.message}}` — Error description
- `{{.steps.id.error.type}}` — Error category
- `{{.steps.id.error}}` — Boolean (true if step failed)

## Timeouts

Prevent steps from hanging:

```conductor
steps:
  - id: long_running
    type: llm
    model: powerful
    prompt: "Complex analysis..."
    timeout: 5m
    on_error:
      strategy: fallback
      fallback_step: quick_alternative
```

**Timeout formats:**
- `30s` — 30 seconds
- `5m` — 5 minutes
- `1h` — 1 hour

## Validation Errors

Catch validation errors before execution:

```conductor
inputs:
  - name: email
    type: string
    required: true
    validation:
      pattern: '^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
      error_message: "Must be a valid email address"

  - name: age
    type: number
    validation:
      min: 0
      max: 120
      error_message: "Age must be between 0 and 120"
```

Conductor validates inputs before running the workflow, providing immediate feedback.

## Common Error Types

**Provider errors:**
- `rate_limit` — Hit API rate limits
- `authentication` — Invalid API key
- `provider_error` — LLM service failure

**Tool errors:**
- `file_not_found` — File doesn't exist
- `permission_denied` — Insufficient permissions
- `command_failed` — Shell command exited with error

**Workflow errors:**
- `validation_error` — Invalid workflow definition
- `timeout` — Step exceeded time limit
- `template_error` — Invalid template syntax

## Best Practices

**1. Retry transient failures:**
```conductor
# Good: Retry API calls that might fail temporarily
on_error:
  strategy: retry
  max_attempts: 3
```

**2. Use fallbacks for critical paths:**
```conductor
# Good: Provide backup for critical steps
on_error:
  strategy: fallback
  fallback_step: backup_step
```

**3. Fail fast for validation:**
```conductor
# Good: Validate inputs immediately
inputs:
  - name: file_path
    required: true
    validation:
      exists: true
```

**4. Ignore optional steps:**
```conductor
# Good: Don't fail workflow for optional notifications
- id: notify_slack
  on_error:
    strategy: ignore
```

**5. Set reasonable timeouts:**
```conductor
# Good: Prevent infinite hangs
timeout: 5m
on_error:
  strategy: fail
```

## Troubleshooting

**Workflow keeps retrying endlessly:**

Check `max_attempts` is set:
```conductor
on_error:
  strategy: retry
  max_attempts: 3  # Add this
```

**Errors are silently ignored:**

Verify `on_error` is intentional:
```conductor
on_error:
  strategy: ignore  # Remove if you want failures to stop workflow
```

**Fallback step never runs:**

Ensure fallback step ID matches:
```conductor
on_error:
  strategy: fallback
  fallback_step: backup_step  # Must match a step ID

- id: backup_step  # Correct ID
```

See [Troubleshooting Guide](../production/troubleshooting.md) for more help.
