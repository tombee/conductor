# Getting Started with Conductor

Conductor is a platform for defining and running AI workflows in simple YAML files.

## What is Conductor?

Instead of building individual applications for every AI task, you define workflows in simple YAML and get production features built-in:

- **Declarative definitions** - Testable, validatable workflows
- **LLM-efficient** - Deterministic steps handle orchestration; LLMs focus on reasoning
- **Production-ready** - Observability, reliability, and cost management built-in
- **Portable** - Switch LLM providers with a config change

```conductor
# code-review.yaml
name: code-review
steps:
  - id: review
    model: balanced
    prompt: |
      Review this code for bugs, security issues, and style:
      {{.inputs.code}}
```

```bash
conductor run code-review.yaml -i code="$(git diff)"
```

## Installation

Choose your preferred installation method:

**Homebrew (macOS/Linux)**
```bash
brew install conductor
```

**Go Install**
```bash
go install github.com/tombee/conductor/cmd/conductor@latest
```

**From Source**
```bash
git clone https://github.com/tombee/conductor
cd conductor
make install
```

**Verify installation:**
```bash
conductor --version
```

## Setup Claude Code

Conductor works best with [Claude Code](https://claude.ai/download). With Claude Code installed, Conductor works out of the box with no API key configuration required.

1. Download and install from [claude.ai/download](https://claude.ai/download)
2. Complete setup and sign in
3. Verify: `claude --version`

For other LLM providers, see [Configuration](../reference/configuration.md).

## Your First Workflow

Let's run a simple workflow:

```bash
conductor run examples/write-song/workflow.yaml
```

Conductor will prompt you for inputs. After a few seconds, you'll get an AI-generated song!

## Create Your Own Workflow

Create `hello.yaml`:

```conductor
name: hello-conductor
description: Your first custom workflow

inputs:
  - name: name
    type: string
    required: true
    description: Your name

steps:
  - id: greet
    type: llm
    model: fast
    prompt: |
      Generate a friendly, personalized greeting for someone named {{.inputs.name}}.
      Make it warm and encouraging. Keep it to 2-3 sentences.

outputs:
  - name: greeting
    value: "{{.steps.greet.response}}"
```

Run it:

```bash
conductor run hello.yaml
```

When prompted, enter your name and you'll get a personalized greeting!

## Core Concepts

### Workflows

A workflow is a YAML file with:
- **Inputs**: Data passed in when running
- **Steps**: Actions to perform in sequence
- **Outputs**: Results to return

### Steps

Each step does one thing:
- **LLM steps**: Send prompts to AI models (`type: llm`)
- **Tool steps**: Run shell commands, read files, make HTTP requests
- **Integration steps**: Interact with external services (GitHub, Slack, etc.)

```conductor
steps:
  # LLM step
  - id: analyze
    type: llm
    prompt: "Analyze this code: {{.inputs.code}}"

  # File tool
  - id: read_config
    file.read:
      path: "config.yaml"

  # Shell tool
  - id: get_diff
    shell.run:
      command: ["git", "diff"]
```

### Template Variables

Reference data with `{{.variable}}` syntax:
- `{{.inputs.name}}` — Workflow inputs
- `{{.steps.analyze.response}}` — Previous step outputs
- `{{.env.API_KEY}}` — Environment variables

### Model Tiers

Use tiers instead of hardcoding model names:
- **fast**: Quick tasks, lower cost
- **balanced**: Most workflows
- **powerful**: Complex reasoning

This lets you swap providers without changing workflows.

### Parallel Execution

Run multiple steps concurrently:

```conductor
- id: reviews
  type: parallel
  steps:
    - id: security
      type: llm
      prompt: "Review for security..."
    - id: performance
      type: llm
      prompt: "Review for performance..."
```

## Common Patterns

**Read-Process-Write:**
```conductor
- id: read
  file.read:
    path: "{{.inputs.file_path}}"

- id: process
  type: llm
  prompt: "Process this: {{.steps.read.content}}"

- id: write
  file.write:
    path: "output.txt"
    content: "{{.steps.process.response}}"
```

**Chain of Thought:**
```conductor
- id: analyze
  type: llm
  prompt: "Analyze this code..."

- id: suggest
  type: llm
  prompt: "Based on this analysis, suggest improvements: {{.steps.analyze.response}}"
```

## What You Can Do

- **Automate repetitive AI tasks**: Code review, documentation, issue triage
- **Chain multiple steps**: Each step uses outputs from previous steps
- **Connect to services**: GitHub, Slack, Jira, Discord integrations
- **Run anywhere**: CLI, scheduled (cron), webhooks, or HTTP API

**Good fit:**
- Automating tasks you'd do with ChatGPT/Claude
- Multi-step AI workflows
- Integrating AI with existing tools

**Not designed for:**
- Building chat applications
- Real-time streaming interfaces
- Complex agent loops with unpredictable tool use

## What's Next?

**Explore all capabilities:**

See everything Conductor can do on the [Features](../features.md) page—from workflow patterns to integrations to production features.

**Try a real-world example:**
```bash
conductor run examples/git-branch-code-review/workflow.yaml
```

**Learn more:**
- [First Workflow Tutorial](first-workflow.md) — Hands-on guide
- [Building Workflows](../building-workflows) — Patterns and best practices
- [Examples](../examples/) — Copy-paste ready workflows
- [Workflow Schema Reference](../reference/workflow-schema.md) — Complete specification

## Troubleshooting

**"conductor: command not found"**

Ensure your `$PATH` includes the installation directory:
```bash
# For Go install
export PATH=$PATH:$(go env GOPATH)/bin
```

**"Provider not configured"**

Ensure Claude Code is installed: `claude --version`

**"Workflow validation failed"**

Common causes:
- Incorrect YAML indentation (whitespace-sensitive)
- Missing required fields (every step needs `id` and `type`)
- Invalid template syntax (use `{{.inputs.name}}` not `{{name}}`)

Validate before running:
```bash
conductor validate hello.yaml
```

**Still stuck?**
- [Troubleshooting Guide](../production/troubleshooting.md)
- [GitHub Issues](https://github.com/tombee/conductor/issues)
- [GitHub Discussions](https://github.com/tombee/conductor/discussions)
