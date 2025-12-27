# Your First Workflow

Build a complete Conductor workflow from scratch. Learn workflow structure, LLM steps, inputs, outputs, and template variables.

**Difficulty:** Beginner
**Prerequisites:** Conductor installed and LLM provider configured (see [Quick Start](../../quick-start.md))

---

## What You'll Build

A greeting workflow that:

1. Accepts a person's name as input
2. Generates a personalized greeting using an LLM
3. Optionally translates the greeting to another language
4. Saves the output to a file

This demonstrates core Conductor concepts: inputs, LLM steps, template variables, tool steps, and outputs.

---

## Step 1: Create the Workflow File

Create `hello.yaml` in your project directory:

```bash
mkdir my-workflows
cd my-workflows
touch hello.yaml
```

Open it in your text editor.

---

## Step 2: Add Workflow Metadata

Start with basic metadata:

```yaml
name: hello-world
description: Generate personalized greetings
version: "1.0"
```

**Fields:**

- `name` — Workflow identifier (lowercase, hyphens)
- `description` — What the workflow does
- `version` — Semantic version (optional)

---

## Step 3: Define Inputs

Add an input for the person's name:

```yaml
inputs:
  - name: user_name
    type: string
    required: true
    description: Name of the person to greet
```

**Input fields:**

- `name` — Parameter name (referenced as `{{.inputs.user_name}}`)
- `type` — Data type (`string`, `number`, `boolean`, `array`, `object`)
- `required` — Must be provided (default: false)
- `description` — Help text for users

---

## Step 4: Add Your First LLM Step

Add a step that generates the greeting:

```yaml
steps:
  - id: generate_greeting
    name: Generate Personalized Greeting
    type: llm
    model: fast
    system: |
      You are a friendly assistant that creates warm, personalized greetings.
      Keep greetings to 2-3 sentences.
    prompt: |
      Create a warm, personalized greeting for {{.inputs.user_name}}.
      Make it welcoming and friendly.
```

**Breaking it down:**

- `id: generate_greeting` — Unique identifier (used to reference this step's output)
- `type: llm` — This is an LLM step
- `model: fast` — Use the fast model tier (Claude Haiku, GPT, etc.)
- `system:` — System prompt (sets LLM behavior)
- `prompt:` — User prompt with template variable `{{.inputs.user_name}}`

:::tip[Model Tiers]
- **fast** — Quick, cheap tasks (1-2s, $$)
- **balanced** — Most workflows (3-5s, $$$)
- **strategic** — Complex reasoning (5-15s, $$$$)

See [Model Tiers](../concepts/model-tiers.md) for details.
:::


---

## Step 5: Add an Output

Extract the greeting as a workflow output:

```yaml
outputs:
  - name: greeting
    type: string
    value: "{{.steps.generate_greeting.response}}"
    description: The personalized greeting message
```

**Output fields:**

- `name` — Output identifier
- `value` — Template expression to extract the value
- `description` — What this output contains

The `{{.steps.generate_greeting.response}}` expression means:

- `.steps` — Access step outputs
- `.generate_greeting` — The step with `id: generate_greeting`
- `.response` — The LLM's text response

---

## Step 6: Run Your First Workflow

Save `hello.yaml` and run it:

```bash
conductor run hello.yaml
```

Conductor prompts for the required input:

```
user_name: Alice
```

**Expected output:**

```
[conductor] Starting workflow: hello-world
[conductor] Step 1/1: generate_greeting (llm)
[conductor] ✓ Completed in 1.8s

--- Output: greeting ---
Hello Alice! It's wonderful to meet you. I hope you're having a fantastic day
filled with exciting opportunities and moments of joy!

[workflow complete]
```

!!! success "Congratulations!"
    You just ran your first AI workflow!

---

## Step 7: Understanding Template Variables

Let's trace how `{{.inputs.user_name}}` works:

1. You provide: `user_name: Alice`
2. Conductor renders template: `Create a warm, personalized greeting for Alice.`
3. LLM receives the rendered prompt
4. LLM responds with the greeting
5. Output references response: `{{.steps.generate_greeting.response}}`

**Template syntax:**

- `{{.inputs.name}}` — Workflow input
- `{{.steps.id.response}}` — LLM step output
- `{{.steps.id.stdout}}` — Shell command output
- `{{.steps.id.content}}` — File content

See [Template Variables](../concepts/template-variables.md) for complete reference.

---

## Step 8: Add a Second Input

Let's make greetings more personal. Add an input for the occasion:

```yaml
inputs:
  - name: user_name
    type: string
    required: true
    description: Name of the person to greet

  - name: occasion
    type: string
    required: false
    default: "meeting you"
    description: Occasion for the greeting (birthday, promotion, etc.)
```

Update the prompt to use it:

```yaml
prompt: |
  Create a warm, personalized greeting for {{.inputs.user_name}} for {{.inputs.occasion}}.
  Make it welcoming and appropriate for the occasion.
```

Run it:

```bash
conductor run hello.yaml -i user_name="Alice" -i occasion="your birthday"
```

**Output:**

```
Happy Birthday Alice! 🎉 Wishing you a wonderful day filled with joy, laughter,
and all the things that make you smile. May this year bring you amazing
adventures and beautiful memories!
```

---

## Step 9: Add a Translation Step

Let's chain two LLM steps together. Add a new input:

```yaml
inputs:
  - name: user_name
    type: string
    required: true
  - name: occasion
    type: string
    default: "meeting you"
  - name: translate_to
    type: string
    required: false
    description: Language to translate greeting to (leave blank for English only)
```

Add a conditional translation step after the greeting:

```yaml
steps:
  - id: generate_greeting
    type: llm
    model: fast
    system: |
      You are a friendly assistant that creates warm, personalized greetings.
      Keep greetings to 2-3 sentences.
    prompt: |
      Create a warm, personalized greeting for {{.inputs.user_name}} for {{.inputs.occasion}}.

  - id: translate
    type: llm
    model: fast
    condition:
      expression: 'inputs.translate_to != ""'
    prompt: |
      Translate this greeting to {{.inputs.translate_to}}:

      {{.steps.generate_greeting.response}}

      Provide only the translation, no explanations.
```

**Key points:**

- `condition:` — Only runs if translate_to is provided
- `{{.steps.generate_greeting.response}}` — References the first step's output

Add a conditional output:

```yaml
outputs:
  - name: greeting
    type: string
    value: |
      {{if .inputs.translate_to}}
      {{.steps.translate.response}}
      {{else}}
      {{.steps.generate_greeting.response}}
      {{end}}
```

Run it:

```bash
conductor run hello.yaml \
  -i user_name="Alice" \
  -i occasion="your promotion" \
  -i translate_to="Spanish"
```

**Output:**

```
¡Felicidades por tu ascenso, Alice! Este es un logro bien merecido que refleja
tu arduo trabajo y dedicación. ¡Que este nuevo capítulo te traiga muchos éxitos
y satisfacciones!
```

---

## Step 10: Save Output to a File

Add a file write step:

```yaml
steps:
  - id: generate_greeting
    type: llm
    model: fast
    prompt: "..."

  - id: translate
    type: llm
    model: fast
    condition: "..."
    prompt: "..."

  - id: save_greeting
    file.write:
      path: "greeting.txt"
      content: |
        {{if .inputs.translate_to}}
        {{.steps.translate.response}}
        {{else}}
        {{.steps.generate_greeting.response}}
        {{end}}
```

**Connector shorthand:**

- `file.write:` — Shorthand for file tool write operation
- `path:` — Where to save the file
- `content:` — What to write (uses template variables)

Run it:

```bash
conductor run hello.yaml -i user_name="Bob" -i occasion="your first day"
```

Check the file:

```bash
cat greeting.txt
```

---

## Complete Workflow

Here's the final `hello.yaml`:

```yaml
name: hello-world
description: Generate personalized greetings with optional translation
version: "1.0"

inputs:
  - name: user_name
    type: string
    required: true
    description: Name of the person to greet

  - name: occasion
    type: string
    required: false
    default: "meeting you"
    description: Occasion for the greeting

  - name: translate_to
    type: string
    required: false
    description: Language to translate to (optional)

  - name: output_file
    type: string
    default: "greeting.txt"
    description: File to save greeting to

steps:
  - id: generate_greeting
    name: Generate Greeting
    type: llm
    model: fast
    system: |
      You are a friendly assistant that creates warm, personalized greetings.
      Keep greetings to 2-3 sentences.
    prompt: |
      Create a warm, personalized greeting for {{.inputs.user_name}} for {{.inputs.occasion}}.

  - id: translate
    name: Translate Greeting
    type: llm
    model: fast
    condition:
      expression: 'inputs.translate_to != ""'
    prompt: |
      Translate this greeting to {{.inputs.translate_to}}:

      {{.steps.generate_greeting.response}}

      Provide only the translation, no explanations.

  - id: save_greeting
    name: Save to File
    file.write:
      path: "{{.inputs.output_file}}"
      content: |
        {{if .inputs.translate_to}}
        {{.steps.translate.response}}
        {{else}}
        {{.steps.generate_greeting.response}}
        {{end}}

outputs:
  - name: greeting
    type: string
    value: |
      {{if .inputs.translate_to}}
      {{.steps.translate.response}}
      {{else}}
      {{.steps.generate_greeting.response}}
      {{end}}
    description: The final greeting (translated if requested)

  - name: file_path
    type: string
    value: "{{.inputs.output_file}}"
    description: Where the greeting was saved
```

---

## Testing Your Workflow

### Validate Syntax

Check for errors before running:

```bash
conductor validate hello.yaml
```

### Test Different Scenarios

```bash
# Basic greeting
conductor run hello.yaml -i user_name="Alice"

# Birthday greeting
conductor run hello.yaml -i user_name="Bob" -i occasion="your birthday"

# Translated promotion greeting
conductor run hello.yaml \
  -i user_name="Carlos" \
  -i occasion="your promotion" \
  -i translate_to="French"

# Custom output file
conductor run hello.yaml \
  -i user_name="Diana" \
  -i output_file="diana_greeting.txt"
```

---

## Customization Ideas

### Different Personalities

Change the system prompt:

```yaml
system: |
  You are a formal, professional assistant.
  Create respectful, business-appropriate greetings.
```

Or:

```yaml
system: |
  You are an enthusiastic cheerleader.
  Create energetic, motivational greetings with lots of excitement!
```

### Add More Context

```yaml
inputs:
  - name: relationship
    type: string
    enum: ["colleague", "friend", "family", "client"]

prompt: |
  Create a greeting for {{.inputs.user_name}}, who is my {{.inputs.relationship}},
  for {{.inputs.occasion}}.
```

### Use Different Model Tiers

For more creative, nuanced greetings:

```yaml
- id: generate_greeting
  type: llm
  model: balanced    # Or strategic for even better quality
```

---

## Troubleshooting

### "workflow validation failed"

**Problem:** YAML syntax error

**Solution:** Check indentation (use spaces, not tabs). Common issues:

```yaml
# Wrong - missing colon
steps
  - id: greet

# Wrong - tabs instead of spaces
steps:
	- id: greet

# Correct
steps:
  - id: greet
```

### "template: undefined variable"

**Problem:** Typo in template variable

**Solution:** Check spelling:

```yaml
# Wrong
prompt: "Hello {{.inputs.name}}"

# Correct (matches input definition)
prompt: "Hello {{.inputs.user_name}}"
```

### "step 'translate' skipped"

**Problem:** Condition not met

**Solution:** This is expected! The translate step only runs when `translate_to` is provided:

```bash
# Translation runs
conductor run hello.yaml -i user_name="Alice" -i translate_to="Spanish"

# Translation skipped (normal behavior)
conductor run hello.yaml -i user_name="Alice"
```

### "provider not configured"

**Problem:** No LLM provider available

**Solution:** Ensure Claude Code is installed and you're signed in:

```bash
claude --version
```

If not installed, see the [Quick Start](../../quick-start.md).

---

## Key Concepts Learned

!!! success "You now understand:"
    - **Workflow structure** — metadata, inputs, steps, outputs
    - **LLM steps** — Using `type: llm` with model tiers
    - **Template variables** — `{{.inputs.*}}` and `{{.steps.*.response}}`
    - **Step chaining** — Referencing previous step outputs
    - **Conditional execution** — Using `condition:` to skip steps
    - **Tool integration** — Using `file.write:` shorthand syntax
    - **Workflow validation** — Checking syntax before running

---

## What's Next?

### Learn More Concepts

- **[Template Variables](../concepts/template-variables.md)** — Master the `{{}}` syntax
- **[Model Tiers](../concepts/model-tiers.md)** — Optimize cost and performance
- **[Error Handling](../concepts/error-handling.md)** — Build production-ready workflows

### Try More Examples

- **[Git Branch Code Review](../../examples/code-review.md)** — Multi-persona parallel reviews
- **[Issue Triage](../../examples/issue-triage.md)** — Classification and analysis
- **[Slack Integration](../../examples/slack-integration.md)** — Send workflow results to Slack

### Build Something Real

Challenge yourself:

1. **Email Summarizer** — Read email, summarize, categorize
2. **Commit Message Generator** — Analyze git diff, suggest commit message
3. **Documentation Updater** — Read code, generate/update docs

### Advanced Topics

- **[Flow Control](../building-workflows/flow-control.md)** — Parallel execution and conditionals
- **[Connectors](../reference/connectors/index.md)** — Integrate GitHub, Jira, Slack
- **[Testing](../building-workflows/testing.md)** — Build reliable automation

---

## Additional Resources

- **[Workflow Schema Reference](../../reference/workflow-schema.md)** — Complete YAML specification
- **[CLI Reference](../../reference/cli.md)** — All conductor commands
- **[Examples Gallery](../../examples/index.md)** — More workflow ideas
