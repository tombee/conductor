# Features

Conductor provides a complete platform for building and running AI agent workflows. This page maps all capabilities to help you find what you need.

---

## Workflow Execution

Build workflows that combine LLM reasoning with deterministic actions.

| Feature | When to Use | Description | Docs |
|---------|-------------|-------------|------|
| **Sequential Steps** | Default execution | Steps run in defined order, each accessing outputs from previous steps | [Workflows and Steps](learn/concepts/workflows-steps.md) |
| **Parallel Execution** | Process multiple items concurrently | Run steps simultaneously with configurable `max_concurrency` | [Flow Control](building-workflows/flow-control.md) |
| **Conditional Logic** | Skip steps based on conditions | Expression-based step conditions using template syntax | [Flow Control](building-workflows/flow-control.md) |
| **Foreach Loops** | Iterate over arrays | Process each item in an array with access to `{{.item}}`, `{{.index}}`, `{{.total}}` | [Flow Control](building-workflows/flow-control.md#iteration-with-foreach) |
| **Error Handling** | Handle failures gracefully | Strategies: `fail`, `retry`, `fallback`, `ignore` | [Error Handling](building-workflows/error-handling.md) |
| **Retries** | Recover from transient failures | Exponential, linear, or fixed backoff with configurable attempts | [Error Handling](building-workflows/error-handling.md) |
| **Timeouts** | Limit step duration | Per-step timeout configuration to prevent runaway execution | [Workflow Schema](reference/workflow-schema.md) |
| **Workflow Composition** | Reuse workflows | Call sub-workflows as steps for modular design | [Building Workflows](building-workflows/) |
| **Template Variables** | Reference data dynamically | Use `{{.inputs.name}}`, `{{.steps.id.response}}` to access data | [Template Variables](learn/concepts/template-variables.md) |
| **Typed Inputs** | Validate workflow inputs | String, number, boolean, object, array with required/default options | [Inputs and Outputs](learn/concepts/inputs-outputs.md) |

---

## LLM Integration

Connect to any LLM provider with consistent syntax and intelligent defaults.

| Feature | When to Use | Description | Docs |
|---------|-------------|-------------|------|
| **Anthropic** | Claude models | Claude Opus 4.5, Sonnet, Haiku with streaming, tool calling, prompt caching, vision | [LLM Providers](architecture/llm-providers.md) |
| **OpenAI** | GPT models | GPT-4, GPT-4 Turbo, GPT-3.5 with streaming, tool calling, vision, fine-tuned models | [LLM Providers](architecture/llm-providers.md) |
| **Google** | Gemini models | Gemini Pro/Ultra with streaming, tool calling, multimodal | [LLM Providers](architecture/llm-providers.md) |
| **Ollama** | Local models | Llama, Mistral, Mixtral with streaming—run locally | [LLM Providers](architecture/llm-providers.md) |
| **Claude Code** | Zero-config option | Auto-detected when installed—no API key needed | [LLM Providers](architecture/llm-providers.md) |
| **Model Tiers** | Provider-agnostic workflows | Use `fast`, `balanced`, or `strategic` to select models by capability, not name | [Model Tiers](guides/model-tiers.md) |

---

## Built-in Actions

Local operations that don't require external services or API calls.

| Action | Operations | When to Use | Docs |
|--------|------------|-------------|------|
| **file** | read, write, list, copy, move, delete, mkdir, stat, exists (17 ops) | Read/write files, list directories, manage filesystem | [File](reference/actions/file.md) |
| **shell** | run | Execute commands with timeout, env vars, exit code capture | [Shell](reference/actions/shell.md) |
| **http** | GET, POST, PUT, DELETE | Make HTTP requests with auth, headers, SSRF protection | [HTTP](reference/actions/http.md) |
| **transform** | parse_json, parse_xml, extract, split, filter, map, merge, and more | Reshape data between steps using jq expressions | [Transform](reference/actions/transform.md) |
| **utility** | id_uuid, id_nanoid, random_int, math_clamp, and more (12 ops) | Generate IDs, random values, and perform math operations | [Utility](reference/actions/utility.md) |

---

## Service Integrations

Connect to external services with configuration, not code.

| Service | Operations | When to Use | Docs |
|---------|------------|-------------|------|
| **GitHub** | Issues, PRs, repos, releases, actions (12 ops) | Automate repository workflows, triage issues, review PRs | [GitHub](reference/integrations/github.md) |
| **Slack** | Messages, channels, reactions, threads (10 ops) | Send notifications, respond to messages, post reports | [Slack](reference/integrations/slack.md) |
| **Jira** | Issues, comments, transitions, search (11 ops) | Manage tickets, automate workflows, sync with development | [Jira](reference/integrations/jira.md) |
| **Discord** | Messages, embeds, channels, webhooks (12 ops) | Build bots, send notifications, manage communities | [Discord](reference/integrations/discord.md) |
| **Jenkins** | Jobs, builds, queue, test results (15 ops) | Trigger builds, monitor pipelines, analyze test results | [Jenkins](reference/integrations/jenkins.md) |
| **Custom** | Define your own | Build integrations for any HTTP API | [Custom Integrations](reference/integrations/custom.md) |

---

## Execution Modes & Triggers

Run workflows your way—CLI, API, webhooks, or scheduled.

| Mode | When to Use | Description | Docs |
|------|-------------|-------------|------|
| **CLI** | Development and manual runs | `conductor run workflow.yaml` with interactive inputs | [CLI Reference](reference/cli.md) |
| **Controller** | Long-running service | Persistent state, background execution, API access | [Controller Mode](building-workflows/controller.md) |
| **HTTP API** | Programmatic access | REST API for triggering, monitoring, and managing workflows | [API Reference](reference/api.md) |
| **Webhooks** | Event-driven automation | GitHub webhooks and custom webhook triggers | [Webhook Triggers](building-workflows/controller.md#webhook-triggers) |
| **Scheduled** | Recurring workflows | Cron-style triggers in controller mode | [Scheduled Triggers](building-workflows/controller.md#scheduled-triggers) |

---

## Production & Operations

Enterprise-grade security, observability, and cost controls.

| Feature | When to Use | Description | Docs |
|---------|-------------|-------------|------|
| **Secrets Management** | Secure credentials | Keychain, Vault, AWS Secrets Manager, environment variables | [Configuration](reference/configuration.md) |
| **SSRF Protection** | HTTP security | Blocks private IPs, validates redirects | [HTTP](reference/actions/http.md) |
| **Shell Sandboxing** | Command security | Command allowlists, working directory isolation | [Shell](reference/actions/shell.md) |
| **Path Security** | Filesystem safety | Traversal prevention, symlink blocking | [File](reference/actions/file.md) |
| **Transform Sandbox** | Expression safety | Dangerous jq functions disabled, 1-second timeout | [Transform](reference/actions/transform.md) |
| **Credential Filtering** | Log safety | Auto-redaction in logs | [Monitoring](production/monitoring.md) |
| **Structured Logging** | Observability | JSON format, configurable levels | [Monitoring](production/monitoring.md) |
| **Correlation IDs** | Distributed tracing | UUID-based request tracking across services | [Monitoring](production/monitoring.md#correlation-ids) |
| **Token Tracking** | Usage monitoring | Per-request token counts including cache tokens | [Cost Tracking](production/cost-tracking.md) |
| **Cost Calculation** | Budget management | Provider-specific pricing with alerts and limits | [Cost Tracking](production/cost-tracking.md) |
| **Health Checks** | Diagnostics | `conductor health` for troubleshooting | [CLI Reference](reference/cli.md) |

---

## MCP Support

Extend LLM capabilities with Model Context Protocol servers.

| Feature | When to Use | Description | Docs |
|---------|-------------|-------------|------|
| **Server Lifecycle** | Managed MCP servers | Automatic startup, health checks, restart, graceful shutdown | [MCP Servers](guides/mcp.md) |
| **Tool Registry** | Extended capabilities | Tools from MCP servers automatically available in LLM calls | [MCP Servers](guides/mcp.md) |
| **Server Configuration** | Custom tools | Connect filesystem, GitHub, or custom HTTP tool servers | [MCP Servers](guides/mcp.md) |

---

## Developer Experience

Tools to build, validate, and debug workflows efficiently.

| Feature | When to Use | Description | Docs |
|---------|-------------|-------------|------|
| **CLI** | Primary interface | 15+ commands: init, run, validate, providers, doctor, and more | [CLI Reference](reference/cli.md) |
| **JSON Schema** | IDE support | Autocompletion and validation in VS Code, IntelliJ, and others | [Workflow Schema](reference/workflow-schema.md) |
| **Validation** | Pre-run checks | YAML syntax, step references, template syntax validation | [CLI Reference](reference/cli.md) |
| **Example Workflows** | Learning | Code review, issue triage, security audit, and more | [Examples](examples/) |
| **Dry Run** | Testing | Validate workflows without making LLM calls or side effects | [CLI Reference](reference/cli.md) |

---

## Next Steps

<div class="grid cards" markdown>

-   :material-rocket-launch:{ .lg .middle } **Get Started**

    ---

    Install Conductor and run your first workflow

    [:octicons-arrow-right-24: Getting Started](getting-started/)

-   :material-book-open-variant:{ .lg .middle } **Tutorials**

    ---

    Build real workflows step by step

    [:octicons-arrow-right-24: First Workflow](learn/tutorials/first-workflow.md)

-   :material-file-document-multiple:{ .lg .middle } **Reference**

    ---

    Complete syntax and configuration reference

    [:octicons-arrow-right-24: Workflow Schema](reference/workflow-schema.md)

-   :material-help-circle:{ .lg .middle } **Examples**

    ---

    Ready-to-use workflow examples

    [:octicons-arrow-right-24: Example Gallery](examples/)

</div>
