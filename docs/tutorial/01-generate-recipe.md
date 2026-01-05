# Step 1: Generate a Recipe

Create a workflow that generates a dinner recipe from ingredients.

## Goal

Build your first useful workflow: accept ingredients as input and generate a recipe.

## The Workflow

Create `recipe.yaml`:

<!-- include: examples/tutorial/01-generate-recipe.yaml -->

## Run It

```bash
# You'll be prompted for ingredients
conductor run recipe.yaml

# Or provide them directly
conductor run recipe.yaml -i ingredients="salmon, asparagus, lemon"
```

You'll see a complete recipe tailored to your ingredients.

## What You Learned

- **Workflows** - YAML files with a name and steps
- **LLM steps** - Use `type: llm` with a `model` and `prompt`
- **Model tiers** - `fast`, `balanced`, or `strategic` (not specific model names)
- **[Inputs](../features/inputs-outputs.md)** - Accept parameters with `-i name=value`
- **[Outputs](../features/inputs-outputs.md)** - Return structured data from workflows
- **Template syntax** - Use `{{.inputs.name}}` to reference inputs
- **Step references** - Use `{{.steps.id.response}}` for step outputs

## Next

[Step 2: Meal Plan](./02-meal-plan.md) - Generate multiple recipes in parallel.
