# What is Conductor?

Conductor is a CLI tool for defining and running AI workflows as YAML files. Instead of writing Python scripts or building applications, you describe what you want in a workflow file and run it with `conductor run`.

## The Idea

Every AI task today requires building an application:

- Want code review? Build a Python script.
- Want issue triage? Build another script.
- Want meeting summaries? Build yet another script.

Conductor's approach: workflows as lightweight as shell scripts.

```yaml
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

Write a YAML file. Run it. Share it via git. Deploy it when needed.

## What You Can Do

### Automate Repetitive AI Tasks

Code review, documentation generation, issue triage, commit message writing—anything you'd otherwise do manually with an LLM.

### Chain Multiple Steps

Each step can use outputs from previous steps:

```yaml
steps:
  - id: analyze
    prompt: "Analyze this code: {{.inputs.code}}"

  - id: suggest
    prompt: "Based on this analysis, suggest improvements: {{.steps.analyze.response}}"
```

### Connect to External Services

Built-in connectors for GitHub, Slack, Jira, Discord, and more:

```yaml
steps:
  - id: summarize
    prompt: "Summarize today's PR activity..."

  - id: notify
    slack.post_message:
      channel: "#engineering"
      text: "{{.steps.summarize.response}}"
```

### Run Anywhere

- **CLI**: `conductor run workflow.yaml`
- **Scheduled**: Cron-based triggers
- **Webhooks**: Respond to GitHub PRs, Slack messages, etc.
- **API**: Run via HTTP when deployed as a daemon

## Key Concepts

### Workflows

A workflow is a YAML file with:
- **Inputs**: Data passed in when running
- **Steps**: Actions to perform in sequence
- **Outputs**: Results to return

### Steps

Each step does one thing:
- **LLM steps**: Send prompts to AI models
- **Tool steps**: Run shell commands, read files, make HTTP requests
- **Connector steps**: Interact with external services

### Model Tiers

Instead of hardcoding model names, use tiers:
- **fast**: Quick tasks, lower cost
- **balanced**: Most workflows
- **powerful**: Complex reasoning

Swap providers without changing workflows.

### Template Variables

Reference data with `{{.variable}}` syntax:
- `{{.inputs.name}}` — Workflow inputs
- `{{.steps.id.response}}` — Previous step outputs
- `{{.env.API_KEY}}` — Environment variables

## When to Use Conductor

**Good fit:**
- Automating tasks you'd do with ChatGPT/Claude
- Multi-step AI workflows
- Integrating AI with existing tools (GitHub, Slack, etc.)
- Sharing workflows across a team

**Not designed for:**
- Building chat applications
- Real-time streaming interfaces
- Complex agent loops with unpredictable tool use

## Getting Started

1. **[Install Conductor](installation.md)** — Homebrew, Go, or binary
2. **[Quick Start](../quick-start.md)** — Run your first workflow
3. **[First Workflow Tutorial](tutorials/first-workflow.md)** — Build one from scratch

## Example Workflows

- **[Code Review](../examples/code-review/)** — Multi-persona review of git changes
- **[Issue Triage](../examples/automation/issue-triage/)** — Classify and prioritize issues
- **[Slack Integration](../examples/automation/slack-integration/)** — Post summaries to channels
