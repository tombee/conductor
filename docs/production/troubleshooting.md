# Troubleshooting

This guide covers common issues you may encounter with Conductor and how to resolve them.

## Installation Issues

### Command Not Found

**Symptoms:**

```bash
$ conductor
bash: conductor: command not found
```

**Cause:** The conductor binary is not in your system's PATH.

**Solutions:**

=== "Homebrew Installation"

    If you installed via Homebrew, ensure the Homebrew bin directory is in your PATH:

    ```bash
    echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.zshrc
    source ~/.zshrc
    ```

    On Apple Silicon Macs:

    ```bash
    echo 'export PATH="/opt/homebrew/bin:$PATH"' >> ~/.zshrc
    source ~/.zshrc
    ```

=== "Go Install"

    Ensure `$GOPATH/bin` is in your PATH:

    ```bash
    export PATH=$PATH:$(go env GOPATH)/bin

    # Make it permanent
    echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
    source ~/.zshrc
    ```

=== "Binary Download"

    Add the directory containing the conductor binary to your PATH:

    ```bash
    export PATH=$PATH:/usr/local/bin

    # Make it permanent
    echo 'export PATH=$PATH:/usr/local/bin' >> ~/.zshrc
    source ~/.zshrc
    ```

:::tip[Verify PATH Changes]
After updating your PATH, verify it worked:

```bash
which conductor
conductor --version
```
:::


### Permission Denied

**Symptoms:**

```bash
$ conductor
bash: /usr/local/bin/conductor: Permission denied
```

**Cause:** The binary doesn't have execute permissions.

**Solution:**

```bash
chmod +x /usr/local/bin/conductor
```

If you don't have sudo access, install to a user-local directory:

```bash
mkdir -p ~/.local/bin
mv conductor ~/.local/bin/
echo 'export PATH=$PATH:$HOME/.local/bin' >> ~/.zshrc
source ~/.zshrc
```

### Go Version Too Old

**Symptoms:**

```bash
go install github.com/tombee/conductor/cmd/conductor@latest
go: github.com/tombee/conductor/cmd/conductor@latest:
    module requires go >= 1.21
```

**Cause:** Your Go version is too old.

**Solution:**

Update Go to version 1.21 or later:

=== "macOS (Homebrew)"

    ```bash
    brew upgrade go
    go version
    ```

=== "Linux"

    ```bash
    # Download latest Go
    wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz

    # Remove old version and install new
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

    # Verify
    go version
    ```

=== "Windows"

    Download and install the latest MSI from [go.dev/dl](https://go.dev/dl/)

## Provider Issues

### Provider Not Configured

**Symptoms:**

```bash
$ conductor run workflow.yaml
Error: no provider configured
```

**Cause:** No LLM provider is available.

**Solution:** Install and configure Claude Code:

```bash
# Check if Claude Code is installed
claude --version

# If not installed, download from https://claude.ai/download
```

Once Claude Code is installed and signed in, Conductor will automatically detect and use it.

:::tip[Alternative Providers]
For API-based providers, see the [Configuration Reference](../reference/configuration.md).
:::


### Authentication Failed

**Symptoms:**

```bash
Error: authentication failed
```

**Cause:** Claude Code isn't signed in, or API key is invalid (if using API providers).

**Solutions:**

1. **For Claude Code:** Re-authenticate by running `claude` and following the sign-in prompts

2. **For API providers:** Verify your API key is correct and not expired. See [Configuration Reference](../reference/configuration.md).

### Rate Limit Exceeded

**Symptoms:**

```bash
Error: rate limit exceeded: too many requests
```

**Cause:** You've exceeded the API provider's rate limits.

**Solutions:**

:::tip[Add Retry Logic]

Configure automatic retries in your workflow:

```yaml
steps:
  - id: llm_call
    type: llm
    inputs:
      model: fast
      prompt: "Your prompt"
    retry:
      max_attempts: 5
      backoff_base: 2
      backoff_multiplier: 2.0
```
:::


:::note[Upgrade API Plan]

Check your API provider's rate limits and consider upgrading:

- **Anthropic:** [Rate limits documentation](https://docs.anthropic.com/claude/reference/rate-limits)
- **OpenAI:** [Rate limits documentation](https://platform.openai.com/docs/guides/rate-limits)
:::


:::note[Use Lower Tier Models]

Switch to faster models that have higher rate limits:

```yaml
inputs:
  model: fast  # Instead of strategic
```
:::


## Workflow Errors

### Workflow Validation Failed

**Symptoms:**

```bash
$ conductor run workflow.yaml
Error: workflow validation failed: missing required field "name"
```

**Cause:** The workflow YAML is missing required fields or has syntax errors.

**Solutions:**

1. **Validate the workflow explicitly:**

   ```bash
   conductor validate workflow.yaml
   ```

2. **Check for common YAML errors:**

   - Missing `name` field
   - Incorrect indentation (YAML requires consistent spaces, not tabs)
   - Duplicate step IDs
   - Missing required step fields

3. **Use the schema reference:**

   See the [Workflow Schema Reference](../reference/workflow-schema.md) for complete field requirements.

**Common validation errors:**

??? example "Missing Required Fields"

    ```yaml
    # ❌ Missing name
    steps:
      - id: greet
        type: llm

    # ✅ Correct
    name: hello-world
    steps:
      - id: greet
        type: llm
        inputs:
          model: fast
          prompt: "Hello"
    ```

??? example "Duplicate Step IDs"

    ```yaml
    # ❌ Duplicate IDs
    steps:
      - id: analyze
        type: llm
      - id: analyze  # Duplicate!
        type: llm

    # ✅ Unique IDs
    steps:
      - id: analyze_content
        type: llm
      - id: analyze_structure
        type: llm
    ```

??? example "Indentation Errors"

    ```yaml
    # ❌ Inconsistent indentation
    steps:
    - id: step1
      type: llm
       inputs:  # Extra space
         model: fast

    # ✅ Consistent indentation (2 spaces)
    steps:
      - id: step1
        type: llm
        inputs:
          model: fast
    ```

### Step Execution Failed

**Symptoms:**

```bash
Error: step "read_file" failed: file not found
```

**Cause:** A step encountered an error during execution.

**Solutions:**

1. **Check step inputs:**

   Verify that inputs to the step are correct:

   ```yaml
   - id: read_file
     type: action
     action: file.read
     inputs:
       path: "{{.file_path}}"  # Verify this path exists
   ```

2. **Add error handling:**

   Configure retry or fallback behavior:

   ```yaml
   - id: risky_step
     type: action
     action: http
     inputs:
       url: "https://api.example.com"
     retry:
       max_attempts: 3
     on_error: continue  # Or: fail, ignore
   ```

3. **Check logs for details:**

   Run with verbose logging:

   ```bash
   conductor run workflow.yaml --log-level debug
   ```

### Template Variable Not Found

**Symptoms:**

```bash
Error: template variable "username" not found
```

**Cause:** The workflow references an input or step output that doesn't exist.

**Solutions:**

1. **Verify input is defined:**

   ```yaml
   inputs:
     - name: username  # Must be defined here
       type: string
       required: true

   steps:
     - id: greet
       type: llm
       inputs:
         prompt: "Hello {{.username}}"  # Matches input name
   ```

2. **Check step references use correct syntax:**

   - Inputs: `{{.input_name}}`
   - Step outputs: `{{$.step_id.field}}`

   ```yaml
   steps:
     - id: step1
       type: llm
       inputs:
         prompt: "Hello"

     - id: step2
       type: llm
       inputs:
         # ✅ Correct
         prompt: "Previous output: {{$.step1.content}}"

         # ❌ Incorrect
         prompt: "Previous output: {{.step1.content}}"
   ```

3. **Ensure referenced step has executed:**

   Steps can only reference outputs from previous steps:

   ```yaml
   # ❌ Wrong order
   steps:
     - id: step2
       inputs:
         data: "{{$.step1.output}}"  # step1 hasn't run yet!

     - id: step1
       type: llm

   # ✅ Correct order
   steps:
     - id: step1
       type: llm

     - id: step2
       inputs:
         data: "{{$.step1.content}}"
   ```

### Timeout Exceeded

**Symptoms:**

```bash
Error: step "analyze" exceeded timeout of 30s
```

**Cause:** The step took longer to execute than the configured timeout.

**Solutions:**

1. **Increase the timeout:**

   ```yaml
   steps:
     - id: analyze
       type: llm
       inputs:
         model: balanced
         prompt: "Long analysis task..."
       timeout: 120  # Increase to 2 minutes
   ```

2. **Use a faster model tier:**

   ```yaml
   inputs:
     model: fast  # Instead of strategic
   ```

3. **Simplify the prompt:**

   Break complex prompts into smaller steps:

   ```yaml
   steps:
     # Instead of one complex step
     - id: extract
       type: llm
       inputs:
         model: fast
         prompt: "Extract key points"

     - id: analyze
       type: llm
       inputs:
         model: balanced
         prompt: "Analyze: {{$.extract.content}}"
   ```

## Network and Connectivity Issues

### Connection Timeout

**Symptoms:**

```bash
Error: connection timeout: failed to reach api.anthropic.com
```

**Cause:** Network connectivity issues or firewall blocking access to the API.

**Solutions:**

1. **Check internet connection:**

   ```bash
   ping -c 3 api.anthropic.com
   ```

2. **Check proxy settings:**

   If behind a corporate proxy:

   ```bash
   export HTTP_PROXY="http://proxy.company.com:8080"
   export HTTPS_PROXY="http://proxy.company.com:8080"
   ```

3. **Verify firewall rules:**

   Ensure your firewall allows outbound HTTPS connections to:
   - `api.anthropic.com` (Anthropic)
   - `api.openai.com` (OpenAI)

4. **Test API connectivity:**

   ```bash
   curl -v https://api.anthropic.com/v1/messages \
     -H "x-api-key: $ANTHROPIC_API_KEY" \
     -H "anthropic-version: 2023-06-01"
   ```

### SSL Certificate Errors

**Symptoms:**

```bash
Error: x509: certificate signed by unknown authority
```

**Cause:** SSL certificate validation issues, common in corporate environments.

**Solutions:**

:::caution[Security Risk]
Disabling certificate verification is a security risk. Only use this as a last resort in trusted environments.
:::


For testing only:

```bash
export CONDUCTOR_SKIP_TLS_VERIFY=true
```

Better solution: Install corporate CA certificates:

=== "macOS"

    Add the certificate to Keychain Access and trust it.

=== "Linux"

    ```bash
    sudo cp corporate-ca.crt /usr/local/share/ca-certificates/
    sudo update-ca-certificates
    ```

## Performance Issues

### Workflow Running Slowly

**Symptoms:**

Workflows take much longer than expected.

**Solutions:**

1. **Use appropriate model tiers:**

   ```yaml
   # Use fast tier for simple tasks
   - id: simple_task
     type: llm
     inputs:
       model: fast  # Much faster than strategic
   ```

2. **Optimize prompts:**

   - Remove unnecessary context
   - Be concise and specific
   - Use smaller `max_tokens` values

   ```yaml
   inputs:
     model: fast
     prompt: "Summarize in 3 bullet points"
     max_tokens: 200  # Limit output length
   ```

3. **Use parallel execution (when available):**

   ```yaml
   - id: parallel_analysis
     type: parallel
     steps:
       - id: check1
         type: llm
       - id: check2
         type: llm
   ```

### High Token Usage

**Symptoms:**

Workflows consume more tokens than expected, increasing costs.

**Solutions:**

:::tip[Monitor Token Usage]

Check token usage in outputs:

```yaml
outputs:
  - name: total_tokens
    value: $.analyze.usage.total_tokens
  - name: cost
    value: $.analyze.cost
```
:::


:::note[Reduce Prompt Size]

- Remove verbose system prompts
- Trim unnecessary context
- Use focused prompts

```yaml
# ❌ Verbose
system: |
  You are an expert assistant with deep knowledge of...
  [500 words of instructions]

# ✅ Concise
system: "Analyze code for security issues"
```
:::


:::note[Use Lower Tiers When Possible]

Reserve `strategic` tier for complex tasks:

```yaml
# Simple extraction: use fast
- id: extract
  inputs:
    model: fast

# Complex reasoning: use strategic
- id: analyze
  inputs:
    model: strategic
```
:::


## Daemon Mode Issues

### Daemon Won't Start

**Symptoms:**

```bash
$ conductord
Error: failed to start daemon: address already in use
```

**Cause:** Port 8080 is already in use.

**Solutions:**

1. **Check what's using the port:**

   ```bash
   lsof -i :8080
   ```

2. **Stop the conflicting process:**

   ```bash
   # If another conductord instance
   pkill conductord

   # If another service
   # Stop it or use a different port
   ```

3. **Use a different port:**

   ```bash
   conductord --port 8081
   ```

   Or in config:

   ```yaml
   # conductord.yaml
   server:
     port: 8081
   ```

### Webhooks Not Triggering

**Symptoms:**

Workflows don't execute when webhooks are received.

**Solutions:**

1. **Verify webhook configuration:**

   ```yaml
   # conductord.yaml
   webhooks:
     - path: /github
       workflow: workflows/pr-review.yaml
       secret: your-webhook-secret
   ```

2. **Check webhook URL is correct:**

   ```bash
   # Test webhook locally
   curl -X POST http://localhost:8080/github \
     -H "Content-Type: application/json" \
     -d '{"test": "data"}'
   ```

3. **Verify webhook secret:**

   Ensure the secret matches between GitHub and your config.

4. **Check daemon logs:**

   ```bash
   conductord --log-level debug
   ```

## Getting Help

If you're still stuck after trying these solutions:

### Check Documentation

- [Quick Start Guide](../quick-start.md) - Getting started
- [Workflow Schema Reference](../reference/workflow-schema.md) - Complete YAML reference
- [CLI Reference](../reference/cli.md) - Command-line options
- [Examples](../examples/index.md) - Working workflow examples

### Community Support

- **GitHub Issues**: [Report bugs](https://github.com/tombee/conductor/issues)
- **GitHub Discussions**: [Ask questions](https://github.com/tombee/conductor/discussions)
- **Stack Overflow**: Tag questions with `conductor-workflow`

### Provide Debug Information

When asking for help, include:

1. **Conductor version:**

   ```bash
   conductor --version
   ```

2. **Your workflow (sanitized):**

   Remove sensitive data like API keys, then share the YAML.

3. **Full error message:**

   ```bash
   conductor run workflow.yaml --log-level debug
   ```

4. **Environment details:**

   - Operating system
   - Installation method (Homebrew, Go install, etc.)
   - Go version (if relevant)

5. **Steps to reproduce:**

   Minimal steps that trigger the issue.

:::tip[Debug Mode]
Running with `--log-level debug` provides detailed information about what Conductor is doing, which can help identify issues.
:::


## Related Resources

- [Error Handling Guide](../building-workflows/error-handling.md) - Retry and fallback strategies
- [Configuration Reference](../reference/configuration.md) - API keys and providers
- [Workflows and Steps](../getting-started/concepts.md) - Workflow structure
