# Step 6: Save to Notion

Store your weekly meal plan in a Notion database.

## Goal

Save the generated meal plan to Notion for easy access and sharing.

## Prerequisites

1. Create a [Notion integration](https://www.notion.so/my-integrations)
2. Create a database in Notion with properties: Day (title), Breakfast (text), Lunch (text), Dinner (text)
3. Share the database with your integration
4. Set `NOTION_TOKEN` environment variable with your integration token

## The Workflow

Update `recipe.yaml`:

```yaml
name: save-to-notion
inputs:
  pantryFile:
    type: string
    default: "pantry.txt"
  notionDatabaseId:
    type: string
    description: Notion database ID
steps:
  - id: readPantry
    file:
      action: read
      path: ${inputs.pantryFile}
  - id: planWeek
    foreach:
      items:
        - Monday
        - Tuesday
        - Wednesday
        - Thursday
        - Friday
        - Saturday
        - Sunday
      steps:
        - id: dailyMeals
          llm:
            model: claude-3-5-sonnet-20241022
            prompt: |
              Available ingredients:
              ${steps.readPantry.output}

              Generate meals for ${item}.
              Previous days: ${steps.planWeek.outputs}

              Return JSON:
              {
                "day": "${item}",
                "breakfast": {"name": "...", "ingredients": [...]},
                "lunch": {"name": "...", "ingredients": [...]},
                "dinner": {"name": "...", "ingredients": [...]}
              }
  - id: saveToNotion
    foreach:
      items: ${steps.planWeek.outputs}
      steps:
        - id: createPage
          http:
            method: POST
            url: https://api.notion.com/v1/pages
            headers:
              Authorization: "Bearer ${NOTION_TOKEN}"
              Notion-Version: "2022-06-28"
              Content-Type: application/json
            body:
              parent:
                database_id: ${inputs.notionDatabaseId}
              properties:
                Day:
                  title:
                    - text:
                        content: ${item.day}
                Breakfast:
                  rich_text:
                    - text:
                        content: ${item.breakfast.name}
                Lunch:
                  rich_text:
                    - text:
                        content: ${item.lunch.name}
                Dinner:
                  rich_text:
                    - text:
                        content: ${item.dinner.name}
outputs:
  weeklyPlan: ${steps.planWeek.outputs}
```

## Run It

```bash
export NOTION_TOKEN="your-integration-token"
conductor run recipe.yaml -i notionDatabaseId="abc123..."
```

Your meal plan appears in Notion.

## What You Learned

- **[Integrations](../features/integrations.md)** - Connect to external services
- **[HTTP actions](../features/actions.md)** - Make API requests
- **Environment variables** - Securely reference credentials with `${VAR_NAME}`
- **Nested loops** - Iterate over loop outputs

## Security Note

Never hardcode API tokens in workflows. Always use environment variables or a secrets manager.

## Next

[Step 7: Deploy](./07-deploy.md) - Deploy your workflow to run on a schedule.
