# Part 5: File Input

This section covers reading external files into workflows.

## Workflow

`examples/tutorial/05-input.yaml`:

<!-- include: examples/tutorial/05-input.yaml -->

## Data File

`examples/tutorial/pantry.txt`:

<!-- include: examples/tutorial/pantry.txt -->

## Execution

```bash
conductor run examples/tutorial/05-input.yaml
conductor run examples/tutorial/05-input.yaml -i pantry_file=custom.txt
```

## Concepts

| Element | Description |
|---------|-------------|
| `file.read` | Reads file contents into step response |
| `{{.steps.id.response}}` | File contents available in templates |

Reference: [File Action](../reference/actions/file/)

[Next: HTTP Output →](output)
