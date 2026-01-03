# Notion

The Notion integration provides integration with the Notion API for creating and updating pages, appending content blocks, and managing database items.

## Quick Start

```conductor
integrations:
  notion:
    from: integrations/notion
    auth:
      token: ${NOTION_TOKEN}
```

## Getting a Notion Integration Token

1. Go to [notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click "+ New integration"
3. Give it a name and select the workspace
4. Copy the "Internal Integration Token" (starts with `secret_`)
5. Share target pages/databases with the integration:
   - Open the page in Notion
   - Click "..." menu → "Connections" → "Connect to" → Select your integration

```bash
export NOTION_TOKEN=secret_your-token-here
```

## Operations

### Page Operations

#### create_page

Create a new page under a parent page or database.

```conductor
- id: create_recipe
  type: integration
  integration: notion.create_page
  inputs:
    parent_id: "abc123def456789012345678901234ab"
    title: "Weekly Meal Plan"
```

**Inputs:**
- `parent_id` (required): 32-character Notion page or database ID
- `title` (required): Page title (1-500 characters)
- `properties`: Optional page properties (for database pages)
- `icon`: Optional page icon `{type: "emoji", emoji: "📅"}`
- `cover`: Optional cover image `{type: "external", external: {url: "https://..."}}`

**Output:** `{id, url, created_at}` - The ID is used for subsequent operations

**Notes:**
- To get a page/database ID, open it in Notion and copy the 32-character hex string from the URL
- For database pages, use the database ID as the parent_id

#### get_page

Retrieve a page's properties.

```conductor
- id: fetch_page
  type: integration
  integration: notion.get_page
  inputs:
    page_id: "abc123def456789012345678901234ab"
```

**Inputs:**
- `page_id` (required): 32-character Notion page ID

**Output:** `{id, url, properties, parent}`

#### update_page

Update page properties like title, icon, or cover.

```conductor
- id: update_title
  type: integration
  integration: notion.update_page
  inputs:
    page_id: "abc123def456789012345678901234ab"
    properties:
      title:
        title:
          - text: {content: "Updated Title"}
    icon:
      type: "emoji"
      emoji: "✨"
```

**Inputs:**
- `page_id` (required): 32-character Notion page ID
- `properties`: Page properties to update
- `icon`: Page icon
- `cover`: Page cover image

**Output:** `{id, url}`

#### upsert_page

Create a page if it doesn't exist, or update it if it does (matched by title).

```conductor
- id: weekly_plan
  type: integration
  integration: notion.upsert_page
  inputs:
    parent_id: "abc123def456789012345678901234ab"
    title: "Weekly Meal Plan - {{.date}}"
```

**Inputs:**
- `parent_id` (required): 32-character Notion page ID
- `title` (required): Page title for matching/creation

**Output:** `{id, url, created_at}`

**Notes:**
- Matches pages by exact title within the parent scope
- If multiple pages with the same title exist, returns an error
- Ideal for idempotent workflows (re-running doesn't create duplicates)

### Block Operations

#### append_blocks

Append content blocks to an existing page.

```conductor
- id: add_content
  type: integration
  integration: notion.append_blocks
  inputs:
    page_id: "abc123def456789012345678901234ab"
    blocks:
      - type: heading_1
        text: "Monday"
      - type: bulleted_list_item
        text: "Breakfast: Oatmeal with berries"
      - type: bulleted_list_item
        text: "Lunch: Grilled chicken salad"
      - type: divider
      - type: paragraph
        text: "Enjoy your meal!"
```

**Inputs:**
- `page_id` (required): 32-character Notion page ID
- `blocks` (required): Array of block objects (max 100 blocks per call)

**Supported Block Types:**
- `paragraph`: Text paragraph (max 2000 characters)
  ```
  {type: "paragraph", text: "Your text here"}
  ```
- `heading_1`, `heading_2`, `heading_3`: Headings (max 200 characters each)
  ```
  {type: "heading_1", text: "Main Title"}
  ```
- `bulleted_list_item`: Bulleted list item
  ```
  {type: "bulleted_list_item", text: "List item"}
  ```
- `numbered_list_item`: Numbered list item
  ```
  {type: "numbered_list_item", text: "Step 1"}
  ```
- `to_do`: Checkbox item
  ```
  {type: "to_do", text: "Task", checked: false}
  ```
- `code`: Code block
  ```
  {type: "code", text: "console.log('hello')", language: "javascript"}
  ```
- `quote`: Quote block
  ```
  {type: "quote", text: "Quoted text"}
  ```
- `divider`: Horizontal divider
  ```
  {type: "divider"}
  ```

**Output:** `{blocks_added}` - Number of blocks successfully appended

**Validation:**
- Maximum 100 blocks per request
- Paragraph/code blocks: 2000 character limit
- Heading blocks: 200 character limit
- Existing page content is preserved (blocks are appended, not replaced)

### Database Operations

#### query_database

Query a Notion database with optional filters and sorting.

```conductor
- id: search_recipes
  type: integration
  integration: notion.query_database
  inputs:
    database_id: "abc123def456789012345678901234ab"
    filter:
      property: "Status"
      select:
        equals: "Published"
```

**Inputs:**
- `database_id` (required): 32-character Notion database ID
- `filter`: Optional filter object (Notion API filter format)
- `sorts`: Optional sorts array (Notion API sort format)

**Output:** `{results, has_more, next_cursor}` - Array of database items (max 100)

**Notes:**
- Returns up to 100 items by default
- Use `has_more` and `next_cursor` for pagination
- Filter format follows [Notion's filter spec](https://developers.notion.com/reference/post-database-query-filter)

#### create_database_item

Create a new item in a database.

```conductor
- id: add_recipe
  type: integration
  integration: notion.create_database_item
  inputs:
    database_id: "abc123def456789012345678901234ab"
    properties:
      Name:
        title:
          - text: {content: "Chicken Soup"}
      Category:
        select: {name: "Dinner"}
      Servings:
        number: 4
```

**Inputs:**
- `database_id` (required): 32-character Notion database ID
- `properties` (required): Database properties object

**Output:** `{id, url, created_at}`

**Notes:**
- Property names must match the database schema exactly
- Property values must match the expected type (title, text, number, select, etc.)
- Required database properties must be included

#### update_database_item

Update properties on an existing database item.

```conductor
- id: update_status
  type: integration
  integration: notion.update_database_item
  inputs:
    item_id: "abc123def456789012345678901234ab"
    properties:
      Status:
        select: {name: "Complete"}
```

**Inputs:**
- `item_id` (required): 32-character Notion database item (page) ID
- `properties` (required): Properties to update

**Output:** `{id, url}`

## Error Handling

The integration provides helpful error messages for common issues:

- **401 Unauthorized**: Invalid or expired integration token
  - Solution: Check your token at [notion.so/my-integrations](https://www.notion.so/my-integrations)

- **403 Forbidden**: Page/database not shared with integration
  - Solution: Share the page with your integration (click "..." → "Connections" → Select integration)

- **404 Not Found**: Page or database doesn't exist
  - Solution: Verify the ID is correct (32-character hex string from URL)

- **429 Rate Limited**: Too many requests
  - Notion free tier allows 3-6 requests/second
  - The integration automatically retries with exponential backoff

- **Validation Error**: Invalid input parameters
  - Check ID format (32 hex characters)
  - Verify block content doesn't exceed limits (2000 chars for paragraphs, 200 for headings)
  - Ensure you're not appending more than 100 blocks at once

## Example: Meal Planner to Notion

```conductor
steps:
  - id: create_weekly_plan
    type: integration
    integration: notion.upsert_page
    inputs:
      parent_id: "${NOTION_PARENT_PAGE_ID}"
      title: "Weekly Meal Plan - {{.week_start}}"

  - id: add_monday
    type: integration
    integration: notion.append_blocks
    inputs:
      page_id: "{{.steps.create_weekly_plan.id}}"
      blocks:
        - type: heading_1
          text: "Monday"
        - type: bulleted_list_item
          text: "Breakfast: {{.meals.monday.breakfast}}"
        - type: bulleted_list_item
          text: "Lunch: {{.meals.monday.lunch}}"
        - type: bulleted_list_item
          text: "Dinner: {{.meals.monday.dinner}}"
```

## Best Practices

1. **Use Environment Variables for Tokens**: Never hardcode tokens in workflows
   ```bash
   export NOTION_TOKEN=secret_...
   ```

2. **Share Pages First**: Always share target pages/databases with your integration before use

3. **Batch Block Operations**: Append multiple blocks in a single call (up to 100) for efficiency

4. **Use Upsert for Idempotency**: When re-running workflows, use `upsert_page` to avoid duplicates

5. **Validate IDs**: Ensure page/database IDs are 32-character hex strings (copy from Notion URLs)

6. **Handle Rate Limits**: The integration automatically retries on 429 errors with exponential backoff

## References

- [Notion API Documentation](https://developers.notion.com/)
- [Notion Integration Setup](https://www.notion.so/my-integrations)
- [Notion API Rate Limits](https://developers.notion.com/reference/request-limits)
