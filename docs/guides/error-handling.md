# Error Handling

Strategies for building robust workflows that handle failures gracefully.

## Error Strategies

Conductor provides four strategies for step failures:

### Fail (Default)

Stop workflow immediately:

```yaml
  - id: critical_step
    http.post: "https://api.example.com/critical"
    on_error:
      strategy: fail
```

### Ignore

Continue despite errors:

```yaml
  - id: optional_notification
    http.post: "https://slack.example.com/webhook"
    on_error:
      strategy: ignore
```

### Retry

Retry with exponential backoff:

```yaml
  - id: flaky_api
    http.get: "https://flaky-api.example.com"
    retry:
      max_attempts: 3
      backoff_base: 2
      backoff_multiplier: 2.0
```

Backoff calculation: `delay = base * (multiplier ^ attempt)`

### Fallback

Execute alternative step:

```yaml
  - id: primary_api
    http.get: "https://primary.example.com/data"
    on_error:
      strategy: fallback
      fallback_step: backup_api

  - id: backup_api
    http.get: "https://backup.example.com/data"
```

## Timeout Configuration

Set maximum execution time:

```yaml
  - id: slow_api
    http.get: "https://slow-api.example.com"
    timeout: 60  # seconds
```

Recommended timeouts:
- LLM calls: 30-120 seconds
- HTTP APIs: 10-60 seconds
- File operations: 5-30 seconds

## Common Patterns

### Critical Path with Retry

```yaml
  - id: save_data
    http.post: "https://db.example.com/save"
    timeout: 30
    retry:
      max_attempts: 3
      backoff_base: 2
      backoff_multiplier: 2.0
    on_error:
      strategy: fail  # Stop if all retries fail
```

### Optional Best-Effort

```yaml
  - id: send_notification
    http.post: "https://notifications.example.com"
    timeout: 5
    retry:
      max_attempts: 2
      backoff_base: 1
      backoff_multiplier: 1.5
    on_error:
      strategy: ignore
```

## Best Practices

1. **Use exponential backoff** - Avoid overwhelming failing services
2. **Limit retry attempts** - 3-5 for transient failures, 2-3 for critical operations
3. **Set appropriate timeouts** - Match expected operation duration
4. **Fail fast for invalid inputs** - Don't retry validation errors
5. **Consider total time** - Account for retry overhead in workflow time

## See Also

- [Debugging](debugging.md) - Diagnose workflow issues
- [Troubleshooting](../operations/troubleshooting.md) - Common problems
