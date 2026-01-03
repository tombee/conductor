# Step 7: Deploy

Deploy your workflow to run automatically on a schedule.

## Goal

Deploy the meal planning workflow to run every Sunday at 6pm, generating the week's meals automatically.

## The Workflow

Update `recipe.yaml`:

```yaml
name: weekly-meal-planner
inputs:
  pantryFile:
    type: string
    default: "pantry.txt"
  notionDatabaseId:
    type: string
    description: Notion database ID
triggers:
  - cron:
      schedule: "0 18 * * 0"
      timezone: America/Los_Angeles
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

## Deploy It

### Option 1: exe.dev

```bash
conductor deploy recipe.yaml --target exe.dev
```

### Option 2: Your Own Server

```bash
# Copy workflow to server
scp recipe.yaml pantry.txt user@server:/opt/conductor/

# SSH to server and start controller
ssh user@server
conductor controller start --workflows /opt/conductor/
```

The workflow runs every Sunday at 6pm, automatically updating your Notion meal plan.

## What You Learned

- **[Triggers](../features/triggers.md)** - Schedule workflows with cron syntax
- **Deployment** - Run workflows on remote servers
- **Controller** - The long-running service that executes triggered workflows

## Cron Schedule Format

```
* * * * *
│ │ │ │ │
│ │ │ │ └─ Day of week (0-6, Sunday=0)
│ │ │ └─── Month (1-12)
│ │ └───── Day of month (1-31)
│ └─────── Hour (0-23)
└───────── Minute (0-59)
```

## Next Steps

You've completed the tutorial! You now know:

- Workflows, steps, and prompts
- Inputs and outputs
- Parallel execution
- File actions
- Loops and conditions
- Integrations and HTTP
- Triggers and deployment

Explore the [Features](../features/inputs-outputs.md) documentation for deeper dives into each capability.
