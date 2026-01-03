# Part 4: Scheduled Triggers

Run your meal planner automatically every week.

## What You'll Learn

- How triggers configure when workflows run
- Adding schedule triggers with cron syntax
- Running the controller for automated execution

## How Triggers Work

Triggers define **when** a workflow runs. They're configured separately from workflow files so your workflows stay portable—the same workflow can be triggered different ways:
- Manually via CLI during development
- On a schedule in production
- Via webhook when integrated with other systems

The workflow file stays the same. Only the trigger configuration changes.

## Adding a Schedule Trigger

Add a weekly schedule to run the meal planner every Sunday at 9 AM:

```bash
conductor triggers add schedule meal-plan \
  --workflow examples/tutorial/03-loops.yaml \
  --cron "0 9 * * 0" \
  --input days=7
```

### Understanding Cron Syntax

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sunday=0)
│ │ │ │ │
0 9 * * 0
```

Common patterns:
- `0 9 * * 0` — Every Sunday at 9:00 AM
- `0 9 * * 1-5` — Every weekday at 9:00 AM
- `0 */6 * * *` — Every 6 hours
- `0 0 1 * *` — First of each month at midnight

## View Your Triggers

List all configured triggers:

```bash
conductor triggers list
```

Output:
```
NAME        TYPE      SCHEDULE     WORKFLOW                           INPUTS
meal-plan   schedule  0 9 * * 0    examples/tutorial/03-loops.yaml   days=7
```

## Running the Controller

Triggers require the controller to be running. The controller is Conductor's background service that monitors triggers and executes workflows.

### Development Mode

For testing, run the controller in the foreground:

```bash
conductor controller start --foreground
```

Keep this running in a terminal. The controller will execute your scheduled workflow when the time comes.

### Production Mode

For production, run the controller as a background service:

```bash
conductor controller start
```

Check controller status:
```bash
conductor controller status
```

## Testing Your Trigger

Don't wait until Sunday—test it now:

```bash
conductor triggers test meal-plan
```

This runs the workflow immediately as if the trigger fired, letting you verify everything works.

## The Workflow (Unchanged)

Note that the workflow file itself hasn't changed from Part 3. We didn't add any trigger configuration to the YAML—that's the point. The workflow stays clean and portable.

For reference, save this as `examples/tutorial/04-triggers.yaml` (identical to 03-loops.yaml):

```conductor
# This is the same workflow as 03-loops.yaml
# Triggers are configured separately via: conductor triggers add schedule

name: refined-meal-plan
description: Generate and refine a meal plan until quality standards are met

inputs:
  - name: days
    type: string
    required: false
    default: "3"
    description: Number of days to plan

steps:
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
          1. Nutritional balance
          2. Practical prep times
          3. Ingredient efficiency
          4. Variety and appeal

          If the plan meets all criteria, respond with exactly: APPROVED
          Otherwise, provide specific improvements needed.

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

          Create an improved version.

outputs:
  - name: meal_plan
    type: string
    value: "{{.steps.refine.improve.response}}"
```

## Managing Triggers

Remove a trigger:
```bash
conductor triggers remove meal-plan
```

Update a trigger's schedule:
```bash
conductor triggers remove meal-plan
conductor triggers add schedule meal-plan \
  --workflow examples/tutorial/04-triggers.yaml \
  --cron "0 18 * * 5" \
  --input days=7
```
(Now runs Friday at 6 PM instead)

## Pattern Spotlight: Scheduled Automation

This demonstrates the **Scheduled Automation** pattern:
1. **Define workflow** — Keep it portable, no trigger config
2. **Add trigger** — Configure when/how it runs separately
3. **Run controller** — Background service monitors triggers
4. **Automatic execution** — Workflows run on schedule

You'll use this pattern for:
- Daily reports (aggregate data → summarize → deliver)
- Weekly reviews (collect metrics → analyze trends → notify)
- Periodic cleanup (scan → evaluate → archive)

## What's Next

Our meal planner generates plans, but it doesn't know what's in our kitchen. In Part 5, we'll read a pantry inventory file to make suggestions based on ingredients we actually have.

[Part 5: Reading External Data →](input)
