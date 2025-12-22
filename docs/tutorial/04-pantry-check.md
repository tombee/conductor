# Step 4: Pantry Check

Read available ingredients from a file.

## Goal

Generate recipes using only ingredients listed in your pantry file.

## Setup

Create `pantry.txt`:

```
Eggs (12)
Milk (1 gallon)
Bread (1 loaf)
Butter (1 lb)
Chicken breast (2 lbs)
Rice (2 lbs)
Broccoli (1 bunch)
Onions (3)
Garlic (1 head)
Olive oil
Salt
Pepper
```

## The Workflow

Update `recipe.yaml`:

```yaml
name: pantry-check
inputs:
  - name: pantry_file
    type: string
    default: "pantry.txt"
  - name: diet
    type: string
    default: "any"

steps:
  - id: read_pantry
    file.read: "{{.inputs.pantry_file}}"

  - id: meals
    type: parallel
    max_concurrency: 3
    steps:
      - id: breakfast
        type: llm
        model: balanced
        prompt: |
          Available ingredients:
          {{.steps.read_pantry.content}}

          Generate a {{.inputs.diet}} breakfast using only these ingredients.
          Include the recipe name, ingredients with quantities, and cooking steps.

      - id: lunch
        type: llm
        model: balanced
        prompt: |
          Available ingredients:
          {{.steps.read_pantry.content}}

          Generate a {{.inputs.diet}} lunch using only these ingredients.
          Include the recipe name, ingredients with quantities, and cooking steps.

      - id: dinner
        type: llm
        model: balanced
        prompt: |
          Available ingredients:
          {{.steps.read_pantry.content}}

          Generate a {{.inputs.diet}} dinner using only these ingredients.
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
```

Recipes will only use ingredients from your pantry file.

## What You Learned

- **[Actions](../features/actions.md)** - Use `file.read:` shorthand for file operations
- **File content** - Access with `{{.steps.id.content}}`
- **Dynamic prompts** - Inject file contents into LLM prompts

## Next

[Step 5: Weekly Plan](./05-weekly-plan.md) - Generate a full week of meals with variety checking.
