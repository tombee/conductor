# Part 1: Your First Workflow

Create a workflow that takes input, calls an LLM, and returns output.

## The Workflow

Create `examples/tutorial/01-first-workflow.yaml`:

<!-- include: examples/tutorial/01-first-workflow.yaml -->

## Try It

```bash
conductor run examples/tutorial/01-first-workflow.yaml -i minutes=15
```

Output:
```
Running: quick-recipe
[1/1] suggest... OK

**15-Minute Garlic Butter Shrimp**
...
```

Try different values:
```bash
conductor run examples/tutorial/01-first-workflow.yaml -i minutes=5
conductor run examples/tutorial/01-first-workflow.yaml -i minutes=45
```

## Key Concepts

- **Inputs** — Data users provide when running (`-i minutes=15`)
- **Steps** — Work to perform; `type: llm` calls an AI model
- **Templates** — `{{.minutes}}` inserts the input value into the prompt
- **Outputs** — Values the workflow returns; `{{.steps.suggest.response}}` references step output

See [Workflow Schema](/reference/workflow-schema/) for full syntax reference.

## What's Next

Generate breakfast, lunch, and dinner at the same time using parallel execution.

[Part 2: Parallel Execution →](parallel)
