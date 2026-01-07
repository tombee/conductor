# Parallel Execution

Conductor supports two forms of parallel execution:
1. **Automatic parallelization** - Steps without dependencies run concurrently
2. **Explicit parallel blocks** - `type: parallel` with nested steps for controlled concurrent execution

## Automatic Parallelization

Steps that don't reference each other run in parallel automatically:

```yaml
steps:
  - id: task1
    llm:
      prompt: "Generate a recipe"
  - id: task2
    llm:
      prompt: "Generate a workout plan"
  - id: task3
    llm:
      prompt: "Generate a reading list"
```

All three steps execute simultaneously since none depends on another.

## Explicit Parallel Blocks

Use `type: parallel` for explicit control over concurrent execution:

```yaml
steps:
  - id: reviews
    type: parallel
    max_concurrency: 3
    steps:
      - id: security_review
        type: llm
        prompt: "Review for security issues: {{.inputs.code}}"

      - id: performance_review
        type: llm
        prompt: "Review for performance issues: {{.inputs.code}}"

      - id: style_review
        type: llm
        prompt: "Review for style issues: {{.inputs.code}}"

  - id: combine
    type: llm
    prompt: |
      Combine the reviews:
      Security: {{.steps.reviews.security_review.response}}
      Performance: {{.steps.reviews.performance_review.response}}
      Style: {{.steps.reviews.style_review.response}}
```

### Shorthand Syntax

Use `parallel:` as a shorthand for `type: parallel` with nested `steps`:

```yaml
# Shorthand form
- id: reviews
  parallel:
    - id: security
      llm:
        prompt: "Security review..."
    - id: performance
      llm:
        prompt: "Performance review..."

# Equivalent explicit form
- id: reviews
  type: parallel
  steps:
    - id: security
      type: llm
      prompt: "Security review..."
    - id: performance
      type: llm
      prompt: "Performance review..."
```

The shorthand supports all parallel options:

```yaml
- id: batch
  parallel:
    - id: task1
      llm:
        prompt: "Task 1"
    - id: task2
      llm:
        prompt: "Task 2"
  max_concurrency: 2
  foreach: "{{.inputs.items}}"
  on_error:
    strategy: ignore
```

### When to Use Explicit Parallel Blocks

Use `type: parallel` when you need:
- **Controlled concurrency** - Limit simultaneous executions with `max_concurrency`
- **Error handling strategies** - Choose fail-fast or continue-on-error behavior
- **Grouped execution** - Ensure a set of steps completes before continuing
- **Batch processing** - Process arrays in parallel with `foreach`

## Concurrency Control

### max_concurrency

Limit how many nested steps run simultaneously:

```yaml
- id: batch_process
  type: parallel
  max_concurrency: 5  # At most 5 steps run at once
  steps:
    # ... many nested steps
```

Default is 3 if not specified. This prevents overwhelming external APIs or exhausting system resources.

### Workflow-Level Concurrency

Set a default concurrency limit for the entire workflow:

```yaml
name: my-workflow
config:
  maxConcurrency: 10
steps:
  # ...
```

## Error Handling

### Fail-Fast (Default)

By default, the first error cancels all other parallel steps:

```yaml
- id: critical_checks
  type: parallel
  steps:
    - id: check1
      # If this fails, check2 and check3 are cancelled
    - id: check2
    - id: check3
```

### Continue on Error

Use `on_error` to let all steps complete even if some fail:

```yaml
- id: optional_checks
  type: parallel
  on_error:
    strategy: ignore
  steps:
    - id: check1
      # If this fails, check2 and check3 still run
    - id: check2
    - id: check3
```

With `ignore` strategy:
- All steps run to completion
- Partial results from successful steps are available
- The parallel block reports how many steps failed

## Accessing Nested Step Outputs

Access nested step outputs using the path `{{.steps.parallel_id.nested_id.field}}`:

```yaml
- id: analysis
  type: parallel
  steps:
    - id: summary
      type: llm
      prompt: "Summarize this document"
    - id: keywords
      type: llm
      output_schema:
        type: object
        properties:
          terms:
            type: array
            items:
              type: string
      prompt: "Extract keywords from this document"

- id: report
  type: llm
  prompt: |
    Summary: {{.steps.analysis.summary.response}}
    Keywords: {{.steps.analysis.keywords.output.terms}}
```

**Output path patterns:**
- `{{.steps.parallel_id.nested_id.response}}` - LLM response text
- `{{.steps.parallel_id.nested_id.output.field}}` - Structured output field
- `{{.steps.parallel_id.nested_id.stdout}}` - Shell command output

## Parallel Foreach

Process array elements in parallel using `foreach` on a parallel step:

```yaml
- id: process_files
  type: parallel
  foreach: "{{.inputs.files}}"
  max_concurrency: 3
  steps:
    - id: analyze
      type: llm
      prompt: "Analyze file: {{.item}}"
```

### Foreach Context Variables

Within each iteration:
- `{{.item}}` - Current array element
- `{{.index}}` - Current index (0-based)
- `{{.total}}` - Total array length

### Foreach Results

Results are collected in order as `.results`:

```yaml
- id: reviews
  type: parallel
  foreach: "{{.inputs.documents}}"
  steps:
    - id: review
      type: llm
      prompt: "Review: {{.item}}"

- id: summary
  type: llm
  prompt: "Summarize all reviews: {{.steps.reviews.results}}"
```

Results maintain input array order regardless of completion order.

## Conditional Execution in Parallel

Nested steps can have conditions:

```yaml
- id: reviews
  type: parallel
  steps:
    - id: security
      type: llm
      condition:
        expression: '"security" in inputs.personas'
      prompt: "Security review..."

    - id: performance
      type: llm
      condition:
        expression: '"performance" in inputs.personas'
      prompt: "Performance review..."
```

Skipped steps don't block or affect other parallel steps.

## Token and Cost Aggregation

Token usage and costs from all nested steps are automatically aggregated on the parent parallel step. This provides accurate totals for:
- Input/output tokens
- Cache tokens
- Cost in USD

## Timeouts

Set a timeout for the entire parallel block:

```yaml
- id: time_limited
  type: parallel
  timeout: 60  # 60 seconds for all nested steps combined
  steps:
    - id: step1
    - id: step2
```

When timeout expires:
- Running steps receive cancellation signal
- Partial results from completed steps are returned

## Limits and Constraints

For system stability:
- **Array size**: Foreach arrays limited to 10,000 items
- **Nesting depth**: Parallel blocks can nest up to 3 levels deep
- **Concurrency**: `max_concurrency` must be between 1-100

## Real-World Example

From `examples/showcase/code-review.yaml`:

```yaml
- id: reviews
  name: Parallel Reviews
  type: parallel
  max_concurrency: 3
  steps:
    - id: security_review
      name: Security Review
      type: llm
      model: strategic
      condition:
        expression: '"security" in inputs.personas'
      prompt: |
        Review these code changes for security issues.
        Changes: {{.steps.get_diff.stdout}}

    - id: performance_review
      name: Performance Review
      type: llm
      model: balanced
      condition:
        expression: '"performance" in inputs.personas'
      prompt: |
        Review for performance issues.
        Changes: {{.steps.get_diff.stdout}}

- id: generate_report
  type: llm
  prompt: |
    Generate a code review report from:
    Security: {{.steps.reviews.security_review.response}}
    Performance: {{.steps.reviews.performance_review.response}}
```

This workflow:
1. Runs security and performance reviews in parallel
2. Limits to 3 concurrent LLM calls
3. Conditionally skips reviews based on input
4. Combines all reviews into a final report
