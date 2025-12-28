# Troubleshooting

Common issues and solutions for Conductor.

## Installation Issues

### "conductor: command not found"

**Solution:** Add installation directory to PATH:

```bash
# For Homebrew
echo 'export PATH="/opt/homebrew/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc

# For Go install
export PATH=$PATH:$(go env GOPATH)/bin
```

### "Provider not configured"

**Solution:** Install and configure Claude Code:

```bash
# Check if Claude Code is installed
claude --version

# If not, download from https://claude.ai/download
```

Or configure another LLM provider in `~/.config/conductor/config.yaml`.

## Workflow Validation Errors

### "Invalid YAML syntax"

YAML is whitespace-sensitive. Common issues:

```conductor
# Wrong: tabs instead of spaces
steps:
	-id: analyze

# Wrong: inconsistent indentation
steps:
  - id: analyze
   type: llm

# Correct: 2-space indentation
steps:
  - id: analyze
    type: llm
```

**Validate before running:**
```bash
conductor validate workflow.yaml
```

### "Missing required field"

Every step needs `id` and `type`:

```conductor
# Wrong: missing id
- type: llm
  prompt: "..."

# Correct
- id: analyze
  type: llm
  prompt: "..."
```

### "Invalid template syntax"

Use correct template variable syntax:

```conductor
# Wrong
prompt: "Analyze ${inputs.code}"
prompt: "Analyze {{code}}"

# Correct
prompt: "Analyze {{.inputs.code}}"
prompt: "Use {{.steps.analyze.response}}"
```

## Runtime Errors

### "Rate limit exceeded"

**Cause:** Hit LLM provider's rate limit.

**Solutions:**
- Wait a moment and retry
- Use slower model tier (`fast` instead of `powerful`)
- Add delays between steps
- Check provider rate limits

### "Timeout exceeded"

**Cause:** Step took longer than timeout limit.

**Solutions:**
- Increase timeout:
  ```yaml
  - id: slow_step
    timeout: 10m
  ```
- Break into smaller steps
- Use faster model tier

### "Template rendering failed"

**Cause:** Referenced variable doesn't exist.

**Solution:** Check variable paths:

```conductor
# Verify step IDs match
- id: analyze
  ...

- id: summarize
  prompt: "{{.steps.analyze.response}}"  # Must match ID above
```

## Daemon Issues

### "Port already in use"

**Solution:** Check what's using the port:

```bash
lsof -i :9000
```

Kill the process or use a different port:
```bash
conductord --tcp=:9001
```

### "Workflows not loading"

**Solution:** Check workflows directory:

```bash
# Verify files exist
ls -la ./workflows/*.yaml

# Check for validation errors
conductor validate ./workflows/*.yaml

# Enable debug logging
conductord --workflows-dir=./workflows --log-level=debug
```

### "Webhook not triggering"

**Solutions:**
1. Verify daemon is accessible:
   ```bash
   curl http://your-server:9000/health
   ```

2. Check webhook delivery in GitHub/Slack settings

3. Verify webhook URL matches workflow name exactly

4. Check logs for errors:
   ```bash
   sudo journalctl -u conductor -f | grep webhook
   ```

## Performance Issues

### "Workflows running slowly"

**Solutions:**
- Use faster model tier for simple tasks
- Enable parallel execution for independent steps
- Cache results when possible
- Check network latency to LLM provider

### "High memory usage"

**Solutions:**
- Limit concurrent workflows:
  ```yaml
  daemon:
    max_concurrent_runs: 5
  ```
- Avoid storing large outputs in variables
- Process large files in chunks

## Error Codes

Common error codes and meanings:

- `E001` — Workflow validation failed
- `E002` — Missing required input
- `E003` — Step execution failed
- `E004` — Template rendering error
- `E005` — Provider authentication failed
- `E006` — Rate limit exceeded
- `E007` — Timeout exceeded
- `E008` — File not found
- `E009` — Permission denied
- `E010` — Invalid configuration

See [Error Codes Reference](../reference/error-codes.md) for complete list.

## Getting Help

If you're still stuck:

1. **Check logs:**
   ```bash
   conductor run workflow.yaml --log-level=debug
   ```

2. **Search existing issues:**
   https://github.com/tombee/conductor/issues

3. **Ask in discussions:**
   https://github.com/tombee/conductor/discussions

4. **Open an issue:**
   Include workflow YAML, error messages, and logs
