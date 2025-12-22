# Conductor Vision

> **Tagline:** AI workflows as simple as shell scripts.

## Executive Summary

**What:** Conductor is a CLI tool and daemon for defining and running AI workflows as simple YAML files.

**Why:** Every AI-powered task today requires building an app. Conductor lets you write a workflow file instead.

**End Goal:** Personal automation that happens to scale. Write workflows locally, share via git, deploy to production when needed.

## The Problem We're Solving

**Today:** Every AI task requires building an app
```
Want code review?      → Build a Python app
Want issue triage?     → Build another Python app
Want meeting summaries? → Build another Python app
```

**Conductor's vision:** Workflows as lightweight as shell scripts
```
Want code review?      → Write a YAML file, run it
Want issue triage?     → Write a YAML file, run it
Share it?              → Push to GitHub, others run it too
Useful enough?         → Deploy to daemon with webhooks
```

## What Conductor Is

**Conductor is a personal automation tool for AI tasks.**

Like shell scripts automate command-line tasks, Conductor workflows automate AI tasks:

| Shell Scripts | Conductor Workflows |
|---------------|---------------------|
| Automate CLI tasks | Automate AI tasks |
| Text files you can share | YAML files you can share |
| Run with `./script.sh` | Run with `conductor run` |
| Just works | Just works |

## Core Principles

1. **Personal automation first** - Individual developers solving their own problems
2. **As simple as shell scripts** - Plain YAML files you can read and edit
3. **Local-first, daemon when needed** - Works on your laptop, scales if needed
4. **Shareable by default** - Text files you can version, fork, share
5. **Portable definitions** - YAML files that work anywhere

## Sharing Model

### Via Git (Primary)

Developers share workflows the same way they share shell scripts:
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
- Zero dependency hell - just the binary

## Messaging

### Do Say
- "AI workflows as simple as shell scripts"
- "Write once, run anywhere"
- "Just YAML files"
- "Personal automation for AI tasks"

### Don't Say
- "LangChain alternative"
- "Enterprise AI platform"
- "AI agent framework"
- "Build production AI apps"

---
*Last updated: 2025-12-22*
