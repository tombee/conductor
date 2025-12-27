# System Context Diagram

High-level view of Conductor and its external interactions.

## Overview

```mermaid
graph TB
    subgraph Actors["External Actors"]
        Dev["Developer<br/>CLI user"]
        CICD["CI/CD Pipeline<br/>GitHub Actions, etc."]
        Webhook["Webhook Sources<br/>GitHub, Slack, Discord"]
        API["API Clients<br/>Custom integrations"]
    end

    subgraph Conductor["Conductor System"]
        CLI["conductor<br/>CLI Client"]
        Daemon["conductord<br/>Daemon Server"]
    end

    subgraph Providers["LLM Providers"]
        Anthropic["Anthropic<br/>Claude models"]
        OpenAI["OpenAI<br/>GPT models"]
        Ollama["Ollama<br/>Local models"]
        ClaudeCode["Claude Code<br/>via claude CLI"]
    end

    subgraph Storage["State Storage"]
        SQLite["SQLite<br/>Local default"]
        Postgres["PostgreSQL<br/>Distributed mode"]
    end

    subgraph Tools["External Tools"]
        HTTP["HTTP APIs<br/>REST endpoints"]
        Shell["Shell<br/>System commands"]
        MCP["MCP Servers<br/>Tool extensions"]
        Files["Filesystem<br/>Read/write files"]
    end

    Dev --> CLI
    CICD --> Daemon
    Webhook --> Daemon
    API --> Daemon

    CLI --> Daemon
    Daemon --> Anthropic
    Daemon --> OpenAI
    Daemon --> Ollama
    Daemon --> ClaudeCode

    Daemon --> SQLite
    Daemon --> Postgres

    Daemon --> HTTP
    Daemon --> Shell
    Daemon --> MCP
    Daemon --> Files
```

## Component Descriptions

### External Actors

| Actor | Description |
|-------|-------------|
| **Developer** | Runs workflows via CLI, manages configuration |
| **CI/CD Pipeline** | Triggers workflows via API or webhooks |
| **Webhook Sources** | GitHub PRs, Slack messages, Discord events |
| **API Clients** | Custom tools built on Conductor API |

### Conductor System

| Component | Description |
|-----------|-------------|
| **conductor** | CLI client, communicates with daemon via socket |
| **conductord** | Daemon server, handles all workflow execution |

### LLM Providers

| Provider | Use Case |
|----------|----------|
| **Anthropic** | Primary provider (Claude models) |
| **OpenAI** | Alternative provider (GPT models) |
| **Ollama** | Local models, no network required |
| **Claude Code** | Zero-config via `claude` CLI if installed |

### State Storage

| Backend | When to Use |
|---------|-------------|
| **SQLite** | Single-node, local development |
| **PostgreSQL** | Distributed mode, production |

### External Tools

| Tool | Purpose |
|------|---------|
| **HTTP** | Call REST APIs |
| **Shell** | Execute system commands |
| **MCP** | Connect to Model Context Protocol servers |
| **Files** | Read/write local files |

## Key Relationships

1. **CLI to Daemon**: All CLI commands route through the daemon (Unix socket or HTTP)
2. **Daemon to Providers**: Daemon manages provider credentials and rate limiting
3. **Daemon to Storage**: Persists workflow state, checkpoints, and execution history
4. **Daemon to Tools**: Executes tool calls requested by LLM during workflow steps

---
*See [Components](components.md) for internal package structure.*
