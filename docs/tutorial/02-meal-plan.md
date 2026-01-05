# Step 2: Meal Plan

Generate multiple recipes at once using parallel execution.

## Goal

Generate breakfast, lunch, and dinner recipes simultaneously.

## The Workflow

Update `recipe.yaml`:

```yaml
name: meal-plan
inputs:
  - name: diet
    type: string
    default: "vegetarian"

steps:
  - id: meals
    type: parallel
    max_concurrency: 3
    steps:
      - id: breakfast
        type: llm
        model: balanced
        prompt: |
          Generate a {{.inputs.diet}} breakfast recipe.
          Include the recipe name, ingredients with quantities, and cooking steps.

      - id: lunch
        type: llm
        model: balanced
        prompt: |
          Generate a {{.inputs.diet}} lunch recipe.
          Include the recipe name, ingredients with quantities, and cooking steps.

      - id: dinner
        type: llm
        model: balanced
        prompt: |
          Generate a {{.inputs.diet}} dinner recipe.
          Include the recipe name, ingredients with quantities, and cooking steps.

outputs:
  - name: breakfast
    type: string
    value: "{{.steps.meals.breakfast.response}}"
  - name: lunch
    type: string
    value: "{{.steps.meals.lunch.response}}"
  - name: dinner
    type: string
    value: "{{.steps.meals.dinner.response}}"
```

## Run It

```bash
conductor run recipe.yaml
conductor run recipe.yaml -i diet="keto"
```

All three recipes generate at the same time.

## What You Learned

- **[Parallel](../features/parallel.md)** - Use `type: parallel` with nested `steps`
- **max_concurrency** - Limit concurrent executions
- **Nested outputs** - Reference parallel step outputs with `{{.steps.parent.child.response}}`

## Next

[Step 3: Pantry Check](./03-pantry-check.md) - Read ingredients from a file.
