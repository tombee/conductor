# Poll Triggers Guide

Poll triggers enable workflows to react to events from external services (PagerDuty, Slack, Jira, Datadog) without requiring webhooks or public endpoints. This makes them ideal for personal automation running on laptops.

## Why Poll Triggers?

**Webhooks don't work for personal automation:**
- Laptops behind NAT/VPN have no public IP
- Enterprise tools require admin access to configure webhooks
- Setting up ngrok tunnels or port forwarding is complex
- Laptops sleep, networks change, tunnels disconnect

**Poll triggers solve this by:**
- Using API tokens (no admin access needed)
- Querying APIs periodically (30-60 second latency is acceptable)
- Handling laptop sleep/wake gracefully
- Tracking state to avoid duplicate triggers

## Quick Start

### 1. Configure Integration

First, configure the integration you want to poll:

```bash
# PagerDuty
conductor integrations pagerduty configure

# Slack (requires bot token)
conductor integrations slack configure

# Jira
conductor integrations jira configure

# Datadog
conductor integrations datadog configure
```

### 2. Find Your IDs

Poll triggers use explicit filters - you need to provide your user ID or service IDs:

**PagerDuty:**
```bash
# Get your user ID from profile URL: https://mycompany.pagerduty.com/users/PUSER123
# Or use: conductor integrations pagerduty whoami
```

**Slack:**
- Use your @username (e.g., "@jsmith")
- Or get member ID from profile

**Jira:**
- Use your Jira username (visible in profile)
- Project key is the prefix (e.g., "MYTEAM" from "MYTEAM-123")

**Datadog:**
- Use existing monitor tags (e.g., "service:api", "team:platform")
- Monitor IDs visible in monitor URL

### 3. Create Poll Trigger Workflow

Create a workflow file with a poll trigger:

```yaml
name: pagerduty-triage-assistant
description: AI triage when I get paged

trigger:
  type: poll
  poll:
    integration: pagerduty
    query:
      user_id: "PUSER123"  # Replace with your user ID
      services: ["PSVC001", "PSVC002"]  # Services you care about
      statuses: [triggered, acknowledged]
    interval: 30s
    input_mapping:
      incident_id: "{{.trigger.event.id}}"
      incident_title: "{{.trigger.event.title}}"
      urgency: "{{.trigger.event.urgency}}"

steps:
  - id: triage
    action: llm
    model: balanced
    prompt: |
      I just got paged:

      Title: {{.incident_title}}
      Urgency: {{.urgency}}

      Help me triage:
      1. What might be causing this?
      2. What should I check first?
      3. Any quick fixes to try?
```

### 4. Start the Controller

Poll triggers require the controller to be running:

```bash
conductor controller start
```

The controller will:
- Register your poll trigger
- Poll every 30 seconds
- Fire your workflow for new incidents
- Track seen events to avoid duplicates

## Complete Examples

### PagerDuty: Personal Triage Assistant

Triggers when YOU specifically are assigned to an incident:

```yaml
name: my-pagerduty-triage
description: AI-assisted triage when I personally get paged

trigger:
  type: poll
  poll:
    integration: pagerduty
    query:
      user_id: "PUSER123"  # Your PagerDuty user ID
      statuses: [triggered, acknowledged]
    interval: 30s

steps:
  - id: triage
    action: llm
    model: balanced
    prompt: |
      I got paged:

      Title: {{.trigger.event.title}}
      Service: {{.trigger.event.service.name}}
      Urgency: {{.trigger.event.urgency}}
      URL: {{.trigger.event.html_url}}

      Provide triage assistance:
      1. Likely causes
      2. First steps to investigate
      3. Quick fixes to try
```

### PagerDuty: Team Service Monitor

Triggers for ANY incident on services you care about (not just assigned to you):

```yaml
name: team-service-monitor
description: Stay informed about incidents on my team's services

trigger:
  type: poll
  poll:
    integration: pagerduty
    query:
      services: ["PSVC001", "PSVC002", "PSVC003"]  # Your team's services
      statuses: [triggered]
    interval: 60s

steps:
  - id: summarize
    action: llm
    model: fast
    prompt: |
      New incident on a service I work on:

      Title: {{.trigger.event.title}}
      Service: {{.trigger.event.service.name}}
      Assigned to: {{.trigger.event.assignments}}

      Quick summary: should I be aware of this?
```

### Slack: Mention Responder

```yaml
name: slack-mentions
description: Draft responses when mentioned in Slack

trigger:
  type: poll
  poll:
    integration: slack
    query:
      mentions: "@jsmith"  # Your Slack username
      channels: [engineering, oncall, incidents]
      include_threads: true
      exclude_bots: true
    interval: 30s

steps:
  - id: draft
    action: llm
    model: fast
    prompt: |
      I was mentioned in Slack:

      Channel: #{{.trigger.event.channel.name}}
      From: {{.trigger.event.user.name}}
      Message: {{.trigger.event.text}}

      Draft a helpful, concise response.
```

### Jira: Ticket Summarizer

```yaml
name: jira-assigned
description: Summarize tickets assigned to me

trigger:
  type: poll
  poll:
    integration: jira
    query:
      assignee: "jsmith"  # Your Jira username
      project: MYTEAM
    interval: 60s

steps:
  - id: summarize
    action: llm
    model: balanced
    prompt: |
      New ticket assigned to me:

      Key: {{.trigger.event.key}}
      Summary: {{.trigger.event.summary}}
      Type: {{.trigger.event.issue_type}}
      Priority: {{.trigger.event.priority}}
      Reporter: {{.trigger.event.reporter.name}}

      Description:
      {{.trigger.event.description}}

      Please:
      1. Summarize in 2-3 sentences
      2. Identify ambiguities
      3. Suggest approach
      4. Estimate complexity
```

### Datadog: Alert Digest

```yaml
name: datadog-service-alerts
description: Get summaries of alerts on my services

trigger:
  type: poll
  poll:
    integration: datadog
    query:
      tags: ["team:platform", "service:api", "service:worker"]
      statuses: [triggered, warn]
    interval: 60s

steps:
  - id: analyze
    action: llm
    model: balanced
    prompt: |
      Alert on a service I work on:

      Monitor: {{.trigger.event.name}}
      Status: {{.trigger.event.status}}
      Tags: {{.trigger.event.tags}}
      Message: {{.trigger.event.message}}

      Analyze:
      1. What's happening?
      2. Customer-impacting?
      3. What to check first?
```

## Advanced Configuration

### Startup Behavior

Control what happens when the controller starts:

```yaml
poll:
  integration: pagerduty
  startup: since_last  # Default: process events since last poll

  # OR
  startup: ignore_historical  # Only new events from now forward

  # OR
  startup: backfill  # Process events from specified time ago
  backfill: 4h  # Last 4 hours
```

**When to use:**
- `since_last` - Default, handles laptop sleep/wake well
- `ignore_historical` - First run, don't want old events
- `backfill` - Testing or catching up after downtime

### Rate Limiting

The controller automatically:
- Enforces minimum 10s poll interval
- Applies exponential backoff on 429 errors (30s, 60s, 120s, max 10m)
- Shares rate limit budget across triggers for same integration

### Circuit Breaker

After consecutive errors:
- 5 errors: Log ERROR level warning
- 10 errors: Pause polling, require manual reset

Reset a paused trigger:

```bash
conductor triggers reset <trigger-name>
```

### Event Deduplication

Poll triggers use timestamp-first deduplication:

1. **Primary:** Always query API with `since` timestamp filter
   - API only returns events newer than last poll time
   - Old events never seen again, regardless of TTL

2. **Secondary:** Dedupe within poll window using seen events
   - Handles retries and overlapping time windows
   - Seen events pruned after 24h (safe due to timestamp filter)

**Result:** No duplicate triggers, even after controller restart

## Monitoring

### View Poll Triggers

```bash
# List all poll triggers
conductor triggers list --type poll

# Show trigger details
conductor triggers show <trigger-name>

# Test without firing (dry run)
conductor triggers test <trigger-name>
```

### Metrics

If metrics are enabled, poll triggers expose:

- `conductor_poll_trigger_polls_total` - Poll executions (by integration, status)
- `conductor_poll_trigger_events_total` - Events detected (by integration, type)
- `conductor_poll_trigger_latency_seconds` - Poll execution time
- `conductor_poll_trigger_errors_total` - Errors (by integration, error_type)
- `conductor_poll_trigger_active` - Number of active triggers

### Troubleshooting

**Poll trigger not detecting events:**
1. Verify credentials: `conductor integrations <name> test`
2. Check trigger status: `conductor triggers list --type poll`
3. Test manually: `conductor triggers test <name>`
4. Check error count - 10 errors pauses polling
5. Verify query filters match your account
6. Check integration API rate limits

**Duplicate triggers:**
1. Check deduplication state: `conductor triggers show <name>`
2. Reset if state corrupted: `conductor triggers reset <name>`
3. Verify timestamp-first dedup working

**High API usage:**
1. Increase poll interval (default 30s, min 10s)
2. Consolidate multiple triggers on same integration
3. Check for 429 rate limit errors

**Events missed during laptop sleep:**
- Poll triggers resume automatically on wake
- `startup: since_last` catches up from last poll time
- If very old, may hit API historical query limits

## Best Practices

1. **Use explicit filters:**
   - Always provide your user_id, username, or service IDs
   - Don't rely on "current user" - be explicit

2. **Choose appropriate intervals:**
   - 30s for urgent (PagerDuty incidents)
   - 60s for normal (Jira tickets, Datadog alerts)
   - 5m+ for low-priority (reports, summaries)

3. **Filter at the API level:**
   - Use `services:`, `channels:`, `projects:` to reduce events
   - More efficient than filtering in workflow steps

4. **Handle laptop sleep:**
   - Use `startup: since_last` (default)
   - Consider `backfill: 4h` for critical workflows

5. **Test before deploying:**
   - Use `conductor triggers test <name>` to verify
   - Check what events match your filters
   - Ensure credentials are valid

## Security

- API tokens stored securely via credential provider
- Tokens never appear in logs or error messages
- Poll state at rest encrypted when `CONDUCTOR_ENCRYPTION_KEY` is set
- Query parameters validated to prevent injection attacks

## Limitations

- Minimum poll interval: 10 seconds
- Maximum backfill: 24 hours
- Seen events: 10,000 per trigger (FIFO eviction)
- Poll timeout: 10 seconds per execution
- Circuit breaker: 10 consecutive errors pauses polling

## Integration-Specific Notes

### Slack
- **Requires bot tokens (`xoxb-`)** - user tokens with rotation NOT supported
- Bot never expires, works for any channel bot is invited to
- For private channels, invite bot first

### PagerDuty
- Use `services:` to filter by services you care about
- `user_id:` filters to incidents assigned to you
- Combine both for "incidents on my services assigned to me"

### Jira
- Query parameters are validated and escaped
- Special characters rejected to prevent JQL injection
- Supports both Jira Cloud and Server

### Datadog
- Use monitor tags to filter by services you own
- Can specify `monitor_ids:` for specific monitors
- Alert resolution events (`status: ok`) can be included/excluded

## Next Steps

- [Workflow Schema Reference](../reference/workflow-schema.md#poll-trigger)
- [Integration Guides](../reference/integrations/)
- [Controller Configuration](../reference/configuration.md)
