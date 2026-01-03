# Part 5: Reading External Data

Make the meal planner smarter by reading what's actually in your pantry.

## What You'll Learn

- Reading files with `file.read` action
- Using external data in prompts
- Building context-aware workflows

## The Workflow

Create `examples/tutorial/05-input.yaml`:

```conductor
name: pantry-aware-meals
description: Generate meal plans based on available ingredients

inputs:
  - name: days
    type: string
    required: false
    default: "3"
    description: Number of days to plan
  - name: pantry_file
    type: string
    required: false
    default: "pantry.txt"
    description: Path to pantry inventory file

steps:
  # Read pantry inventory
  - id: read_pantry
    file.read: "{{.pantry_file}}"

  # Generate meal plan using pantry data
  - id: plan
    type: llm
    model: balanced
    prompt: |
      Create a {{.days}}-day meal plan using primarily these available ingredients:

      PANTRY INVENTORY:
      {{.steps.read_pantry.response}}

      Requirements:
      - Prioritize using ingredients from the pantry
      - Only suggest purchasing fresh items (produce, dairy, meat)
      - Include breakfast, lunch, and dinner for each day
      - Note which pantry items each meal uses

      Format as a daily plan with a shopping list for items not in pantry.

  # Critique and improve
  - id: refine
    type: loop
    max_iterations: 2
    until: 'steps.critique.response contains "APPROVED"'
    steps:
      - id: critique
        type: llm
        model: balanced
        prompt: |
          Review this meal plan for pantry efficiency:

          AVAILABLE INGREDIENTS:
          {{.steps.read_pantry.response}}

          MEAL PLAN:
          {{.steps.plan.response}}

          Check:
          1. Are pantry ingredients well-utilized?
          2. Is the shopping list minimal?
          3. Are meals practical and balanced?

          If good, respond with: APPROVED
          Otherwise, suggest specific improvements.

      - id: improve
        type: llm
        model: balanced
        when: 'not (steps.critique.response contains "APPROVED")'
        prompt: |
          Improve this meal plan based on feedback:

          PANTRY: {{.steps.read_pantry.response}}

          CURRENT PLAN:
          {{.steps.plan.response}}

          FEEDBACK: {{.steps.critique.response}}

          Create an improved version that better uses pantry ingredients.

outputs:
  - name: meal_plan
    type: string
    value: "{{.steps.refine.improve.response}}"
```

## The Pantry File

Create `examples/tutorial/pantry.txt`:

```
# Pantry Inventory
# Last updated: This week

## Grains & Pasta
- Rice (jasmine and brown)
- Pasta (spaghetti, penne)
- Oats (rolled)
- Flour (all-purpose)
- Bread crumbs

## Canned Goods
- Diced tomatoes (3 cans)
- Coconut milk (2 cans)
- Black beans (2 cans)
- Chickpeas (1 can)
- Tuna (2 cans)

## Oils & Vinegars
- Olive oil
- Vegetable oil
- Soy sauce
- Rice vinegar
- Balsamic vinegar

## Spices
- Salt, pepper
- Garlic powder
- Cumin
- Paprika
- Italian seasoning
- Curry powder

## Baking
- Sugar
- Brown sugar
- Honey
- Vanilla extract
- Baking powder

## Other
- Peanut butter
- Chicken broth (2 boxes)
- Onions (4)
- Garlic (1 head)
- Potatoes (bag)
```

## How It Works

### Reading Files
```yaml
- id: read_pantry
  file.read: "{{.pantry_file}}"
```
The `file.read` action reads file contents into `.steps.read_pantry.response`. The path can be dynamic via template variables.

### Using File Content in Prompts
```yaml
prompt: |
  Create a meal plan using these ingredients:

  PANTRY INVENTORY:
  {{.steps.read_pantry.response}}
```
The file contents become part of the LLM context. This is how you build context-aware workflows.

### Parameterized File Paths
```yaml
inputs:
  - name: pantry_file
    default: "pantry.txt"
```
Making the file path an input allows flexibility—different households can use different inventory files.

## Try It

First, make sure you have the pantry file:
```bash
cat examples/tutorial/pantry.txt
```

Run the workflow:
```bash
conductor run examples/tutorial/05-input.yaml
```

The generated meal plan should reference ingredients from your pantry file!

## Experiment

Update your pantry and re-run:
```bash
echo "- Eggs (dozen)" >> examples/tutorial/pantry.txt
conductor run examples/tutorial/05-input.yaml
```

Try with a different file:
```bash
conductor run examples/tutorial/05-input.yaml -i pantry_file=my-pantry.txt
```

## Pattern Spotlight: Context Injection

This workflow demonstrates the **Context Injection** pattern:
1. **Read context** — Load external data (files, APIs, databases)
2. **Inject into prompt** — Include context in LLM prompt
3. **Generate aware output** — LLM responds with context in mind

You'll use this pattern for:
- Code review with file contents
- Document Q&A with source material
- Personalized responses with user data
- Analysis based on configuration files

## What's Next

We have a complete meal planner that reads our pantry. In Part 6, we'll send the results somewhere useful—like a Notion page your family can access.

[Part 6: Delivering Results →](output)
