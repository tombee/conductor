# Step 2: Better Recipe

Accept specific ingredients as input and return structured output.

## Goal

Accept ingredients as input and capture the recipe as a workflow output.

## The Workflow

Update `recipe.yaml`:

```yaml
name: better-recipe
inputs:
  - name: ingredients
    type: string
    default: "chicken, rice, broccoli"

steps:
  - id: generate
    type: llm
    model: balanced
    prompt: |
      Generate a dinner recipe using these ingredients: {{.inputs.ingredients}}

      Include:
      - Recipe name
      - Full ingredient list with quantities
      - Step-by-step cooking instructions
      - Prep and cook time

outputs:
  - name: recipe
    type: string
    value: "{{.steps.generate.response}}"
```

## Run It

```bash
# Use default ingredients
conductor run recipe.yaml

# Specify custom ingredients
conductor run recipe.yaml -i ingredients="salmon, asparagus, lemon"
```

## What You Learned

- **[Inputs](../features/inputs-outputs.md)** - Accept parameters with `-i name=value`
- **[Outputs](../features/inputs-outputs.md)** - Return structured data from workflows
- **Template syntax** - Use `{{.inputs.name}}` to reference inputs
- **Step references** - Use `{{.steps.id.response}}` for step outputs

## Next

[Step 3: Meal Plan](./03-meal-plan.md) - Generate multiple recipes in parallel.
