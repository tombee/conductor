# Quick Start

Get Conductor running and execute your first AI workflow.

---

## Step 1: Install Conductor

Choose your preferred installation method:

=== "Homebrew (macOS/Linux)"

    ```bash
    brew install conductor
    ```

=== "Go Install"

    ```bash
    go install github.com/tombee/conductor/cmd/conductor@latest
    ```

=== "From Source"

    ```bash
    git clone https://github.com/tombee/conductor
    cd conductor
    make install
    ```

**Verify installation:**

```bash
conductor --version
```

You should see the version number. If not, see [Troubleshooting](#troubleshooting) below.

---

## Step 2: Install Claude Code

Conductor works best with [Claude Code](https://claude.ai/download) as the LLM provider. With Claude Code installed, Conductor works out of the box with no API key configuration required.

1. Download and install Claude Code from [claude.ai/download](https://claude.ai/download)
2. Complete the Claude Code setup and sign in
3. Verify it's working:

```bash
claude --version
```

That's it! Conductor will automatically detect and use Claude Code.

:::tip[Other Providers]
For API-based providers (Anthropic API, OpenAI, etc.), see [Configuration](reference/configuration.md).
:::

---

## Step 3: Run Your First Workflow

Let's run a simple workflow that generates a song:

```bash
conductor run examples/write-song/workflow.yaml
```

Conductor will prompt you for inputs:

```
genre: blues
topic: debugging at 3am
key: (leave blank for default C Major)
```

**Expected output:**

```
[conductor] Starting workflow: write-song
[conductor] Step 1/1: compose (llm)
[conductor] ✓ Completed in 3.2s

--- Output: song ---
[The LLM generates a song based on your inputs]

[workflow complete]
```

You just ran your first AI workflow!

---

## Step 4: Create Your Own Workflow

Create a file called `hello.yaml`:

```yaml
name: hello-conductor
description: Your first custom workflow

inputs:
  - name: name
    type: string
    required: true
    description: Your name

steps:
  - id: greet
    type: llm
    model: fast
    prompt: |
      Generate a friendly, personalized greeting for someone named {{.inputs.name}}.
      Make it warm and encouraging. Keep it to 2-3 sentences.

outputs:
  - name: greeting
    value: "{{.steps.greet.response}}"
```

Run it:

```bash
conductor run hello.yaml
```

When prompted, enter your name:

```
name: Alex
```

**Output:**

```
[conductor] Starting workflow: hello-conductor
[conductor] Step 1/1: greet (llm)
[conductor] ✓ Completed in 1.8s

--- Output: greeting ---
Hey Alex! It's great to meet you. I hope you're having a wonderful day and
finding exciting possibilities in whatever you're working on. Welcome aboard!

[workflow complete]
```

---

## Understanding What Just Happened

Let's break down the workflow:

```yaml
steps:
  - id: greet                    # Step identifier (used to reference outputs)
    type: llm                    # Step type (llm, tool, parallel)
    model: fast                  # Model tier (fast, balanced, strategic)
    prompt: |                    # Prompt sent to the LLM
      Generate a greeting for {{.inputs.name}}
```

**Key concepts:**

1. **Template variables:** `{{.inputs.name}}` inserts the input value
2. **Step outputs:** Each step stores its response (accessible as `{{.steps.greet.response}}`)
3. **Model tiers:** `fast`, `balanced`, `strategic` map to appropriate models for your provider
4. **Sequential execution:** Steps run in order, each can reference previous outputs

---

## What's Next?

### Try a Real-World Example

Run a code review on your current git branch:

```bash
conductor run examples/git-branch-code-review/workflow.yaml
```

This workflow:

- Gets your git diff
- Runs parallel reviews (security, performance, style)
- Generates a comprehensive `code-review.md` report

### Learn Core Concepts

Understand the fundamentals:

- [Workflows and Steps](learn/concepts/workflows-steps.md) — How workflows are structured
- [Inputs and Outputs](learn/concepts/inputs-outputs.md) — Pass data into and out of workflows
- [Template Variables](learn/concepts/template-variables.md) — Using `{{.inputs.*}}` and `{{.steps.*}}`

### Follow a Tutorial

Build a workflow from scratch:

- [Your First Workflow](learn/tutorials/first-workflow.md) — Hands-on guide

### Browse Examples

See what's possible:

- [Examples](examples/) — Copy-paste ready workflows

---

## Troubleshooting

### "conductor: command not found"

**Solution:** Ensure your `$PATH` includes the installation directory.

For Homebrew:
```bash
echo $PATH | grep -q homebrew && echo "Homebrew is in PATH" || echo "Add Homebrew to PATH"
```

For Go install:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

### "Provider not configured"

**Solution:** Ensure Claude Code is installed and working:

```bash
claude --version
```

If Claude Code isn't installed, see [Step 2](#step-2-install-claude-code).

### "Workflow validation failed"

**Common causes:**

1. **Incorrect YAML indentation** — YAML is whitespace-sensitive
2. **Missing required fields** — Every step needs `id` and `type`
3. **Invalid template syntax** — Use `{{.inputs.name}}` not `{{name}}`

**Validation tip:**
```bash
conductor validate hello.yaml
```

### "Rate limit exceeded"

**Solution:** You've hit your provider's rate limit. Wait a moment and try again, or:

- Use a lower model tier (`fast` instead of `strategic`)
- Add delays between steps if running many workflows
- Check your provider's rate limits

### Still stuck?

- Check the [Troubleshooting Guide](troubleshooting.md)
- Open an issue on [GitHub](https://github.com/tombee/conductor/issues)
- Ask in [Discussions](https://github.com/tombee/conductor/discussions)

---

## Quick Reference

### Basic Commands

```bash
# Run a workflow
conductor run workflow.yaml

# Run with inline inputs (skip prompts)
conductor run workflow.yaml -i name=Alex -i age=30

# Validate workflow syntax
conductor validate workflow.yaml

# List available examples
ls examples/

# Get help
conductor --help
conductor run --help
```

### Workflow Syntax Cheat Sheet

```yaml
# Minimal workflow
name: my-workflow
steps:
  - id: step1
    type: llm
    prompt: "Your prompt here"

# With inputs
inputs:
  - name: user_name
    type: string
    required: true

# With outputs
outputs:
  - name: result
    value: "{{.steps.step1.response}}"

# Using previous step output
steps:
  - id: analyze
    type: llm
    prompt: "Analyze this: {{.inputs.text}}"

  - id: summarize
    type: llm
    prompt: "Summarize: {{.steps.analyze.response}}"

# File operations
  - id: save
    file.write:
      path: "output.txt"
      content: "{{.steps.summarize.response}}"

# Shell commands
  - id: get_files
    shell.run:
      command: ["ls", "-la"]

# Parallel execution
  - id: parallel_reviews
    type: parallel
    steps:
      - id: review1
        type: llm
        prompt: "Review A..."
      - id: review2
        type: llm
        prompt: "Review B..."
```

---

**Congratulations!** You've successfully installed Conductor and run your first workflows. Ready to dive deeper? Check out [Workflows and Steps](learn/concepts/workflows-steps.md) or browse [Examples](examples/).
