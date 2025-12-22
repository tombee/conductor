# Code Review

Multi-persona AI code review that analyzes changes from security, performance, and style perspectives.

## Usage

```bash
# Review staged changes
git diff --cached | conductor run examples/code-review -i diff=-

# Review a branch against main
git diff main...HEAD | conductor run examples/code-review -i diff=-

# Review last commit
git show HEAD | conductor run examples/code-review -i diff=-

# Add context about the changes
git diff main | conductor run examples/code-review \
  -i diff=- \
  -i context="Refactoring auth to use OAuth2"
```

## Output

The workflow produces a consolidated report with findings grouped by severity:

- **BLOCKERS** - Must fix before merge
- **REQUIRED** - Should fix before merge
- **SUGGESTED** - Consider addressing
- **INFORMATIONAL** - Keep in mind

Each finding includes file/line references and suggested fixes.

## CI/CD Integration

### GitHub Actions

```yaml
- name: AI Code Review
  run: |
    git diff origin/main..HEAD | conductor run examples/code-review -i diff=- > review.md

- name: Comment on PR
  uses: actions/github-script@v7
  with:
    script: |
      const fs = require('fs');
      github.rest.issues.createComment({
        issue_number: context.issue.number,
        body: fs.readFileSync('review.md', 'utf8')
      });
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit
git diff --cached | conductor run examples/code-review -i diff=-
if [ $? -eq 1 ]; then
  echo "Blocking issues found"
  exit 1
fi
```

## Customization

### Model Selection

Edit `workflow.yaml` to adjust model tiers:

- `fast` (Haiku) - Quick feedback, lower cost
- `balanced` (Sonnet) - Better analysis
- `strategic` (Opus) - Deep reasoning for complex code

### Add Review Personas

Add steps for domain-specific reviews (accessibility, API design, etc.):

```yaml
- id: api_review
  type: llm
  model: balanced
  system: "You are an API design expert..."
```
