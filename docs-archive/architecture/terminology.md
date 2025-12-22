# Terminology Guide

This document defines the canonical terminology for Conductor's workflow operations.

## Core Components

### Controller

The **controller** is the long-running service process that manages workflow execution. It provides:
- HTTP API for triggering workflows
- Webhook endpoints for external services
- Workflow state management
- Execution scheduling

The controller runs as a persistent service (started with `conductor` command) and listens for workflow triggers.

### Trigger

A **trigger** defines how a workflow is invoked. Triggers are configured exclusively via the `conductor triggers` CLI command (not inline in workflow files). Trigger types include:
- API triggers for programmatic invocation
- Webhook triggers for GitHub, Slack, and other services
- Schedule triggers for cron-based execution

**Example (CLI configuration):**
```bash
# Add a webhook trigger for GitHub pull requests
conductor triggers add webhook pr-review.yaml \
  --path=/webhooks/github/pr-review \
  --source=github \
  --events=pull_request.opened,pull_request.synchronize

# Add a schedule trigger
conductor triggers add schedule daily-report.yaml \
  --name=daily-9am \
  --cron="0 9 * * *"
```

## Actions vs Service Integrations

Conductor workflows use two types of non-LLM operations:

### Actions

**Actions** are built-in, deterministic operations that execute locally without external service calls.

| Action | Description |
|--------|-------------|
| `file` | File system operations (read, write, list, copy) |
| `shell` | Shell command execution |
| `http` | Generic HTTP requests |
| `utility` | Random numbers, IDs, math operations |
| `transform` | Data transformation (JSON, YAML, text) |

**Usage in workflows:**
```yaml
steps:
  - file.read: ./config.yaml
  - shell.run: npm test
  - utility.random_int:
      min: 1
      max: 100
```

### Service Integrations

**Service integrations** connect to external services via their APIs. They handle authentication, rate limiting, and platform-specific conventions.

| Category | Examples |
|----------|----------|
| Productivity | GitHub, Slack, Jira, Linear, Confluence |
| Messaging | Discord, Twilio |
| Observability | Datadog, Splunk, Loki, CloudWatch, Elasticsearch |
| Cloud | AWS services (via CloudWatch, etc.) |
| Automation | Jenkins |

**Usage in workflows:**
```yaml
steps:
  - github.create_issue:
      owner: myorg
      repo: myrepo
      title: "Bug report"

  - datadog.log:
      message: "Workflow completed"
      status: info
```

## Legacy Terminology

The term **"connector"** was previously used for both actions and service integrations. As of late 2025, the codebase has been fully migrated to use consistent terminology:

- Use **"action"** for local, built-in operations
- Use **"service integration"** (or just "integration") for external API integrations
- The term "connector" should not appear in new code or documentation

## Code Structure

The code is organized to reflect the conceptual separation:

```
internal/
├── operation/                # Shared framework (executor, registry, errors, metrics)
├── action/                   # Local actions
│   ├── file/
│   ├── shell/
│   ├── http/
│   ├── utility/
│   └── transform/
└── integration/              # External service integrations
    ├── github/
    ├── slack/
    ├── jira/
    ├── discord/
    ├── jenkins/
    ├── cloudwatch/
    ├── datadog/
    ├── elasticsearch/
    ├── loki/
    └── splunk/
```

---
*Last updated: 2025-12-29*
