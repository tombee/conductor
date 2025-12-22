# Conductor

> A workflow orchestration library and runtime for AI agents

[![Go Version](https://img.shields.io/badge/go-1.22%2B-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](LICENSE)

## Overview

Conductor is an embeddable Go library and standalone runtime for building AI-powered workflow automation. It provides a flexible architecture for orchestrating LLM-powered agents, managing workflow state, and executing tools in a transparent, observable manner.

## Features

### LLM Provider Abstraction

- **Multi-Provider Support**: Pluggable provider interface with Anthropic Claude, OpenAI, and Ollama support
- **Model Tiers**: Abstract model selection (fast/balanced/strategic) that maps to provider-specific models
- **Streaming Support**: Channel-based streaming API for real-time LLM responses
- **Tool Calling**: Function calling abstraction that works across providers
- **Cost Tracking**: Per-request token usage and cost calculation with correlation IDs
- **Connection Pooling**: HTTP connection reuse (10 connections per provider, 30s idle timeout)
- **Retry Logic**: Exponential backoff retry on transient failures (5xx, 429, timeouts)
- **Provider Failover**: Circuit breaker pattern with automatic failover to secondary providers

### Workflow Orchestration

- **YAML Definitions**: Human-readable workflow specifications with type validation
- **State Machine Engine**: Explicit state transitions (Created → Running → Completed/Failed)
- **Multiple Step Types**: Action (tool execution), LLM, Condition, Parallel
- **Error Handling**: Configurable strategies per step (fail, ignore, retry, fallback)
- **Event System**: Pub/sub events for workflow observability (StateChanged, StepCompleted, Error)
- **Persistence**: SQLite storage with auto-upgrade and configurable retention

### Agent Execution

- **ReAct Pattern**: Reasoning + Acting loop for LLM-powered agents
- **Tool Integration**: Automatic tool discovery and execution via registry
- **Context Management**: Token tracking with automatic pruning at 80% capacity
- **Max Iterations**: Configurable iteration limit to prevent runaway costs (default: 20)
- **Streaming Handler**: Optional callback for real-time agent events

### Tool System

- **Tool Registry**: Centralized discovery and execution with JSON Schema validation
- **Builtin Tools**: File (read/write), Shell (command execution), HTTP (API calls)
- **Safety Limits**: Timeouts (30s default), allowlists, max file sizes
- **Extensible**: Implement Tool interface to add custom tools

### Production Ready

- **Zero Global State**: All components accept dependencies via constructors
- **Clean Interfaces**: Stable public APIs in `pkg/*` packages
- **High Test Coverage**: 80%+ for pkg/*, 70%+ for internal/*
- **Structured Logging**: JSON logs with slog
- **Graceful Shutdown**: SIGTERM handling with request draining

## Quick Start

### As a Library

```go
import (
    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/llm/providers"
    "github.com/tombee/conductor/pkg/agent"
)

func main() {
    // Register LLM provider
    provider := providers.NewAnthropicProvider(apiKey)
    llm.Register(provider)

    // Create and run agent
    a := agent.New(provider)
    result, err := a.Run(ctx, "Analyze this code for security issues", nil)
}
```

### As a Runtime

```bash
# Install
go install github.com/tombee/conductor/cmd/conductor@latest

# Run a workflow
conductor run workflow.yaml
```

## Installation

```bash
go get github.com/tombee/conductor
```

## Documentation

- [Getting Started](docs/getting-started.md) - Quick start guide
- [Architecture](docs/architecture.md) - How Conductor works internally
- [API Reference](docs/api-reference.md) - Public API documentation
- [Embedding Guide](docs/embedding.md) - How to embed Conductor in your Go project

## Examples

See [examples/](examples/) for working examples:

- [code-review](examples/code-review/) - Multi-agent code review workflow
- [issue-triage](examples/issue-triage/) - Automatic issue classification

## Project Status

Conductor is in **active development**. The public API is stabilizing but may change before v1.0.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code style guidelines, and how to submit pull requests.

## License

Apache 2.0 - see [LICENSE](LICENSE) for details.
