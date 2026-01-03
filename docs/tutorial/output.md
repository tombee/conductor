# Part 6: Delivering Results

Send your meal plan somewhere useful.

## The Workflow

Create `examples/tutorial/06-complete.yaml`:

<!-- include: examples/tutorial/06-complete.yaml -->

## Try It

### Local Only (No Webhook)
```bash
conductor run examples/tutorial/06-complete.yaml
cat meal-plan.md
```

### With Slack Webhook
```bash
conductor run examples/tutorial/06-complete.yaml \
  -i webhook_url="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
```

Works with any HTTP webhook: Slack, Make.com, Zapier, n8n, or custom APIs.

## Key Concepts

- **`when`** — Conditional execution: `when: 'webhook_url != ""'`
- **`http.request`** — Send data to HTTP endpoints
- **Always save locally** — Keep a backup even when delivering externally

See [HTTP Action](../reference/actions/http/) for more HTTP operations.

## What's Next

Deploy to a server for fully automated weekly meal planning.

[Part 7: Deploy to Production →](deploy)
