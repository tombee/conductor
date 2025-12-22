# Part 6: HTTP Output

This section covers delivering workflow results via HTTP.

## Workflow

`examples/tutorial/06-complete.yaml`:

<!-- include: examples/tutorial/06-complete.yaml -->

## Execution

```bash
# Local file output only
conductor run examples/tutorial/06-complete.yaml

# With webhook delivery
conductor run examples/tutorial/06-complete.yaml \
  -i webhook_url="https://hooks.slack.com/services/..."
```

## Concepts

| Element | Description |
|---------|-------------|
| `when` | Conditional execution based on expression |
| `http.request` | HTTP POST/GET to external endpoint |

Reference: [HTTP Action](../reference/actions/http/)

[Next: Deployment →](deploy)
