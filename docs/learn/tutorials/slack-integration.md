# Creating Slack Integrations

Build workflows that integrate with Slack to send notifications, post reports, and update messages dynamically.

**Difficulty:** Intermediate
**Prerequisites:**
- Complete [Your First Workflow](first-workflow.md)
- Slack workspace where you can install apps
- Basic understanding of Slack bots and channels

---

## What You'll Build

A Slack notification workflow that:

1. Analyzes input data (e.g., test results, deployment status)
2. Uses an LLM to generate a formatted summary
3. Posts the summary to a Slack channel
4. Adds a reaction emoji to the message
5. Optionally uploads a detailed report file

This tutorial demonstrates:
- Connector configuration and authentication
- Using the Slack connector for messaging
- Template variables in connector inputs
- Workflow composition with LLMs and connectors
- File upload to Slack

---

## Step 1: Get a Slack Bot Token

Before you can use the Slack connector, you need a bot token:

### Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Click **"Create New App"**
3. Choose **"From scratch"**
4. Name: `conductor-bot`
5. Select your workspace

### Add Bot Scopes

1. Navigate to **"OAuth & Permissions"** in the sidebar
2. Scroll to **"Scopes" → "Bot Token Scopes"**
3. Add these scopes:
   - `chat:write` — Post messages
   - `chat:write.public` — Post to channels without joining
   - `files:write` — Upload files
   - `reactions:write` — Add emoji reactions

### Install to Workspace

1. Scroll to **"OAuth Tokens for Your Workspace"**
2. Click **"Install to Workspace"**
3. Authorize the app
4. Copy the **"Bot User OAuth Token"** (starts with `xoxb-`)

### Set Environment Variable

```bash
export SLACK_BOT_TOKEN="xoxb-your-token-here"
```

:::caution[Never Hardcode Tokens]
Always use environment variables for secrets:
```yaml
# GOOD - uses environment variable
auth:
  token: ${SLACK_BOT_TOKEN}

# BAD - hardcoded secret
auth:
  token: xoxb-1234-5678-abcd  # Never do this!
```
:::


---

## Step 2: Create the Workflow

Create `slack-notify.yaml`:

```yaml
name: slack-notification
description: Send AI-generated summaries to Slack
version: "1.0"
```

---

## Step 3: Configure the Slack Connector

Add connector configuration at the top level (same level as `inputs` and `steps`):

```yaml
connectors:
  slack:
    from: connectors/slack
    auth:
      token: ${SLACK_BOT_TOKEN}
```

**Breakdown:**

- `connectors:` — Top-level connector configuration
- `slack:` — Connector name (used in steps as `slack.operation`)
- `from: connectors/slack` — Import the official Slack connector
- `auth:` — Authentication configuration
- `token: ${SLACK_BOT_TOKEN}` — Reference environment variable

---

## Step 4: Define Inputs

Add inputs for the notification:

```yaml
inputs:
  - name: event_type
    type: string
    required: true
    description: Type of event (deployment, test_run, alert, etc.)

  - name: status
    type: string
    required: true
    enum: ["success", "failure", "warning", "info"]
    description: Event status

  - name: details
    type: string
    required: true
    description: Event details or raw data

  - name: channel
    type: string
    default: "#engineering"
    description: Slack channel to post to

  - name: include_report
    type: boolean
    default: false
    description: Whether to upload a detailed report file
```

**Key features:**

- `enum:` — Restricts input to specific values
- `default:` — Provides sensible defaults
- `type: boolean` — For yes/no flags

---

## Step 5: Generate the Summary with an LLM

Use an LLM to create a well-formatted Slack message:

```yaml
steps:
  - id: generate_summary
    name: Generate Slack Message
    type: llm
    model: fast
    system: |
      You are a notification assistant that creates clear, concise Slack messages.

      Format rules:
      - Use Slack markdown (*bold*, _italic_, `code`)
      - Keep it under 200 words
      - Start with an emoji that matches the status
      - Include key metrics if available
      - End with actionable next steps if needed
    prompt: |
      Create a Slack notification for this event:

      **Type:** {{.inputs.event_type}}
      **Status:** {{.inputs.status}}
      **Details:**
      {{.inputs.details}}

      Generate a well-formatted message suitable for Slack.
      Use appropriate emojis and markdown formatting.
```

**Why use an LLM?** It intelligently formats raw data into readable messages, extracts key information, and adapts tone to the status.

---

## Step 6: Post to Slack

Add a step to post the message:

```yaml
  - id: post_message
    name: Post to Slack Channel
    slack.post_message:
      channel: "{{.inputs.channel}}"
      text: "{{.steps.generate_summary.response}}"
```

**Connector shorthand syntax:**

- `slack.post_message:` — Shorthand for Slack connector operation
- Expands to: `type: connector` + `connector: slack.post_message`
- Inputs are operation-specific (see connector docs)

**Template variables in connector inputs:**

- `{{.inputs.channel}}` — Uses the workflow input
- `{{.steps.generate_summary.response}}` — Uses the LLM's output

---

## Step 7: Add a Reaction Emoji

React to the message based on status:

```yaml
  - id: add_reaction
    name: Add Status Reaction
    slack.add_reaction:
      channel: "{{.inputs.channel}}"
      timestamp: "{{.steps.post_message.ts}}"
      name: |
        {{if eq .inputs.status "success"}}white_check_mark{{end}}
        {{if eq .inputs.status "failure"}}x{{end}}
        {{if eq .inputs.status "warning"}}warning{{end}}
        {{if eq .inputs.status "info"}}information_source{{end}}
```

**Key concepts:**

- `{{.steps.post_message.ts}}` — Message timestamp from previous step
- `name:` — Emoji name without colons (e.g., `white_check_mark` not `:white_check_mark:`)
- Template conditionals — `{{if eq .inputs.status "success"}}`

**Connector outputs:**

The `slack.post_message` operation returns:
```json
{
  "ok": true,
  "ts": "1234567890.123456",
  "channel": "C1234567890",
  "message": {...}
}
```

Access with `{{.steps.post_message.ts}}`

---

## Step 8: Conditionally Upload a Report

If `include_report` is true, generate and upload a detailed report:

```yaml
  - id: generate_report
    name: Generate Detailed Report
    type: llm
    model: balanced
    condition:
      expression: 'inputs.include_report == true'
    system: |
      You are a technical report writer.
      Create detailed reports in markdown format with:
      - Executive summary
      - Detailed findings
      - Metrics and data
      - Recommendations
    prompt: |
      Create a detailed report for this {{.inputs.event_type}}:

      Status: {{.inputs.status}}
      Details: {{.inputs.details}}

      Output a complete markdown document.

  - id: upload_report
    name: Upload Report to Slack
    condition:
      expression: 'inputs.include_report == true'
    slack.upload_file:
      channels: "{{.inputs.channel}}"
      content: "{{.steps.generate_report.response}}"
      filename: "report-{{.inputs.event_type}}-{{.inputs.status}}.md"
      title: "Detailed {{.inputs.event_type}} Report"
      initial_comment: "Full report attached"
```

**Conditional execution:**

- `condition:` — Only runs if expression is true
- Both steps check `inputs.include_report`
- If false, these steps are skipped

**File upload parameters:**

- `channels:` — Can be a single channel or comma-separated list
- `content:` — File content (text or binary)
- `filename:` — Name of the uploaded file
- `initial_comment:` — Message posted with the file

---

## Step 9: Add Outputs

Define workflow outputs:

```yaml
outputs:
  - name: message_ts
    type: string
    value: "{{.steps.post_message.ts}}"
    description: Slack message timestamp

  - name: channel
    type: string
    value: "{{.steps.post_message.channel}}"
    description: Channel where message was posted

  - name: message_url
    type: string
    value: "https://slack.com/app_redirect?channel={{.steps.post_message.channel}}&message_ts={{.steps.post_message.ts}}"
    description: Direct link to the Slack message
```

**Computed outputs:**

- Construct the message URL from channel and timestamp
- Users can click the URL to view the message

---

## Step 10: Run the Workflow

Test with different scenarios:

### Success Notification

```bash
conductor run slack-notify.yaml \
  -i event_type="deployment" \
  -i status="success" \
  -i details="Deployed v2.1.0 to production. 127 files changed, 4,523 insertions, 982 deletions. All health checks passed." \
  -i channel="#engineering"
```

### Failure with Report

```bash
conductor run slack-notify.yaml \
  -i event_type="test_run" \
  -i status="failure" \
  -i details="Test suite failed: 23 passed, 5 failed, 2 skipped. Failed tests: auth_test, db_migration_test, api_rate_limit_test, cache_invalidation_test, user_permissions_test" \
  -i channel="#alerts" \
  -i include_report=true
```

### Warning

```bash
conductor run slack-notify.yaml \
  -i event_type="alert" \
  -i status="warning" \
  -i details="CPU usage at 87% on production-api-3. Memory usage normal. Disk I/O elevated." \
  -i channel="#ops"
```

**Expected output:**

```
[conductor] Starting workflow: slack-notification
[conductor] Step 1/5: generate_summary (llm)
[conductor] ✓ Completed in 1.2s

[conductor] Step 2/5: post_message (slack.post_message)
[conductor] ✓ Completed in 0.3s

[conductor] Step 3/5: add_reaction (slack.add_reaction)
[conductor] ✓ Completed in 0.2s

[conductor] Step 4/5: generate_report (llm)
[conductor] ⊘ Skipped (condition not met)

[conductor] Step 5/5: upload_report (slack.upload_file)
[conductor] ⊘ Skipped (condition not met)

--- Output: message_ts ---
1703123456.789012

--- Output: message_url ---
https://slack.com/app_redirect?channel=C1234567890&message_ts=1703123456.789012

[workflow complete]
```

---

## Complete Workflow

Here's the full `slack-notify.yaml`:

```yaml
name: slack-notification
description: Send AI-generated summaries to Slack
version: "1.0"

connectors:
  slack:
    from: connectors/slack
    auth:
      token: ${SLACK_BOT_TOKEN}

inputs:
  - name: event_type
    type: string
    required: true
    description: Type of event (deployment, test_run, alert, etc.)

  - name: status
    type: string
    required: true
    enum: ["success", "failure", "warning", "info"]
    description: Event status

  - name: details
    type: string
    required: true
    description: Event details or raw data

  - name: channel
    type: string
    default: "#engineering"
    description: Slack channel to post to

  - name: include_report
    type: boolean
    default: false
    description: Whether to upload a detailed report file

steps:
  - id: generate_summary
    name: Generate Slack Message
    type: llm
    model: fast
    system: |
      You are a notification assistant that creates clear, concise Slack messages.

      Format rules:
      - Use Slack markdown (*bold*, _italic_, `code`)
      - Keep it under 200 words
      - Start with an emoji that matches the status
      - Include key metrics if available
      - End with actionable next steps if needed
    prompt: |
      Create a Slack notification for this event:

      **Type:** {{.inputs.event_type}}
      **Status:** {{.inputs.status}}
      **Details:**
      {{.inputs.details}}

      Generate a well-formatted message suitable for Slack.
      Use appropriate emojis and markdown formatting.

  - id: post_message
    name: Post to Slack Channel
    slack.post_message:
      channel: "{{.inputs.channel}}"
      text: "{{.steps.generate_summary.response}}"

  - id: add_reaction
    name: Add Status Reaction
    slack.add_reaction:
      channel: "{{.inputs.channel}}"
      timestamp: "{{.steps.post_message.ts}}"
      name: |
        {{if eq .inputs.status "success"}}white_check_mark{{end}}
        {{if eq .inputs.status "failure"}}x{{end}}
        {{if eq .inputs.status "warning"}}warning{{end}}
        {{if eq .inputs.status "info"}}information_source{{end}}

  - id: generate_report
    name: Generate Detailed Report
    type: llm
    model: balanced
    condition:
      expression: 'inputs.include_report == true'
    system: |
      You are a technical report writer.
      Create detailed reports in markdown format with:
      - Executive summary
      - Detailed findings
      - Metrics and data
      - Recommendations
    prompt: |
      Create a detailed report for this {{.inputs.event_type}}:

      Status: {{.inputs.status}}
      Details: {{.inputs.details}}

      Output a complete markdown document.

  - id: upload_report
    name: Upload Report to Slack
    condition:
      expression: 'inputs.include_report == true'
    slack.upload_file:
      channels: "{{.inputs.channel}}"
      content: "{{.steps.generate_report.response}}"
      filename: "report-{{.inputs.event_type}}-{{.inputs.status}}.md"
      title: "Detailed {{.inputs.event_type}} Report"
      initial_comment: "Full report attached"

outputs:
  - name: message_ts
    type: string
    value: "{{.steps.post_message.ts}}"
    description: Slack message timestamp

  - name: channel
    type: string
    value: "{{.steps.post_message.channel}}"
    description: Channel where message was posted

  - name: message_url
    type: string
    value: "https://slack.com/app_redirect?channel={{.steps.post_message.channel}}&message_ts={{.steps.post_message.ts}}"
    description: Direct link to the Slack message
```

---

## Customization Ideas

### Thread Notifications

Post follow-up messages in a thread:

```yaml
  - id: post_update
    name: Post Update in Thread
    slack.post_message:
      channel: "{{.inputs.channel}}"
      text: "Update: Rollback completed successfully"
      thread_ts: "{{.steps.post_message.ts}}"
```

The `thread_ts` parameter makes it reply in a thread.

### Rich Block Formatting

Use Slack's Block Kit for richer messages:

```yaml
  - id: post_formatted
    slack.post_message:
      channel: "#engineering"
      text: "Deployment notification"
      blocks: |
        [
          {
            "type": "header",
            "text": {
              "type": "plain_text",
              "text": "🚀 Deployment Complete"
            }
          },
          {
            "type": "section",
            "fields": [
              {"type": "mrkdwn", "text": "*Version:*\nv2.1.0"},
              {"type": "mrkdwn", "text": "*Status:*\n✅ Success"}
            ]
          }
        ]
```

See [Block Kit Builder](https://api.slack.com/block-kit/building) for designing blocks.

### Update Messages Dynamically

Update a message as work progresses:

```yaml
  - id: initial_post
    slack.post_message:
      channel: "#deploys"
      text: "⏳ Deployment starting..."

  - id: deploy
    shell.run: "./deploy.sh"

  - id: update_status
    slack.update_message:
      channel: "{{.steps.initial_post.channel}}"
      ts: "{{.steps.initial_post.ts}}"
      text: "✅ Deployment completed successfully!"
```

### Multi-Channel Notifications

Post to multiple channels:

```yaml
  - id: notify_engineering
    slack.post_message:
      channel: "#engineering"
      text: "{{.steps.generate_summary.response}}"

  - id: notify_leadership
    slack.post_message:
      channel: "#leadership"
      text: "{{.steps.generate_summary.response}}"
```

Or use parallel execution:

```yaml
  - id: notify_all
    type: parallel
    steps:
      - id: eng_channel
        slack.post_message:
          channel: "#engineering"
          text: "{{.steps.generate_summary.response}}"

      - id: leadership_channel
        slack.post_message:
          channel: "#leadership"
          text: "{{.steps.generate_summary.response}}"
```

### Integration with CI/CD

Use in GitHub Actions:

```yaml
# .github/workflows/notify.yml
name: Notify on Deployment
on:
  workflow_run:
    workflows: ["Deploy"]
    types: [completed]

jobs:
  notify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Notify Slack
        env:
          SLACK_BOT_TOKEN: ${{ secrets.SLACK_BOT_TOKEN }}
        run: |
          conductor run slack-notify.yaml \
            -i event_type="deployment" \
            -i status="${{ job.status }}" \
            -i details="Workflow: ${{ github.workflow }}, Run: ${{ github.run_id }}" \
            -i channel="#deployments"
```

---

## Troubleshooting

### "not_in_channel" error

**Problem:** Bot not invited to private channels

**Solution:** Invite the bot:

```
/invite @conductor-bot
```

Or use `chat:write.public` scope to post without joining.

### "invalid_auth" error

**Problem:** Token invalid or not set

**Solution:** Verify token:

```bash
echo $SLACK_BOT_TOKEN
# Should output: xoxb-...

# Test with curl
curl -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  https://slack.com/api/auth.test
```

### "missing_scope" error

**Problem:** Bot doesn't have required permissions

**Solution:** Add scopes in Slack app settings:

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Select your app → "OAuth & Permissions"
3. Add required scope (e.g., `files:write`)
4. **Reinstall app to workspace**

### Messages not formatted correctly

**Problem:** Markdown not rendering

**Solution:** Use Slack's markdown syntax:

- Bold: `*text*` (not `**text**`)
- Italic: `_text_` (not `*text*`)
- Code: `` `code` `` or ``` ```code``` ```
- Link: `<url|text>` (not `[text](url)`)

Instruct the LLM:

```yaml
system: |
  Use Slack markdown formatting:
  - Bold: *text*
  - Italic: _text_
  - Code: `code`
  - Links: <url|text>
```

### File upload fails

**Problem:** Content too large or invalid format

**Solution:**

- Max file size: 1GB (but Slack API prefers <50MB)
- For large files, upload to S3 and share link
- Ensure `content:` is a string

---

## Key Concepts Learned

!!! success "You now understand:"
    - **Connector configuration** — Importing and authenticating connectors
    - **Connector shorthand** — Using `connector.operation:` syntax
    - **Connector outputs** — Accessing operation results like `ts` and `channel`
    - **Conditional steps** — Using `condition:` to skip steps
    - **Environment variables** — Secure secret management with `${VAR}`
    - **File uploads** — Sending files to Slack with metadata
    - **Workflow composition** — Combining LLMs with external APIs
    - **Enum inputs** — Restricting input values with `enum:`

---

## What's Next?

### Related Guides

- **[Flow Control](../../guides/flow-control.md)** — Parallel execution and conditionals

### More Connectors

- **[GitHub Connector](../../reference/connectors/github.md)** — Create issues, comment on PRs
- **[Jira Connector](../../reference/connectors/jira.md)** — Ticket automation
- **[Creating Custom Connectors](../../reference/connectors/custom.md)** — Build your own

### Production Enhancements

Make this production-ready:

1. **Error handling** — Retry failed Slack API calls
2. **Rate limiting** — Respect Slack API limits
3. **Message templates** — Reusable message formats
4. **User mentions** — Notify specific users with `<@USER_ID>`
5. **Scheduled notifications** — Run on a schedule with cron

See [Error Handling](../../guides/error-handling.md) for retry strategies.

---

## Additional Resources

- **[Slack API Reference](https://api.slack.com/methods)**
- **[Slack Connector Documentation](../../reference/connectors/slack.md)**
- **[Block Kit Builder](https://api.slack.com/block-kit/building)**
- **[Slack Bot Token Scopes](https://api.slack.com/scopes)**
- **[Workflow Schema Reference](../../reference/workflow-schema.md)**
