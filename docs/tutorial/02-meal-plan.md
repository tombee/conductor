# Step 2: Meal Plan

Generate multiple recipes at once using parallel execution.

## Goal

Generate breakfast, lunch, and dinner recipes simultaneously.

## The Workflow

Update `recipe.yaml`:

<!-- include: examples/tutorial/02-meal-plan.yaml -->

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
