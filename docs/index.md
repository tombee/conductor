# Conductor

**A production-ready platform for AI agent workflows**

Define agent workflows in YAML. Get observability, security, cost controls, and flexible deployment built-in—so you can focus on what your agents actually do.

---

## Why Conductor?

- **Focus on workflow logic** — Define what your agents do in YAML. The platform handles retries, fallbacks, and error handling.
- **Production-ready** — Observability, cost tracking, and security are built into the platform.
- **Flexible deployment** — Run the same workflow from the CLI, as an API, on a schedule, or triggered by webhooks.
- **Ops-friendly controls** — Security profiles, cost limits, secret management, and container sandboxing give teams the governance they need.
- **Declarative connectors** — Connect to GitHub, Slack, Jira, and more with configuration, not code.
- **Any LLM provider** — Use Anthropic, OpenAI, Ollama, or others. Swap providers without changing workflow logic.
- **Deterministic by default** — Connectors and tools handle API calls, file operations, and integrations. LLMs focus on reasoning and summarization—keeping workflows fast and costs low.
- **MCP support** — When LLMs need to call tools, add MCP servers. Conductor manages their lifecycle alongside your workflows.

---

## Quick Example

Write a song generator in 20 lines of YAML:

```yaml
# song.yaml
name: write-song
inputs:
  - name: genre
    type: string
    required: true
  - name: topic
    type: string
    required: true
  - name: key
    type: string
    default: "C Major"

steps:
  - id: compose
    type: llm
    model: balanced
    prompt: |
      Write a short {{.inputs.genre}} song about "{{.inputs.topic}}" in the key of {{.inputs.key}}.

      Use a song structure authentic to {{.inputs.genre}} (e.g., 12-bar blues, verse-chorus,
      AABA, etc.). Include chord symbols above the lyrics, using chords diatonic to
      {{.inputs.key}} with progressions that fit the style.
```

Run it:

```bash
$ conductor run song.yaml

genre: blues
topic: morning coffee
key: C Major
```

The LLM generates a 12-bar blues with AAB lyrics, dominant 7th chords, and the classic I-IV-V progression. Try "folk" or "country" for different structures.

---

## Real-World Use Cases

### Code Review Automation

Run multi-persona code reviews on every branch. Get security, performance, and style feedback in parallel.

```bash
conductor run examples/code-review
# → Analyzes your changes with multiple AI reviewers
# → Generates prioritized feedback
```

**Key features:** Parallel execution, shell tool integration, structured output

---

### Slack Integration

Send workflow results to Slack channels. Build bots that respond to messages or schedule reports.

```yaml
steps:
  - id: analyze
    type: llm
    prompt: "Summarize today's pull requests..."

  - id: notify
    slack.post_message:
      channel: "#engineering"
      text: "{{.steps.analyze.response}}"
```

**Key features:** Connector shorthand syntax, webhook support, message formatting

---

## How It Works

1. **Define steps** — Each step runs in order. Reference outputs from previous steps.
2. **Template syntax** — Use `{{.inputs.name}}` for inputs, `{{.steps.id.response}}` for outputs.
3. **LLM steps** — Set `type: llm` to call your configured provider (Claude, GPT, Gemini, etc.).
4. **Tool steps** — Use built-in tools (`file`, `http`, `shell`) or connectors (`github`, `slack`, `jira`).
5. **Parallel steps** — Set `type: parallel` to run multiple steps concurrently.

[Learn More →](learn/concepts/workflows-steps.md){ .md-button .md-button--primary }

---

## Getting Started

### 1. Install Conductor

=== "Homebrew (macOS/Linux)"

    ```bash
    brew install conductor
    ```

=== "Go Install"

    ```bash
    go install github.com/tombee/conductor/cmd/conductor@latest
    ```

=== "From Source"

    ```bash
    git clone https://github.com/tombee/conductor
    cd conductor
    make install
    ```

### 2. Run Your First Workflow

```bash
conductor run examples/write-song/workflow.yaml
```

Conductor prompts you for inputs. In seconds, you'll get a complete song with genre-appropriate structure and chord progressions.

### 3. Learn More

- **[Quick Start](quick-start.md)** — Get started quickly
- **[Tutorial: First Workflow](learn/tutorials/first-workflow.md)** — Build a workflow from scratch
- **[Workflows and Steps](learn/concepts/workflows-steps.md)** — Understand how workflows work

---

## Documentation

### Learn

- [Installation](learn/installation.md) — Install Conductor
- [Workflows and Steps](learn/concepts/workflows-steps.md) — How workflows are structured
- [Inputs and Outputs](learn/concepts/inputs-outputs.md) — Pass data into and out of workflows
- [Template Variables](learn/concepts/template-variables.md) — Use `{{}}` syntax to reference data

### Tutorials

- [First Workflow](learn/tutorials/first-workflow.md) — Build a workflow from scratch
- [Code Review Bot](learn/tutorials/code-review-bot.md) — Multi-persona code review
- [Slack Integration](learn/tutorials/slack-integration.md) — Send results to Slack
- [Multi-Agent Workflows](learn/tutorials/multi-agent-workflows.md) — Coordinate multiple agents

### Guides

- [Flow Control](guides/flow-control.md) — Sequential, parallel, and conditional execution
- [Error Handling](guides/error-handling.md) — Retries, fallbacks, and failure modes
- [Performance](guides/performance.md) — Speed and cost optimization
- [Debugging](guides/debugging.md) — Troubleshoot workflow issues
- [Testing](guides/testing.md) — Validate and test workflows
- [Daemon Mode](guides/daemon-mode.md) — Run as a service

### Reference

- [CLI](reference/cli.md) — Command-line interface
- [Workflow Schema](reference/workflow-schema.md) — Complete YAML reference
- [Configuration](reference/configuration.md) — Config file options
- [Error Codes](reference/error-codes.md) — Error reference

### Operations

- [File](reference/connectors/file.md), [Shell](reference/connectors/shell.md), [HTTP](reference/connectors/http.md), [Transform](reference/connectors/transform.md)

### Service Integrations

- [GitHub](reference/connectors/github.md), [Slack](reference/connectors/slack.md), [Discord](reference/connectors/discord.md), [Jira](reference/connectors/jira.md), [Jenkins](reference/connectors/jenkins.md)
- [Custom Connectors](reference/connectors/custom.md) — Build your own

---

## Community & Support

- **[GitHub Issues](https://github.com/tombee/conductor/issues)** — Report bugs or request features
- **[Discussions](https://github.com/tombee/conductor/discussions)** — Ask questions and share workflows
- **[Contributing](extending/contributing.md)** — Help build Conductor
