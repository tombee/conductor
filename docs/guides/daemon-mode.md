# Daemon Mode

This guide covers running Conductor as a daemon (`conductord`) for production deployments, webhook handling, scheduled workflows, and distributed operation.

## What is Daemon Mode?

Daemon mode runs Conductor as a long-running service that can:

- Accept webhook requests to trigger workflows
- Execute workflows on schedules (cron)
- Provide an API for remote workflow execution
- Persist workflow state for crash recovery
- Run in distributed mode with multiple instances

## When to Use Daemon Mode

Use daemon mode when you need:

**Automated Triggering**
- GitHub webhooks for PR analysis
- Slack events for notifications
- Custom webhooks from external systems

**Scheduled Execution**
- Daily reports
- Periodic data syncing
- Scheduled maintenance tasks

**Always-On Operation**
- Production workflows that must be available 24/7
- Team-shared workflow execution
- Central workflow management

**Don't use daemon mode** for:
- One-off workflow execution (use `conductor run` instead)
- Local development and testing
- Personal automation (CLI is simpler)

## Quick Start

### Install conductord

```bash
# Install from releases
curl -L https://github.com/tombee/conductor/releases/latest/download/conductord-macos-amd64 -o conductord
chmod +x conductord
sudo mv conductord /usr/local/bin/

# Or build from source
cd cmd/conductord
go build -o conductord
```

### Basic Daemon Start

Start with defaults:

```bash
conductord
```

This starts conductord with:
- Unix socket listener at `~/.conductor/conductor.sock`
- In-memory workflow storage
- Workflows loaded from `./workflows` directory
- Info-level logging

### Configuration via Flags

Customize behavior with flags:

```bash
conductord \
  --socket=/var/run/conductor.sock \
  --workflows-dir=/etc/conductor/workflows \
  --backend=memory \
  --log-level=debug
```

### Configuration via Environment

Set configuration through environment variables:

```bash
export CONDUCTOR_SOCKET=/var/run/conductor.sock
export CONDUCTOR_LOG_LEVEL=info
export CONDUCTOR_PID_FILE=/var/run/conductor.pid

conductord
```

## Configuration

### Listen Configuration

conductord can listen on Unix sockets or TCP:

**Unix Socket (Default - Recommended)**

```bash
conductord --socket=/var/run/conductor.sock
```

Advantages:
- File-system permissions for access control
- No network exposure
- Better performance for local clients

**TCP Address**

```bash
conductord --tcp=:9000
```

Warning: Requires authentication (see Security section)

**Both Unix Socket and TCP**

```bash
conductord \
  --socket=/var/run/conductor.sock \
  --tcp=localhost:9000
```

**Remote TCP (Use with Caution)**

```bash
conductord \
  --tcp=0.0.0.0:9000 \
  --allow-remote \
  --tls-cert=/path/to/cert.pem \
  --tls-key=/path/to/key.pem
```

### Storage Backend

#### Memory Backend (Default)

Fast but not persistent - workflows lost on restart:

```bash
conductord --backend=memory
```

Use for:
- Development and testing
- Stateless workflows
- Temporary deployments

#### PostgreSQL Backend

Persistent storage with support for distributed mode:

```bash
conductord \
  --backend=postgres \
  --postgres-url="postgresql://user:pass@localhost/conductor"
```

PostgreSQL configuration options:

```bash
# Connection string
--postgres-url="postgresql://user:pass@host:5432/conductor?sslmode=require"

# Connection pool settings (via environment)
export CONDUCTOR_POSTGRES_MAX_OPEN_CONNS=25
export CONDUCTOR_POSTGRES_MAX_IDLE_CONNS=10
export CONDUCTOR_POSTGRES_CONN_MAX_LIFETIME=300  # seconds
```

Use for:
- Production deployments
- Long-running workflows
- Distributed mode
- Workflow history and auditing

### Workflow Directory

Specify where workflow YAML files are located:

```bash
conductord --workflows-dir=/etc/conductor/workflows
```

conductord will:
- Load all `.yaml` files from this directory
- Watch for changes and reload automatically
- Validate workflows on startup

Directory structure:

```
/etc/conductor/workflows/
├── pr-review.yaml
├── issue-triage.yaml
├── daily-report.yaml
└── webhooks/
    ├── github-pr.yaml
    └── slack-events.yaml
```

### Logging

Configure logging level and format:

```bash
conductord \
  --log-level=debug \
  --log-format=json
```

**Log levels:**
- `debug` - Verbose debugging information
- `info` - General operational information (default)
- `warn` - Warning messages
- `error` - Error messages only

**Log formats:**
- `text` - Human-readable (default)
- `json` - Structured JSON for log aggregation

### PID File

Write process ID to file for process management:

```bash
conductord --pid-file=/var/run/conductor.pid
```

Useful for:
- Init scripts
- Process monitoring
- Graceful restarts

## Webhooks

Webhooks trigger workflows from external events.

### Defining Webhook Workflows

Add webhook triggers to workflow definitions:

```yaml
name: github-pr-review
description: Analyze pull requests from GitHub webhooks

triggers:
  - type: webhook
    webhook:
      path: /webhooks/github/pr
      source: github
      events:
        - pull_request.opened
        - pull_request.synchronize
      secret: ${GITHUB_WEBHOOK_SECRET}
      input_mapping:
        pr_url: $.pull_request.html_url
        pr_number: $.pull_request.number
        repo: $.repository.full_name

inputs:
  - name: pr_url
    type: string
    required: true
  - name: pr_number
    type: number
    required: true
  - name: repo
    type: string
    required: true

steps:
  - id: fetch_pr_diff
    type: action
    action: http
    inputs:
      url: "{{.pr_url}}.diff"

  - id: review
    type: llm
    inputs:
      model: balanced
      system: "You are a code reviewer"
      prompt: "Review this PR:\n{{$.fetch_pr_diff.body}}"

outputs:
  - name: review_comments
    value: $.review.content
```

### Webhook Sources

**GitHub**

```yaml
webhook:
  path: /webhooks/github/pr
  source: github
  secret: ${GITHUB_WEBHOOK_SECRET}
  events:
    - pull_request
    - push
    - issues
```

conductord verifies GitHub webhook signatures using the secret.

**Slack**

```yaml
webhook:
  path: /webhooks/slack/commands
  source: slack
  secret: ${SLACK_SIGNING_SECRET}
```

**Generic (Custom)**

```yaml
webhook:
  path: /webhooks/custom/trigger
  source: generic
  input_mapping:
    custom_field: $.data.field
```

### Configuring GitHub Webhooks

1. In your GitHub repository, go to Settings → Webhooks
2. Click "Add webhook"
3. Set Payload URL: `https://your-server.com/webhooks/github/pr`
4. Set Content type: `application/json`
5. Set Secret: (same as GITHUB_WEBHOOK_SECRET)
6. Select events: Pull requests, Pushes, etc.
7. Click "Add webhook"

### Testing Webhooks Locally

Use ngrok to expose local conductord:

```bash
# Start conductord
conductord --tcp=localhost:9000

# In another terminal, start ngrok
ngrok http 9000

# Use the ngrok URL in webhook configuration
# https://abc123.ngrok.io/webhooks/github/pr
```

## Scheduled Workflows

Execute workflows on a schedule using cron expressions.

### Defining Scheduled Workflows

```yaml
name: daily-report
description: Generate daily analytics report

triggers:
  - type: schedule
    schedule:
      cron: "0 9 * * *"  # Every day at 9 AM
      timezone: "America/New_York"
      enabled: true
      inputs:
        report_type: "daily"

inputs:
  - name: report_type
    type: string
    default: "daily"

steps:
  - id: fetch_data
    type: action
    action: http
    inputs:
      url: "https://analytics-api.example.com/stats"

  - id: generate_report
    type: llm
    inputs:
      model: balanced
      prompt: "Generate a {{.report_type}} report from: {{$.fetch_data.body}}"

  - id: send_email
    type: action
    action: http
    inputs:
      method: POST
      url: "https://email-api.example.com/send"
      body: "{{$.generate_report.content}}"
```

### Cron Expression Format

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday = 0)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

**Common schedules:**

```yaml
# Every hour
cron: "0 * * * *"

# Every day at 2:30 AM
cron: "30 2 * * *"

# Every Monday at 9 AM
cron: "0 9 * * 1"

# Every 15 minutes
cron: "*/15 * * * *"

# First day of month at midnight
cron: "0 0 1 * *"
```

### Timezone Support

Specify timezone for cron evaluation:

```yaml
schedule:
  cron: "0 9 * * *"
  timezone: "America/Los_Angeles"
```

Without timezone, uses server's local time.

## Distributed Mode

Run multiple conductord instances for high availability and load distribution.

### Requirements

Distributed mode requires:
- PostgreSQL backend (for shared state)
- Unique instance IDs
- Leader election for scheduler

### Configuration

```bash
conductord \
  --backend=postgres \
  --postgres-url="postgresql://user:pass@db-host/conductor" \
  --distributed \
  --instance-id="conductor-1"
```

**Instance ID:**

If not provided, a random UUID is generated. For production, set explicit IDs:

```bash
# Instance 1
conductord --distributed --instance-id=conductor-prod-1

# Instance 2
conductord --distributed --instance-id=conductor-prod-2

# Instance 3
conductord --distributed --instance-id=conductor-prod-3
```

### Leader Election

Only one instance runs the scheduler to avoid duplicate executions.

conductord automatically:
- Elects a leader on startup
- Re-elects if leader fails
- Distributes webhook/API requests across all instances

### Stalled Job Recovery

If an instance crashes mid-workflow, other instances detect and recover:

```bash
conductord \
  --distributed \
  --stalled-job-timeout=300  # 5 minutes
```

Jobs locked longer than timeout are considered stalled and reassigned.

### Monitoring Distributed Instances

Check instance health:

```bash
# Get cluster status
curl http://localhost:9000/api/v1/cluster/status

# Response:
{
  "instances": [
    {
      "id": "conductor-prod-1",
      "is_leader": true,
      "last_heartbeat": "2025-12-23T15:30:00Z",
      "active_workflows": 5
    },
    {
      "id": "conductor-prod-2",
      "is_leader": false,
      "last_heartbeat": "2025-12-23T15:30:01Z",
      "active_workflows": 3
    }
  ]
}
```

## Checkpoints and Recovery

conductord saves workflow state periodically for crash recovery.

### Enabling Checkpoints

```bash
conductord --checkpoint-interval=60  # Save state every 60 seconds
```

### Checkpoint Storage

Checkpoints are stored in:
- Memory backend: Not persistent (lost on restart)
- PostgreSQL backend: Database table

### Recovery on Restart

When conductord restarts:
1. Loads last checkpoint
2. Resumes in-progress workflows from last saved state
3. Retries failed steps based on retry configuration

## Process Management

### systemd Service

Create `/etc/systemd/system/conductord.service`:

```ini
[Unit]
Description=Conductor Workflow Daemon
After=network.target postgresql.service

[Service]
Type=simple
User=conductor
Group=conductor
WorkingDirectory=/var/lib/conductor
ExecStart=/usr/local/bin/conductord \
  --socket=/var/run/conductor.sock \
  --workflows-dir=/etc/conductor/workflows \
  --backend=postgres \
  --postgres-url=postgresql://conductor:pass@localhost/conductor \
  --pid-file=/var/run/conductor.pid \
  --log-level=info
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Manage the service:

```bash
# Start
sudo systemctl start conductord

# Enable on boot
sudo systemctl enable conductord

# Check status
sudo systemctl status conductord

# View logs
sudo journalctl -u conductord -f

# Restart
sudo systemctl restart conductord

# Stop
sudo systemctl stop conductord
```

### Docker

Run conductord in Docker:

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o conductord cmd/conductord/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/conductord .
COPY workflows/ /workflows/

EXPOSE 9000

CMD ["./conductord", \
     "--tcp=:9000", \
     "--workflows-dir=/workflows", \
     "--backend=postgres", \
     "--postgres-url=${POSTGRES_URL}"]
```

Run with docker-compose:

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: conductor
      POSTGRES_USER: conductor
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data

  conductord:
    build: .
    ports:
      - "9000:9000"
    environment:
      POSTGRES_URL: postgresql://conductor:${DB_PASSWORD}@postgres/conductor
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
    volumes:
      - ./workflows:/workflows
    depends_on:
      - postgres
    restart: unless-stopped

volumes:
  postgres-data:
```

## Security

### Authentication

Enable API key authentication:

```bash
export CONDUCTOR_AUTH_ENABLED=true
export CONDUCTOR_AUTH_API_KEYS="key1,key2,key3"

conductord --tcp=:9000
```

Clients must include API key using one of these secure methods:

```bash
# Option 1: X-API-Key header (recommended)
curl -H "X-API-Key: key1" http://localhost:9000/api/v1/workflows

# Option 2: Authorization header with Bearer token
curl -H "Authorization: Bearer key1" http://localhost:9000/api/v1/workflows
```

**Note:** Query parameter authentication (`?api_key=key`) is not supported for security reasons (prevents credential leakage in logs and browser history). Use header-based authentication only.

### Unix Socket Permissions

Control access via file permissions:

```bash
# Start daemon
conductord --socket=/var/run/conductor.sock

# Set permissions
sudo chmod 660 /var/run/conductor.sock
sudo chown conductor:conductor-users /var/run/conductor.sock
```

Only members of `conductor-users` group can connect.

### TLS for TCP

Use TLS to encrypt TCP connections:

```bash
conductord \
  --tcp=:9000 \
  --tls-cert=/path/to/server.crt \
  --tls-key=/path/to/server.key
```

Generate self-signed certificates for testing:

```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
```

### Webhook Secret Verification

Always configure secrets for webhook sources:

```yaml
webhook:
  secret: ${WEBHOOK_SECRET}
```

conductord verifies signatures to prevent unauthorized workflow triggers.

### Environment Variable Security

Never commit secrets to version control:

```bash
# Good - use environment variables
webhook:
  secret: ${GITHUB_WEBHOOK_SECRET}

# Bad - hardcoded secret
webhook:
  secret: "my-secret-key"
```

Store secrets in:
- Environment files (`.env` with restricted permissions)
- Secret management systems (HashiCorp Vault, AWS Secrets Manager)
- Container orchestration secrets (Kubernetes Secrets)

## Monitoring

### Health Check Endpoint

Check daemon health:

```bash
curl http://localhost:9000/health

# Response:
{
  "status": "healthy",
  "uptime": "24h15m30s",
  "version": "v0.1.0"
}
```

### Metrics

conductord exposes metrics for monitoring:

```bash
curl http://localhost:9000/metrics
```

Metrics include:
- Active workflows
- Completed workflows
- Failed workflows
- Average execution time
- Queue depth
- LLM token usage

### Logging

View structured logs:

```bash
# If using systemd
sudo journalctl -u conductord -f

# If using Docker
docker logs -f conductord

# If running in foreground
conductord --log-format=json | jq
```

## Troubleshooting

### conductord won't start

Check socket/port availability:

```bash
# Unix socket
ls -la /var/run/conductor.sock
sudo rm /var/run/conductor.sock  # If stale

# TCP port
sudo lsof -i :9000
```

### Workflows not triggering

Check webhook configuration:

```bash
# Test webhook manually
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"test": "data"}' \
  http://localhost:9000/webhooks/test
```

Check logs for webhook verification failures.

### High memory usage

For long-running daemons with many workflows:

```bash
# Use PostgreSQL backend instead of memory
conductord --backend=postgres

# Limit concurrent workflows
conductord --max-concurrent-runs=10
```

### Database connection issues

Verify PostgreSQL connectivity:

```bash
psql "postgresql://conductor:pass@localhost/conductor"
```

Check connection pool settings if hitting connection limits.

## Best Practices

1. **Use PostgreSQL for production** - Memory backend is for development only

2. **Enable checkpoints** - For crash recovery

3. **Set reasonable timeouts** - Prevent runaway workflows

4. **Use distributed mode** - For high availability

5. **Monitor metrics** - Track workflow execution and failures

6. **Secure webhooks** - Always verify signatures

7. **Limit concurrency** - Prevent resource exhaustion

8. **Use TLS for remote TCP** - Encrypt network traffic

9. **Rotate logs** - Prevent disk space issues

10. **Test failure scenarios** - Verify recovery mechanisms work

## Next Steps

- Read [Workflows and Steps](../learn/concepts/workflows-steps.md) to create workflows
- Read [Error Handling](error-handling.md) for production-ready workflows
- See [Connectors](../reference/connectors/index.md) for integrating with external services
