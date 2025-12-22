# Performance Optimization

Strategies for optimizing workflow execution time, resource usage, and cost.

## Measuring Performance

```bash
# Time execution
time conductor run workflow.yaml

# Get step timings
conductor run workflow.yaml --output json | jq '.steps | map({id: .id, duration: .duration})'
```

## Optimizing Speed

### Parallel Execution

Run independent steps concurrently:

```conductor
steps:
  - id: parallel_analysis
    type: parallel
    max_concurrency: 3
    steps:
      - id: security_review
        model: balanced
        prompt: "Security scan: {{.inputs.code}}"

      - id: style_review
        model: balanced
        prompt: "Style check: {{.inputs.code}}"

      - id: logic_review
        model: balanced
        prompt: "Logic review: {{.inputs.code}}"
```

### Model Selection

Use the fastest model that meets requirements:

| Tier | Speed | Use Case |
|------|-------|----------|
| fast | ~1-3s | Extraction, classification |
| balanced | ~3-8s | Analysis, summarization |
| strategic | ~10-30s | Complex reasoning |

### Reduce Context Size

Filter data before processing:

```conductor
  - id: extract_relevant
    model: fast
    prompt: "Extract authentication functions from: {{.inputs.code}}"

  - id: analyze
    model: balanced
    prompt: "Analyze: {{.steps.extract_relevant.response}}"
```

### Early Termination

Skip expensive steps when possible:

```conductor
  - id: quick_check
    model: fast
    prompt: "Is this input valid? {{.inputs.data}}"

  - id: deep_analysis
    condition: 'steps.quick_check.response contains "VALID"'
    model: strategic
    prompt: "Deep analysis: {{.inputs.data}}"
```

## Optimizing Cost

### Model Tier Selection

| Tier | Relative Cost |
|------|---------------|
| fast | 1x |
| balanced | 3x |
| strategic | 10x |

### Reduce Token Usage

Keep prompts concise:

```conductor
# Before: Verbose
prompt: "Please analyze this code carefully and identify any bugs..."

# After: Concise
prompt: "Find bugs in this code. Output JSON array of {line, issue, fix}."
```

### Caching

Cache expensive LLM results:

```conductor
  - id: check_cache
    file.read: "/tmp/cache/{{.inputs.hash}}.json"
    on_error: { strategy: ignore }

  - id: llm_if_needed
    condition: 'steps.check_cache.status == "failed"'
    model: balanced
    prompt: "Analyze: {{.inputs.code}}"
```

## Concurrency Limits

Control parallel execution:

```conductor
  - id: bulk_process
    type: parallel
    max_concurrency: 5
    steps: [...]
```

Guidelines:
- LLM calls: 3-10 concurrent
- HTTP requests: 5-20 concurrent
- File operations: 10-50 concurrent

## Checklist

- [ ] Independent steps run in parallel
- [ ] Fastest appropriate model tier used
- [ ] Prompts are concise
- [ ] Expensive operations cached
- [ ] Timeouts configured
- [ ] Early termination for invalid inputs

## See Also

- [Cost Management](cost-management.md) - Detailed cost strategies
- [Flow Control](flow-control.md) - Parallel and conditional execution
