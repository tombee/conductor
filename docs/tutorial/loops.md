# Part 3: Refinement Loops

Improve quality by iterating until a critic approves the output.

## What You'll Learn

- Using `type: loop` for iterative refinement
- The `until` condition for loop termination
- Implementing a critique-and-improve pattern

## The Workflow

Create `examples/tutorial/03-loops.yaml`:

```conductor
name: refined-meal-plan
description: Generate and refine a meal plan until quality standards are met

inputs:
  - name: days
    type: string
    required: false
    default: "3"
    description: Number of days to plan

steps:
  # Generate initial meal plan
  - id: draft
    type: llm
    model: balanced
    prompt: |
      Create a {{.days}}-day meal plan with breakfast, lunch, and dinner for each day.

      Requirements:
      - Variety in proteins, cuisines, and cooking methods
      - Reasonable prep times for weeknight cooking
      - Some ingredient overlap to reduce waste

      Format as a simple list by day.

  # Refinement loop
  - id: refine
    type: loop
    max_iterations: 3
    until: 'steps.critique.response contains "APPROVED"'
    steps:
      - id: critique
        type: llm
        model: balanced
        prompt: |
          Review this meal plan (iteration {{.loop.iteration}}):

          {{.steps.draft.response}}

          Evaluate against these criteria:
          1. Nutritional balance (proteins, vegetables, variety)
          2. Practical prep times
          3. Ingredient efficiency (some reuse across meals)
          4. Overall variety and appeal

          If the plan meets all criteria well, respond with exactly: APPROVED

          Otherwise, provide specific, actionable improvements needed.

      - id: improve
        type: llm
        model: balanced
        when: 'not (steps.critique.response contains "APPROVED")'
        prompt: |
          Improve this meal plan based on the feedback:

          CURRENT PLAN:
          {{.steps.draft.response}}

          FEEDBACK:
          {{.steps.critique.response}}

          Create an improved version addressing all feedback points.

outputs:
  - name: meal_plan
    type: string
    value: "{{.steps.refine.improve.response}}"
  - name: iterations
    type: string
    value: "{{.loop.iteration}}"
```

## How It Works

### Loop Structure
```yaml
- id: refine
  type: loop
  max_iterations: 3
  until: 'steps.critique.response contains "APPROVED"'
  steps:
    - id: critique
    - id: improve
```
The loop runs until either:
- The `until` condition becomes true (critic says "APPROVED")
- `max_iterations` is reached (safety limit)

### Conditional Execution
```yaml
when: 'not (steps.critique.response contains "APPROVED")'
```
The `improve` step only runs if the critique didn't approve. This prevents unnecessary work. Note that `when` and `until` use expression syntax, not template syntax.

### Accessing Loop State
```yaml
{{.loop.iteration}}  # Current iteration (0-indexed)
```
Use `.loop.iteration` to know which iteration you're on. This is useful for logging or conditional logic.

## Try It

Run the workflow:

```bash
conductor run examples/tutorial/03-loops.yaml
```

Or for a longer plan:
```bash
conductor run examples/tutorial/03-loops.yaml -i days=7
```

Watch the iterations:
```
[1/2] draft... OK
[2/2] refine (iteration 1)...
  - critique... OK
  - improve... OK
[2/2] refine (iteration 2)...
  - critique... APPROVED
```

## Understanding the Output

The output shows the refined meal plan. It should be better than the initial draft, with:
- Better nutritional balance
- More efficient ingredient use
- Improved variety

## Experiment

Try adjusting the loop:
```yaml
max_iterations: 5  # Allow more refinement
```

Or make the critic stricter:
```yaml
prompt: |
  Be very critical. Only approve if the plan is exceptional...
```

## Pattern Spotlight: Critique-Improve Loop

This workflow demonstrates the **Critique-Improve Loop** pattern:
1. **Generate**: Create initial output
2. **Critique**: Evaluate against criteria
3. **Improve**: Address critique feedback
4. **Repeat**: Until quality threshold met

You'll use this pattern for:
- Document editing (draft → review → revise)
- Code generation (generate → lint → fix)
- Content refinement (write → fact-check → correct)

## What's Next

Our meal planner works great when we run it manually. In Part 4, we'll set it up to run automatically every week using scheduled triggers.

[Part 4: Scheduled Triggers →](triggers)
