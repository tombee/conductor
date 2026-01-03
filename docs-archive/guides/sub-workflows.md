# Sub-workflow Composition Guide

## Introduction

Sub-workflows let you break large workflows into smaller, reusable pieces. This guide teaches you how to compose modular workflows using sub-workflow composition.

## Quick Start

### Creating Your First Sub-workflow

Create a simple sentiment analysis sub-workflow:

```yaml
# sentiment.yaml
name: sentiment-analyzer
description: Analyzes text sentiment

inputs:
  - name: text
    type: string
    required: true
    description: Text to analyze

outputs:
  - name: sentiment
    type: string
    value: "{{.steps.analyze.outputs.category}}"
  - name: score
    type: number
    value: "{{.steps.analyze.outputs.confidence}}"

steps:
  - id: analyze
    type: llm
    model: balanced
    prompt: |
      Analyze the sentiment of this text:
      {{.inputs.text}}

      Respond in JSON format with category (positive/negative/neutral) and confidence (0-1).
    output_type: classification
    output_options:
      categories: [positive, negative, neutral]
```

### Using the Sub-workflow

Reference it from a parent workflow:

```yaml
# main.yaml
name: feedback-processor
description: Processes user feedback

inputs:
  - name: feedback
    type: string
    required: true

steps:
  - id: analyze_sentiment
    type: workflow
    workflow: ./sentiment.yaml
    inputs:
      text: "{{.inputs.feedback}}"

  - id: generate_response
    type: llm
    model: balanced
    prompt: |
      The user's feedback has {{.steps.analyze_sentiment.outputs.sentiment}} sentiment.
      Draft a response acknowledging their feedback.
```

### Running the Workflow

```bash
$ conductor run main.yaml -i feedback="Your product is amazing!"

Workflow: feedback-processor
  ✓ analyze_sentiment (1.2s)
    └─ sentiment: positive
    └─ score: 0.95
  ✓ generate_response (0.8s)

Result: Thank you for your positive feedback! We're thrilled to hear you're enjoying our product...
```

## Use Cases

### 1. Breaking Up Large Workflows

Instead of a monolithic 500-line workflow:

```yaml
# Before: monolithic-workflow.yaml (500 lines)
name: pr-review
steps:
  - id: fetch_pr
    # ... 50 lines ...
  - id: analyze_code
    # ... 100 lines ...
  - id: check_tests
    # ... 80 lines ...
  - id: security_scan
    # ... 120 lines ...
  - id: post_comment
    # ... 150 lines ...
```

Split into focused sub-workflows:

```yaml
# After: main.yaml
name: pr-review
steps:
  - id: fetch
    type: workflow
    workflow: ./steps/fetch-pr.yaml
    inputs: { pr_number: "{{.inputs.pr_number}}" }

  - id: analyze
    type: workflow
    workflow: ./steps/analyze-code.yaml
    inputs: { code: "{{.steps.fetch.outputs.code}}" }

  - id: test_check
    type: workflow
    workflow: ./steps/check-tests.yaml
    inputs: { pr_number: "{{.inputs.pr_number}}" }

  - id: security
    type: workflow
    workflow: ./steps/security-scan.yaml
    inputs: { code: "{{.steps.fetch.outputs.code}}" }

  - id: comment
    type: workflow
    workflow: ./steps/post-comment.yaml
    inputs:
      pr_number: "{{.inputs.pr_number}}"
      analysis: "{{.steps.analyze.outputs}}"
      security: "{{.steps.security.outputs}}"
```

Each sub-workflow is now:
- Easy to understand (< 100 lines)
- Testable in isolation
- Reusable across workflows

### 2. Creating Reusable Libraries

Build a library of common workflows:

```
workflows/
├── library/
│   ├── github/
│   │   ├── fetch-pr.yaml
│   │   ├── post-comment.yaml
│   │   └── merge-pr.yaml
│   ├── analysis/
│   │   ├── code-review.yaml
│   │   ├── security-scan.yaml
│   │   └── performance-check.yaml
│   └── notifications/
│       ├── send-email.yaml
│       ├── post-slack.yaml
│       └── create-ticket.yaml
└── apps/
    ├── pr-review.yaml
    ├── release-automation.yaml
    └── security-audit.yaml
```

Your application workflows reference the library:

```yaml
# apps/pr-review.yaml
steps:
  - id: fetch
    type: workflow
    workflow: ../library/github/fetch-pr.yaml
    inputs: { pr_number: "{{.inputs.pr_number}}" }

  - id: review
    type: workflow
    workflow: ../library/analysis/code-review.yaml
    inputs: { code: "{{.steps.fetch.outputs.code}}" }

  - id: notify
    type: workflow
    workflow: ../library/notifications/post-slack.yaml
    inputs:
      channel: "#code-reviews"
      message: "PR #{{.inputs.pr_number}} reviewed"
```

### 3. Agent-Style Routing Pattern

Use an LLM to decide which sub-workflow to invoke:

```yaml
# personal-assistant.yaml
name: personal-assistant
description: AI assistant that routes requests to specialized sub-workflows

steps:
  - id: understand_intent
    type: llm
    model: balanced
    prompt: |
      Analyze the user's request and determine their intent.

      User request: {{.inputs.request}}

      Output JSON with intent (email|calendar|task|search) and details.
    output_schema:
      type: object
      properties:
        intent: { type: string, enum: [email, calendar, task, search] }
        details: { type: string }
      required: [intent, details]

  - id: handle_email
    condition:
      expression: 'steps.understand_intent.outputs.intent == "email"'
    type: workflow
    workflow: ./capabilities/email-triage.yaml
    inputs:
      request: "{{.steps.understand_intent.outputs.details}}"

  - id: handle_calendar
    condition:
      expression: 'steps.understand_intent.outputs.intent == "calendar"'
    type: workflow
    workflow: ./capabilities/calendar-management.yaml
    inputs:
      request: "{{.steps.understand_intent.outputs.details}}"

  - id: handle_task
    condition:
      expression: 'steps.understand_intent.outputs.intent == "task"'
    type: workflow
    workflow: ./capabilities/task-prioritization.yaml
    inputs:
      request: "{{.steps.understand_intent.outputs.details}}"

  - id: handle_search
    condition:
      expression: 'steps.understand_intent.outputs.intent == "search"'
    type: workflow
    workflow: ./capabilities/web-search.yaml
    inputs:
      query: "{{.steps.understand_intent.outputs.details}}"

  - id: respond
    type: llm
    model: balanced
    prompt: |
      Summarize what was accomplished:

      Email: {{.steps.handle_email.outputs.result}}
      Calendar: {{.steps.handle_calendar.outputs.result}}
      Task: {{.steps.handle_task.outputs.result}}
      Search: {{.steps.handle_search.outputs.result}}
```

This pattern enables:
- **Specialized capabilities** as testable sub-workflows
- **LLM-driven routing** based on user intent
- **Easy extensibility** - add new capabilities without changing the router

## Advanced Patterns

### Parallel Sub-workflow Execution

Run multiple sub-workflows concurrently:

```yaml
- id: parallel_reviews
  type: parallel
  steps:
    - id: security
      type: workflow
      workflow: ./reviews/security.yaml
      inputs: { code: "{{.inputs.code}}" }

    - id: performance
      type: workflow
      workflow: ./reviews/performance.yaml
      inputs: { code: "{{.inputs.code}}" }

    - id: style
      type: workflow
      workflow: ./reviews/style.yaml
      inputs: { code: "{{.inputs.code}}" }
```

### Foreach with Sub-workflows

Process multiple items using a sub-workflow:

```yaml
- id: review_all_files
  type: parallel
  foreach: "{{.steps.list_files.outputs.files}}"
  steps:
    - id: review_file
      type: workflow
      workflow: ./review-single-file.yaml
      inputs:
        filename: "{{.item.name}}"
        content: "{{.item.content}}"
```

### State Threading Pattern

Thread state through multiple sub-workflow invocations:

```yaml
# refine-code.yaml - a workflow that improves code iteratively
name: refine-code

steps:
  - id: iteration_1
    type: workflow
    workflow: ./improve-code.yaml
    inputs:
      code: "{{.inputs.initial_code}}"
      iteration: 1

  - id: iteration_2
    type: workflow
    workflow: ./improve-code.yaml
    inputs:
      code: "{{.steps.iteration_1.outputs.improved_code}}"
      iteration: 2

  - id: iteration_3
    type: workflow
    workflow: ./improve-code.yaml
    inputs:
      code: "{{.steps.iteration_2.outputs.improved_code}}"
      iteration: 3
```

### Error Handling with Fallback Sub-workflows

Use a fallback sub-workflow when the primary fails:

```yaml
- id: primary_analysis
  type: workflow
  workflow: ./analysis/deep.yaml
  inputs: { text: "{{.inputs.text}}" }
  on_error:
    strategy: fallback
    fallback_step: simple_analysis

- id: simple_analysis
  type: workflow
  workflow: ./analysis/simple.yaml
  inputs: { text: "{{.inputs.text}}" }
```

## Best Practices

### 1. Design for Reusability

**Do:** Make sub-workflows generic and parameterized

```yaml
# ✅ Good: generic and reusable
name: send-notification
inputs:
  - name: channel
    type: string
  - name: message
    type: string
  - name: priority
    type: string
    default: "normal"
```

**Don't:** Hard-code specific values

```yaml
# ❌ Bad: hard-coded, not reusable
name: send-notification
steps:
  - id: send
    slack.post:
      channel: "#alerts"  # Hard-coded!
      message: "{{.inputs.message}}"
```

### 2. Keep Sub-workflows Focused

Each sub-workflow should do one thing well. Aim for <100 lines per sub-workflow.

**Do:** Single responsibility

```
✅ sentiment.yaml        - Only analyzes sentiment
✅ summarize.yaml        - Only summarizes text
✅ translate.yaml        - Only translates text
```

**Don't:** Kitchen sink workflows

```
❌ do-everything.yaml    - Analyzes, summarizes, translates, and sends emails
```

### 3. Use Clear Input/Output Contracts

Always define typed inputs and outputs:

```yaml
name: my-workflow

inputs:
  - name: text
    type: string
    required: true
    description: "Text to process"
  - name: language
    type: string
    default: "en"
    description: "Language code (en, es, fr, etc.)"

outputs:
  - name: result
    type: string
    value: "{{.steps.process.outputs.text}}"
    description: "Processed text"
  - name: word_count
    type: number
    value: "{{.steps.process.outputs.count}}"
    description: "Number of words"
```

### 4. Test Sub-workflows in Isolation

Sub-workflows can be run standalone for testing:

```bash
# Test sentiment analysis directly
$ conductor run sentiment.yaml -i text="This is great!"

# Test with different inputs
$ conductor run sentiment.yaml -i text="This is terrible" --json
```

### 5. Use Descriptive Names

**Do:** Clear, descriptive names

```
✅ analyze-sentiment.yaml
✅ fetch-github-pr.yaml
✅ send-slack-notification.yaml
```

**Don't:** Vague names

```
❌ utils.yaml
❌ helper.yaml
❌ step1.yaml
```

### 6. Document Your Sub-workflows

Add descriptions and comments:

```yaml
name: sentiment-analyzer
description: |
  Analyzes the sentiment of text using LLM classification.

  This workflow provides both a category (positive/negative/neutral)
  and a confidence score (0-1).

  Example usage:
    conductor run sentiment.yaml -i text="Great product!"

inputs:
  - name: text
    type: string
    required: true
    description: "Text to analyze for sentiment"

steps:
  # Use balanced tier for good quality sentiment analysis
  - id: analyze
    type: llm
    model: balanced
    prompt: |
      Analyze sentiment...
```

## Validation and Debugging

### Validating Workflows with Sub-workflows

The `conductor validate` command recursively checks all sub-workflow references:

```bash
$ conductor validate main.yaml

Validation Results:
  [OK] Syntax valid
  [OK] Schema valid
  [OK] All step references resolve correctly
  [OK] All sub-workflow references valid (5 sub-workflow(s))
```

If a sub-workflow is missing or invalid:

```bash
$ conductor validate main.yaml

main.yaml: error: [analyze] Failed to load sub-workflow ./missing.yaml: failed to read workflow file: no such file or directory
  Suggestion: Ensure the sub-workflow file exists and is valid
```

### Debugging Sub-workflow Failures

When a sub-workflow fails, errors include breadcrumb trails:

```
Error: main → analyze_sentiment → classify (trace: 550e8400-...): LLM API call failed: rate limit exceeded
```

This tells you:
- `main` - The parent workflow
- `analyze_sentiment` - The sub-workflow step in the parent
- `classify` - The step inside the sub-workflow that failed
- `trace: 550e8400-...` - The trace ID for correlation in logs

### Viewing Sub-workflow Traces

Sub-workflow executions are tracked with child trace IDs:

```json
{
  "step_id": "analyze_sentiment",
  "type": "workflow",
  "status": "success",
  "duration": "1.2s",
  "child_trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "outputs": {
    "sentiment": "positive",
    "score": 0.95
  }
}
```

Use the child trace ID to view detailed logs for that sub-workflow execution.

## Common Pitfalls

### ❌ Trying to Access Parent Context

Sub-workflows cannot access parent workflow data directly:

```yaml
# ❌ This won't work
name: my-subworkflow
steps:
  - id: do_something
    prompt: "Use {{.parent.steps.some_step.output}}"  # Error: parent not available
```

**Solution:** Pass data as inputs:

```yaml
# ✅ This works
# Parent workflow:
- id: call_sub
  type: workflow
  workflow: ./my-subworkflow.yaml
  inputs:
    data: "{{.steps.some_step.output}}"

# my-subworkflow.yaml:
inputs:
  - name: data
    type: string
    required: true
steps:
  - id: do_something
    prompt: "Use {{.inputs.data}}"
```

### ❌ Circular Dependencies

Sub-workflows cannot call themselves (directly or indirectly):

```yaml
# ❌ This will be rejected
# workflow-a.yaml
steps:
  - type: workflow
    workflow: ./workflow-b.yaml

# workflow-b.yaml
steps:
  - type: workflow
    workflow: ./workflow-a.yaml  # Creates a cycle!
```

**Error:** `recursion detected: workflow-a.yaml → workflow-b.yaml → workflow-a.yaml`

### ❌ Excessive Nesting

Don't nest sub-workflows too deeply:

```yaml
# ❌ Too deep (depth > 5)
main.yaml
  → level1.yaml
    → level2.yaml
      → level3.yaml
        → level4.yaml
          → level5.yaml
            → level6.yaml  # Error: exceeds max depth of 5
```

**Solution:** Flatten your workflow hierarchy or rethink the design.

## Next Steps

- See [Architecture Documentation](../architecture/sub-workflows.md) for implementation details
- Check out [Examples](../examples/sub-workflows.md) for working code
- Review [API Reference](../reference/workflow-schema.md#sub-workflows) for schema details
