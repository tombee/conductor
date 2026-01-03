# Part 4: Scheduled Triggers

Run workflows automatically on a schedule.

## Adding a Schedule Trigger

Schedule the meal planner to run every Sunday at 9 AM:

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

Common patterns:
- `0 9 * * 0` — Every Sunday at 9:00 AM
- `0 9 * * 1-5` — Every weekday at 9:00 AM
- `0 */6 * * *` — Every 6 hours

## Managing Triggers

```bash
# List all triggers
conductor triggers list

# Test trigger immediately
conductor triggers test meal-plan

# Remove trigger
conductor triggers remove meal-plan
```

## Running the Controller

Triggers require the controller to be running:

```bash
# Development (foreground)
conductor controller start --foreground

# Production (background)
conductor controller start
conductor controller status
```

See [Controller](../building-workflows/controller/) for deployment options.

## What's Next

Read external files to make the meal planner aware of what's in your pantry.

[Part 5: Reading External Data →](input)
