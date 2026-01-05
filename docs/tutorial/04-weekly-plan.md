# Step 4: Weekly Plan

Refine a meal plan iteratively until it meets quality criteria.

## Goal

Use `type: loop` to generate and refine a meal plan until variety requirements are satisfied.

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

  - id: refine_plan
    type: loop
    max_iterations: 3
    until: "steps.check.passes == true"
    steps:
      - id: generate
        type: llm
        model: strategic
        prompt: |
          Available ingredients:
          {{.steps.read_pantry.content}}

          {{if .loop.history}}
          Previous attempt feedback:
          {{.loop.history.check.feedback}}

          Improve the plan based on this feedback.
          {{else}}
          Generate a {{.inputs.diet}} meal plan for Monday through Sunday.
          {{end}}

          For each day, create breakfast, lunch, and dinner.
          Don't repeat main proteins on consecutive days.

      - id: check
        type: llm
        model: fast
        output_schema:
          type: object
          properties:
            passes:
              type: boolean
            feedback:
              type: string
        prompt: |
          Review this meal plan for variety:
          {{.steps.generate.response}}

          Check:
          1. No main protein repeated on consecutive days
          2. Breakfast items vary throughout the week

          Return {"passes": true} if requirements are met.
          Otherwise return {"passes": false, "feedback": "specific issues"}.

outputs:
  - name: weekly_plan
    type: string
    value: "{{.steps.refine_plan.generate.response}}"
```

## Run It

```bash
conductor run recipe.yaml
```

The loop runs until `check.passes` is true or 3 iterations complete.

## What You Learned

- **[Loops](../features/loops.md)** - Use `type: loop` for iterative refinement
- **until** - Termination condition evaluated after each iteration
- **max_iterations** - Safety limit to prevent infinite loops
- **loop.history** - Access previous iteration outputs to improve results

## Next

[Step 5: Save to Notion](./05-save-to-notion.md) - Save your meal plan to Notion.
