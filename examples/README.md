# Examples

## Tutorial

Progressive meal planning workflows from the [documentation tutorial](../docs/tutorial/):

| File | Description |
|------|-------------|
| `tutorial/01-first-recipe.yaml` | Single LLM step generating a recipe |
| `tutorial/02-better-recipe.yaml` | Accept ingredients, output structured data |
| `tutorial/03-meal-plan.yaml` | Three recipes generated in parallel |
| `tutorial/04-pantry-check.yaml` | Read pantry file, use in prompt |
| `tutorial/05-weekly-plan.yaml` | Loop through days with conditions |
| `tutorial/06-save-to-notion.yaml` | Save meal plan to Notion |
| `tutorial/07-deploy.yaml` | Deploy with cron trigger |

## Showcase

Complete real-world workflows demonstrating best practices:

| File | Description |
|------|-------------|
| `showcase/code-review.yaml` | Multi-persona code review |
| `showcase/conditional-workflow.yaml` | Conditional execution patterns with `if` field |
| `showcase/issue-triage.yaml` | GitHub issue classification |
| `showcase/slack-notify.yaml` | Formatted Slack notifications |

## Usage

```bash
# Run a tutorial example
conductor run examples/tutorial/01-first-recipe.yaml

# Run a showcase example
conductor run examples/showcase/code-review.yaml
```

See the [tutorial documentation](../docs/tutorial/) for detailed explanations of each step.
