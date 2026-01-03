# Step 2: Better Recipe

Accept specific ingredients as input and output structured data.

## Goal

Make the recipe generator flexible by accepting ingredients and outputting structured JSON instead of plain text.

## The Workflow

Update `recipe.yaml`:

```yaml
name: better-recipe
inputs:
  ingredients:
    type: string
    description: Comma-separated ingredients to use
    default: "chicken, rice, broccoli"
steps:
  - id: generate
    llm:
      model: claude-3-5-sonnet-20241022
      prompt: |
        Generate a dinner recipe using these ingredients: ${inputs.ingredients}

        Return JSON with this structure:
        {
          "name": "Recipe Name",
          "ingredients": [{"item": "chicken", "quantity": "1 lb"}],
          "steps": ["Step 1", "Step 2"],
          "prepTime": "15 min",
          "cookTime": "30 min"
        }
outputs:
  recipe: ${steps.generate.output}
```

## Run It

```bash
# Use default ingredients
conductor run recipe.yaml

# Specify custom ingredients
conductor run recipe.yaml -i ingredients="salmon, asparagus, lemon"
```

## What You Learned

- **[Inputs](../features/inputs-outputs.md)** - Accept parameters when running workflows
- **[Outputs](../features/inputs-outputs.md)** - Return structured data from workflows
- **Variable syntax** - Use `${inputs.name}` to reference inputs
- **Step references** - Use `${steps.id.output}` to reference step outputs

## Next

[Step 3: Meal Plan](./03-meal-plan.md) - Generate multiple recipes in parallel.
