# Architecture Diagrams

Visual documentation of Conductor's architecture, designed for quick understanding at different levels of detail.

## Diagram Index

| Diagram | Description | Best For |
|---------|-------------|----------|
| [System Context](system-context.md) | High-level view of Conductor in its environment | New contributors, stakeholders |
| [Components](components.md) | Internal package structure and dependencies | Developers working on the codebase |
| [Execution Flow](execution-flow.md) | Sequence diagrams for workflow execution | Understanding runtime behavior |
| [Deployment Modes](deployment-modes.md) | Deployment topologies and configurations | Operations, infrastructure setup |

## How to Use These Diagrams

**Start with System Context** to understand what Conductor is and how it fits with external systems.

**Move to Components** when you need to understand the internal structure for development or debugging.

**Consult Execution Flow** when tracing through specific runtime behavior or debugging issues.

**Reference Deployment Modes** when setting up Conductor for production or understanding scaling options.

## Diagram Conventions

All diagrams use [Mermaid](https://mermaid.js.org/) syntax for version control and GitHub rendering.

**Colors:**
- Blue boxes: Core Conductor components
- Green boxes: External systems/integrations
- Orange boxes: User-facing entry points

**Arrows:**
- Solid lines: Direct calls or data flow
- Dashed lines: Async communication or optional paths

---
*See [Architecture Overview](overview.md) for detailed written documentation.*
