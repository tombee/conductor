# Step 5: Save to Notion

Save your meal plan to Notion pages using markdown.

## Goal

Generate a meal plan and save it to Notion using the Notion integration with markdown content.

## Setup

1. Sign up for Notion at https://www.notion.so if you don't have an account
2. Create a workspace if you don't have one
3. Create a "Meal Plans" page in your workspace (use the "empty page" option)
4. Create an integration at https://www.notion.so/profile/integrations
5. Share the page with your integration (click ••• at top right → Connections → Add your integration)
6. Copy your integration token
7. Copy the page ID from the URL (the 32-character string, remove hyphens)
8. Configure the Notion integration in Conductor:

```bash
conductor integrations add notion --token "your-integration-token"
```

## The Workflow

Update `recipe.yaml`:

<!-- include: examples/tutorial/05-save-to-notion.yaml -->

## Key Concepts

### Markdown Content

Instead of constructing Notion blocks one by one, use the `markdown` parameter:

```yaml
notion.replace_content:
  page_id: "{{.inputs.parent_page_id}}"
  markdown: |
    # Weekly Meal Plan

    ## Overview
    {{range .steps.generate_plan.output.overview}}
    - {{.}}
    {{end}}
```

This converts markdown to Notion blocks automatically, including:
- Headings, paragraphs, lists
- Checkboxes (`- [ ]` and `- [x]`)
- Code blocks, quotes, dividers
- Rich text formatting

### Default Content

Use `default_markdown` to initialize a page only when it's created:

```yaml
notion.upsert_page:
  parent_id: "{{.inputs.parent_page_id}}"
  title: "Pantry"
  default_markdown: |
    # My Pantry
    Add your ingredients here...
```

If the page already exists, `default_markdown` is ignored. The response includes `is_new: true/false`.

### Reading as Markdown

Read page content as markdown for LLM processing:

```yaml
notion.get_blocks:
  page_id: "{{.steps.pantry.id}}"
  format: markdown
```

Returns `content` as a markdown string instead of a blocks array.

## Run It

```bash
conductor run recipe.yaml -i parent_page_id="your-page-id"
```

## What You Learned

- **Markdown content** - Use `markdown:` parameter instead of `blocks:` arrays
- **Default content** - Initialize pages with `default_markdown` (only on create)
- **Format output** - Read content as markdown with `format: markdown`
- **Template loops** - Use `{{range}}` to generate dynamic lists

## Next

[Step 6: Deploy](./06-deploy.md) - Deploy your workflow to run on a schedule.
