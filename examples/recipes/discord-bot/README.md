# Discord Bot Example

Complete Discord bot implementation for triggering Conductor workflows.

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
docker-compose logs -f discord-bot
```

## Configuration

Create `.env` file:

```bash
# Discord
DISCORD_BOT_TOKEN=your-discord-bot-token

# Conductor
CONDUCTOR_URL=http://conductor:9000
CONDUCTOR_API_KEY=your-conductor-api-key

# For Conductor daemon
ANTHROPIC_API_KEY=your-anthropic-api-key
```

## Testing Locally

For local development without Docker:

```bash
# Install dependencies
pip install -r requirements.txt

# Set local Conductor URL
export CONDUCTOR_URL=http://localhost:9000

# Run bot
python discord_bot.py
```

## See Also

- [Discord Bot Recipe](../../../docs/recipes/chat-bots/discord.md) - Full documentation
