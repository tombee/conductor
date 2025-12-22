# Code Review Workflow Example

A multi-persona AI code review workflow that analyzes code changes from three perspectives:

- **Security Review**: Identifies vulnerabilities, input validation issues, and security best practices
- **Performance Review**: Analyzes algorithmic complexity, resource usage, and optimization opportunities
- **Style Review**: Evaluates code clarity, naming, maintainability, and test coverage

The workflow consolidates findings from all three reviewers into a comprehensive report with prioritized action items.

## Usage

### Running Standalone

```bash
# Review a git diff
git diff main..feature-branch | conductor run examples/code-review

# Review specific files
conductor run examples/code-review --input diff="$(git diff path/to/file.ts)"

# Provide additional context
conductor run examples/code-review \
  --input diff="$(git diff HEAD~1)" \
  --input context="Refactoring authentication to use OAuth2"
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
    data, _ := os.ReadFile("examples/code-review/workflow.yaml")
    def, _ := workflow.ParseDefinition(data)

    // Create engine
    engine := workflow.NewEngine()

    // Execute workflow
    result, err := engine.Execute(context.Background(), def, map[string]interface{}{
        "diff": getDiff(), // Your diff content
        "context": "Implementing new feature X",
    })

    if err != nil {
        panic(err)
    }

    // Get consolidated review
    review := result.Outputs["review"].(string)
    hasBlockers := result.Outputs["has_blockers"].(bool)

    fmt.Println(review)
    if hasBlockers {
        os.Exit(1)
    }
}
```

## Example Output

```markdown
# Code Review Summary

**Findings:** 2 BLOCKERS, 3 REQUIRED, 5 SUGGESTED, 2 INFORMATIONAL

## BLOCKERS - Must fix before merge

1. **SQL Injection Vulnerability** (auth.ts:45)
   - Issue: User input directly concatenated into SQL query
   - Fix: Use parameterized queries or ORM methods
   - Flagged by: Security

2. **Unhandled Promise Rejection** (service.ts:102)
   - Issue: Async operation not wrapped in try/catch
   - Fix: Add error handling or propagate to caller
   - Flagged by: Security, Style

## REQUIRED - Should fix before merge

3. **N+1 Query Pattern** (users.ts:78-92)
   - Issue: Fetching related data in loop causes N database queries
   - Fix: Use eager loading or single query with JOIN
   - Flagged by: Performance

...
```

## Workflow Steps

1. **Security Review** (30s timeout)
   - Analyzes diff for security vulnerabilities
   - Uses fast model (Claude Haiku) for quick feedback
   - Retries up to 2 times on failure

2. **Performance Review** (30s timeout)
   - Evaluates algorithmic complexity and resource usage
   - Identifies optimization opportunities
   - Parallel execution with security and style reviews

3. **Style Review** (30s timeout)
   - Checks code clarity, naming, and maintainability
   - Evaluates error handling and test coverage
   - Parallel execution with other reviews

4. **Consolidation** (45s timeout)
   - Synthesizes findings from all three reviews
   - Groups by severity and deduplicates
   - Uses balanced model (Claude Sonnet) for nuanced synthesis

## Customization

### Adjust Model Tiers

Change `model: fast` to `model: balanced` or `model: strategic` for more thorough analysis:

```yaml
inputs:
  model: balanced  # Use Claude Sonnet instead of Haiku
```

### Add Custom Review Personas

Add additional review steps for domain-specific concerns:

```yaml
- id: accessibility_review
  name: Accessibility Analysis
  type: llm
  action: anthropic.complete
  inputs:
    model: fast
    system: "You are an accessibility expert. Check for WCAG compliance..."
```

### Modify Timeout and Retry

Adjust based on your needs:

```yaml
timeout: 60  # Increase for larger diffs
retry:
  max_attempts: 3  # More retries for flaky networks
```

## Integration with CI/CD

### GitHub Actions

```yaml
- name: AI Code Review
  run: |
    git diff origin/main...${{ github.sha }} > /tmp/diff.txt
    conductor run examples/code-review --input diff="$(cat /tmp/diff.txt)" > review.md

- name: Comment on PR
  uses: actions/github-script@v7
  with:
    script: |
      const fs = require('fs');
      const review = fs.readFileSync('review.md', 'utf8');
      github.rest.issues.createComment({
        issue_number: context.issue.number,
        body: review
      });
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

git diff --cached | conductor run examples/code-review --input context="Pre-commit review"

if [ $? -eq 1 ]; then
  echo "❌ Blocking issues found in code review"
  exit 1
fi
```

## Requirements

- Conductor workflow engine
- Anthropic API key (set in environment or credentials)
- Git (for generating diffs)

## Learn More

- [Workflow Definition Reference](../../docs/workflow.md)
- [LLM Provider Configuration](../../docs/llm-providers.md)
- [Running Workflows](../../docs/getting-started.md)
