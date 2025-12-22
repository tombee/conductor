# Slack App Example

Complete Slack app implementation for triggering Conductor workflows.

## Quick Start

1. **Set up environment variables:**

```bash
cp .env.example .env
# Edit .env with your tokens
```

2. **Build and run:**

```bash
docker-compose up -d
```

3. **View logs:**

```bash
docker-compose logs -f slack-app
```

## Configuration

Create `.env` file:

```bash
# Slack
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_SIGNING_SECRET=your-signing-secret
SLACK_APP_TOKEN=xapp-your-app-token  # For Socket Mode

# Conductor
CONDUCTOR_URL=http://conductor:9000
CONDUCTOR_API_KEY=your-conductor-api-key

# For Conductor daemon
ANTHROPIC_API_KEY=your-anthropic-api-key
```

## Socket Mode vs HTTP Mode

### Socket Mode (Recommended for Development)

- No public URL needed
- Easier local testing
- Requires `SLACK_APP_TOKEN`

### HTTP Mode (Production)

- Requires public URL
- Better for production scale
- Configure webhooks in Slack app settings

## Testing Locally

For local development without Docker:

```bash
# Install dependencies
pip install -r requirements.txt

# Set local Conductor URL
export CONDUCTOR_URL=http://localhost:9000

# Run app
python slack_bot.py
```

## See Also

- [Slack App Recipe](../../../docs/recipes/chat-bots/slack.md) - Full documentation
