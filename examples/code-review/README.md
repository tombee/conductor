# Code Review

Multi-persona AI code review that analyzes git branch changes from security, performance, and style perspectives.

## Usage

Run in any git repository with changes on a feature branch:

```bash
# Review current branch against main
conductor run examples/code-review

# Review against a different base branch
conductor run examples/code-review -i base_branch="develop"

# Run only security review
conductor run examples/code-review -i personas='["security"]'

# Save report to custom location
conductor run examples/code-review -i output_file="reviews/my-review.md"
```

## What It Does

1. **Extracts git info** - Gets current branch, diff against base, and commit messages
2. **Runs parallel reviews** - Security, performance, and style reviews run concurrently
3. **Consolidates findings** - Combines all reviews into a prioritized report
4. **Saves to file** - Writes the report to `code-review.md` (or custom path)

## Output

The workflow produces a Markdown report with:

- **CRITICAL/HIGH** - Security vulnerabilities and blockers
- **MEDIUM/LOW** - Issues to address
- **SUGGESTIONS** - Style and maintainability improvements

Each finding includes file/line references and recommended fixes.

## Inputs

| Input | Default | Description |
|-------|---------|-------------|
| `base_branch` | `"main"` | Branch to compare against |
| `personas` | `["security", "performance", "style"]` | Which reviews to run |
| `output_file` | `"code-review.md"` | Where to save the report |

## Customization

### Model Selection

Edit `workflow.yaml` to adjust model tiers per persona:

- **Security:** `strategic` (deep vulnerability analysis)
- **Performance:** `balanced` (good trade-off)
- **Style:** `fast` (pattern matching)

### Add Review Personas

Add new personas to the parallel reviews section:

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
    - Color contrast and visual indicators
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: AI Code Review
  run: |
    conductor run examples/code-review \
      -i base_branch="${{ github.base_ref }}" \
      -i output_file="review.md"

- name: Comment on PR
  run: gh pr comment ${{ github.event.pull_request.number }} --body-file review.md
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit
conductor run examples/code-review -i personas='["security"]'
```
