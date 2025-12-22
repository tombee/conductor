# Slack Integration Example

Send intelligent notifications to Slack channels using AI-generated summaries and smart formatting.

## Quick Start

```bash
# Send build notification
conductor run examples/slack-integration \
  --input event_type="build" \
  --input status="success" \
  --input details="All tests passed. Build completed in 3m 42s." \
  --input channel="#builds"

# Send critical alert with mentions
conductor run examples/slack-integration \
  --input event_type="alert" \
  --input status="critical" \
  --input details="Database connection pool exhausted" \
  --input channel="#oncall" \
  --input mention_users="@oncall-team"
```

## Prerequisites

1. Conductor CLI installed
2. Slack Bot Token with `chat:write` permission
3. Set environment variable: `export SLACK_BOT_TOKEN="xoxb-your-token"`
4. Invite bot to your Slack channel

## Features

- AI-powered event summarization
- Smart Slack mrkdwn formatting
- Automatic emoji selection based on status
- Support for user mentions
- Thread support for updates

## Use Cases

- CI/CD pipeline notifications
- Deployment announcements
- System alerts and monitoring
- Issue tracking updates

## Expected Output

```
✅ **Build: Success**

Build completed successfully in 3m 42s. All test suites passed with 245 tests. No new warnings or linting issues detected.
```

## Documentation

For detailed usage, customization options, and integration patterns, see:
[Slack Integration Documentation](../../docs/examples/slack-integration.md)
