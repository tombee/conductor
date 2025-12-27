# Build Failure Triage

Automatically analyze CI build failures, classify them as transient or real code issues, and post actionable summaries to Slack.

## Why Use This?

Build failures interrupt developer flow. Distinguishing between:
- **Transient failures** (flaky tests, network timeouts, resource exhaustion)
- **Real failures** (code bugs, missing dependencies, config errors)

...requires reading logs and understanding context. This workflow does that automatically.

## What It Does

```
GitHub Actions Failure
        ↓
[Fetch logs + recent commits]  ← Deterministic (GitHub API)
        ↓
[Classify failure type]        ← LLM reasoning
[Identify root cause]
[Correlate with commits]
        ↓
[Post structured summary]      ← Slack
```

## Example Output

```
🔴 Build Failed: acme/api

Run: #1234
Branch: main
Commit: a1b2c3d

Classification: REAL (code breakage)
Confidence: HIGH
Category: unit-tests

Summary: Test `TestUserAuth` fails with nil pointer in `auth.go:142`.
The error occurs when validating tokens with empty claims.

Likely Cause: Commit `a1b2c3d` by @developer - "Refactor token validation"
Changed `auth.go` which matches the failing component.

Recommended Action: Investigate commit a1b2c3d, likely missing nil check.

[View Logs] | [View Commit]
```

## Usage

### Manual Run

```bash
conductor run examples/ci-cd/build-failure-triage/workflow.yaml \
  --input repo=owner/repo \
  --input run_id=12345678 \
  --input slack_channel="#ci-alerts"
```

### GitHub Actions Integration

Add to `.github/workflows/on-failure.yml`:

```yaml
name: Triage Build Failures
on:
  workflow_run:
    workflows: ["CI"]
    types: [completed]

jobs:
  triage:
    if: ${{ github.event.workflow_run.conclusion == 'failure' }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Triage Failure
        run: |
          conductor run examples/ci-cd/build-failure-triage/workflow.yaml \
            --input repo=${{ github.repository }} \
            --input run_id=${{ github.event.workflow_run.id }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          SLACK_TOKEN: ${{ secrets.SLACK_TOKEN }}
```

## Configuration

### Inputs

| Input | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `repo` | string | Yes | - | Repository in `owner/repo` format |
| `run_id` | string | Yes | - | GitHub Actions run ID |
| `slack_channel` | string | No | `#ci-alerts` | Slack channel for notifications |

### Customization

**Adjust classification criteria** by modifying the system prompt in the `analyze_failure` step. Add your project's common transient failure patterns:

```yaml
system: |
  TRANSIENT failures include:
  - ...
  - TestDatabase timeout (known flaky)
  - S3 upload intermittent failures
```

**Change Slack formatting** by modifying the `post_slack` step template.

## How It Works

1. **Fetch Run Details**: Gets workflow run metadata from GitHub API
2. **Fetch Logs**: Downloads build logs (truncated for LLM context)
3. **Get Commits**: Lists commits since last successful run
4. **Analyze**: LLM classifies failure, identifies cause, suggests action
5. **Notify**: Posts structured summary to Slack

### Classification Logic

The LLM considers:
- Error messages and stack traces
- Whether similar code passed before
- Timing patterns (timeouts vs instant failures)
- External service mentions in logs
- Commit changes that correlate with failing components

## Cost Considerations

- Uses `strategic` model for analysis (best classification accuracy)
- Log truncation keeps context size manageable
- One LLM call per failure

Estimated cost: ~$0.02-0.05 per failure analyzed

## Limitations

- Requires GitHub Actions (adaptable to other CI systems)
- Log analysis limited by context window (8K characters by default)
- Cannot automatically retry or fix failures (notification only)
