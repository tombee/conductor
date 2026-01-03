# Part 5: Reading External Data

Use file contents in your prompts.

## The Workflow

Create `examples/tutorial/05-input.yaml`:

<!-- include: examples/tutorial/05-input.yaml -->

## The Pantry File

Create `examples/tutorial/pantry.txt`:

<!-- include: examples/tutorial/pantry.txt -->

## Try It

```bash
conductor run examples/tutorial/05-input.yaml
```

The generated meal plan references ingredients from your pantry file.

Update the pantry and re-run:
```bash
echo "- Eggs (dozen)" >> examples/tutorial/pantry.txt
conductor run examples/tutorial/05-input.yaml
```

Or use a different file:
```bash
conductor run examples/tutorial/05-input.yaml -i pantry_file=my-pantry.txt
```

## Key Concepts

- **`file.read`** — Reads file contents into `.steps.<id>.response`
- **Dynamic paths** — Use templates in file paths: `{{.pantry_file}}`
- **Context injection** — Include file contents in prompts for context-aware generation

See [File Action](/reference/actions/file/) for more file operations.

## What's Next

Deliver the meal plan to Slack or other services.

[Part 6: Delivering Results →](output)
