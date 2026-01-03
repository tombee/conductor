# Step 4: Pantry Check

Read available ingredients from a file and use them in recipe generation.

## Goal

Check what ingredients are available in your pantry before generating recipes.

## Setup

Create `pantry.txt`:

```
chicken breast
rice
onions
garlic
olive oil
tomatoes
spinach
eggs
```

## The Workflow

Update `recipe.yaml`:

```yaml
name: pantry-check
inputs:
  pantryFile:
    type: string
    description: Path to pantry inventory
    default: "pantry.txt"
  diet:
    type: string
    default: "any"
steps:
  - id: readPantry
    file:
      action: read
      path: ${inputs.pantryFile}
  - id: breakfast
    llm:
      model: claude-3-5-sonnet-20241022
      prompt: |
        Available ingredients:
        ${steps.readPantry.output}

        Generate a ${inputs.diet} breakfast using only these ingredients.
        Return JSON: {"name": "...", "ingredients": [...], "steps": [...]}
  - id: lunch
    llm:
      model: claude-3-5-sonnet-20241022
      prompt: |
        Available ingredients:
        ${steps.readPantry.output}

        Generate a ${inputs.diet} lunch using only these ingredients.
        Return JSON: {"name": "...", "ingredients": [...], "steps": [...]}
  - id: dinner
    llm:
      model: claude-3-5-sonnet-20241022
      prompt: |
        Available ingredients:
        ${steps.readPantry.output}

        Generate a ${inputs.diet} dinner using only these ingredients.
        Return JSON: {"name": "...", "ingredients": [...], "steps": [...]}
outputs:
  mealPlan:
    breakfast: ${steps.breakfast.output}
    lunch: ${steps.lunch.output}
    dinner: ${steps.dinner.output}
```

## Run It

```bash
conductor run recipe.yaml
```

Recipes will only use ingredients from your pantry file.

## What You Learned

- **[File actions](../features/actions.md)** - Read and write files
- **Step dependencies** - Steps reference other step outputs
- **Sequential execution** - File must be read before using its content

## Next

[Step 5: Weekly Plan](./05-weekly-plan.md) - Generate a full week of meals using loops.
