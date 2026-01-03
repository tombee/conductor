# Part 6: Delivering Results

Send your meal plan somewhere your family can actually see it.

## What You'll Learn

- Delivering workflow output to external services
- Using HTTP actions for custom integrations
- Building practical, shareable workflows

## Delivery Options

Conductor supports multiple ways to deliver workflow results:

| Method | Use Case | Setup Difficulty |
|--------|----------|------------------|
| File output | Local access, backups | Already done |
| Email | Simple notifications | Requires SMTP |
| Slack | Team notifications | Slack webhook URL |
| HTTP webhook | Custom integrations | Endpoint URL |
| Notion | Shareable documents | API key + page ID |

We'll show the HTTP webhook approach, which works with any service that accepts webhooks.

## The Workflow

Create `examples/tutorial/06-complete.yaml`:

```conductor
name: complete-meal-planner
description: Full meal planner with delivery to webhook endpoint

inputs:
  - name: days
    type: string
    required: false
    default: "7"
    description: Number of days to plan
  - name: pantry_file
    type: string
    required: false
    default: "pantry.txt"
    description: Path to pantry inventory
  - name: webhook_url
    type: string
    required: false
    description: Webhook URL for delivery (optional)

steps:
  # Read pantry
  - id: read_pantry
    file.read: "{{.pantry_file}}"

  # Generate meals in parallel
  - id: meals
    type: parallel
    steps:
      - id: weekday
        type: llm
        model: balanced
        prompt: |
          Create weekday meals (Monday-Friday) using these pantry items:
          {{.steps.read_pantry.response}}

          Focus on quick, practical meals for busy days.
          Include breakfast, lunch, dinner for each day.

      - id: weekend
        type: llm
        model: balanced
        prompt: |
          Create weekend meals (Saturday-Sunday) using these pantry items:
          {{.steps.read_pantry.response}}

          Weekend meals can be more elaborate with longer prep times.
          Include breakfast, lunch, dinner for each day.

  # Combine and format
  - id: format
    type: llm
    model: balanced
    prompt: |
      Combine these into one cohesive {{.days}}-day meal plan:

      WEEKDAY MEALS:
      {{.steps.meals.weekday.response}}

      WEEKEND MEALS:
      {{.steps.meals.weekend.response}}

      Format as:
      1. Day-by-day meal plan
      2. Consolidated shopping list (only items not in pantry)
      3. Meal prep tips for the week

  # Refine for quality
  - id: refine
    type: loop
    max_iterations: 2
    steps:
      - id: critique
        type: llm
        model: fast
        prompt: |
          Quick review of this meal plan:
          {{if eq .loop.iteration 0}}
          {{.steps.format.response}}
          {{else}}
          {{.steps.refine.improve.response}}
          {{end}}

          Is it practical and well-organized? Reply APPROVED if yes.

      - id: improve
        type: llm
        model: balanced
        when: 'not (steps.critique.response contains "APPROVED")'
        prompt: |
          Improve: {{.steps.refine.critique.response}}

          Current plan:
          {{if eq .loop.iteration 0}}
          {{.steps.format.response}}
          {{else}}
          {{.steps.refine.improve.response}}
          {{end}}

    until: 'steps.critique.response contains "APPROVED"'

  # Always save locally
  - id: save_local
    file.write:
      path: meal-plan.md
      content: |
        # Weekly Meal Plan
        Generated: {{now}}

        {{.steps.refine.improve.response}}

  # Deliver via webhook (if configured)
  - id: deliver
    when: 'webhook_url != ""'
    http.request:
      method: POST
      url: "{{.webhook_url}}"
      headers:
        Content-Type: application/json
      body: |
        {
          "text": "Weekly Meal Plan Ready!",
          "blocks": [
            {
              "type": "section",
              "text": {
                "type": "mrkdwn",
                "text": "{{.steps.refine.improve.response | replace "\n" "\\n" | replace "\"" "\\\""}}"
              }
            }
          ]
        }

outputs:
  - name: meal_plan
    type: string
    value: "{{.steps.refine.improve.response}}"
  - name: delivered
    type: boolean
    value: "{{ne .webhook_url \"\"}}"
```

## How It Works

### Conditional Delivery
```yaml
- id: deliver
  when: 'webhook_url != ""'
  http.request:
    method: POST
    url: "{{.webhook_url}}"
```
The `when` condition makes delivery optional—it uses expression syntax, not templates. If no webhook URL is provided, the step is skipped.

### HTTP Action
```yaml
http.request:
  method: POST
  url: "{{.inputs.webhook_url}}"
  headers:
    Content-Type: application/json
  body: |
    {"text": "..."}
```
The `http.request` action sends data to any HTTP endpoint—webhooks, APIs, notification services.

### Always Have a Fallback
```yaml
- id: save_local
  file.write:
    path: meal-plan.md
```
Even with webhook delivery, we save locally. This ensures you always have the output even if delivery fails.

## Try It

### Without Webhook (Local Only)
```bash
conductor run examples/tutorial/06-complete.yaml
cat meal-plan.md
```

### With Slack Webhook
If you have a Slack webhook:
```bash
conductor run examples/tutorial/06-complete.yaml \
  -i webhook_url="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
```

### With Any Webhook Service
Works with any service that accepts HTTP POST:
- Make.com (Integromat)
- Zapier Webhooks
- n8n
- Custom API endpoints

## Alternative: Email Delivery

For email, you could modify the delivery step:

```yaml
- id: deliver
  when: "{{ne .inputs.email \"\"}}"
  shell.run:
    command:
      - mail
      - -s
      - "Weekly Meal Plan"
      - "{{.inputs.email}}"
    stdin: "{{.steps.refine.improve.response}}"
```

(Requires `mail` command configured on your system)

## Pattern Spotlight: Multi-Channel Delivery

This workflow demonstrates the **Multi-Channel Delivery** pattern:
1. **Generate content** — Create the output
2. **Save locally** — Always have a backup
3. **Deliver conditionally** — Send to configured channels

You'll use this pattern for:
- Report generation (create → save → email/Slack)
- Alert workflows (detect → format → notify)
- Content publishing (write → review → publish)

## What's Next

You have a complete meal planner! It reads your pantry, generates plans, refines quality, and delivers results. In the final part, we'll deploy this to a server so it runs automatically every week without your laptop being on.

[Part 7: Deploy to Production →](deploy)
