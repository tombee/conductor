# Part 3: Refinement Loops

This section covers iterative step execution with exit conditions.

## Workflow

`examples/tutorial/03-loops.yaml`:

<!-- include: examples/tutorial/03-loops.yaml -->

## Execution

```bash
conductor run examples/tutorial/03-loops.yaml -i days=7
```

Output:
```
[1/2] draft... OK
[2/2] refine (iteration 1)...
  - critique... OK
  - improve... OK
[2/2] refine (iteration 2)...
  - critique... APPROVED
```

## Concepts

| Element | Description |
|---------|-------------|
| `type: loop` | Repeats steps until condition is met |
| `until` | Expression evaluated after each iteration |
| `max_iterations` | Upper bound on iterations |
| `when` | Conditional step execution |

Reference: [Flow Control](../building-workflows/flow-control/)

[Next: Scheduled Triggers →](triggers)
