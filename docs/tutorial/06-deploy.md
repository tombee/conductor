# Step 6: Deploy

Deploy your workflow to run automatically on a schedule.

## Goal

Run your meal planner every Sunday evening to plan the week ahead.

## The Workflow

The final `recipe.yaml`:

```yaml
name: weekly-meal-planner
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

## Add a Schedule Trigger

Triggers are configured via the CLI:

```bash
# Run every Sunday at 6pm
conductor triggers add \
  --workflow weekly-meal-planner \
  --cron "0 18 * * 0"
```

## Deploy to a Remote Server

You can deploy Conductor to any remote server. [exe.dev](https://exe.dev) provides a simple deployment option:

```bash
# Deploy your workflow
exe deploy recipe.yaml

# Set secrets
exe secrets set NOTION_TOKEN="your-token"
exe secrets set notion_database_id="your-database-id"
```

Or deploy to any server with SSH access:

```bash
scp recipe.yaml server:/path/to/workflows/
ssh server "conductor triggers add --workflow weekly-meal-planner --cron '0 18 * * 0'"
```

## What You Learned

- **[Triggers](../features/triggers.md)** - Schedule workflows with cron expressions
- **Remote deployment** - Run workflows on servers or cloud platforms
- **Secrets management** - Securely store API tokens

## Tutorial Complete

You've built a complete meal planning workflow that:

1. Reads ingredients from a file
2. Generates recipes using an LLM
3. Runs multiple steps in parallel
4. Checks output for quality
5. Saves results to Notion
6. Runs automatically on a schedule

Explore the [Features](../features/inputs-outputs.md) section for more capabilities.
