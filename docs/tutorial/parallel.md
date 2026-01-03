# Part 2: Parallel Execution

Generate breakfast, lunch, and dinner suggestions simultaneously.

## The Workflow

Create `examples/tutorial/02-parallel.yaml`:

<!-- include: examples/tutorial/02-parallel.yaml -->

## Try It

```bash
conductor run examples/tutorial/02-parallel.yaml
```

Or with a specific style:
```bash
conductor run examples/tutorial/02-parallel.yaml -i style=mediterranean
```

Watch the output—all three meal suggestions complete around the same time instead of sequentially.

## Key Concepts

- **`type: parallel`** — Runs nested steps concurrently
- **Nested references** — `{{.steps.meals.breakfast.response}}` accesses parallel step output
- **Model tiers** — Use `fast` for simple parallel work, `balanced` for synthesis

See [Flow Control](../building-workflows/flow-control/) for more on parallel execution.

## What's Next

Add a refinement loop that critiques and improves the plan until it meets quality standards.

[Part 3: Refinement Loops →](loops)
