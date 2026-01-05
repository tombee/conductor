# Step 5: Save to Notion

Save your meal plan to a Notion database.

## Goal

Generate a meal plan and save it to Notion using the HTTP action.

## Setup

1. Create a Notion integration at https://www.notion.so/profile/integrations
2. Create a database with Name and Content properties
3. Share the database with your integration
4. Set your token as an environment variable:

```bash
export NOTION_TOKEN="your-integration-token"
```

## The Workflow

Update `recipe.yaml`:

```yaml
name: save-to-notion
inputs:
  - name: pantry_file
    type: string
    default: "pantry.txt"
  - name: notion_database_id
    type: string

steps:
  - id: read_pantry
    file.read: "{{.inputs.pantry_file}}"

  - id: plan_week
    type: llm
    model: strategic
    prompt: |
      Available ingredients:
      {{.steps.read_pantry.content}}

      Generate a balanced meal plan for Monday through Sunday.
      For each day, create breakfast, lunch, and dinner.

      Return JSON:
      {
        "monday": {"breakfast": "...", "lunch": "...", "dinner": "..."},
        "tuesday": {"breakfast": "...", "lunch": "...", "dinner": "..."},
        ...
      }

  - id: save_plan
    http.post:
      url: https://api.notion.com/v1/pages
      headers:
        Authorization: "Bearer {{env.NOTION_TOKEN}}"
        Notion-Version: "2022-06-28"
        Content-Type: application/json
      body:
        parent:
          database_id: "{{.inputs.notion_database_id}}"
        properties:
          Name:
            title:
              - text:
                  content: "Weekly Meal Plan"
          Content:
            rich_text:
              - text:
                  content: "{{.steps.plan_week.response}}"

outputs:
  - name: weekly_plan
    type: string
    value: "{{.steps.plan_week.response}}"
```

## Run It

```bash
conductor run recipe.yaml -i notion_database_id="your-database-id"
```

## What You Learned

- **[HTTP actions](../features/actions.md)** - Use `http.post:` for API calls
- **Environment variables** - Access secrets with `{{env.VAR_NAME}}`
- **[Integrations](../features/integrations.md)** - Connect to external services
- **JSON output** - Ask the LLM to return structured data when needed for APIs

## Next

[Step 6: Deploy](./06-deploy.md) - Deploy your workflow to run on a schedule.
