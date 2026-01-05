# Claude Code

Claude Code is the simplest way to get started if you already have it installed.

## Prerequisites

- [Claude Code](https://claude.ai/claude-code) installed and authenticated

## Setup

```bash
conductor provider add claude-code
```

That's it. Conductor automatically uses your existing Claude Code authentication.

## Verify

```bash
conductor provider test claude-code
```

You should see a success message confirming the connection.

## Set as Default

To make Claude Code your default provider:

```bash
conductor provider add claude-code --default
```

## Next Steps

- Learn about [model tiers](../features/model-tiers.md) and when to use each
- Continue to the [tutorial](../tutorial/index.md) to build your first workflow
