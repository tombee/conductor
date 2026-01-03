# Parallel Execution

Steps without dependencies run concurrently for faster execution.

## Automatic Parallelization

Steps that don't reference each other run in parallel:

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

All three steps execute simultaneously.

## Sequential Dependencies

Steps that reference others wait for dependencies:

```yaml
steps:
  - id: fetch
    http:
      method: GET
      url: https://api.example.com/data
  - id: analyze
    llm:
      prompt: "Analyze: ${steps.fetch.output}"
```

`analyze` waits for `fetch` to complete.

## Mixed Parallel and Sequential

Combine parallel and sequential execution:

```yaml
steps:
  - id: read_file
    file:
      action: read
      path: data.txt
  - id: process1
    llm:
      prompt: "Summarize: ${steps.read_file.output}"
  - id: process2
    llm:
      prompt: "Extract key points: ${steps.read_file.output}"
  - id: process3
    llm:
      prompt: "Generate questions: ${steps.read_file.output}"
  - id: combine
    llm:
      prompt: |
        Combine these analyses:
        Summary: ${steps.process1.output}
        Key points: ${steps.process2.output}
        Questions: ${steps.process3.output}
```

Execution flow:
1. `read_file` runs first
2. `process1`, `process2`, `process3` run in parallel (all depend on `read_file`)
3. `combine` runs last (depends on all process steps)

## Performance

Parallel execution reduces total runtime. Three 10-second steps complete in ~10 seconds instead of ~30 seconds.

## Error Handling

If any parallel step fails, the workflow stops. Subsequent steps won't execute.

## Concurrency Limits

Conductor runs up to 10 steps in parallel by default. Configure with:

```yaml
name: my-workflow
config:
  maxConcurrency: 20
steps:
  # ... many parallel steps
```

## When to Use Parallel

Use parallel execution when:
- Steps are independent
- Operations take significant time (API calls, LLM generation)
- Order doesn't matter

Avoid parallelization when:
- Steps modify shared state (file writes)
- Order matters for correctness
- Rate limits prevent concurrent requests
