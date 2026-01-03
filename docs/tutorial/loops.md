# Part 3: Refinement Loops

Iterate until a critic approves the output.

## The Workflow

Create `examples/tutorial/03-loops.yaml`:

<!-- include: examples/tutorial/03-loops.yaml -->

## Try It

```bash
conductor run examples/tutorial/03-loops.yaml
```

Or for a longer plan:
```bash
conductor run examples/tutorial/03-loops.yaml -i days=7
```

Watch the iterations:
```
[1/2] draft... OK
[2/2] refine (iteration 1)...
  - critique... OK
  - improve... OK
[2/2] refine (iteration 2)...
  - critique... APPROVED
```

## Key Concepts

- **`type: loop`** — Repeats steps until a condition is met
- **`until`** — Expression that stops the loop (e.g., `steps.critique.response contains "APPROVED"`)
- **`max_iterations`** — Safety limit to prevent infinite loops
- **`when`** — Conditional step execution based on expressions

See [Flow Control](../building-workflows/flow-control/) for more on loops and conditions.

## What's Next

Set up the workflow to run automatically every week.

[Part 4: Scheduled Triggers →](triggers)
