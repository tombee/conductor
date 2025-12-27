# Nightly Build Summary

Aggregate overnight CI results and deliver a morning digest to Slack. Surface patterns, flaky tests, and actionable insights.

## Why Use This?

Teams often miss overnight failures until standup:
- Failures may not trigger immediate alerts
- Flaky tests blend into the noise
- Trends (getting worse/better) aren't visible
- Teams start the day without context

A morning digest provides a single source of truth for CI health.

## What It Does

```
Scheduled (7am weekdays)
        ↓
[Fetch all CI runs]           ← GitHub API
        ↓
[Aggregate statistics]        ← LLM
[Detect patterns]
[Compare trends]
        ↓
[Generate digest]             ← LLM
[Post to Slack]               ← Slack API
```

## Example Output

```markdown
# ☀️ Morning CI Digest - Dec 27

## Overview
**Last 12 hours:** 47 runs | ✅ 43 passed (91%) | ❌ 4 failed

Overnight was mostly green with a few recurring test failures.

## 📊 By Workflow
| Workflow | Runs | Pass Rate | Avg Time |
|----------|------|-----------|----------|
| tests | 23 | 96% | 4m 12s |
| build | 12 | 100% | 2m 30s |
| deploy-staging | 8 | 75% | 6m 45s |
| lint | 4 | 100% | 45s |

## 🔴 Failures Needing Attention

### 1. `TestDatabaseConnection` (recurring)
- **Failed 3x overnight** (same error)
- Timeout connecting to test DB
- **Likely cause:** Test DB container instability
- **Action:** Check test infrastructure

### 2. `deploy-staging` failed 2x
- Both failures after commit `a1b2c3d`
- Error: Missing environment variable
- **Action:** Review staging config

## ⚠️ Flaky Tests Detected
- `TestS3Upload` - passed 4x, failed 1x (same commit)
- **Suggestion:** Add to quarantine or fix

## 📈 Trends
- Pass rate: 91% (↓ from 95% last week)
- Avg build time: 4m 20s (stable)
- Flaky tests: 3 identified this week

---
*Digest by Conductor 🎼 | [View Dashboard](https://...)*
```

## Usage

### Manual Run

```bash
conductor run examples/ci-cd/nightly-build-summary/workflow.yaml \
  --input repo=owner/repo \
  --input hours_back=12 \
  --input slack_channel="#ci-digest"
```

### Scheduled Run

The workflow includes a schedule definition:

```yaml
schedule: "0 7 * * 1-5"  # 7am weekdays
```

Deploy to Conductor daemon or use GitHub Actions:

```yaml
# .github/workflows/ci-digest.yml
name: Morning CI Digest
on:
  schedule:
    - cron: '0 7 * * 1-5'  # 7am UTC weekdays
  workflow_dispatch:  # Manual trigger

jobs:
  digest:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Generate Digest
        run: |
          conductor run examples/ci-cd/nightly-build-summary/workflow.yaml \
            --input repo=${{ github.repository }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          SLACK_TOKEN: ${{ secrets.SLACK_TOKEN }}
```

## Configuration

### Inputs

| Input | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `repo` | string | Yes | - | Repository in `owner/repo` format |
| `hours_back` | number | No | `12` | Hours to look back |
| `slack_channel` | string | No | `#ci-digest` | Slack channel |

### Adjusting the Schedule

For different timezones, adjust the cron schedule:

```yaml
# 7am EST (12pm UTC)
schedule: "0 12 * * 1-5"

# 7am PST (3pm UTC)
schedule: "0 15 * * 1-5"
```

## How It Works

1. **List Workflows**: Get all workflow definitions
2. **Get Runs**: Fetch runs from the last N hours
3. **Aggregate**: Compute pass rates, durations, by workflow
4. **Detect Patterns**: Identify flaky tests, recurring failures
5. **Compare Trends**: Week-over-week comparison
6. **Generate Digest**: LLM writes actionable summary
7. **Post to Slack**: Deliver to team channel

### Pattern Detection

The LLM identifies:
- **Flaky Tests**: Pass/fail on same code
- **Recurring Failures**: Same error multiple times
- **New Failures**: First occurrence
- **Infrastructure Issues**: Timeouts, resource exhaustion

## Customization

### Focus on Specific Workflows

Add filtering in the `get_runs` step:

```yaml
- id: get_runs
  github.list_workflow_runs:
    repo: "{{.inputs.repo}}"
    workflow_id: "ci.yml"  # Specific workflow
```

### Different Channels by Severity

```yaml
- id: post_critical
  condition:
    expression: 'steps.aggregate_stats.response.pass_rate < 80'
  slack.post_message:
    channel: "#ci-alerts"  # Urgent channel
    text: "🚨 CI Health Critical..."

- id: post_digest
  slack.post_message:
    channel: "#ci-digest"  # Regular channel
    text: "{{.steps.generate_digest.response}}"
```

## Cost Considerations

- Uses `fast` model for aggregation
- Uses `balanced` model for pattern detection and digest
- Cost per digest: ~$0.02-0.05
