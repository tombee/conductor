# Part 2: Parallel Execution

Generate breakfast, lunch, and dinner suggestions at the same time instead of one-by-one.

## What You'll Learn

- Running multiple LLM calls concurrently with `type: parallel`
- Combining results from parallel steps
- How parallel execution improves performance

## The Workflow

Create `examples/tutorial/02-parallel.yaml`:

```conductor
name: meal-suggestions
description: Generate breakfast, lunch, and dinner ideas in parallel

inputs:
  - name: style
    type: string
    required: false
    default: healthy
    description: Cuisine or dietary style (e.g., healthy, comfort, mediterranean)

steps:
  - id: meals
    type: parallel
    steps:
      - id: breakfast
        type: llm
        model: fast
        prompt: |
          Suggest a {{.style}} breakfast idea.
          Include the recipe name and a brief description (2-3 sentences).

      - id: lunch
        type: llm
        model: fast
        prompt: |
          Suggest a {{.style}} lunch idea.
          Include the recipe name and a brief description (2-3 sentences).

      - id: dinner
        type: llm
        model: fast
        prompt: |
          Suggest a {{.style}} dinner idea.
          Include the recipe name and a brief description (2-3 sentences).

  - id: combine
    type: llm
    model: balanced
    prompt: |
      Format these meal suggestions into a daily meal plan:

      BREAKFAST:
      {{.steps.meals.breakfast.response}}

      LUNCH:
      {{.steps.meals.lunch.response}}

      DINNER:
      {{.steps.meals.dinner.response}}

      Create a nicely formatted daily meal plan with these three meals.
      Add a brief shopping list at the end.

outputs:
  - name: meal_plan
    type: string
    value: "{{.steps.combine.response}}"
```

## How It Works

### Parallel Steps
```yaml
- id: meals
  type: parallel
  steps:
    - id: breakfast
      type: llm
      prompt: ...
    - id: lunch
      type: llm
      prompt: ...
    - id: dinner
      type: llm
      prompt: ...
```
The `type: parallel` step runs all nested steps concurrently. Instead of waiting for breakfast → lunch → dinner sequentially, all three LLM calls happen at once.

### Referencing Parallel Results
```yaml
{{.steps.meals.breakfast.response}}
{{.steps.meals.lunch.response}}
{{.steps.meals.dinner.response}}
```
Access parallel step outputs using the parent step ID (`meals`) plus the nested step ID (`breakfast`).

### Using Fast Models for Parallel Work
```yaml
model: fast
```
For simple tasks running in parallel, `fast` tier models reduce cost and latency. We use `balanced` for the final combination step that requires more reasoning.

## Try It

Run the workflow:

```bash
conductor run examples/tutorial/02-parallel.yaml
```

Or with a specific style:
```bash
conductor run examples/tutorial/02-parallel.yaml -i style=mediterranean
```

Watch the output—you'll notice all three meal suggestions complete around the same time.

## Performance Comparison

| Approach | Steps | Estimated Time |
|----------|-------|----------------|
| Sequential | 3 LLM calls, one at a time | ~9 seconds |
| Parallel | 3 LLM calls simultaneously | ~3 seconds |

Parallel execution makes your workflows 3x faster when steps don't depend on each other.

## Experiment

Try different styles:
```bash
conductor run examples/tutorial/02-parallel.yaml -i style=vegan
conductor run examples/tutorial/02-parallel.yaml -i style="comfort food"
conductor run examples/tutorial/02-parallel.yaml -i style=keto
```

## Pattern Spotlight: Fan-Out/Fan-In

This workflow demonstrates the **Fan-Out/Fan-In** pattern:
1. **Fan-out**: Split work into parallel tasks
2. **Execute**: Run tasks concurrently
3. **Fan-in**: Combine results into final output

You'll use this pattern for:
- Multi-perspective code review (security + performance + style reviewers)
- Document analysis (extract from multiple sections in parallel)
- Data processing (analyze chunks concurrently, merge results)

## What's Next

Our meal suggestions are good, but what if we want *great* suggestions? In Part 3, we'll add a refinement loop that critiques and improves the plan until it meets quality standards.

[Part 3: Refinement Loops →](loops)
