# Slack

The Slack integration provides integration with the Slack API for posting messages, managing channels, and interacting with users.

## Quick Start

```conductor
connectors:
  slack:
    from: integrations/slack
    auth:
      token: ${SLACK_BOT_TOKEN}
```

## Getting a Slack Bot Token

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Create a new app or select existing
3. Navigate to "OAuth & Permissions"
4. Add bot token scopes (e.g., `chat:write`, `channels:read`)
5. Install app to workspace
6. Copy the "Bot User OAuth Token" (starts with `xoxb-`)

```bash
export SLACK_BOT_TOKEN=xoxb-your-token-here
```

## Operations

### Messaging

#### post_message

Post a message to a channel.

```conductor
- id: notify_team
  type: integration
  integration: slack.post_message
  inputs:
    channel: "#engineering"
    text: "Deployment to production completed successfully!"
```

**Inputs:**
- `channel` (required): Channel ID or name (e.g., "#engineering" or "C1234567890")
- `text` (required): Message text (supports Slack markdown)
- `thread_ts`: Thread timestamp to reply in a thread
- `blocks`: Rich message blocks (JSON array)
- `attachments`: Legacy attachments (JSON array)
- `unfurl_links`: Automatically unfurl links (boolean, default: true)
- `unfurl_media`: Automatically unfurl media (boolean, default: true)

**Output:** `{ts, channel, message}` - The timestamp is used for replies and updates

**Required Scopes:** `chat:write`

#### update_message

Update an existing message.

```conductor
- id: update_status
  type: integration
  integration: slack.update_message
  inputs:
    channel: "C1234567890"
    ts: "1234567890.123456"
    text: "Updated: Deployment completed with warnings"
```

**Inputs:**
- `channel` (required): Channel ID
- `ts` (required): Message timestamp from post_message
- `text` (required): New message text
- `blocks`: Updated message blocks
- `attachments`: Updated attachments

**Output:** `{ts, channel, text}`

**Required Scopes:** `chat:write`

#### delete_message

Delete a message.

```conductor
- id: remove_message
  type: integration
  integration: slack.delete_message
  inputs:
    channel: "C1234567890"
    ts: "1234567890.123456"
```

**Inputs:**
- `channel` (required): Channel ID
- `ts` (required): Message timestamp

**Output:** `{ok: true}`

**Required Scopes:** `chat:write`

#### add_reaction

Add an emoji reaction to a message.

```conductor
- id: acknowledge
  type: integration
  integration: slack.add_reaction
  inputs:
    channel: "C1234567890"
    timestamp: "1234567890.123456"
    name: "white_check_mark"
```

**Inputs:**
- `channel` (required): Channel ID
- `timestamp` (required): Message timestamp
- `name` (required): Emoji name (without colons, e.g., "thumbsup")

**Output:** `{ok: true}`

**Required Scopes:** `reactions:write`

### Files

#### upload_file

Upload a file to channels.

```conductor
- id: share_report
  type: integration
  integration: slack.upload_file
  inputs:
    channels: ["#engineering", "#leadership"]
    file_content: "{{.steps.generate_report.content}}"
    filename: "weekly-report.pdf"
    title: "Weekly Engineering Report"
    initial_comment: "Here's this week's report"
```

**Inputs:**
- `channels` (required): Array of channel IDs or names
- `file_content` (required): File content (string or base64)
- `filename` (required): Name of the file
- `title`: Title of the file
- `initial_comment`: Message to post with the file
- `filetype`: File type (auto-detected if not specified)

**Output:** `{file: {id, name, url_private}}`

**Required Scopes:** `files:write`

### Channels

#### list_channels

List all channels in the workspace.

```conductor
- id: get_all_channels
  type: integration
  integration: slack.list_channels
  inputs:
    exclude_archived: true
    types: "public_channel,private_channel"
```

**Inputs:**
- `exclude_archived`: Exclude archived channels (boolean, default: false)
- `types`: Comma-separated channel types (`public_channel`, `private_channel`, `mpim`, `im`)
- `limit`: Maximum channels to return (default: 100)

**Output:** `[{id, name, is_private, is_archived, num_members}]`

**Required Scopes:** `channels:read`, `groups:read` (for private channels)

#### create_channel

Create a new channel.

```conductor
- id: new_project_channel
  type: integration
  integration: slack.create_channel
  inputs:
    name: "project-apollo"
    is_private: false
```

**Inputs:**
- `name` (required): Channel name (lowercase, no spaces, max 80 chars)
- `is_private`: Create as private channel (boolean, default: false)

**Output:** `{id, name, is_private, created}`

**Required Scopes:** `channels:manage`, `groups:write` (for private)

#### invite_to_channel

Invite users to a channel.

```conductor
- id: add_team_members
  type: integration
  integration: slack.invite_to_channel
  inputs:
    channel: "C1234567890"
    users: ["U1111111111", "U2222222222"]
```

**Inputs:**
- `channel` (required): Channel ID
- `users` (required): Array of user IDs

**Output:** `{channel: {id, name}}`

**Required Scopes:** `channels:manage`, `groups:write` (for private)

### Users

#### list_users

List all users in the workspace.

```conductor
- id: get_team
  type: integration
  integration: slack.list_users
  inputs:
    limit: 200
```

**Inputs:**
- `limit`: Maximum users to return (default: 100)

**Output:** `[{id, name, real_name, email, is_bot, is_admin, deleted}]`

**Required Scopes:** `users:read`, `users:read.email` (for email field)

#### get_user

Get information about a specific user.

```conductor
- id: get_user_info
  type: integration
  integration: slack.get_user
  inputs:
    user: "U1234567890"
```

**Inputs:**
- `user` (required): User ID

**Output:** `{id, name, real_name, email, tz, profile: {title, phone, image_url}}`

**Required Scopes:** `users:read`, `users:read.email` (for email field)

## Example: Alert Bot

```conductor
steps:
  - id: analyze_issue
    type: llm
    model: fast
    prompt: "Analyze this alert: {{.inputs.alert}}"

  - id: post_alert
    type: integration
    integration: slack.post_message
    inputs:
      channel: "#alerts"
      text: |
        :warning: *Alert*: {{.steps.analyze_issue.title}}
        
        *Severity*: {{.steps.analyze_issue.severity}}
        *Details*: {{.steps.analyze_issue.summary}}
```

## See Also

- [Slack API Documentation](https://api.slack.com/methods)
- [Bot Token Scopes](https://api.slack.com/scopes)
