# Step 3: Pantry Check

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

<!-- include: examples/tutorial/03-pantry-check.yaml -->

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

[Step 4: Weekly Plan](./04-weekly-plan.md) - Generate a full week of meals with variety checking.
