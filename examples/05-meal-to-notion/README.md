# Meal Planner to Notion

Generate a personalized 3-day meal plan and publish it to a Notion page for easy access from any device.

## What it does

1. Generates breakfast, lunch, and dinner plans in parallel
2. Consolidates into a 3-day meal plan
3. Creates or updates a Notion page with the meal plan
4. Generates and appends a shopping list
5. Returns the Notion page URL for bookmarking

## Setup

### 1. Create a Notion Integration

1. Go to [notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click "+ New integration"
3. Give it a name (e.g., "Conductor Meal Planner")
4. Select your workspace
5. Copy the "Internal Integration Token"

```bash
export NOTION_TOKEN=secret_your-token-here
```

### 2. Get your Parent Page ID

1. Open the Notion page where you want meal plans to appear
2. Copy the 32-character ID from the URL:
   ```
   https://notion.so/My-Recipes-abc123def456789012345678901234ab
                                     ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
   ```
3. Share the page with your integration:
   - Click "..." menu → "Connections" → Select your integration

### 3. Configure the Integration

Add to your `workspace.yaml`:

```yaml
integrations:
  notion:
    type: notion
    auth:
      type: token
      token: ${NOTION_TOKEN}
```

## Usage

### Basic meal plan

```bash
conductor run examples/05-meal-to-notion \
  --input notion_parent_id=abc123def456789012345678901234ab
```

### With dietary goals

```bash
conductor run examples/05-meal-to-notion \
  --input notion_parent_id=abc123def456789012345678901234ab \
  --input dietary_goals="high protein, ~1800 cal/day" \
  --input preferences="Mediterranean style, quick meals" \
  --input servings=4
```

## Output

The workflow creates a Notion page with:
- **Title**: "Weekly Meal Plan - YYYY-MM-DD"
- **Metadata**: Generation date, servings, dietary goals
- **Meal Plan**: 3-day schedule with breakfast, lunch, dinner
- **Shopping List**: Categorized ingredients grouped by type

The page URL is returned in `outputs.notion_url`, perfect for bookmarking on your phone!

## Re-running

The workflow uses `upsert_page`, which means:
- First run: Creates a new page
- Subsequent runs: Updates the existing page (no duplicates!)

This is perfect for weekly meal planning - just re-run the workflow each week.

## What you'll learn

- **Parallel LLM calls**: Generate meal types concurrently for speed
- **Notion integration**: Create pages, append blocks, structure content
- **Idempotent workflows**: Re-run safely with `upsert_page`
- **Template functions**: Use `{{now.Format}}` for dates

## Next steps

- **Add a trigger**: Run this weekly on Sunday evenings
- **Read from calendar**: Check your schedule to plan around events
- **Track favorites**: Query a Notion database of past meal favorites
- **Send reminders**: Use Slack integration to notify when the meal plan is ready
