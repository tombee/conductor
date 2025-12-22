# Part 2: Parallel Execution

This section covers concurrent step execution.

## Workflow

`examples/tutorial/02-parallel.yaml`:

<!-- include: examples/tutorial/02-parallel.yaml -->

## Execution

```bash
conductor run examples/tutorial/02-parallel.yaml -i style=mediterranean
```

## Concepts

| Element | Description |
|---------|-------------|
| `type: parallel` | Executes nested steps concurrently |
| `{{.steps.parent.child.response}}` | References nested step output |
| `model: fast` | Lower-cost model tier for simple tasks |

Reference: [Flow Control](../building-workflows/flow-control/)

[Next: Refinement Loops →](loops)
