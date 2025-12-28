# Code Review Example

Multi-persona AI code review that analyzes git branch changes from security, performance, and style perspectives.

## Description

This workflow automatically reviews code changes in a git branch using three specialized AI personas running in parallel. Each persona focuses on different aspects (security vulnerabilities, performance issues, code style), then consolidates findings into a single prioritized review report.

## Use Cases

- **Pre-commit reviews** - Catch issues before code reaches main branch
- **PR automation** - Add AI review comments to pull requests automatically
- **CI/CD integration** - Block merges if critical issues are found
- **Developer assistance** - Get quick feedback during development

## Prerequisites

### Required

- Git repository with changes on a feature branch
- Conductor installed ([Getting Started](../getting-started/))
- LLM provider configured (Claude Code, Anthropic API, or OpenAI)

### Optional

- GitHub CLI (`gh`) for PR integration
- CI/CD system for automation

## How to Run It

### Basic Usage

Run in any git repository with uncommitted changes or a feature branch:

```bash
# Review current branch against main
conductor run examples/code-review

# Review against a different base branch
conductor run examples/code-review -i base_branch="develop"
```

### Selective Reviews

Run only specific review personas:

```bash
# Security review only
conductor run examples/code-review -i personas='["security"]'

# Performance and style only
conductor run examples/code-review -i personas='["performance", "style"]'
```

### Custom Output Location

Save the review report to a specific file:

```bash
conductor run examples/code-review -i output_file="reviews/pr-123.md"
```

### GitHub Integration

Review a pull request and post results as a comment:

```bash
# Run review
conductor run examples/code-review -i base_branch="${{ github.base_ref }}"

# Post to PR
gh pr comment 123 --body-file code-review.md
```

## Code Walkthrough

The workflow consists of four main steps:

### 1. Extract Git Information (Steps 1-3)

```yaml
- id: get_branch
  name: Get Current Branch
  shell.run: ["git", "rev-parse", "--abbrev-ref", "HEAD"]

- id: get_diff
  name: Get Git Diff
  shell.run: "git diff {{.inputs.base_branch}}...HEAD..."

- id: get_commits
  name: Get Commit Messages
  shell.run: ["git", "log", "{{.inputs.base_branch}}..HEAD", "--oneline"]
```

**What it does**: Gathers context about the changes being reviewed. Gets the current branch name, the full diff against the base branch, and commit messages. This information provides context for the AI reviewers.

**Why it's structured this way**: Each piece of information is fetched separately for clarity and debugging. The `shell.run` connector executes git commands directly in the repository.

### 2. Parallel Persona Reviews (Step 4)

```yaml
- id: reviews
  name: Parallel Persona Reviews
  type: parallel
  max_concurrency: 3
  steps:
    - id: security_review
      type: llm
      model: strategic
      condition:
        expression: '"security" in inputs.personas'
      system: "You are a security engineer..."
      prompt: "Review these code changes for security issues..."
```

**What it does**: Runs three reviews simultaneously (security, performance, style), each with its own specialized prompt and focus areas. The `parallel` type ensures reviews run concurrently for speed.

**Why parallel execution**: Running reviews in parallel reduces total execution time from ~90 seconds (sequential) to ~30 seconds (parallel), since LLM calls are the slowest part of the workflow.

**Model tier selection**:
- Security uses `strategic` (highest quality) for thorough vulnerability analysis
- Performance uses `balanced` for good quality/speed trade-off
- Style uses `fast` since patterns are easier to detect

**Conditional execution**: Each review only runs if its persona is in the `personas` input array, allowing selective reviews.

### 3. Consolidate Findings (Step 5)

```yaml
- id: generate_report
  name: Generate Review Report
  type: llm
  model: balanced
  prompt: |
    Generate a comprehensive code review report in Markdown format.

    {{if .steps.reviews.security_review}}
    ### Security Review
    {{.steps.reviews.security_review.response}}
    {{end}}
    ...
    Create a well-formatted Markdown report with:
    1. Overall summary with recommendation (APPROVE, REQUEST_CHANGES, NEEDS_DISCUSSION)
    2. Each review section
    3. Prioritized action items
    4. Conclusion
```

**What it does**: Combines all review outputs into a single coherent report. Uses conditional logic to only include reviews that actually ran. The LLM reformats and prioritizes findings into actionable feedback.

**Why use an LLM for consolidation**: While string concatenation would work, an LLM can intelligently prioritize findings, remove duplicates, and create a more readable summary with consistent formatting.

### 4. Write Report to File (Step 6)

```yaml
- id: write_report
  name: Write Report File
  file.write:
    path: "{{.inputs.output_file}}"
    content: "{{.steps.generate_report.response}}"
```

**What it does**: Saves the final report to a markdown file. Uses the `file.write` connector with a templated path from inputs.

**Output format**: The report includes severity ratings (CRITICAL/HIGH/MEDIUM/LOW), specific file:line references, and recommended fixes for each finding.

## Customization Options

### 1. Adjust Model Tiers

Change model tiers based on your speed/quality needs:

```yaml
- id: security_review
  model: balanced  # Faster than strategic, still good quality
```

### 2. Add Custom Review Personas

Add new review focuses to the parallel section:

```yaml
- id: accessibility_review
  name: Accessibility Review
  model: balanced
  condition:
    expression: '"accessibility" in inputs.personas'
  system: |
    You are an accessibility expert. Review for:
    - ARIA attributes and semantic HTML
    - Keyboard navigation support
    - Color contrast and screen reader compatibility
  prompt: "Review for accessibility: {{.steps.get_diff.stdout}}"
```

### 3. Filter by File Type

Only review specific files:

```yaml
- id: get_diff
  shell.run: "git diff {{.inputs.base_branch}}...HEAD -- '*.go' '*.js'"
```

### 4. Customize Review Criteria

Modify system prompts to match your team's standards:

```yaml
system: |
  You are a security engineer reviewing code for:
  - OWASP Top 10 vulnerabilities
  - Company-specific security policies
  - PCI-DSS compliance requirements
```

### 5. Block Merges on Critical Issues

Add exit code based on findings:

```yaml
- id: check_critical
  shell.run: |
    if grep -q "CRITICAL" {{.inputs.output_file}}; then
      echo "Critical issues found - blocking merge"
      exit 1
    fi
```

## Common Issues and Solutions

### Issue: "Not a git repository"

**Symptom**: Workflow fails with "fatal: not a git repository"

**Solution**: Run the workflow from within a git repository directory:

```bash
cd /path/to/your/repo
conductor run /path/to/conductor/examples/code-review
```

### Issue: Empty diff

**Symptom**: Report says "no changes found"

**Solution**: Ensure you're on a branch with changes:

```bash
# Check if you have changes
git diff main...HEAD

# If empty, make some changes first
git checkout -b feature-branch
# ... make changes ...
git commit -am "Changes to review"
```

### Issue: "Permission denied" writing output

**Symptom**: Cannot write review file

**Solution**: Ensure the output directory exists and is writable:

```bash
mkdir -p reviews
conductor run examples/code-review -i output_file="reviews/review.md"
```

### Issue: Rate limit exceeded

**Symptom**: "429 Too Many Requests" errors

**Solution**: Reduce concurrency or use lower-tier models:

```yaml
type: parallel
max_concurrency: 1  # Run reviews sequentially
```

Or switch all reviews to `fast` model tier.

### Issue: Incomplete reviews

**Symptom**: Some review sections are missing or truncated

**Solution**: Increase `max_tokens` for better completeness:

```yaml
- id: security_review
  type: llm
  model: strategic
  max_tokens: 2000  # Default is 1000
```

## Related Examples

- [Issue Triage](issue-triage.md) - AI-powered GitHub issue classification
- [IaC Review](iac-review.md) - Infrastructure-as-Code security review
- [Slack Integration](slack-integration.md) - Post results to Slack

## Workflow Files

Full workflow definition: [examples/code-review/workflow.yaml](https://github.com/tombee/conductor/blob/main/examples/code-review/workflow.yaml)

## Further Reading

- [Parallel Execution Pattern](../building-workflows/patterns.md#parallel-execution)
- [Error Handling](../building-workflows/error-handling.md)
- [GitHub Integration Guide](../building-workflows/daemon-mode.md#github-webhooks)
