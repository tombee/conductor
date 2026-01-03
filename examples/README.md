# Examples

## Tutorial

Step-by-step workflows from the [documentation tutorial](../docs/tutorial/):

| File | Description |
|------|-------------|
| `tutorial/01-first-workflow.yaml` | Basic LLM step with inputs and outputs |
| `tutorial/02-parallel.yaml` | Concurrent step execution |
| `tutorial/03-loops.yaml` | Iterative refinement with exit conditions |
| `tutorial/04-triggers.yaml` | Workflow for scheduled execution |
| `tutorial/05-input.yaml` | Reading external files |
| `tutorial/06-complete.yaml` | Full workflow with HTTP delivery |

## Production Examples

| Directory | Description |
|-----------|-------------|
| `code-review/` | Multi-persona code review (security, performance, style) |
| `iac-review/` | Infrastructure as Code risk assessment |
| `issue-triage/` | GitHub issue classification and routing |
| `security-audit/` | Vulnerability scanning and compliance |
| `slack-integration/` | Formatted Slack notifications |
| `write-song/` | Creative writing with musical structure |

## Usage

```bash
conductor run examples/tutorial/01-first-workflow.yaml -i minutes=15
conductor run examples/code-review/workflow.yaml
```

See [documentation](../docs/) for detailed usage.
