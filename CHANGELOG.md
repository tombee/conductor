# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Core Features
- **Workflow Engine**: Declarative YAML-based workflow definitions
  - Multi-step workflows with LLM orchestration
  - Parallel step execution with configurable concurrency
  - Conditional execution with expression evaluation
  - Retry policies and error handling strategies
  - Input validation and templating

- **LLM Providers**: Multi-provider support with automatic failover
  - Anthropic Claude (Claude 4 Opus, Sonnet, Haiku)
  - Claude Code CLI integration (auto-detected when installed)
  - Google Gemini (2.0 Flash, 1.5 Pro, 1.5 Flash)
  - xAI Grok (grok-beta, grok-2)
  - Model tier abstraction (fast/balanced/strategic)
  - Provider failover with circuit breaker pattern
  - Per-request cost tracking

- **Actions**: Built-in local operations
  - File operations (read, write, list, search)
  - Shell execution with timeout enforcement
  - HTTP requests with SSRF protection
  - Transform operations (JSON, YAML, text)
  - Utility functions (random, ID generation, math)

- **Integrations**: External service connectors
  - GitHub (issues, PRs, workflows, releases)
  - Slack (messages, channels, users)
  - Jira (issues, projects, comments)
  - Discord (messages, channels, webhooks)
  - Jenkins (builds, jobs, queue)

- **Controller**: Long-running service for workflow management
  - REST API for workflow submission and monitoring
  - WebSocket streaming for real-time logs
  - Automatic startup when CLI runs workflows
  - Health monitoring and graceful shutdown
  - Run history and status tracking

- **CLI**: Command-line interface
  - `conductor run` - Execute workflows
  - `conductor validate` - Validate workflow syntax
  - `conductor setup` - Interactive configuration
  - `conductor providers` - Manage LLM providers
  - `conductor controller` - Controller management
  - `conductor runs` - View run history
  - Shell completion for bash, zsh, fish

- **Security**
  - Secure credential storage (macOS Keychain, encrypted file)
  - Security profiles for sandboxed execution
  - Private IP blocking for HTTP requests
  - Redirect validation for SSRF prevention
  - API authentication with rate limiting

- **Observability**
  - Prometheus metrics endpoint (`/metrics`)
  - Structured logging with correlation IDs
  - OpenTelemetry tracing support
  - Per-step cost and duration tracking

#### MCP Support
- MCP server for AI assistant integration
  - Workflow validation and scaffolding tools
  - Template listing and generation
  - Schema access for IDE completion

### Security

- **HTTP Tool SSRF Protection**: Fixed Server-Side Request Forgery vulnerability
  - Proper URL parsing with hostname validation
  - Private IP blocking (RFC1918, loopback, link-local) by default
  - Redirect target validation
  - Security audit logging

### Documentation

- Getting started guide and tutorials
- Workflow schema reference
- CLI reference
- Architecture overview
- Example workflows (code review, issue triage, Slack integration)

[Unreleased]: https://github.com/tombee/conductor/compare/v0.0.0...HEAD
