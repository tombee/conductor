# Conductor

> **A production-ready platform for AI agent workflows**

[![Go Version](https://img.shields.io/badge/go-1.22%2B-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-latest-blue)](https://tombee.github.io/conductor/)

Define agent workflows in YAML. Get observability, security, cost controls, and flexible deployment built-in—so you can focus on what your agents actually do.

## Why Conductor?

- **Focus on workflow logic** — Define what your agents do in YAML. The platform handles retries, fallbacks, and error handling.
- **Production-ready** — Observability, cost tracking, and security are built into the platform.
- **Flexible deployment** — Run the same workflow from the CLI, as an API, on a schedule, or triggered by webhooks.
- **Ops-friendly controls** — Security profiles, cost limits, secret management, and container sandboxing give teams the governance they need.
- **Declarative connectors** — Connect to GitHub, Slack, Jira, and more with configuration, not code.
- **Any LLM provider** — Use Anthropic, OpenAI, Ollama, or others. Swap providers without changing workflow logic.
- **Deterministic by default** — Connectors and tools handle API calls, file operations, and integrations. LLMs focus on reasoning and summarization—keeping workflows fast and costs low.
- **MCP support** — When LLMs need to call tools, add MCP servers. Conductor manages their lifecycle alongside your workflows.

## Example

```yaml
name: write-song
inputs:
  - name: genre
    required: true
  - name: topic
    required: true

steps:
  - id: compose
    type: llm
    prompt: |
      Write a short {{.inputs.genre}} song about "{{.inputs.topic}}".
      Include chord symbols above the lyrics.
```

```bash
$ conductor run song.yaml
genre: blues
topic: morning coffee
```

## Installation

```bash
brew install tombee/tap/conductor
```

Or with Go:

```bash
go install github.com/tombee/conductor/cmd/conductor@latest
```

## Getting Started

If you have [Claude Code](https://claude.ai/code) installed, you're ready to go:

```bash
conductor run examples/write-song/workflow.yaml
```

Conductor also supports Anthropic, OpenAI, and Ollama APIs. Run `conductor init` to configure a different provider.

## Documentation

**[Read the full documentation →](https://tombee.github.io/conductor/)**

- [Quick Start](https://tombee.github.io/conductor/quick-start/) — Get running in 5 minutes
- [Examples](https://tombee.github.io/conductor/examples/) — Workflows to copy and adapt
- [CLI Reference](https://tombee.github.io/conductor/reference/cli/) — All commands and options

## License

Apache 2.0 — see [LICENSE](LICENSE) for details.
