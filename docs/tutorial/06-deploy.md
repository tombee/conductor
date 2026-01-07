# Step 6: Deploy

Deploy your workflow to run automatically on a schedule.

## Goal

Run your meal planner every Sunday evening to plan the week ahead.

## The Workflow

The final `recipe.yaml`:

<!-- include: examples/tutorial/06-deploy.yaml -->

## Key Concepts

### Loop Refinement

Use loops with LLM feedback to improve outputs:

```yaml
- id: refine_plan
  type: loop
  max_iterations: 3
  until: "steps.check.passes == true"
  steps:
    - id: generate
      type: llm
      prompt: |
        {{if gt .loop.iteration 0}}
        Previous feedback: {{.steps.check.feedback}}
        {{end}}
        Generate a meal plan...

    - id: check
      type: llm
      output_schema:
        type: object
        properties:
          passes: { type: boolean }
          feedback: { type: string }
      prompt: Review this plan...
```

### Formatted Output with Markdown

Save formatted content to Notion using markdown with template loops:

```yaml
notion.upsert_page:
  parent_id: "{{.inputs.notion_page_id}}"
  title: "Weekly Meal Plan"
  markdown: |
    # Weekly Meal Plan

    ## Monday
    - **Breakfast:** {{.steps.refine_plan.generate.output.monday.breakfast}}
    - **Lunch:** {{.steps.refine_plan.generate.output.monday.lunch}}
    - **Dinner:** {{.steps.refine_plan.generate.output.monday.dinner}}
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
exe secrets set notion_page_id="your-page-id"
```

Or deploy to any server with SSH access:

```bash
scp recipe.yaml server:/path/to/workflows/
ssh server "conductor triggers add --workflow weekly-meal-planner --cron '0 18 * * 0'"
```

## What You Learned

- **[Loops](../features/loops.md)** - Refine outputs with LLM feedback loops
- **[Triggers](../features/triggers.md)** - Schedule workflows with cron expressions
- **Markdown output** - Format content for Notion using markdown templates
- **Remote deployment** - Run workflows on servers or cloud platforms

## Tutorial Complete

You've built a complete meal planning workflow that:

1. Reads ingredients from a file
2. Generates recipes using an LLM
3. Refines output with quality checks
4. Saves formatted results to Notion using markdown
5. Runs automatically on a schedule

Explore the [Features](../features/inputs-outputs.md) section for more capabilities.
