# Issue Triage Workflow Example

An intelligent issue classification workflow that automatically analyzes GitHub issues and provides:

- **Type Classification**: bug, feature, enhancement, documentation, etc.
- **Priority Assessment**: critical, high, medium, low
- **Label Suggestions**: Platform, component, expertise tags
- **Team Assignment**: Which team should handle this issue
- **Triage Summary**: Markdown comment ready to post

This workflow helps maintain consistent issue organization and routes work to the right teams efficiently.

## Usage

### Running Standalone

```bash
# Triage a specific issue
conductor run examples/issue-triage \
  --input title="App crashes on startup" \
  --input body="Steps to reproduce: 1. Launch app 2. It immediately crashes"

# Include repository context for better classification
conductor run examples/issue-triage \
  --input title="Add dark mode support" \
  --input body="It would be great to have a dark theme option" \
  --input repository="my-project" \
  --input author="user123"
```

### Programmatic Usage

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/tombee/conductor/pkg/workflow"
)

func main() {
    // Load workflow definition
    data, _ := os.ReadFile("examples/issue-triage/workflow.yaml")
    def, _ := workflow.ParseDefinition(data)

    // Create engine
    engine := workflow.NewEngine()

    // Execute workflow
    result, err := engine.Execute(context.Background(), def, map[string]interface{}{
        "title": issue.Title,
        "body": issue.Body,
        "author": issue.Author,
        "repository": "my-project",
    })

    if err != nil {
        panic(err)
    }

    // Extract triage results
    issueType := result.Outputs["type"].(string)
    priority := result.Outputs["priority"].(string)
    labels := result.Outputs["labels"].([]string)
    team := result.Outputs["team"].(string)
    summary := result.Outputs["summary"].(string)
    urgent := result.Outputs["needs_urgent_attention"].(bool)

    // Apply labels and assignments
    applyLabels(labels)
    assignToTeam(team)

    if urgent {
        notifyOnCall()
    }

    // Post triage summary as comment
    postComment(summary)
}
```

## Example Output

```markdown
## Triage Summary

**Classification:** bug - high
**Labels:** frontend, ui, needs-reproduction
**Suggested Team:** frontend

**Analysis:**
This appears to be a critical user-facing bug affecting app startup. The issue
description indicates a reproducible crash that prevents users from using the
application. The priority is set to high given the impact on user experience,
though not critical since it may not affect all users.

**Recommended Actions:**
- [ ] Verify reproduction steps in development environment
- [ ] Check error logs for stack traces
- [ ] Test across different platforms (macOS, Windows, Linux)
- [ ] Review recent commits that may have introduced the regression
- [ ] Add automated test to prevent regression

**Additional Context:**
User sentiment appears neutral, suggesting a straightforward bug report rather
than an emergency situation. However, startup crashes should be prioritized for
quick resolution to maintain product quality.
```

## Workflow Steps

1. **Issue Classification** (20s timeout)
   - Determines type, priority, and user sentiment
   - Provides confidence score and reasoning
   - Fast model for quick classification

2. **Label Extraction** (20s timeout)
   - Selects relevant labels from predefined taxonomy
   - Maximum 5 labels to avoid over-labeling
   - Explains reasoning for transparency

3. **Team Assignment** (20s timeout)
   - Suggests which team should handle the issue
   - Indicates if multiple teams may be needed
   - Provides confidence score

4. **Summary Generation** (30s timeout)
   - Creates actionable triage summary
   - Formatted as markdown comment
   - Includes recommended next steps
   - Uses balanced model for nuanced summary

## Integration with GitHub

### GitHub Actions (Automatic Triage)

```yaml
name: Auto Triage Issues
on:
  issues:
    types: [opened]

jobs:
  triage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Triage Workflow
        id: triage
        run: |
          conductor run examples/issue-triage \
            --input title="${{ github.event.issue.title }}" \
            --input body="${{ github.event.issue.body }}" \
            --input author="${{ github.event.issue.user.login }}" \
            --input repository="${{ github.repository }}" \
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

      - name: Post Triage Summary
        uses: actions/github-script@v7
        with:
          script: |
            const triage = require('./triage.json');
            await github.rest.issues.createComment({
              issue_number: context.issue.number,
              body: triage.summary
            });

      - name: Notify Team
        if: fromJSON(triage).needs_urgent_attention
        run: |
          # Send notification to on-call team
          curl -X POST ${{ secrets.SLACK_WEBHOOK }} \
            -d "{\"text\": \"🚨 Urgent issue requires attention: ${{ github.event.issue.html_url }}\"}"
```

### Manual Triage with GitHub CLI

```bash
#!/bin/bash
# triage-issue.sh - Triage a GitHub issue

ISSUE_NUMBER=$1

# Fetch issue data
ISSUE_DATA=$(gh issue view $ISSUE_NUMBER --json title,body,author)
TITLE=$(echo "$ISSUE_DATA" | jq -r '.title')
BODY=$(echo "$ISSUE_DATA" | jq -r '.body')
AUTHOR=$(echo "$ISSUE_DATA" | jq -r '.author.login')

# Run triage workflow
TRIAGE=$(conductor run examples/issue-triage \
  --input title="$TITLE" \
  --input body="$BODY" \
  --input author="$AUTHOR" \
  --output-json)

# Extract results
TYPE=$(echo "$TRIAGE" | jq -r '.type')
PRIORITY=$(echo "$TRIAGE" | jq -r '.priority')
LABELS=$(echo "$TRIAGE" | jq -r '.labels | join(",")')
SUMMARY=$(echo "$TRIAGE" | jq -r '.summary')

# Apply labels
gh issue edit $ISSUE_NUMBER --add-label "$TYPE,$PRIORITY,$LABELS"

# Post comment
echo "$SUMMARY" | gh issue comment $ISSUE_NUMBER --body-file -

echo "✓ Issue #$ISSUE_NUMBER triaged successfully"
```

## Customization

### Modify Classification Rules

Edit the system prompts to match your team structure and label taxonomy:

```yaml
system: |
  **Teams:**
  - mobile: iOS and Android app development
  - web: Web application frontend
  - api: Backend API development
  - data: Data engineering and analytics
```

### Add Custom Priority Logic

Adjust priority assessment based on your needs:

```yaml
system: |
  **Priority Levels:**
  - p0: Production outage affecting all users
  - p1: Major feature broken, affects >50% of users
  - p2: Minor feature broken or moderate issue
  - p3: Enhancement or low-impact bug
```

### Integrate with Custom Tools

Add additional steps for custom integrations:

```yaml
- id: create_jira_ticket
  name: Create JIRA Ticket
  type: action
  action: jira.create
  inputs:
    project: "ENG"
    type: "{{$.classify_issue.type}}"
    priority: "{{$.classify_issue.priority}}"
  condition:
    expression: $.classify_issue.priority in ["critical", "high"]
```

## Best Practices

### Review AI Suggestions

While the workflow provides intelligent suggestions, always review:
- Priority assignments for business context
- Team assignments for current capacity
- Label selections for project-specific taxonomy

### Iterate on Prompts

Monitor classification accuracy and refine system prompts based on:
- Misclassified issues
- User feedback
- Team preferences

### Combine with Human Judgment

Use this workflow to:
- Reduce initial triage time
- Provide consistent first-pass classification
- Highlight issues needing urgent attention

But rely on human judgment for:
- Final priority decisions
- Cross-team coordination
- Strategic feature requests

## Requirements

- Conductor workflow engine
- Anthropic API key (set in environment or credentials)
- GitHub CLI (optional, for automation examples)

## Learn More

- [Workflow Definition Reference](../../docs/workflow.md)
- [LLM Provider Configuration](../../docs/llm-providers.md)
- [GitHub Actions Integration](../../docs/integrations/github-actions.md)
