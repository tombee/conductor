# Step 1: First Recipe

Generate a single recipe using an LLM step.

## Goal

Create the simplest useful workflow: ask an LLM to generate a dinner recipe.

## The Workflow

Create `recipe.yaml`:

```yaml
name: first-recipe
steps:
  - id: generate
    type: llm
    model: balanced
    prompt: |
      Generate a dinner recipe. Include:
      - Recipe name
      - Ingredients with quantities
      - Cooking steps
      - Prep and cook time
```

## Run It

```bash
conductor run recipe.yaml
```

You'll see a complete recipe with ingredients and instructions.

## What You Learned

- **Workflows** - YAML files with a name and steps
- **LLM steps** - Use `type: llm` with a `model` and `prompt`
- **Model tiers** - `fast`, `balanced`, or `strategic` (not specific model names)

## Next

[Step 2: Better Recipe](./02-better-recipe.md) - Add inputs and outputs.
