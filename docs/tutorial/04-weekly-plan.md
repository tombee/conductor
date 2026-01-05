# Step 4: Weekly Plan

Refine a meal plan iteratively until it meets quality criteria.

## Goal

Use `type: loop` to generate and refine a meal plan until variety requirements are satisfied.

## The Workflow

Update `recipe.yaml`:

<!-- include: examples/tutorial/04-weekly-plan.yaml -->

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
