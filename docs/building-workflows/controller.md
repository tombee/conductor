# Controller

Run Conductor as a long-lived service with API and webhook support.

## Overview

Controller mode enables:
- **HTTP API** for triggering workflows remotely
- **Webhooks** from GitHub, Slack, and other services
- **Scheduled workflows** (cron-style triggers)
- **Persistent workflow state**

## Quick Start

Install and start the controller:

```bash
# Install conductor
curl -L https://github.com/tombee/conductor/releases/latest/download/conductor-$(uname -s)-$(uname -m) -o conductor
chmod +x conductor
sudo mv conductor /usr/local/bin/

# Start with defaults
conductor
```

The controller starts with:
- Socket listener at `~/.conductor/conductor.sock`
- TCP listener at `:9000` (disabled by default, enable with `--tcp`)
- Workflows loaded from `./workflows`

## Configuration

### Command-Line Flags

```bash
conductor \
  --tcp=:9000 \
  --workflows-dir=/etc/conductor/workflows \
  --log-level=info
```

**Key flags:**
- `--tcp` — Enable TCP listener (e.g., `:9000`)
- `--workflows-dir` — Directory containing workflow YAML files
- `--log-level` — `debug`, `info`, `warn`, `error`
- `--socket` — Unix socket path (default: `~/.conductor/conductor.sock`)

### Configuration File

Create `~/.config/conductor/config.yaml`:

```conductor
controller:
  listen:
    tcp: :9000
    socket: ~/.conductor/conductor.sock

workflows:
  directory: ./workflows
  auto_reload: true

logging:
  level: info
  format: json
```

Start with config file:
```bash
conductor --config ~/.config/conductor/config.yaml
```

## HTTP API

### Trigger a Workflow

```bash
curl -X POST http://localhost:9000/workflows/code-review/run \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": {
      "pr_number": "123",
      "repo": "owner/repo"
    }
  }'
```

**Response:**
```json
{
  "run_id": "run_abc123",
  "status": "running",
  "workflow": "code-review"
}
```

### Get Run Status

```bash
curl http://localhost:9000/runs/run_abc123
```

**Response:**
```json
{
  "run_id": "run_abc123",
  "workflow": "code-review",
  "status": "completed",
  "outputs": {
    "report": "# Code Review\n..."
  },
  "started_at": "2025-01-15T10:00:00Z",
  "completed_at": "2025-01-15T10:02:15Z"
}
```

### List Runs

```bash
curl http://localhost:9000/runs?workflow=code-review&status=completed
```

## Webhooks

Triggers are configured via the CLI, not inline in workflow files. This keeps workflows portable across environments.

### GitHub Webhooks

**Workflow file (portable, no trigger):**
```yaml
name: pr-review
inputs:
  - name: pr_number
    type: number
  - name: repo
    type: string

steps:
  - id: review
    type: llm
    prompt: "Review PR #{{.inputs.pr_number}} in {{.inputs.repo}}..."
```

**Add the trigger via CLI:**
```bash
conductor triggers add webhook pr-review.yaml \
  --path=/webhooks/github/pr-review \
  --source=github \
  --events=pull_request.opened,pull_request.synchronize \
  --secret='${GITHUB_WEBHOOK_SECRET}'
```

**GitHub webhook URL:**
```
http://your-server:9000/webhooks/github/pr-review
```

**GitHub webhook settings:**
- Payload URL: Your controller URL
- Content type: `application/json`
- Events: Pull requests
- Secret: Match the secret configured in the trigger

### Slack Webhooks

**Workflow file:**
```yaml
name: slack-assistant
inputs:
  - name: text
    type: string
  - name: channel
    type: string

steps:
  - id: respond
    type: llm
    prompt: "Respond to: {{.inputs.text}}"

  - id: post
    slack.post_message:
      channel: "{{.inputs.channel}}"
      text: "{{.steps.respond.response}}"
```

**Add the trigger via CLI:**
```bash
conductor triggers add webhook slack-assistant.yaml \
  --path=/webhooks/slack/slack-assistant \
  --source=slack \
  --secret='${SLACK_SIGNING_SECRET}'
```

**Slack webhook URL:**
```
http://your-server:9000/webhooks/slack/slack-assistant
```

## Scheduled Workflows

**Workflow file:**
```yaml
name: daily-report
steps:
  - id: generate
    type: llm
    prompt: "Generate today's summary report..."

  - id: send
    slack.post_message:
      channel: "#reports"
      text: "{{.steps.generate.response}}"
```

**Add the schedule trigger via CLI:**
```bash
conductor triggers add schedule daily-report.yaml \
  --name=daily-9am \
  --cron="0 9 * * *"
```

**Cron format:**
```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sunday=0)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

**Common schedules:**
- `0 * * * *` — Every hour
- `0 9 * * *` — Daily at 9 AM
- `0 9 * * 1` — Every Monday at 9 AM
- `*/15 * * * *` — Every 15 minutes

## Authentication

Secure your controller with API key authentication:

```conductor
controller:
  auth:
    enabled: true
    api_key: your-secret-key-here
```

**Use API key in requests:**
```bash
curl -X POST http://localhost:9000/workflows/code-review/run \
  -H "Authorization: Bearer your-secret-key-here" \
  -H "Content-Type: application/json" \
  -d '{"inputs": {...}}'
```

## Systemd Service

Run as a system service on Linux:

```ini
# /etc/systemd/system/conductor.service
[Unit]
Description=Conductor Controller
After=network.target

[Service]
Type=simple
User=conductor
ExecStart=/usr/local/bin/conductor --config /etc/conductor/config.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

**Enable and start:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable conductor
sudo systemctl start conductor
sudo systemctl status conductor
```

## Monitoring

### Health Check

```bash
curl http://localhost:9000/health
```

**Response:**
```json
{
  "status": "healthy",
  "uptime": "2h15m30s",
  "workflows": 5,
  "active_runs": 2
}
```

### Metrics

Access Prometheus-compatible metrics:

```bash
curl http://localhost:9000/metrics
```

## Logs

**View logs:**
```bash
# If running with systemd
sudo journalctl -u conductor -f

# If running in foreground
# Logs to stdout/stderr
```

**Log formats:**
- `text` — Human-readable (default)
- `json` — Structured for log aggregation

**Configure in config.yaml:**
```conductor
logging:
  level: info
  format: json
  output: /var/log/conductor/conductor.log
```

## Best Practices

**1. Use authentication in production:**
```conductor
controller:
  auth:
    enabled: true
    api_key: ${API_KEY}  # From environment
```

**2. Validate webhook signatures:**
```bash
# Include secret when adding webhook triggers
conductor triggers add webhook pr-review.yaml \
  --path=/webhooks/github/pr-review \
  --source=github \
  --secret='${GITHUB_WEBHOOK_SECRET}'
```

**3. Set resource limits:**
```conductor
controller:
  max_concurrent_runs: 10
  run_timeout: 30m
```

**4. Enable structured logging:**
```conductor
logging:
  format: json
  level: info
```

**5. Monitor workflow execution:**
- Check `/health` endpoint
- Track metrics at `/metrics`
- Set up alerts for failures

## Deployment

See [Deployment Guide](../production/deployment.md) for:
- Docker deployment
- Kubernetes setup
- exe.dev hosting
- Bare metal installation

## Troubleshooting

**Controller won't start:**

Check port availability:
```bash
lsof -i :9000
```

Check configuration:
```bash
conductor --config config.yaml --validate
```

**Workflows not loading:**

Verify workflows directory:
```bash
ls -la ./workflows/*.yaml
conductor --workflows-dir=./workflows --log-level=debug
```

**Webhooks not triggering:**

Check webhook delivery in GitHub/Slack settings.

Verify URL is accessible:
```bash
curl http://your-server:9000/health
```

Check logs for webhook errors:
```bash
sudo journalctl -u conductor -f | grep webhook
```

For more help, see [Troubleshooting Guide](../production/troubleshooting.md).
