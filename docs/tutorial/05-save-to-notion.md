# Step 5: Save to Notion

Save your meal plan to a Notion database.

## Goal

Generate a meal plan and save it to Notion using the HTTP action.

## Setup

1. Sign up for Notion at https://www.notion.so if you don't have an account
2. Create a workspace if you don't have one
3. Create a "Meal Plans" page in your workspace (use the "empty page" option)
4. Add a database to the page with Name (title) and Content (text) properties
5. Create an integration at https://www.notion.so/profile/integrations
6. Share the database with your integration (click ••• → Connections → Add your integration)
7. Copy your integration token
8. Copy the database ID from the URL (the 32-character string after the page name)
9. Set your token as an environment variable:

```bash
export NOTION_TOKEN="your-integration-token"
```

## The Workflow

Update `recipe.yaml`:

<!-- include: examples/tutorial/05-save-to-notion.yaml -->

## Run It

```bash
conductor run recipe.yaml -i notion_database_id="your-database-id"
```

## What You Learned

- **[HTTP actions](../features/actions.md)** - Use `http.post:` for API calls
- **Environment variables** - Access secrets with `{{env.VAR_NAME}}`
- **Input validation** - Use `pattern` for regex validation on inputs

## Next

[Step 6: Deploy](./06-deploy.md) - Deploy your workflow to run on a schedule.
