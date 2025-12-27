# Conductor Vision

> **Tagline:** Portable AI workflows in simple YAML.

## Executive Summary

**What:** Conductor is a platform for defining and running AI workflows as simple YAML files.

**Why:** Managing multiple AI workflows shouldn't require building separate applications for each one. Conductor provides a unified platform with production features built-in.

**End Goal:** A common platform for all your AI workflows—with observability, reliability, cost management, and portability included.

## The Problem We're Solving

Organizations and developers building AI-powered automation face common challenges:

- **Fragmentation** - Each AI task becomes its own application with its own patterns
- **Missing infrastructure** - Logging, metrics, error handling, and retries built from scratch each time
- **Cost opacity** - No visibility into token usage across different workflows
- **Provider lock-in** - Tightly coupled to specific LLM providers
- **Security gaps** - Inconsistent secret management and execution controls

## What Conductor Provides

**A unified platform for AI workflow management:**

| Challenge | Conductor Solution |
|-----------|-------------------|
| Fragmentation | Single YAML format for all workflows |
| Missing infrastructure | Built-in observability, retries, timeouts |
| Cost opacity | Token tracking and budget controls |
| Provider lock-in | Swap providers with a config change |
| Security gaps | Sandboxed execution, secret management |

## Core Principles

1. **Declarative by design** - Workflows define *what* to do, not *how*—enabling testing, validation, and predictable execution
2. **Platform features included** - Observability, reliability, and security out of the box
3. **LLM-efficient** - Deterministic steps handle orchestration; LLMs focus on reasoning tasks
4. **Provider portability** - Switch LLM providers without rewriting workflows
5. **Local-first, daemon when needed** - Works on your laptop, scales to production
6. **Shareable by default** - Text files you can version, fork, and share

## Sharing Model

### Via Git (Primary)

Workflows are just files—share them like any other code:
- Push workflow files to GitHub
- Others run with `conductor run github:user/repo`
- Fork, customize, share back

### No Central Platform Required

Unlike npm or Docker Hub, sharing doesn't require a central registry:
- Workflows are text files
- Git is the distribution mechanism
- Optional registry for discovery (future)

## Success Metrics

### Adoption Indicators
- Workflows shared on GitHub
- `conductor run github:...` commands in the wild
- Blog posts and tutorials
- Diversity of use cases

### Quality Indicators
- Time to first workflow < 15 minutes
- Lines of YAML for common tasks < 20
- Error messages that users can act on
- Zero dependency hell—just the binary

## Messaging

### Do Say
- "Portable AI workflows in simple YAML"
- "One platform for all your AI workflows"
- "Production features built-in"
- "Switch providers without rewriting"

### Don't Say
- "LangChain alternative"
- "Enterprise AI platform"
- "AI agent framework"

---
*Last updated: 2025-12-27*
