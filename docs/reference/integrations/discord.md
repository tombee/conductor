# Discord

The Discord integration provides integration with Discord for bot messages and webhooks.

## Quick Start

```conductor
connectors:
  discord:
    from: integrations/discord
    auth:
      token: ${DISCORD_BOT_TOKEN}
```

## Authentication

Create a bot at [discord.com/developers/applications](https://discord.com/developers/applications)

```bash
export DISCORD_BOT_TOKEN=your-bot-token
```

## Required Scopes

Configure bot permissions at [discord.com/developers/applications](https://discord.com/developers/applications):
- `Send Messages` - Required for send_message
- `Manage Messages` - Required for edit_message, delete_message
- `Add Reactions` - Required for add_reaction
- `Create Public Threads` / `Create Private Threads` - Required for create_thread
- `View Channels` - Required for list_channels, get_channel
- `Manage Webhooks` - Required for create_webhook

## Operations

### Messaging

#### send_message

Send a message to a channel.

```conductor
- id: notify
  type: integration
  integration: discord.send_message
  inputs:
    channel_id: "1234567890123456789"
    content: "Deployment to production complete!"
```

**Inputs:**
- `channel_id` (required): Channel ID (not channel name)
- `content`: Message text (up to 2000 characters)
- `embeds`: Array of embed objects (max 10)
- `tts`: Text-to-speech message (boolean, default: false)
- `allowed_mentions`: Control mention behavior

**Output:** `{id, channel_id, content, timestamp, author}`

**Required Permissions:** `Send Messages`

#### edit_message

Edit an existing message sent by the bot.

```conductor
- id: update_status
  type: integration
  integration: discord.edit_message
  inputs:
    channel_id: "1234567890123456789"
    message_id: "9876543210987654321"
    content: "Updated: Deployment complete with warnings"
```

**Inputs:**
- `channel_id` (required): Channel ID
- `message_id` (required): Message ID from send_message
- `content`: New message text
- `embeds`: New embeds array

**Output:** `{id, channel_id, content, edited_timestamp}`

**Required Permissions:** `Send Messages`, `Manage Messages`

#### delete_message

Delete a message.

```conductor
- id: remove_temp_message
  type: integration
  integration: discord.delete_message
  inputs:
    channel_id: "1234567890123456789"
    message_id: "9876543210987654321"
```

**Inputs:**
- `channel_id` (required): Channel ID
- `message_id` (required): Message ID

**Output:** `{ok: true}`

**Required Permissions:** `Manage Messages`

#### add_reaction

Add an emoji reaction to a message.

```conductor
- id: acknowledge
  type: integration
  integration: discord.add_reaction
  inputs:
    channel_id: "1234567890123456789"
    message_id: "9876543210987654321"
    emoji: "✅"
```

**Inputs:**
- `channel_id` (required): Channel ID
- `message_id` (required): Message ID
- `emoji` (required): Unicode emoji or custom emoji (`name:id`)

**Output:** `{ok: true}`

**Required Permissions:** `Add Reactions`

#### send_embed

Send a rich embedded message.

```conductor
- id: send_alert
  type: integration
  integration: discord.send_embed
  inputs:
    channel_id: "1234567890123456789"
    title: "Production Alert"
    description: "High CPU usage detected"
    color: 15158332  # Red color
    fields:
      - name: "Severity"
        value: "High"
        inline: true
      - name: "Server"
        value: "prod-01"
        inline: true
    footer:
      text: "Monitoring Bot"
    timestamp: true
```

**Inputs:**
- `channel_id` (required): Channel ID
- `title`: Embed title
- `description`: Embed description
- `color`: Decimal color code (e.g., 15158332 for red)
- `fields`: Array of `{name, value, inline}` objects
- `thumbnail`: Thumbnail image URL
- `image`: Main image URL
- `footer`: Footer object `{text, icon_url}`
- `timestamp`: Include current timestamp (boolean) or ISO 8601 string

**Output:** `{id, channel_id, embeds}`

**Required Permissions:** `Send Messages`, `Embed Links`

### Threads

#### create_thread

Create a thread in a channel.

```conductor
- id: discussion_thread
  type: integration
  integration: discord.create_thread
  inputs:
    channel_id: "1234567890123456789"
    name: "Deployment Discussion"
    auto_archive_duration: 1440  # 24 hours
    message_id: "9876543210987654321"  # Optional: create from message
```

**Inputs:**
- `channel_id` (required): Channel ID
- `name` (required): Thread name (up to 100 characters)
- `auto_archive_duration`: Archive after inactive minutes (60, 1440, 4320, 10080)
- `type`: Thread type (10 for announcement, 11 for public, 12 for private)
- `message_id`: Create thread from existing message

**Output:** `{id, name, type, parent_id}`

**Required Permissions:** `Create Public Threads`, `Create Private Threads` (for private)

### Channels

#### list_channels

List all channels in a guild (server).

```conductor
- id: get_channels
  type: integration
  integration: discord.list_channels
  inputs:
    guild_id: "1111111111111111111"
```

**Inputs:**
- `guild_id` (required): Guild (server) ID

**Output:** `[{id, name, type, position, parent_id}]`

**Required Permissions:** `View Channels`

#### get_channel

Get details about a specific channel.

```conductor
- id: channel_info
  type: integration
  integration: discord.get_channel
  inputs:
    channel_id: "1234567890123456789"
```

**Inputs:**
- `channel_id` (required): Channel ID

**Output:** `{id, name, type, guild_id, position, topic, nsfw, rate_limit_per_user}`

**Required Permissions:** `View Channels`

### Members

#### list_members

List members in a guild.

```conductor
- id: get_guild_members
  type: integration
  integration: discord.list_members
  inputs:
    guild_id: "1111111111111111111"
    limit: 100
```

**Inputs:**
- `guild_id` (required): Guild ID
- `limit`: Maximum members to return (1-1000, default: 1)
- `after`: Get members after this user ID (for pagination)

**Output:** `[{user: {id, username, discriminator}, nick, roles, joined_at}]`

**Required Permissions:** `Guild Members` intent (privileged)

#### get_member

Get information about a specific guild member.

```conductor
- id: member_info
  type: integration
  integration: discord.get_member
  inputs:
    guild_id: "1111111111111111111"
    user_id: "2222222222222222222"
```

**Inputs:**
- `guild_id` (required): Guild ID
- `user_id` (required): User ID

**Output:** `{user: {id, username, discriminator, avatar}, nick, roles, joined_at, premium_since}`

**Required Permissions:** `Guild Members` intent

### Webhooks

#### create_webhook

Create a webhook for a channel.

```conductor
- id: setup_webhook
  type: integration
  integration: discord.create_webhook
  inputs:
    channel_id: "1234567890123456789"
    name: "Monitoring Bot"
```

**Inputs:**
- `channel_id` (required): Channel ID
- `name` (required): Webhook name (up to 80 characters)
- `avatar`: Avatar image data URL

**Output:** `{id, token, name, channel_id, url}`

**Required Permissions:** `Manage Webhooks`

#### send_webhook

Send a message via webhook.

```conductor
- id: webhook_notify
  type: integration
  integration: discord.send_webhook
  inputs:
    webhook_id: "3333333333333333333"
    webhook_token: "webhook-token-here"
    content: "Automated notification from monitoring"
    username: "Alert Bot"
```

**Inputs:**
- `webhook_id` (required): Webhook ID
- `webhook_token` (required): Webhook token
- `content`: Message content
- `username`: Override webhook username
- `avatar_url`: Override webhook avatar
- `embeds`: Array of embeds
- `tts`: Text-to-speech (boolean)

**Output:** `{id, channel_id, content, timestamp}`

**Required Permissions:** None (uses webhook token)

## Examples

### Deployment Notification

```conductor
steps:
  - id: send_notification
    type: integration
    integration: discord.send_embed
    inputs:
      channel_id: "{{.inputs.channel_id}}"
      title: "🚀 Deployment Complete"
      description: "Application deployed to production"
      color: 5763719  # Green
      fields:
        - name: "Version"
          value: "{{.inputs.version}}"
          inline: true
        - name: "Environment"
          value: "Production"
          inline: true
        - name: "Duration"
          value: "{{.inputs.duration}}"
          inline: true
      timestamp: true

  - id: add_checkmark
    type: integration
    integration: discord.add_reaction
    inputs:
      channel_id: "{{.inputs.channel_id}}"
      message_id: "{{.steps.send_notification.id}}"
      emoji: "✅"
```

### Alert with Thread Discussion

```conductor
steps:
  - id: post_alert
    type: integration
    integration: discord.send_message
    inputs:
      channel_id: "{{.inputs.alerts_channel}}"
      content: |
        @here Critical alert: High error rate detected
        Error rate: {{.inputs.error_rate}}%

  - id: create_discussion
    type: integration
    integration: discord.create_thread
    inputs:
      channel_id: "{{.inputs.alerts_channel}}"
      message_id: "{{.steps.post_alert.id}}"
      name: "Alert Investigation - {{.inputs.timestamp}}"
      auto_archive_duration: 1440
```

## Troubleshooting

### 401 Unauthorized

**Problem**: Bot token invalid

**Solutions**:
1. Regenerate token in Developer Portal
2. Verify token starts with correct prefix
3. Check token is for the right bot

### 403 Forbidden

**Problem**: Missing permissions

**Solutions**:
1. Check bot has required permissions in server settings
2. Verify bot role is high enough in role hierarchy
3. Enable required intents in Developer Portal (for list_members)

### 404 Not Found

**Problem**: Channel or message not found

**Solutions**:
1. Verify channel/message IDs are correct (must be strings)
2. Check bot has access to the channel
3. Confirm message still exists (not deleted)

### 400 Bad Request - Invalid Form Body

**Problem**: Invalid input data

**Solutions**:
1. Check message content is under 2000 characters
2. Verify embed fields don't exceed limits (25 fields max)
3. Ensure color is decimal number, not hex string

## See Also

- [Discord Developer Portal](https://discord.com/developers/docs)
- [Discord Bot Permissions Calculator](https://discordapi.com/permissions.html)
- [Embed Visualizer](https://leovoel.github.io/embed-visualizer/)
