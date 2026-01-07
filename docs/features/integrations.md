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

Create and update Notion pages using markdown or block arrays.

### Markdown Content

The simplest way to create Notion content is with markdown:

```yaml
steps:
  - id: save_plan
    notion.upsert_page:
      parent_id: "{{.inputs.parent_page_id}}"
      title: "Weekly Meal Plan"
      markdown: |
        # Weekly Meal Plan

        ## Overview
        {{range .steps.generate_plan.output.overview}}
        - {{.}}
        {{end}}

        ---

        ## Shopping List
        {{range .steps.shopping_list}}
        - [ ] {{.}}
        {{end}}
```

Supported markdown elements:
- Headings (`#`, `##`, `###`)
- Paragraphs
- Bulleted lists (`-`)
- Numbered lists (`1.`)
- Checkboxes (`- [ ]`, `- [x]`)
- Code blocks (`` ``` ``)
- Quotes (`>`)
- Dividers (`---`)
- Images (`![alt](url)`)
- Callouts (`> [!NOTE]`, `> [!WARNING]`)
- Rich text: **bold**, *italic*, ~~strikethrough~~, `code`, [links](url)

### Default Content

Use `default_markdown` to initialize a page only when it's created:

```yaml
steps:
  - id: pantry
    notion.upsert_page:
      parent_id: "{{.inputs.parent_page_id}}"
      title: "Pantry"
      default_markdown: |
        # My Pantry
        Add your ingredients here...
```

The response includes `is_new: true/false` indicating if the page was created or found.

### Reading Content

Read page content as markdown for LLM processing:

```yaml
steps:
  - id: read_content
    notion.get_blocks:
      page_id: "{{.steps.pantry.id}}"
      format: markdown  # or "blocks" (default), "text"
```

### Block Arrays

For more control, use block arrays:

```yaml
steps:
  - id: save_item
    notion.create_database_item:
      database_id: "{{.inputs.database_id}}"
      properties:
        Name:
          title:
            - text:
                content: "{{.steps.generate.response}}"
        Status:
          select:
            name: "New"
```

### Notion Operations

**Page Operations:**
- `create_page` - Create page under a parent
- `get_page` - Retrieve page properties
- `update_page` - Update page title, icon, or cover
- `delete_page` - Archive a page
- `upsert_page` - Create or find by title (supports `default_markdown`, `markdown`)

**Content Operations:**
- `get_blocks` - Get page content (supports `format: markdown|blocks|text`)
- `append_blocks` - Add content (supports `markdown` parameter)
- `replace_content` - Replace all content (supports `markdown` parameter)

**Database Operations:**
- `query_database` - Query with filters and sorts
- `create_database_item` - Add item to database
- `update_database_item` - Update item properties
- `delete_database_item` - Archive database item
- `list_databases` - List accessible databases
- `get_database_schema` - Get property definitions
- `update_database_schema` - Modify property definitions
- `batch_create_pages` - Create multiple pages (rate-limited)

**Other Operations:**
- `search` - Search pages and databases
- `get_comments` - Get page/block comments
- `add_comment` - Add comment or reply

## Authentication

All integrations require credentials. Configure them using `conductor integrations add`:

```bash
conductor integrations add notion --token "secret_..."
conductor integrations add github --token "ghp_..."
conductor integrations add slack --token "xoxb-..."
```

Then declare and use the integration in your workflow:

```yaml
integrations:
  notion:
    from: integrations/notion

steps:
  - id: save
    type: integration
    integration: notion.create_page
    inputs:
      # ...
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
