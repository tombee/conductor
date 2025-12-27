# CI/CD Workflow Examples

This collection demonstrates Conductor workflows for CI/CD automation. Each example combines **deterministic tools** (APIs, git, scanners) with **LLM reasoning** to solve problems that can't be addressed with rules alone.

## Examples

| Example | Description | Key Capabilities |
|---------|-------------|------------------|
| [build-failure-triage](./build-failure-triage/) | Analyze CI failures, classify as transient or real | Log interpretation, root cause analysis |
| [release-notes](./release-notes/) | Generate release notes from commits and PRs | Change categorization, prose generation |
| [security-scan-interpreter](./security-scan-interpreter/) | Prioritize security findings with context | Risk assessment, false positive detection |
| [dependency-update-reviewer](./dependency-update-reviewer/) | Review dependency updates for breaking changes | Changelog analysis, risk assessment |
| [pr-size-gate](./pr-size-gate/) | Analyze PR size, suggest logical splits | Code structure analysis, split recommendations |
| [nightly-build-summary](./nightly-build-summary/) | Morning digest of overnight CI activity | Pattern detection, trend analysis |
| [multi-persona-review](./multi-persona-review/) | Multi-perspective code review | Parallel personas, finding aggregation |

## Why LLM + Deterministic?

These examples follow the **hybrid pattern**:

```
[Deterministic Tools]     →    [LLM Reasoning]    →    [Action]
 GitHub API, git, scanners     Interpret, assess       Slack, PR comment
```

**Deterministic tools** are great at:
- Fetching structured data (API calls, git log)
- Running existing analysis (security scans, linters)
- Performing actions (post comment, send message)

**LLMs** add value when you need:
- Interpretation of unstructured text (logs, changelogs)
- Contextual judgment (is this exploitable *here*?)
- Natural language generation (release notes, summaries)
- Multi-factor reasoning (should we split this PR?)

## Common Patterns

### 1. Fetch → Analyze → Act

Most CI/CD workflows follow this pattern:

```yaml
steps:
  - id: fetch_data
    github.get_workflow_run:  # Deterministic
      ...

  - id: analyze
    type: llm               # LLM reasoning
    model: balanced
    prompt: |
      Analyze this data...

  - id: notify
    slack.post_message:     # Deterministic
      ...
```

### 2. Parallel Specialized Analysis

When you need multiple perspectives:

```yaml
steps:
  - id: reviews
    type: parallel
    steps:
      - id: security_review
        type: llm
        system: "You are a security engineer..."
      - id: performance_review
        type: llm
        system: "You are a performance engineer..."
```

### 3. Conditional Alerting

Alert only when analysis finds issues:

```yaml
steps:
  - id: alert
    condition:
      expression: 'contains(steps.analyze.response, "CRITICAL")'
    slack.post_message:
      ...
```

## Prerequisites

These examples use the following connectors:
- `github` - GitHub API operations
- `slack` - Slack messaging
- `file` - File read/write
- `shell` - Shell commands

Ensure you have API credentials configured:

```bash
# GitHub token (for API access)
export GITHUB_TOKEN=ghp_...

# Slack webhook or token (for notifications)
export SLACK_TOKEN=xoxb-...
```

## Running Examples

```bash
# Run with inputs
conductor run examples/ci-cd/build-failure-triage/workflow.yaml \
  --input repo=owner/repo \
  --input run_id=12345

# Dry run (validate without executing)
conductor run examples/ci-cd/release-notes/workflow.yaml --dry-run
```

## Customization

Each example is designed to be customized:

1. **Model Selection**: Adjust `model:` for cost/quality tradeoff
   - `fast`: Quick analysis, lower cost
   - `balanced`: Good tradeoff
   - `strategic`: Best quality, highest cost

2. **Personas**: Modify system prompts to match your team's standards

3. **Thresholds**: Adjust numeric thresholds (PR size limits, severity levels)

4. **Outputs**: Change where results go (Slack, GitHub, file)

## Integration Ideas

- **GitHub Actions**: Trigger on workflow failures, PR events
- **Scheduled**: Run nightly summary on cron
- **Webhooks**: React to Dependabot PRs, security scans
- **Manual**: Run release notes before tagging

See the [Conductor documentation](../../docs/) for integration guides.
