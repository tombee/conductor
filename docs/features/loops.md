# Loops

Conductor supports two loop patterns: `foreach` for iterating over lists, and `type: loop` for iterative refinement.

## Iterative Refinement

Use `type: loop` to repeatedly execute steps until a condition is met:

```yaml
steps:
  - id: refine
    type: loop
    max_iterations: 5
    until: "steps.validate.score >= 8"
    steps:
      - id: generate
        type: llm
        prompt: |
          {{if .loop.history}}
          Previous feedback: {{.loop.history.validate.feedback}}
          Improve based on this feedback.
          {{else}}
          Generate initial content.
          {{end}}

      - id: validate
        type: llm
        output_schema:
          type: object
          properties:
            score:
              type: number
            feedback:
              type: string
        prompt: "Rate this content 1-10 and provide feedback: {{.steps.generate.response}}"
```

### Loop Context

Within a loop, access:
- `{{.loop.iteration}}` - Current iteration (0-based)
- `{{.loop.history}}` - Previous iteration outputs
- `{{.loop.history.stepId.field}}` - Specific field from previous iteration

### Termination

Loops terminate when:
1. The `until` condition evaluates to true
2. `max_iterations` is reached (required, max 100)

## Foreach

Iterate over a list with `foreach`:

```yaml
steps:
  - id: greet_all
    foreach: "{{.inputs.names}}"
    steps:
      - id: greet
        type: llm
        prompt: "Say hello to {{.item}}"
```

Access the current item with `{{.item}}` and index with `{{.index}}`.

### Static Lists

```yaml
steps:
  - id: process
    foreach:
      items:
        - Alice
        - Bob
        - Carol
    steps:
      - id: greet
        type: llm
        prompt: "Greet {{.item}}"
```

### Dynamic Lists

Iterate over step outputs:

```yaml
steps:
  - id: fetch
    http.get: https://api.example.com/users

  - id: process
    foreach: "{{.steps.fetch.users}}"
    steps:
      - id: analyze
        type: llm
        prompt: "Analyze user: {{.item.name}}"
```

### Parallel Foreach

Run iterations concurrently:

```yaml
steps:
  - id: process
    foreach: "{{.inputs.items}}"
    parallel: true
    steps:
      - id: handle
        type: llm
        prompt: "Process {{.item}}"
```

For more control over parallel execution (concurrency limits, error handling), use `type: parallel` with `foreach`. See [Parallel Execution](parallel.md#parallel-foreach) for details.

### Loop Results

Access all outputs from a foreach:

```yaml
steps:
  - id: recipes
    foreach:
      items: [breakfast, lunch, dinner]
    steps:
      - id: generate
        type: llm
        prompt: "Generate a {{.item}} recipe"

  - id: combine
    type: llm
    prompt: "Create meal plan from: {{.steps.recipes.outputs}}"
```

`{{.steps.loopId.outputs}}` contains an array of all iteration outputs.
