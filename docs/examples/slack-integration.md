# Slack Integration Example

AI-powered Slack notification workflow that analyzes events, generates intelligent summaries, and formats messages for Slack channels.

## Description

This workflow transforms verbose event data (build logs, alerts, deployment info) into concise, well-formatted Slack messages. It uses AI to summarize content, select appropriate emojis, and apply Slack's mrkdwn formatting automatically.

## Use Cases

- **CI/CD notifications** - Post build and deployment results to team channels
- **System alerts** - Send formatted alerts from monitoring systems
- **Deployment announcements** - Notify teams of production changes
- **Issue tracking updates** - Share issue status changes with stakeholders

## Prerequisites

### Required

- Conductor installed ([Getting Started](../getting-started/))
- LLM provider configured (Claude Code, Anthropic API, or OpenAI)
- Slack Bot Token with `chat:write` permission

### Optional

- Slack webhook URL for simpler posting
- GitHub Actions or CI/CD system for automation

## How to Run It

### Basic Usage

Send a build notification:

```bash
conductor run examples/slack-integration \
  -i event_type="build" \
  -i status="success" \
  -i details="All tests passed. Build completed in 3m 42s." \
  -i channel="#builds"
```

### Critical Alerts with Mentions

Send urgent alerts that mention users or groups:

```bash
conductor run examples/slack-integration \
  -i event_type="alert" \
  -i status="critical" \
  -i details="Database connection pool exhausted. 50+ queries queued." \
  -i channel="#oncall" \
  -i mention_users="@oncall-team"
```

### Deployment Announcements

Announce production deployments:

```bash
conductor run examples/slack-integration \
  -i event_type="deployment" \
  -i status="success" \
  -i details="Version 2.4.0 deployed to production. Includes bug fixes for #123 and #456." \
  -i channel="#deployments"
```

### Integration with CI/CD

#### GitHub Actions

```conductor
# .github/workflows/notify-slack.yml
name: Notify Slack
on:
  workflow_run:
    workflows: ["CI"]
    types: [completed]

jobs:
  notify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Send Slack Notification
        run: |
          conductor run examples/slack-integration \
            -i event_type="build" \
            -i status="${{ job.status }}" \
            -i details="Build ${{ github.run_number }} completed. ${{ github.event.workflow_run.conclusion }}" \
            -i channel="#ci-notifications"
        env:
          SLACK_BOT_TOKEN: ${{ secrets.SLACK_BOT_TOKEN }}
```

#### GitLab CI

```conductor
notify-slack:
  stage: notify
  script:
    - |
      conductor run examples/slack-integration \
        -i event_type="pipeline" \
        -i status="${CI_JOB_STATUS}" \
        -i details="Pipeline ${CI_PIPELINE_ID} ${CI_JOB_STATUS}" \
        -i channel="#gitlab-ci"
  when: always
```

## Code Walkthrough

The workflow consists of three sequential steps that transform raw event data into a polished Slack message:

### 1. Generate Intelligent Summary (Step 1)

```conductor
- id: generate_summary
  name: Generate Event Summary
  type: llm
  model: fast
  system: |
    You are an expert at creating concise, actionable summaries for engineering teams.

    Create a brief summary (2-3 sentences) that captures:
    - What happened
    - Impact or significance
    - Next steps (if any)
  prompt: |
    Summarize this {{.event_type}} event:
    **Status:** {{.status}}
    **Details:** {{.details}}

    Create a concise summary for a Slack notification.
```

**What it does**: Analyzes the raw event details and condenses them into 2-3 clear sentences. Removes noise and focuses on the key information engineers need to know.

**Why AI for summarization**: Event details can be verbose (stack traces, full logs, etc.). An LLM extracts the essence while maintaining technical accuracy. For example, it might transform a 500-line error log into "Authentication service failed with JWT signature mismatch. Likely caused by certificate rotation."

**Model tier choice**: Uses `fast` tier because summarization is a pattern-matching task that doesn't require deep reasoning. Typical response time is 2-3 seconds.

### 2. Format for Slack (Step 2)

```conductor
- id: format_message
  name: Format Slack Message
  type: llm
  model: fast
  system: |
    You are a Slack message formatter. Create a well-formatted message using Slack's mrkdwn syntax.

    Formatting rules:
    - *bold* for emphasis
    - `code` for technical terms
    - > blockquote for important notes
    - Emoji for status (✅ success, ❌ failed, ⚠️ warning, 🚨 critical)

    Structure:
    [emoji] **[Event Type]: [Status]**

    [Summary]

    [Additional details if needed]
  prompt: |
    Format this event for Slack:
    **Event Type:** {{.event_type}}
    **Status:** {{.status}}
    **Summary:** {{$.generate_summary.content}}

    Create a well-formatted Slack message with appropriate emoji and formatting.
```

**What it does**: Converts the summary into Slack's markdown format (mrkdwn), selects appropriate emojis based on status, and structures the message for readability.

**Emoji selection logic**: The LLM chooses contextually appropriate emojis:
- ✅ for successful operations
- ❌ for failures
- ⚠️ for warnings
- 🚨 for critical alerts
- 🚀 for deployments

**Formatting benefits**: Using AI for formatting ensures consistency across different event types while adapting to context. For example, it might bold critical terms or use blockquotes for important warnings.

### 3. Post to Slack (Step 3)

```conductor
- id: post_to_slack
  name: Post Message to Slack
  type: llm
  model: fast
  prompt: |
    [Note: This is a placeholder. In production, use the Slack integration.]

    Prepare this message for posting to {{.channel}}:
    {{$.format_message.content}}

    Return the message exactly as-is.
```

**What it does**: In this example, this is a placeholder step. In production, you would replace this with an actual Slack action that posts via the Slack API.

**Production implementation**: Replace with a shell command using `curl` or the Slack CLI:

```conductor
- id: post_to_slack
  type: action
  action: shell.run
  inputs:
    command: |
      curl -X POST https://slack.com/api/chat.postMessage \
        -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
          \"channel\": \"{{.inputs.channel}}\",
          \"text\": {{$.format_message.content | json}}
        }"
```

## Customization Options

### 1. Add Custom Emoji Mappings

Define specific emojis for your event types:

```conductor
system: |
  Use these emojis for event types:
  - build: 🏗️ (success), 💥 (failed)
  - deployment: 🚀 (success), 🚨 (failed)
  - test: ✅ (passed), ❌ (failed)
  - alert: ⚠️ (warning), 🚨 (critical)
```

### 2. Include Links and Mentions

Add dynamic links to build logs or dashboards:

```conductor
- id: format_message
  prompt: |
    Format this message and include:
    - Link to build: https://ci.example.com/builds/{{.build_id}}
    - Mention {{.mention_users}} if status is "failed"
```

### 3. Add Slack Blocks for Rich Formatting

Use Slack's Block Kit for more sophisticated formatting:

```conductor
- id: create_blocks
  type: llm
  model: fast
  prompt: |
    Create Slack blocks JSON for this event:
    {{$.generate_summary.content}}

    Use sections, dividers, and context blocks.
    Return valid JSON for Slack's Block Kit.
```

### 4. Thread Replies for Updates

Post follow-up messages as thread replies:

```conductor
inputs:
  - name: thread_ts
    type: string
    required: false
    description: Thread timestamp for reply

# In the post step:
-d "{
  \"channel\": \"{{.inputs.channel}}\",
  \"text\": {{$.format_message.content | json}},
  {{if .inputs.thread_ts}}\"thread_ts\": \"{{.inputs.thread_ts}}\",{{end}}
}"
```

### 5. Conditional Notifications

Only notify on failures or specific conditions:

```conductor
- id: post_to_slack
  condition:
    expression: 'inputs.status in ["failed", "critical"]'
  type: action
  action: shell.run
```

## Common Issues and Solutions

### Issue: Message formatting looks wrong in Slack

**Symptom**: Bold, code blocks, or emojis don't render correctly

**Solution**: Ensure you're using Slack's mrkdwn syntax, not standard Markdown:

```conductor
# Slack mrkdwn (correct)
*bold* `code` ~strike~

# Standard Markdown (won't work)
**bold** `code` ~~strike~~
```

### Issue: Mentions don't notify users

**Symptom**: User mentions appear but don't send notifications

**Solution**: Use Slack user IDs instead of @username:

```bash
# Get user ID
slack_user_id=$(curl -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  "https://slack.com/api/users.list" | jq -r '.members[] | select(.name=="username") | .id')

# Use in workflow
conductor run examples/slack-integration -i mention_users="<@${slack_user_id}>"
```

### Issue: Bot can't post to channel

**Symptom**: "not_in_channel" or "channel_not_found" errors

**Solution**: Invite the bot to the channel first:

```
/invite @your-bot-name
```

Or use the Slack API to join:

```bash
curl -X POST https://slack.com/api/conversations.join \
  -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  -d "channel=C1234567890"
```

### Issue: Messages are too verbose

**Symptom**: Summaries are still too long

**Solution**: Add length constraints to the prompt:

```conductor
prompt: |
  Summarize in 1-2 sentences maximum (under 200 characters).
  Focus only on the most critical information.
```

### Issue: Rate limiting from Slack

**Symptom**: "rate_limited" errors when posting many messages

**Solution**: Add delays between posts or batch notifications:

```conductor
- id: rate_limit_delay
  type: action
  action: shell.run
  inputs:
    command: "sleep 1"  # 1 second delay
```

## Related Examples

- [Issue Triage](issue-triage.md) - Could post triage results to Slack
- [Code Review](code-review.md) - Notify teams of review completion
- [IaC Review](iac-review.md) - Alert on infrastructure changes

## Workflow Files

Full workflow definition: [examples/slack-integration/workflow.yaml](https://github.com/tombee/conductor/blob/main/examples/slack-integration/workflow.yaml)

## Further Reading

- [Slack Block Kit](https://api.slack.com/block-kit) - Rich message formatting
- [Slack mrkdwn](https://api.slack.com/reference/surfaces/formatting) - Text formatting reference
- [Workflow Patterns](../building-workflows/patterns.md#sequential-processing)
- [Controller Mode](../building-workflows/controller.md) - Automated notifications
