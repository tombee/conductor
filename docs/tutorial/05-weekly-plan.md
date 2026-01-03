# Step 5: Weekly Plan

Generate a full week of meal plans using loops and conditions.

## Goal

Create 7 days of meal plans by iterating through weekdays, with variety checks to avoid repetition.

## The Workflow

Update `recipe.yaml`:

```yaml
name: weekly-plan
inputs:
  pantryFile:
    type: string
    default: "pantry.txt"
  diet:
    type: string
    default: "balanced"
steps:
  - id: readPantry
    file:
      action: read
      path: ${inputs.pantryFile}
  - id: planWeek
    foreach:
      items:
        - Monday
        - Tuesday
        - Wednesday
        - Thursday
        - Friday
        - Saturday
        - Sunday
      steps:
        - id: dailyMeals
          llm:
            model: claude-3-5-sonnet-20241022
            prompt: |
              Available ingredients:
              ${steps.readPantry.output}

              Generate meals for ${item}.
              Previous days: ${steps.planWeek.outputs}

              Create breakfast, lunch, and dinner using available ingredients.
              Ensure variety - don't repeat main proteins from previous days.

              Return JSON:
              {
                "day": "${item}",
                "breakfast": {"name": "...", "ingredients": [...]},
                "lunch": {"name": "...", "ingredients": [...]},
                "dinner": {"name": "...", "ingredients": [...]}
              }
outputs:
  weeklyPlan: ${steps.planWeek.outputs}
```

## Run It

```bash
conductor run recipe.yaml
```

You'll get 7 days of varied meal plans.

## What You Learned

- **[Loops](../features/loops.md)** - Iterate with `foreach` over a list
- **[Conditions](../features/conditions.md)** - LLM considers previous iterations for variety
- **Loop context** - Access `${item}` for current iteration and `${steps.id.outputs}` for previous results

## Next

[Step 6: Save to Notion](./06-save-to-notion.md) - Store your meal plan in Notion.
