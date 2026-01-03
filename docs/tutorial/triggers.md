# Part 4: Scheduled Triggers

This section covers cron-based workflow scheduling.

## Configuration

Add a schedule trigger:

```bash
conductor triggers add schedule meal-plan \
  --workflow examples/tutorial/03-loops.yaml \
  --cron "0 9 * * 0" \
  --input days=7
```

### Cron Syntax

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sunday=0)
│ │ │ │ │
0 9 * * 0
```

## Management

```bash
conductor triggers list          # List triggers
conductor triggers test meal-plan    # Execute immediately
conductor triggers remove meal-plan  # Delete trigger
```

## Controller

Triggers require the controller process:

```bash
conductor controller start --foreground  # Development
conductor controller start               # Background
conductor controller status              # Check status
```

Reference: [Controller](../building-workflows/controller/)

[Next: File Input →](input)
