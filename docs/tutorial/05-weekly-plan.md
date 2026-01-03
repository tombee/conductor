# Step 5: Weekly Plan

Generate a full week of meals with a variety check.

## Goal

Create a weekly meal plan and validate it for variety.

## The Workflow

Update `recipe.yaml`:

```yaml
name: weekly-plan
inputs:
  - name: pantry_file
    type: string
    default: "pantry.txt"
  - name: diet
    type: string
    default: "balanced"

steps:
  - id: read_pantry
    file.read: "{{.inputs.pantry_file}}"

  - id: plan_week
    type: llm
    model: strategic
    prompt: |
      Available ingredients:
      {{.steps.read_pantry.content}}

      Generate a {{.inputs.diet}} meal plan for the entire week (Monday through Sunday).
      For each day, create breakfast, lunch, and dinner recipes.

      Requirements:
      - Use only the available ingredients
      - Ensure variety - don't repeat main proteins on consecutive days
      - Include prep times for each meal

      Format each day clearly with the day name as a header.

  - id: check_variety
    type: llm
    model: balanced
    condition:
      expression: "true"
    prompt: |
      Review this meal plan for variety:
      {{.steps.plan_week.response}}

      Check that:
      1. No main protein is repeated on consecutive days
      2. Breakfast items have variety
      3. Different cuisines are represented

      If there are issues, suggest specific swaps.

outputs:
  - name: weekly_plan
    type: string
    value: "{{.steps.plan_week.response}}"
  - name: variety_check
    type: string
    value: "{{.steps.check_variety.response}}"
```

## Run It

```bash
conductor run recipe.yaml
```

## What You Learned

- **[Model tiers](../features/model-tiers.md)** - Use `strategic` for complex reasoning
- **[Conditions](../features/conditions.md)** - Use `condition.expression` to control step execution
- **Multi-step workflows** - Chain LLM steps to review and refine output

## Next

[Step 6: Save to Notion](./06-save-to-notion.md) - Save your meal plan to Notion.
