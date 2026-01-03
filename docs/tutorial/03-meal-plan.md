# Step 3: Meal Plan

Generate three recipes in parallel for a complete meal plan.

## Goal

Create breakfast, lunch, and dinner recipes simultaneously using parallel execution.

## The Workflow

Update `recipe.yaml`:

```yaml
name: meal-plan
inputs:
  diet:
    type: string
    description: Dietary preference
    default: "vegetarian"
steps:
  - id: breakfast
    llm:
      model: claude-3-5-sonnet-20241022
      prompt: |
        Generate a ${inputs.diet} breakfast recipe.
        Return JSON: {"name": "...", "ingredients": [...], "steps": [...]}
  - id: lunch
    llm:
      model: claude-3-5-sonnet-20241022
      prompt: |
        Generate a ${inputs.diet} lunch recipe.
        Return JSON: {"name": "...", "ingredients": [...], "steps": [...]}
  - id: dinner
    llm:
      model: claude-3-5-sonnet-20241022
      prompt: |
        Generate a ${inputs.diet} dinner recipe.
        Return JSON: {"name": "...", "ingredients": [...], "steps": [...]}
outputs:
  mealPlan:
    breakfast: ${steps.breakfast.output}
    lunch: ${steps.lunch.output}
    dinner: ${steps.dinner.output}
```

## Run It

```bash
conductor run recipe.yaml -i diet="Mediterranean"
```

All three recipes generate simultaneously, completing in roughly the same time as one.

## What You Learned

- **[Parallel execution](../features/parallel.md)** - Steps without dependencies run concurrently
- **Multiple outputs** - Structure complex output data
- **Step independence** - Steps that don't reference each other run in parallel

## Next

[Step 4: Pantry Check](./04-pantry-check.md) - Read available ingredients from a file.
