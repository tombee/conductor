# Debugging Workflows

Techniques for diagnosing and fixing workflow issues.

## Quick Debugging

### Verbose Logging

Enable detailed logging to see what's happening:

```bash
# Debug logging
conductor run workflow.yaml --log-level debug

# Trace logging (very verbose)
conductor run workflow.yaml --log-level trace
```

### Dry Run

Preview execution without running steps:

```bash
conductor run workflow.yaml --dry-run
```

### Step Limiting

Run only the first N steps:

```bash
conductor run workflow.yaml --step-limit 3
```

### JSON Output

Get structured output for analysis:

```bash
conductor run workflow.yaml --output json > result.json

# Inspect specific step
cat result.json | jq '.steps.step_id'

# Find failed steps
cat result.json | jq '.steps | to_entries[] | select(.value.status == "failed")'
```

## Debug Steps

Add temporary debug steps to inspect state:

```yaml
steps:
  - id: fetch_data
    http.get: "https://api.example.com/users/{{.inputs.user_id}}"

  # Debug: Write intermediate state to file
  - id: debug_data
    file.write:
      path: /tmp/debug-output.txt
      content: "{{.steps.fetch_data.body}}"

  - id: process
    model: balanced
    prompt: "Process: {{.steps.fetch_data.body}}"
```

### Conditional Debug Steps

Only run debug steps when needed:

```yaml
  - id: debug_output
    condition: 'inputs.debug == true'
    file.write:
      path: /tmp/debug.txt
      content: "{{.steps.process.response}}"
```

```bash
conductor run workflow.yaml --input debug=true
```

## Common Issues

### Template Variable Not Found

**Error:** `variable not found: steps.step_id`

**Causes:**
- Typo in step ID
- Step hasn't executed yet
- Step is in parallel block

**Fix:** Check step ID spelling and execution order.

### Empty LLM Response

**Causes:**
- Unclear prompt
- Context too long
- Temperature too low

**Fix:** Simplify prompt, reduce context, or increase temperature.

### Timeout on HTTP Request

**Fix:** Increase timeout or add retry logic:

```yaml
  - id: api_call
    http.get: "https://slow-api.example.com"
    timeout: 60
    retry:
      max_attempts: 3
```

### Workflow Hangs

**Causes:**
- Waiting for condition that's never true
- External system not responding

**Debug:**
```bash
timeout 60s conductor run workflow.yaml --log-level debug
```

## Validation

```bash
# Validate workflow syntax
conductor validate workflow.yaml

# Check YAML syntax
yamllint workflow.yaml
```

## See Also

- [Error Handling](error-handling.md) - Handle failures gracefully
- [Troubleshooting](../operations/troubleshooting.md) - Common problems and solutions
