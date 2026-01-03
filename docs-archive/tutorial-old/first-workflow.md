# Part 1: First Workflow

This section covers workflow inputs, LLM steps, template variables, and outputs.

## Workflow

`examples/tutorial/01-first-workflow.yaml`:

<!-- include: examples/tutorial/01-first-workflow.yaml -->

## Execution

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

## Concepts

| Element | Description |
|---------|-------------|
| `inputs` | Parameters passed via `-i name=value` |
| `steps` | Sequential operations; `type: llm` invokes a model |
| `{{.name}}` | Template variable referencing input or step output |

Reference: [Workflow Schema](../reference/workflow-schema/)

[Next: Parallel Execution →](parallel)
