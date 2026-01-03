# Workflow Patterns

Common workflow patterns you can copy and adapt for your use cases.

Each pattern includes a complete, working code snippet ready to use in your workflows.

## Sequential Processing

Process data through multiple stages, where each step depends on the previous one.

### Pattern

```conductor
name: sequential-processing
description: Multi-stage content generation

inputs:
  - name: topic
    type: string
    required: true

steps:
  - id: brainstorm
    type: llm
    model: fast
    prompt: "Generate 5 key points about: {{.inputs.topic}}"

  - id: outline
    type: llm
    model: balanced
    prompt: |
      Create a structured outline using these points:
      {{$.brainstorm.response}}

  - id: write
    type: llm
    model: strategic
    prompt: |
      Write a comprehensive article following this outline:
      {{$.outline.response}}
```

### Use Cases

- Content generation pipelines
- Multi-stage data transformation
- Progressive refinement (draft → edit → polish)
- Analysis workflows (extract → classify → summarize)

### Customization

Adjust model tiers per stage complexity:
- **Draft stages**: Use `fast` tier for speed
- **Refinement stages**: Use `balanced` for quality
- **Final output**: Use `strategic` for best results

## Parallel Execution

Run independent tasks concurrently to reduce total execution time.

### Pattern

```conductor
name: parallel-reviews
description: Multi-perspective code review

inputs:
  - name: code_diff
    type: string
    required: true

steps:
  - id: reviews
    type: parallel
    max_concurrency: 3
    steps:
      - id: security
        type: llm
        model: strategic
        system: "You are a security engineer. Review for vulnerabilities."
        prompt: "Review this code:\n{{$.inputs.code_diff}}"

      - id: performance
        type: llm
        model: balanced
        system: "You are a performance engineer. Review for efficiency issues."
        prompt: "Review this code:\n{{$.inputs.code_diff}}"

      - id: style
        type: llm
        model: fast
        system: "You are a code reviewer. Focus on readability and conventions."
        prompt: "Review this code:\n{{$.inputs.code_diff}}"

  - id: consolidate
    type: llm
    model: balanced
    prompt: |
      Combine these reviews into a prioritized report:

      Security: {{$.reviews.security.response}}
      Performance: {{$.reviews.performance.response}}
      Style: {{$.reviews.style.response}}
```

### Use Cases

- Multi-perspective analysis (code review, content analysis)
- Independent API calls
- Batch processing of unrelated items
- Fan-out operations (one input → many analyses)

### Customization

Control concurrency for rate limits:
```conductor
max_concurrency: 2  # Limit concurrent LLM calls
```

Add conditional branches within parallel steps:
```conductor
- id: optional_check
  condition:
    expression: '"advanced" in inputs.mode'
  type: llm
```

## Conditional Branching

Execute different logic based on data or classification.

### Pattern

```conductor
name: conditional-routing
description: Route support tickets by type

inputs:
  - name: ticket_text
    type: string
    required: true

steps:
  - id: classify
    type: llm
    model: fast
    prompt: |
      Classify this support ticket as: BUG, FEATURE, or QUESTION
      Ticket: {{.inputs.ticket_text}}
      Output ONLY the classification word.

  - id: handle_bug
    type: llm
    model: strategic
    condition:
      expression: '"BUG" in steps.classify.response'
    prompt: |
      Triage this bug report and suggest priority:
      {{.inputs.ticket_text}}

  - id: handle_feature
    type: llm
    model: balanced
    condition:
      expression: '"FEATURE" in steps.classify.response'
    prompt: |
      Analyze this feature request and categorize:
      {{.inputs.ticket_text}}

  - id: handle_question
    type: llm
    model: fast
    condition:
      expression: '"QUESTION" in steps.classify.response'
    prompt: |
      Draft a helpful response to this question:
      {{.inputs.ticket_text}}

  - id: assign_labels
    type: action
    action: github.add_labels
    inputs:
      labels: |
        {{if contains .steps.classify.response "BUG"}}["bug", "needs-triage"]
        {{else if contains .steps.classify.response "FEATURE"}}["enhancement"]
        {{else}}["question", "support"]{{end}}
```

### Use Cases

- Ticket triage and routing
- Content moderation (safe/unsafe)
- Complexity-based processing (simple → fast, complex → strategic)
- Multi-path workflows

### Customization

Use multiple conditions:
```conductor
condition:
  expression: '"urgent" in inputs.priority && "bug" in steps.classify.response'
```

Chain classifications for multi-level routing:
```conductor
- id: severity_classify
  condition:
    expression: '"BUG" in steps.classify.response'
  prompt: "Rate severity: LOW, MEDIUM, HIGH, or CRITICAL"
```

## Error Recovery

Handle failures gracefully with retry logic and fallbacks.

### Pattern

```conductor
name: error-recovery
description: Resilient API integration with fallbacks

inputs:
  - name: query
    type: string
    required: true

steps:
  - id: primary_api
    type: action
    action: http
    inputs:
      url: "https://api.example.com/search"
      method: "POST"
      body:
        query: "{{.inputs.query}}"
    retry:
      max_attempts: 3
      backoff_base: 2
      backoff_multiplier: 2.0
    on_error: continue

  - id: fallback_api
    type: action
    action: http
    condition:
      expression: 'steps.primary_api.error != nil'
    inputs:
      url: "https://backup-api.example.com/search"
      method: "POST"
      body:
        query: "{{.inputs.query}}"
    retry:
      max_attempts: 2
    on_error: continue

  - id: local_fallback
    type: llm
    model: strategic
    condition:
      expression: 'steps.primary_api.error != nil && steps.fallback_api.error != nil'
    prompt: |
      Both APIs failed. Provide a best-effort answer for:
      {{.inputs.query}}

  - id: format_result
    type: llm
    model: fast
    prompt: |
      Format this result for the user:
      {{if .steps.primary_api.response}}
        {{.steps.primary_api.response}}
      {{else if .steps.fallback_api.response}}
        {{.steps.fallback_api.response}}
      {{else}}
        {{.steps.local_fallback.response}}
      {{end}}
```

### Use Cases

- External API integration
- Network-dependent operations
- Database queries with failover
- Critical workflows requiring high availability

### Customization

Adjust retry strategies:
```conductor
retry:
  max_attempts: 5
  backoff_base: 1      # Start with 1 second
  backoff_multiplier: 1.5  # Increase by 1.5x each retry
  max_backoff: 60      # Cap at 60 seconds
```

Add circuit breaker pattern:
```conductor
- id: check_health
  type: action
  action: http
  inputs:
    url: "https://api.example.com/health"

- id: call_api
  condition:
    expression: 'steps.check_health.status_code == 200'
  type: action
  action: http
```

## Output Aggregation

Combine multiple outputs into a single structured result.

### Pattern

```conductor
name: output-aggregation
description: Analyze multiple files and create summary report

inputs:
  - name: file_paths
    type: array
    required: true

steps:
  - id: read_files
    type: foreach
    items: "{{.inputs.file_paths}}"
    steps:
      - id: read
        type: action
        action: file.read
        inputs:
          path: "{{.item}}"

      - id: analyze
        type: llm
        model: balanced
        prompt: |
          Analyze this file and extract:
          1. Purpose (1 sentence)
          2. Key functions (bullet list)
          3. Dependencies (list)

          File: {{.item}}
          Content: {{.steps.read.content}}

  - id: aggregate
    type: llm
    model: strategic
    prompt: |
      Create a comprehensive summary from these file analyses:

      {{range $index, $result := .steps.read_files.results}}
      File {{add $index 1}}: {{$result.analyze.response}}
      {{end}}

      Structure your summary as:
      # Project Overview
      ## Architecture
      ## Main Components
      ## Dependencies

  - id: write_report
    type: action
    action: file.write
    inputs:
      path: "project-summary.md"
      content: "{{.steps.aggregate.response}}"

outputs:
  - name: summary
    type: string
    value: "{{.steps.aggregate.response}}"
    description: "Project summary report"
```

### Use Cases

- Multi-file analysis (codebase review)
- Batch data processing with summary
- Report generation from multiple sources
- Log aggregation and analysis

### Customization

Filter results before aggregation:
```conductor
- id: filter
  type: llm
  model: fast
  prompt: |
    From these results, keep only items marked as HIGH priority:
    {{range .steps.batch_process.results}}
    - {{.analysis.response}}
    {{end}}
```

Use structured output for easier aggregation:
```conductor
- id: analyze
  type: llm
  model: balanced
  response_format:
    type: json_schema
    schema:
      type: object
      properties:
        purpose: { type: string }
        functions: { type: array, items: { type: string } }
```

## Map-Reduce

Process items in parallel, then combine results.

### Pattern

```conductor
name: map-reduce
description: Analyze pull request files in parallel, then consolidate

inputs:
  - name: pr_files
    type: array
    required: true

steps:
  # Map phase: Process each file
  - id: analyze_files
    type: parallel
    max_concurrency: 5
    items: "{{.inputs.pr_files}}"
    steps:
      - id: get_content
        type: action
        action: github.get_file
        inputs:
          path: "{{.item}}"

      - id: review
        type: llm
        model: balanced
        prompt: |
          Review this file for issues:
          File: {{.item}}
          {{.steps.get_content.content}}

          Output JSON with: severity, issues (array), suggestions (array)
        response_format:
          type: json_schema

  # Reduce phase: Consolidate all reviews
  - id: consolidate
    type: llm
    model: strategic
    prompt: |
      Create a prioritized review report from these file reviews:

      {{range $index, $file := .inputs.pr_files}}
      File: {{$file}}
      {{index $.steps.analyze_files.results $index .review.response}}
      {{end}}

      Group by severity and provide actionable recommendations.

outputs:
  - name: review
    type: string
    value: "{{.steps.consolidate.response}}"
```

### Use Cases

- Code review (per-file analysis → overall report)
- Log analysis (per-server logs → incident summary)
- Content moderation (per-item check → batch decision)
- Test result aggregation

### Customization

Add filtering between map and reduce:
```conductor
- id: filter_critical
  type: action
  action: transform.filter
  inputs:
    items: "{{.steps.analyze_files.results}}"
    condition: 'item.severity == "CRITICAL"'
```

Process in batches to manage rate limits:
```conductor
- id: batch_process
  type: foreach
  items: "{{chunk .inputs.pr_files 10}}"  # Process 10 at a time
  steps:
    - id: process_batch
      type: parallel
      max_concurrency: 5
```

## Related Resources

- [Flow Control](flow-control.md) - Detailed control flow documentation
- [Error Handling](error-handling.md) - Error strategies and retry configuration
- [Performance](performance.md) - Optimization techniques
- [Examples](../examples/index.md) - Real-world workflow examples
