# Frequently Asked Questions

Common questions about Conductor, answered based on real user experiences.

## Installation

### How do I install Conductor?

The easiest way is via Homebrew:

```bash
brew install conductor
```

For other methods, see the [Getting Started Guide](getting-started/).

### Why isn't the conductor command found after installation?

The conductor binary isn't in your system's PATH. Add it based on your installation method:

=== "Homebrew"

    ```bash
    echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.zshrc
    source ~/.zshrc
    ```

    On Apple Silicon:
    ```bash
    echo 'export PATH="/opt/homebrew/bin:$PATH"' >> ~/.zshrc
    source ~/.zshrc
    ```

=== "Go Install"

    ```bash
    echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
    source ~/.zshrc
    ```

See [Troubleshooting: Command Not Found](production/troubleshooting.md#command-not-found) for more details.

### Can I run Conductor in Docker?

Yes! Use the official image:

```bash
docker run --rm -v $(pwd):/workspace \
  -e ANTHROPIC_API_KEY \
  ghcr.io/tombee/conductor:latest run /workspace/workflow.yaml
```

See [Deployment Guide](production/deployment.md) for full details.

## Workflows

### What's the difference between a workflow and a step?

A **workflow** is the complete automation, defined in a YAML file with inputs, steps, and outputs. A **step** is a single action within the workflow, like calling an LLM or running a shell command.

See [Getting Started](getting-started/) for details.

### How do I reference outputs from previous steps?

Use the template syntax `{{$.step_id.field}}`:

```yaml
steps:
  - id: analyze
    type: llm
    inputs:
      prompt: "Analyze this code"

  - id: summarize
    type: llm
    inputs:
      prompt: "Summarize: {{$.analyze.response}}"
```

The `$` prefix indicates a step reference. For workflow inputs, use `{{.input_name}}` without the `$`.

### Can steps run in parallel?

Yes! Use the `parallel` step type:

```yaml
- id: reviews
  type: parallel
  max_concurrency: 3
  steps:
    - id: security_check
      type: llm
    - id: performance_check
      type: llm
    - id: style_check
      type: llm
```

See [Flow Control: Parallel Execution](building-workflows/flow-control.md#parallel-execution) for examples.

### How do I handle errors in workflows?

Configure retry and fallback behavior on individual steps:

```yaml
steps:
  - id: api_call
    type: action
    action: http
    inputs:
      url: "https://api.example.com/data"
    retry:
      max_attempts: 3
      backoff_base: 2
    on_error: continue  # or: fail, ignore
```

See [Error Handling](building-workflows/error-handling.md) for comprehensive strategies.

### Can I use environment variables in workflows?

Yes, through workflow inputs with default values from environment variables, or by using the shell connector:

```yaml
# Method 1: Environment variable as input default
inputs:
  - name: api_key
    type: string
    default: "${API_KEY}"

# Method 2: Read in a shell step
steps:
  - id: get_env
    type: action
    action: shell.run
    inputs:
      command: ["echo", "$API_KEY"]
```

For secrets, use the configuration file. See [Configuration Reference](reference/configuration.md).

## LLM Providers

### Which LLM providers does Conductor support?

Conductor supports multiple providers:

- **Claude Code** (default, auto-detected)
- **Anthropic API** (Claude models)
- **OpenAI API** (GPT models)
- **Azure OpenAI**
- **AWS Bedrock**

See [LLM Providers](architecture/llm-providers.md) for configuration details.

### Do I need Claude Code installed?

No, but it's the easiest option. Claude Code is auto-detected and doesn't require API key configuration.

Alternatively, configure API-based providers in your conductor config file. See [Configuration Reference](reference/configuration.md).

### What are model tiers (fast, balanced, strategic)?

Model tiers abstract provider-specific model names:

- **fast**: Quick responses, lower cost (Claude Haiku, GPT-3.5)
- **balanced**: Good quality/speed trade-off (Claude Sonnet, GPT-4)
- **strategic**: Best quality, slower (Claude Opus, GPT-4 Turbo)

This lets you switch providers without changing workflow files.

See [LLM Providers: Model Tiers](architecture/llm-providers.md#model-tiers) for mapping details.

### How do I reduce token costs?

Several strategies:

1. **Use appropriate model tiers** - Reserve `strategic` for complex tasks
2. **Limit output tokens** - Set `max_tokens` on LLM steps
3. **Optimize prompts** - Be concise, remove unnecessary context
4. **Cache reusable content** - Store frequently-used data

```yaml
steps:
  - id: quick_task
    type: llm
    inputs:
      model: fast  # Use fast tier when possible
      prompt: "Summarize in 3 bullet points"
      max_tokens: 200  # Limit output
```

See [Performance: Token Optimization](building-workflows/performance.md#token-optimization).

## Production

### How do I run workflows automatically?

Use **daemon mode** to run workflows on a schedule or via webhooks:

```bash
conductor daemon
```

Configure schedules in `conductord.yaml`:

```yaml
schedules:
  - cron: "0 9 * * *"  # 9 AM daily
    workflow: workflows/daily-report.yaml

webhooks:
  - path: /github
    workflow: workflows/pr-review.yaml
```

See [Daemon Mode](building-workflows/daemon-mode.md) and [Deployment Guide](production/deployment.md).

### Can I run Conductor in production?

Yes! Conductor is designed for production use. Deploy with:

- **Docker** - Containerized deployment
- **Kubernetes** - Scalable orchestration
- **Bare metal** - Direct server installation
- **exe.dev** - Cloud deployment platform

See [Deployment Guide](production/deployment.md) for best practices.

### How do I secure API keys in production?

Use environment variables or secure secret management:

```yaml
# conductord.yaml
providers:
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"  # From environment
```

In Kubernetes, use Secrets:

```yaml
env:
  - name: ANTHROPIC_API_KEY
    valueFrom:
      secretKeyRef:
        name: conductor-secrets
        key: anthropic-api-key
```

See [Security: Secrets Management](production/security.md#secrets-management).

### How do I monitor workflow execution?

Conductor provides structured logs and metrics:

```bash
# Run with debug logging
conductor run workflow.yaml --log-level debug

# Daemon mode logs
conductord --log-level info
```

For production monitoring, integrate with:
- Prometheus (metrics endpoint)
- Elasticsearch (log aggregation)
- Datadog or New Relic (APM)

See [Monitoring](production/monitoring.md) for setup details.

### What happens if a workflow fails mid-execution?

By default, workflow execution stops at the first failed step. You can configure per-step error handling:

```yaml
steps:
  - id: critical_step
    on_error: fail  # Stop workflow (default)

  - id: optional_step
    on_error: continue  # Continue to next step

  - id: logged_only
    on_error: ignore  # Don't fail, just log
```

For long-running workflows, consider implementing checkpoints to resume from failure points.

See [Error Handling: Recovery Strategies](building-workflows/error-handling.md#recovery-strategies).

## Advanced Topics

### Can I create custom connectors?

Yes! Conductor supports custom tools written in Go:

```go
// tools/custom/mytool.go
package custom

import "github.com/tombee/conductor/pkg/registry"

func init() {
    registry.RegisterTool("mytool", MyToolFunc)
}

func MyToolFunc(inputs map[string]interface{}) (interface{}, error) {
    // Implementation
}
```

See [Custom Tools Guide](contributing/custom-tools.md).

### How do I test workflows before deploying?

Use the `validate` command and run with test inputs:

```bash
# Validate syntax
conductor validate workflow.yaml

# Test with sample inputs
conductor run workflow.yaml -i user="testuser" -i action="preview"

# Dry run (no side effects)
conductor run workflow.yaml --dry-run
```

See [Testing Workflows](building-workflows/testing.md) for test strategies.

### Can workflows call other workflows?

Yes, use the workflow connector:

```yaml
steps:
  - id: run_subworkflow
    type: action
    action: workflow.run
    inputs:
      path: "workflows/helper.yaml"
      inputs:
        param1: "value"
```

See [Workflow Composition](building-workflows/flow-control.md#workflow-composition).

## Getting More Help

Still have questions?

- **Documentation**: Browse the full [Documentation](index.md)
- **Examples**: See real workflows in [Examples](examples/index.md)
- **GitHub Discussions**: Ask the community at [github.com/tombee/conductor/discussions](https://github.com/tombee/conductor/discussions)
- **GitHub Issues**: Report bugs at [github.com/tombee/conductor/issues](https://github.com/tombee/conductor/issues)
- **Troubleshooting**: Check [Troubleshooting Guide](production/troubleshooting.md)
