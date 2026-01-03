# Flow Control

Control how steps execute in your workflows.

## Sequential Execution

By default, steps run in order:

```conductor
steps:
  - id: step1
    model: fast
    prompt: "Generate topic for: {{.inputs.subject}}"

  - id: step2
    model: balanced
    prompt: "Write outline for: {{.steps.step1.response}}"

  - id: step3
    model: balanced
    prompt: "Write full post from: {{.steps.step2.response}}"
```

## Parallel Execution

Run independent steps concurrently:

```conductor
steps:
  - id: fetch_data
    file.read: "{{.inputs.data_file}}"

  - id: parallel_analysis
    type: parallel
    max_concurrency: 4
    steps:
      - id: sentiment
        model: fast
        prompt: "Analyze sentiment: {{.steps.fetch_data.content}}"

      - id: keywords
        model: fast
        prompt: "Extract keywords: {{.steps.fetch_data.content}}"

      - id: summary
        model: balanced
        prompt: "Summarize: {{.steps.fetch_data.content}}"
```

Access parallel results:

```conductor
  - id: consolidate
    model: balanced
    prompt: |
      Combine results:
      Sentiment: {{.steps.parallel_analysis.sentiment.response}}
      Keywords: {{.steps.parallel_analysis.keywords.response}}
      Summary: {{.steps.parallel_analysis.summary.response}}
```

## Conditional Execution

Run steps only when conditions are met:

```conductor
  - id: security_scan
    condition: 'inputs.include_security == true'
    model: strategic
    prompt: "Security analysis: {{.inputs.code}}"

  - id: deep_analysis
    condition: 'steps.classify.response contains "complex"'
    model: strategic
    prompt: "Deep analysis: {{.inputs.code}}"
```

### Decision Routing

Route to different steps based on classification:

```conductor
  - id: classify
    model: fast
    prompt: |
      Classify ticket: {{.inputs.ticket}}
      Output ONLY: bug, feature, or question

  - id: handle_bug
    condition: 'steps.classify.response contains "bug"'
    model: balanced
    prompt: "Bug triage: {{.inputs.ticket}}"

  - id: handle_feature
    condition: 'steps.classify.response contains "feature"'
    model: fast
    prompt: "Feature analysis: {{.inputs.ticket}}"
```

## Iterative Loops

Execute steps repeatedly until a condition is met or a maximum iteration count is reached. Loops use do-while semantics: execute at least once, then check the condition.

### Basic Loop

```conductor
steps:
  - id: refine_code
    type: loop
    max_iterations: 5
    until: 'steps.review.response contains "approved"'
    steps:
      - id: improve
        model: balanced
        prompt: |
          Improve this code based on feedback:
          Code: {{.steps.refine_code.step_outputs.improve.response}}
          Feedback: {{.steps.refine_code.step_outputs.review.response}}

      - id: review
        model: strategic
        prompt: |
          Review this code. Output "approved" if ready, otherwise provide feedback.
          Code: {{.steps.improve.response}}
```

### Loop Context Variables

Inside a loop, you have access to:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.loop.iteration}}` | Current iteration (0-indexed) | `0`, `1`, `2`, ... |
| `{{.loop.max_iterations}}` | Maximum iterations | `5` |
| `{{.loop.history}}` | Array of previous iteration results | See below |

```conductor
  - id: iterative_task
    type: loop
    max_iterations: 3
    until: 'steps.check.response contains "done"'
    steps:
      - id: process
        model: balanced
        prompt: |
          Iteration {{.loop.iteration}} of {{.loop.max_iterations}}
          Previous attempts: {{.loop.history | len}}

      - id: check
        model: fast
        prompt: "Is the task complete? Output: done or continue"
```

### Skip Steps on First Iteration

Use conditions to skip steps on specific iterations:

```conductor
  - id: refinement_loop
    type: loop
    max_iterations: 3
    until: 'steps.review.response contains "approved"'
    steps:
      # Skip apply_feedback on first iteration (no feedback yet)
      - id: apply_feedback
        condition: 'loop.iteration > 0'
        model: balanced
        prompt: |
          Apply this feedback to the code:
          {{.steps.review.response}}

      - id: review
        model: strategic
        prompt: |
          Review the code. If good, output "approved".
          Otherwise provide specific feedback.
```

### Accessing Loop History

The `loop.history` array contains results from all previous iterations:

```conductor
  - id: iterative_review
    type: loop
    max_iterations: 5
    until: 'steps.decision.response == "final"'
    steps:
      - id: analyze
        model: balanced
        prompt: |
          Analyze the current state.
          Previous analyses:
          {{range .loop.history}}
          - Iteration {{.iteration}}: {{index .steps "analyze" "response"}}
          {{end}}

      - id: decision
        model: fast
        prompt: "Based on all history, is this final? Output: final or continue"
```

### Loop Output Structure

When a loop completes, results are accessible at:

| Field | Description |
|-------|-------------|
| `.step_outputs` | Final iteration's step results |
| `.iteration_count` | Total iterations executed |
| `.terminated_by` | `"condition"`, `"max_iterations"`, `"timeout"`, or `"error"` |
| `.history` | Array of all iteration records |

```conductor
  - id: summarize
    model: balanced
    prompt: |
      Loop completed after {{.steps.my_loop.iteration_count}} iterations.
      Termination reason: {{.steps.my_loop.terminated_by}}
      Final result: {{.steps.my_loop.step_outputs.final_step.response}}
```

### Error Handling in Loops

Control what happens when a step fails:

```conductor
  - id: resilient_loop
    type: loop
    max_iterations: 5
    until: 'steps.check.response == "done"'
    on_error:
      strategy: ignore  # Continue to next iteration on error
    steps:
      - id: risky_step
        model: balanced
        prompt: "Attempt risky operation"
        on_error:
          strategy: ignore  # Continue to next step on error

      - id: check
        model: fast
        prompt: "Check status"
```

**Error strategies:**

| Strategy | Behavior |
|----------|----------|
| `fail` (default) | Stop loop immediately on first failure |
| `ignore` | Continue to next step/iteration, record error in history |

### Loop Timeout

Set a timeout for the entire loop:

```conductor
  - id: time_limited_loop
    type: loop
    max_iterations: 100
    timeout: 60  # Maximum 60 seconds total
    until: 'steps.check.response == "done"'
    steps:
      - id: process
        model: fast
        prompt: "Quick processing step"

      - id: check
        model: fast
        prompt: "Check if done"
```

### Loop vs Foreach

| Feature | Loop | Foreach |
|---------|------|---------|
| Use case | Iterative refinement until condition | Process each item in array |
| Iterations | Unknown, condition-based | Known, one per array item |
| Execution | Sequential iterations | Parallel by default |
| Context | `loop.iteration`, `loop.history` | `item`, `index`, `total` |
| Termination | Condition, max_iterations, timeout | All items processed |

**When to use loop:**
- Iterative code review/refinement
- Retry with learning from failures
- Convergence algorithms
- Chat-like multi-turn conversations

**When to use foreach:**
- Process list of files
- Analyze multiple PRs
- Batch API calls

### Example: Iterative Code Review

```conductor
name: iterative-code-review
description: Review and refine code until approved

inputs:
  - name: code
    type: string

steps:
  - id: code_review_loop
    type: loop
    max_iterations: 3
    until: 'steps.review.response contains "APPROVED"'
    steps:
      # Skip on first iteration - no feedback yet
      - id: apply_changes
        condition: 'loop.iteration > 0'
        model: balanced
        prompt: |
          Apply the following changes to improve the code:

          Original code:
          {{.inputs.code}}

          Feedback from review:
          {{.steps.review.response}}

          Output ONLY the improved code.

      - id: review
        model: strategic
        prompt: |
          Review this code for quality, security, and best practices.

          Code to review:
          {{if eq .loop.iteration 0}}
          {{.inputs.code}}
          {{else}}
          {{.steps.apply_changes.response}}
          {{end}}

          Previous feedback history:
          {{range .loop.history}}
          Round {{.iteration}}: {{index .steps "review" "response" | truncate 200}}
          {{end}}

          If the code is production-ready, output "APPROVED".
          Otherwise, provide specific actionable feedback.

outputs:
  - name: final_code
    value: "{{.steps.code_review_loop.step_outputs.apply_changes.response}}"
  - name: iterations
    value: "{{.steps.code_review_loop.iteration_count}}"
```

## Workflow Composition

Break complex workflows into reusable components:

```conductor
# analyze-sentiment.yaml
name: analyze-sentiment
inputs:
  - name: text
    type: string
steps:
  - id: sentiment
    model: fast
    prompt: "Sentiment analysis: {{.inputs.text}}"
outputs:
  - name: result
    value: "{{.steps.sentiment.response}}"
```

Call from another workflow:

```conductor
# main.yaml
steps:
  - id: analyze
    workflow.run:
      workflow: "analyze-sentiment.yaml"
      inputs:
        text: "{{.inputs.feedback}}"

  - id: respond
    model: balanced
    prompt: "Based on sentiment {{.steps.analyze.result}}, generate response."
```

## Common Patterns

### Read-Process-Write

```conductor
steps:
  - id: read
    file.read: "{{.inputs.file}}"

  - id: process
    model: balanced
    prompt: "Process: {{.steps.read.content}}"

  - id: write
    file.write:
      path: "output.txt"
      content: "{{.steps.process.response}}"
```

### Early Termination

Validate before expensive operations:

```conductor
  - id: validate
    model: fast
    prompt: "Is this valid code? Output: VALID or INVALID"

  - id: expensive_review
    condition: 'steps.validate.response contains "VALID"'
    model: strategic
    prompt: "Full code review: {{.inputs.code}}"
```

## Iteration with Foreach

Process each item in an array with `foreach`. Each iteration has access to the current item, index, and total count.

### Basic Iteration

```conductor
steps:
  - id: get_issues
    github.list_issues:
      repo: "{{.inputs.repo}}"
      state: open

  - id: process_issues
    type: parallel
    foreach: '{{.steps.get_issues.result}}'
    steps:
      - id: analyze
        type: llm
        model: fast
        prompt: |
          Analyze issue #{{.item.number}}: {{.item.title}}
          Body: {{.item.body}}
```

### Context Variables

Inside a foreach block, you have access to:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.item}}` | Current array element | `{{.item.title}}` |
| `{{.index}}` | Zero-based index | `0`, `1`, `2`, ... |
| `{{.total}}` | Total number of elements | `5` |

```conductor
  - id: review_files
    type: parallel
    foreach: '{{.steps.get_files.result}}'
    steps:
      - id: review
        type: llm
        prompt: |
          Reviewing file {{.index | add 1}} of {{.total}}: {{.item.path}}

          Content:
          {{.item.content}}
```

### Controlling Concurrency

Limit parallel iterations with `max_concurrency`:

```conductor
  - id: process_all
    type: parallel
    foreach: '{{.steps.items.result}}'
    max_concurrency: 3  # Process 3 items at a time
    steps:
      - id: process
        type: llm
        prompt: "Process: {{.item}}"
```

Default: `3` concurrent iterations. Use lower values for rate-limited APIs or memory-intensive operations.

### Accessing Nested Data

Access nested fields within array elements:

```conductor
  - id: process_users
    type: parallel
    foreach: '{{.steps.get_users.result}}'
    steps:
      - id: greet
        type: llm
        prompt: |
          Create a personalized message for {{.item.profile.name}}
          who works at {{.item.profile.company.name}}
          in the {{.item.profile.company.department}} department.
```

### Collecting Results

Foreach results are available as an array, indexed by iteration:

```conductor
  - id: analyze_all
    type: parallel
    foreach: '{{.steps.items.result}}'
    steps:
      - id: analyze
        type: llm
        prompt: "Analyze: {{.item}}"

  - id: summarize
    type: llm
    prompt: |
      Summarize these analyses:
      {{range $i, $result := .steps.analyze_all.results}}
      - Item {{$i}}: {{$result.analyze.response}}
      {{end}}
```

### Error Handling

By default, if any iteration fails, the entire foreach fails. Control this with error strategies:

```conductor
  - id: process_all
    type: parallel
    foreach: '{{.steps.items.result}}'
    error_strategy: ignore  # Continue even if some iterations fail
    steps:
      - id: process
        type: llm
        prompt: "Process: {{.item}}"
```

**Error strategies:**

| Strategy | Behavior |
|----------|----------|
| `fail` (default) | Stop immediately on first failure |
| `ignore` | Continue processing, collect partial results |

### Edge Cases

**Empty arrays:**
```conductor
  - id: process
    type: parallel
    foreach: '{{.steps.items.result}}'  # Empty array = 0 iterations
    steps:
      - id: step
        type: llm
        prompt: "..."
# Completes immediately with empty results
```

**Null items:**
```conductor
  # If .items contains [1, null, 3]:
  - id: process
    type: parallel
    foreach: '{{.steps.items.result}}'
    steps:
      - id: step
        type: llm
        prompt: "Value: {{.item}}"  # .item may be null
```

**Result ordering:**

Results maintain original array order (by index), not completion order. Even if item 3 finishes before item 1, results are returned in index order.

### Performance Considerations

!!! tip "Memory usage with large arrays"
    For arrays with >1000 items, consider:

    - Batching items first with `transform.split`
    - Using lower `max_concurrency`
    - Processing in chunks across multiple workflow runs

!!! note "Timeouts"
    Foreach respects step-level timeouts. The timeout applies to the entire foreach operation, not per iteration. For long-running iterations, set an appropriate timeout at the parallel step level.

### Using with Transform

Prepare arrays for foreach with transform operations:

```conductor
  # Split a string into lines
  - id: split_lines
    transform.extract:
      data: '{{.steps.read_file.content}}'
      expr: 'split("\n") | map(select(length > 0))'

  # Process each line
  - id: process_lines
    type: parallel
    foreach: '{{.steps.split_lines.result}}'
    steps:
      - id: analyze
        type: llm
        prompt: "Analyze line: {{.item}}"
```

## See Also

- [Workflows and Steps](../learn/concepts/workflows-steps.md) - Step fundamentals
- [Error Handling](error-handling.md) - Retries and fallbacks
- [Transform Action](../reference/actions/transform.md) - Prepare data for iteration
- [Expression Reference](../reference/expressions.md) - Condition expressions
