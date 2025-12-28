# Issue Triage Example

Intelligent GitHub issue classification workflow that automatically analyzes issues and provides type classification, priority assessment, label suggestions, and team assignments.

## Description

This workflow analyzes GitHub issue content (title and body) to automatically classify issues, suggest priorities, recommend labels, and assign to appropriate teams. It provides a consistent, automated first-pass triage that reduces manual work and ensures issues are routed correctly.

## Use Cases

- **Automated triage** - Classify issues immediately when opened
- **Consistent labeling** - Apply standardized labels across repositories
- **Team routing** - Direct issues to the right team based on content
- **Priority flagging** - Identify urgent issues requiring immediate attention

## Prerequisites

### Required

- Conductor installed ([Getting Started](../getting-started/))
- LLM provider configured (Claude Code, Anthropic API, or OpenAI)
- GitHub issue content (title and body)

### Optional

- GitHub CLI (`gh`) for applying labels and posting comments
- GitHub Actions for automation
- Slack webhook for urgent issue notifications

## How to Run It

### Standalone Testing

Test the triage workflow with sample issue content:

```bash
conductor run examples/issue-triage \
  -i title="App crashes on startup" \
  -i body="Steps to reproduce: 1. Launch app 2. It immediately crashes"
```

### With Repository Context

Include repository and author information for better classification:

```bash
conductor run examples/issue-triage \
  -i title="Add dark mode support" \
  -i body="It would be great to have a dark theme option" \
  -i repository="my-project" \
  -i author="user123"
```

### Triage Existing GitHub Issue

Use GitHub CLI to fetch and triage a real issue:

```bash
#!/bin/bash
ISSUE_NUMBER=42

# Fetch issue data
ISSUE_DATA=$(gh issue view $ISSUE_NUMBER --json title,body,author)
TITLE=$(echo "$ISSUE_DATA" | jq -r '.title')
BODY=$(echo "$ISSUE_DATA" | jq -r '.body')
AUTHOR=$(echo "$ISSUE_DATA" | jq -r '.author.login')

# Run triage
conductor run examples/issue-triage \
  -i title="$TITLE" \
  -i body="$BODY" \
  -i author="$AUTHOR" \
  --output-json > triage.json

# Apply results
gh issue edit $ISSUE_NUMBER \
  --add-label "$(jq -r '.labels | join(",")' triage.json)"

# Post summary
jq -r '.summary' triage.json | gh issue comment $ISSUE_NUMBER --body-file -
```

### GitHub Actions Integration

Automatically triage new issues:

```conductor
# .github/workflows/auto-triage.yml
name: Auto Triage Issues
on:
  issues:
    types: [opened]

jobs:
  triage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Triage
        run: |
          conductor run examples/issue-triage \
            -i title="${{ github.event.issue.title }}" \
            -i body="${{ github.event.issue.body }}" \
            -i author="${{ github.event.issue.user.login }}" \
            --output-json > triage.json

      - name: Apply Labels
        uses: actions/github-script@v7
        with:
          script: |
            const triage = require('./triage.json');
            await github.rest.issues.addLabels({
              issue_number: context.issue.number,
              labels: [triage.type, `priority:${triage.priority}`, ...triage.labels]
            });

      - name: Post Summary
        uses: actions/github-script@v7
        with:
          script: |
            const triage = require('./triage.json');
            await github.rest.issues.createComment({
              issue_number: context.issue.number,
              body: triage.summary
            });
```

## Code Walkthrough

The workflow consists of four sequential analysis steps:

### 1. Classify Issue (Step 1)

```conductor
- id: classify_issue
  name: Issue Classification
  type: llm
  model: fast
  system: |
    You are an expert at triaging software issues.

    **Issue Types:**
    - bug, feature, enhancement, documentation, question, refactor, performance, security

    **Priority Levels:**
    - critical: Production down, data loss, security breach
    - high: Major functionality broken, affects many users
    - medium: Moderate impact, workaround exists
    - low: Minor issue, cosmetic, or nice-to-have

    Respond in JSON format: {type, priority, sentiment, confidence, reasoning}
  prompt: |
    Classify this issue:
    **Title:** {{.title}}
    **Body:** {{.body}}
```

**What it does**: Analyzes the issue title and description to determine the issue type (bug, feature, etc.), priority level (critical through low), and user sentiment (frustrated, neutral, positive). Returns structured JSON with confidence scores and reasoning.

**Why use `fast` model**: Classification is pattern-matching work that doesn't require deep reasoning. The `fast` tier provides quick results (typically 2-3 seconds) at lower cost while maintaining good accuracy.

**Confidence scoring**: The workflow asks the model to provide a confidence score (0.0-1.0) which helps identify borderline cases that may need human review.

### 2. Extract Labels (Step 2)

```conductor
- id: extract_labels
  name: Label Extraction
  type: llm
  model: fast
  system: |
    Extract relevant labels for this issue based on its content.

    **Available Labels:**
    - Platform: frontend, backend, database, api, cli, mobile
    - Component: auth, ui, workflow, storage, networking, deployment
    - Status: needs-reproduction, needs-design, ready-to-implement
    - Expertise: accessibility, performance, security, i18n
    - Special: breaking-change, good-first-issue, help-wanted

    Maximum 5 labels.
  prompt: |
    Extract labels for this issue:
    **Type:** {{.steps.classify_issue.response}}
    **Title:** {{.title}}
    **Body:** {{.body}}
```

**What it does**: Analyzes the issue content against a predefined label taxonomy to suggest relevant labels. Limits to 5 labels to avoid over-labeling.

**Why separate from classification**: Keeping label extraction separate allows for easier customization of label taxonomies per project. You can modify the available labels without changing classification logic.

**Context from previous step**: References the classification result (`{{.steps.classify_issue.response}}`) to inform label selection. For example, a "security" type issue will more likely get security-related labels.

### 3. Suggest Team Assignment (Step 3)

```conductor
- id: suggest_assignment
  name: Team Assignment Suggestion
  type: llm
  model: fast
  system: |
    Suggest which team should handle this issue.

    **Teams:**
    - platform, frontend, backend, devops, security, support

    Respond in JSON: {team, confidence, reasoning, requires_multiple_teams}
  prompt: |
    Suggest team assignment:
    **Type:** {{.steps.classify_issue.response}}
    **Priority:** {{.steps.classify_issue.response}}
    **Labels:** {{.steps.extract_labels.response}}
    **Title:** {{.title}}
```

**What it does**: Recommends which team should handle the issue based on all previous analysis. Includes a flag for whether multiple teams may be needed (e.g., a bug affecting both frontend and backend).

**Cumulative context**: Uses results from both previous steps to make an informed decision. The type, priority, and labels all influence team assignment.

**Team structure customization**: The system prompt defines your team structure. Customize these team names and descriptions to match your organization.

### 4. Generate Triage Summary (Step 4)

```conductor
- id: generate_summary
  name: Generate Triage Summary
  type: llm
  model: balanced
  system: |
    Create a concise triage summary for this issue that can be posted as a comment.

    Format:
    ## Triage Summary
    **Classification:** [type] - [priority]
    **Labels:** [labels]
    **Suggested Team:** [team]
    **Analysis:** [brief analysis]
    **Recommended Actions:** [checklist]
  prompt: |
    Generate triage summary for:
    **Classification:** {{.steps.classify_issue.response}}
    **Labels:** {{.steps.extract_labels.response}}
    **Suggested Team:** {{.steps.suggest_assignment.response}}
    **Original Issue:** Title: {{.title}}, Body: {{.body | truncate 1000}}
```

**What it does**: Creates a markdown-formatted summary comment that consolidates all triage information into a human-readable format. Includes analysis and actionable next steps.

**Why use `balanced` model**: Summary generation requires better writing quality than classification, but doesn't need the highest tier. The `balanced` model provides good output quality at reasonable speed (5-8 seconds) and cost.

**Truncation for context limits**: The `truncate 1000` filter prevents extremely long issue bodies from exceeding token limits while preserving enough context for summary generation.

## Customization Options

### 1. Modify Label Taxonomy

Customize labels to match your project:

```conductor
system: |
  **Available Labels:**
  - Type: bug, feature, enhancement
  - Area: ios, android, web, api
  - Priority: p0, p1, p2, p3
  - Status: needs-triage, in-progress, blocked
```

### 2. Add Custom Priority Logic

Define priority levels specific to your needs:

```conductor
system: |
  **Priority Levels:**
  - p0: Production outage affecting all users (immediate response)
  - p1: Major feature broken, >50% users affected (same day)
  - p2: Minor feature broken or moderate issue (this week)
  - p3: Enhancement or low-impact bug (backlog)
```

### 3. Customize Team Structure

Adjust team definitions for your organization:

```conductor
system: |
  **Teams:**
  - mobile: iOS and Android development
  - web-frontend: React web application
  - api: Backend API and services
  - data: Data pipeline and analytics
  - infra: Cloud infrastructure and DevOps
```

### 4. Add Urgency Detection

Create a separate step to flag urgent issues:

```conductor
- id: check_urgency
  type: llm
  model: fast
  prompt: |
    Based on this classification, does this issue need immediate attention?
    Classification: {{.steps.classify_issue.response}}

    Return JSON: {"urgent": true/false, "reason": "..."}

# Then in outputs:
outputs:
  - name: needs_urgent_attention
    type: boolean
    value: "{{.steps.check_urgency.urgent}}"
```

### 5. Integrate with External Systems

Add steps to create tickets in other systems:

```conductor
- id: create_jira_ticket
  condition:
    expression: 'steps.classify_issue.priority in ["critical", "high"]'
  type: action
  action: jira.create_issue
  inputs:
    project: "ENG"
    issue_type: "{{.steps.classify_issue.type}}"
    priority: "{{.steps.classify_issue.priority}}"
    summary: "{{.inputs.title}}"
    description: "{{.inputs.body}}"
```

## Common Issues and Solutions

### Issue: Inconsistent classifications

**Symptom**: Same type of issue gets classified differently

**Solution**: Refine system prompts with more specific examples:

```conductor
system: |
  **Issue Types with Examples:**
  - bug: "App crashes", "Error message appears", "Feature doesn't work"
  - feature: "Add ability to", "Support for", "New functionality"
  - enhancement: "Improve performance of", "Better UX for", "Optimize"
```

### Issue: Too many labels suggested

**Symptom**: Issues consistently get 5 labels even when fewer are appropriate

**Solution**: Adjust the prompt to be more selective:

```conductor
prompt: |
  Extract 2-3 most relevant labels (maximum 5 only if truly necessary).
  Be selective - only include labels that clearly apply.
```

### Issue: Wrong team assignments

**Symptom**: Issues frequently assigned to incorrect teams

**Solution**: Add domain keywords to team descriptions:

```conductor
**Teams:**
- frontend: UI, React, TypeScript, CSS, user interface, browser
- backend: API, database, Python, server, authentication, data processing
```

### Issue: API timeouts

**Symptom**: Workflow fails with timeout errors

**Solution**: Increase timeout values:

```conductor
- id: classify_issue
  timeout: 30  # Increased from default 20
  retry:
    max_attempts: 3
    backoff_base: 2
```

### Issue: Rate limits with GitHub Actions

**Symptom**: "429 Too Many Requests" when many issues opened simultaneously

**Solution**: Add rate limiting to GitHub Actions workflow:

```conductor
- name: Rate Limit
  run: sleep $((RANDOM % 10))  # Random delay 0-10 seconds
```

## Related Examples

- [Code Review](code-review.md) - Multi-persona code analysis
- [Slack Integration](slack-integration.md) - Post triage summaries to Slack
- [IaC Review](iac-review.md) - Infrastructure code review patterns

## Workflow Files

Full workflow definition: [examples/issue-triage/workflow.yaml](https://github.com/tombee/conductor/blob/main/examples/issue-triage/workflow.yaml)

## Further Reading

- [Sequential Processing Pattern](../building-workflows/patterns.md#sequential-processing)
- [Structured JSON Output](../reference/workflow-schema.md#response-format)
- [GitHub Actions Integration](../building-workflows/daemon-mode.md#github-webhooks)
