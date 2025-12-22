# Integrations

Integrations connect workflows to external services.

## GitHub

Interact with GitHub repositories:

```yaml
steps:
  - id: create_issue
    github:
      action: create_issue
      repository: owner/repo
      title: "Bug Report"
      body: ${steps.analyze.output}
      labels:
        - bug
        - automated
      token: ${GITHUB_TOKEN}
```

### GitHub Actions

- `create_issue` - Create an issue
- `comment_on_pr` - Comment on pull request
- `create_pr` - Create pull request
- `get_pr` - Get PR details
- `list_issues` - List repository issues

## Slack

Send messages to Slack:

```yaml
steps:
  - id: notify
    slack:
      action: send_message
      channel: "#alerts"
      text: ${steps.summary.output}
      token: ${SLACK_TOKEN}
```

### Slack Actions

- `send_message` - Post message to channel
- `send_dm` - Send direct message
- `upload_file` - Upload file to channel
- `list_channels` - Get channel list

## Jira

Manage Jira tickets:

```yaml
steps:
  - id: create_ticket
    jira:
      action: create_issue
      project: PROJ
      issue_type: Task
      summary: ${steps.generate.output}
      description: "Automated task creation"
      credentials:
        url: https://company.atlassian.net
        email: ${JIRA_EMAIL}
        token: ${JIRA_TOKEN}
```

### Jira Actions

- `create_issue` - Create ticket
- `update_issue` - Update ticket
- `add_comment` - Comment on ticket
- `transition` - Move ticket status
- `search` - JQL search

## Discord

Post to Discord channels:

```yaml
steps:
  - id: announce
    discord:
      action: send_message
      webhook_url: ${DISCORD_WEBHOOK}
      content: ${steps.announcement.output}
      embeds:
        - title: "Weekly Update"
          description: ${steps.summary.output}
          color: 5814783
```

### Discord Actions

- `send_message` - Post to channel
- `send_embed` - Rich embedded message
- `upload_file` - Attach file

## Notion

Create and update Notion pages:

```yaml
steps:
  - id: create_page
    http:
      method: POST
      url: https://api.notion.com/v1/pages
      headers:
        Authorization: "Bearer ${NOTION_TOKEN}"
        Notion-Version: "2022-06-28"
        Content-Type: application/json
      body:
        parent:
          database_id: ${inputs.databaseId}
        properties:
          Name:
            title:
              - text:
                  content: ${steps.generate.output}
```

Notion uses the HTTP action with their API. See [Notion API docs](https://developers.notion.com) for details.

## Authentication

All integrations require credentials. Use environment variables:

```bash
export GITHUB_TOKEN="ghp_..."
export SLACK_TOKEN="xoxb-..."
export JIRA_TOKEN="..."
export DISCORD_WEBHOOK="https://discord.com/api/webhooks/..."
export NOTION_TOKEN="secret_..."

conductor run workflow.yaml
```

Never hardcode tokens in workflow files.

## Rate Limits

Each service has rate limits:

- **GitHub**: 5000 requests/hour (authenticated)
- **Slack**: Tier-based limits (typically 1 req/sec)
- **Jira**: Cloud plans vary (10-25 req/sec)
- **Discord**: 50 requests/sec per webhook
- **Notion**: 3 requests/sec

Conductor does not automatically handle rate limiting. Add delays or implement retry logic if needed.
