# Triggers

Triggers define how workflows are invoked automatically.

## Cron Triggers

Run workflows on a schedule:

```yaml
name: daily-report
triggers:
  - cron:
      schedule: "0 9 * * *"
      timezone: America/New_York
steps:
  - id: generate
    llm:
      prompt: "Generate daily report"
```

### Cron Schedule Format

```
* * * * *
│ │ │ │ │
│ │ │ │ └─ Day of week (0-6, Sunday=0)
│ │ │ └─── Month (1-12)
│ │ └───── Day of month (1-31)
│ └─────── Hour (0-23)
└───────── Minute (0-59)
```

### Common Schedules

```yaml
# Every day at 9 AM
schedule: "0 9 * * *"

# Every hour
schedule: "0 * * * *"

# Every Monday at 6 PM
schedule: "0 18 * * 1"

# First day of month at midnight
schedule: "0 0 1 * *"

# Every 15 minutes
schedule: "*/15 * * * *"
```

## Webhook Triggers

Trigger workflows via HTTP POST:

```yaml
name: github-webhook
triggers:
  - webhook:
      path: /github
      secret: ${WEBHOOK_SECRET}
steps:
  - id: process
    llm:
      prompt: "Process webhook: ${trigger.payload}"
```

Access webhook data with `${trigger.payload}`.

### Securing Webhooks

Validate requests with a secret:

```yaml
triggers:
  - webhook:
      path: /secure
      secret: ${WEBHOOK_SECRET}
      validateSignature: true
```

### Webhook URL

After deployment, the webhook is available at:

```
https://your-controller.example.com/webhooks/secure
```

## Poll Triggers

Check for changes periodically:

```yaml
name: check-rss
triggers:
  - poll:
      interval: 5m
      source:
        http:
          method: GET
          url: https://example.com/feed.xml
      condition: ${trigger.data != trigger.previous}
steps:
  - id: process
    llm:
      prompt: "New content: ${trigger.data}"
```

### Poll Intervals

- `1m` - Every minute
- `5m` - Every 5 minutes
- `1h` - Every hour
- `24h` - Once per day

## Multiple Triggers

Workflows can have multiple triggers:

```yaml
triggers:
  - cron:
      schedule: "0 9 * * *"
  - webhook:
      path: /manual
steps:
  - id: run
    llm:
      prompt: "Execute task"
```

## Trigger Context

Access trigger information in steps:

```yaml
steps:
  - id: log
    llm:
      prompt: |
        Trigger type: ${trigger.type}
        Triggered at: ${trigger.timestamp}
        Data: ${trigger.payload}
```

Available fields:
- `${trigger.type}` - cron, webhook, or poll
- `${trigger.timestamp}` - When triggered
- `${trigger.payload}` - Webhook request body
- `${trigger.data}` - Poll source data

## Running Triggered Workflows

### With Controller

Deploy to a controller for automatic execution:

```bash
conductor controller start --workflows ./workflows/
```

The controller monitors triggers and executes workflows.

### Manual Execution

Run triggered workflows manually:

```bash
conductor run workflow.yaml
```

Triggers are ignored when running manually.

## Deployment

Deploy workflows with triggers:

```bash
# Deploy to exe.dev
conductor deploy workflow.yaml --target exe.dev

# Deploy to your server
scp workflow.yaml server:/opt/conductor/
ssh server "conductor controller start --workflows /opt/conductor/"
```
