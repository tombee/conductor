# Terminology Guide

This document defines the canonical terminology for Conductor's workflow operations.

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

The term **"connector"** was previously used for both actions and service integrations. Going forward:

- Use **"action"** for local, built-in operations
- Use **"service integration"** for external API integrations
- The `internal/connector/` package name is retained for code compatibility

## Code Structure

Despite the terminology update, the code organization remains:

```
internal/connector/           # Runtime execution framework
├── builtin/                  # Service integrations (GitHub, Slack, etc.)
│   ├── github/
│   ├── slack/
│   └── ...
├── file/                     # File action
├── shell/                    # Shell action
├── utility/                  # Utility action
└── transform/                # Transform utilities
```

---
*Last updated: 2025-12-28*
