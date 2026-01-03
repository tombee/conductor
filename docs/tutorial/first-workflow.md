# Part 1: Your First Workflow

Create a workflow that takes user input, sends it to an LLM, and returns structured output.

## What You'll Learn

- Defining inputs that users provide
- Making LLM calls with prompts
- Using templates to reference data
- Defining outputs that workflows return

## The Workflow

Create `examples/tutorial/01-first-workflow.yaml`:

```conductor
name: quick-recipe
description: Suggest a quick recipe based on available time

inputs:
  - name: minutes
    type: string
    required: true
    description: How many minutes you have to cook

steps:
  - id: suggest
    type: llm
    model: balanced
    prompt: |
      Suggest one quick recipe that can be made in {{.minutes}} minutes or less.

      Include:
      - Recipe name
      - Brief ingredient list
      - Simple step-by-step instructions

      Keep it practical and achievable for a home cook.

outputs:
  - name: recipe
    value: "{{.steps.suggest.response}}"
```

## How It Works

### Inputs: Data Users Provide

```yaml
inputs:
  - name: minutes
    type: string
    required: true
    description: How many minutes you have to cook
```

Inputs are data passed in when running the workflow. Each input has:
- `name` — How you reference it (`minutes`)
- `type` — The data type (`string`, `number`, `boolean`)
- `required` — Whether it must be provided
- `description` — Help text shown to users

### LLM Steps: Calling AI Models

```yaml
steps:
  - id: suggest
    type: llm
    model: balanced
    prompt: |
      Suggest one quick recipe...
```

Each step needs:
- `id` — Unique identifier to reference this step's output
- `type: llm` — Tells Conductor to send this to an LLM
- `model` — Which tier to use (`fast`, `balanced`, `powerful`)
- `prompt` — The text to send to the model

### Templates: Referencing Data

```yaml
prompt: |
  Suggest one quick recipe that can be made in {{.minutes}} minutes...
```

Templates use `{{.path.to.data}}` syntax:
- `{{.minutes}}` — References the `minutes` input directly
- `{{.steps.suggest.response}}` — References output from a step
- `{{.env.API_KEY}}` — References environment variables

The double braces `{{ }}` mark where values get inserted.

### Outputs: What the Workflow Returns

```yaml
outputs:
  - name: recipe
    value: "{{.steps.suggest.response}}"
```

Outputs define what the workflow returns. This is useful when:
- Calling workflows from other workflows
- Exposing workflows as API endpoints
- Capturing specific values from complex step outputs

## Try It

Run the workflow:

```bash
conductor run examples/tutorial/01-first-workflow.yaml -i minutes=15
```

You should see:
```
Running: quick-recipe
[1/1] suggest... OK

**15-Minute Garlic Butter Shrimp**

Ingredients:
- 1 lb shrimp, peeled and deveined
- 4 cloves garlic, minced
- 3 tbsp butter
...
```

## Experiment

Try different inputs:

```bash
# Quick breakfast
conductor run examples/tutorial/01-first-workflow.yaml -i minutes=5

# Longer dinner
conductor run examples/tutorial/01-first-workflow.yaml -i minutes=45
```

### Multiple Inputs

Modify the workflow to accept dietary preferences:

```yaml
inputs:
  - name: minutes
    type: string
    required: true
  - name: diet
    type: string
    required: false
    default: "any"
    description: Dietary preference (vegetarian, vegan, keto, etc.)

steps:
  - id: suggest
    type: llm
    prompt: |
      Suggest a {{.diet}} recipe that takes {{.minutes}} minutes...
```

Run with both inputs:
```bash
conductor run recipe.yaml -i minutes=20 -i diet=vegetarian
```

## Template Reference

Common template patterns:

| Pattern | What It References |
|---------|-------------------|
| `{{.name}}` | Input named "name" |
| `{{.steps.id.response}}` | LLM response from step "id" |
| `{{.steps.id.content}}` | File content from step "id" |
| `{{.env.VAR}}` | Environment variable |
| `{{now}}` | Current timestamp |

## What's Next

We can generate one recipe, but what if we want breakfast, lunch, AND dinner at the same time? In Part 2, we'll run multiple LLM calls in parallel for speed.

[Part 2: Parallel Execution →](parallel)
