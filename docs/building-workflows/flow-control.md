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
