# Building a Code Review Bot

Build an automated code review system that analyzes git changes from multiple perspectives using parallel LLM agents.

**Difficulty:** Intermediate
**Prerequisites:**
- Complete [Your First Workflow](first-workflow.md)
- Git repository with uncommitted changes or a feature branch
- Basic understanding of git diff

---

## What You'll Build

A production-ready code review bot that:

1. Extracts git diff from your current branch
2. Runs **parallel reviews** from three perspectives:
   - Security vulnerabilities
   - Performance issues
   - Code style and maintainability
3. Consolidates findings into a Markdown report
4. Saves the report to a file

This tutorial demonstrates:
- Shell tool for git commands
- Parallel execution for speed
- Conditional step execution
- Multi-step workflows with data flow
- File output

---

## Why Parallel Reviews?

Traditional code reviews run sequentially. With Conductor's parallel execution:

- **3 reviews in ~5 seconds** instead of 15 seconds
- Each review persona runs independently
- Results are combined into one report

---

## Step 1: Create the Workflow

Create `code-review.yaml`:

```bash
touch code-review.yaml
```

Start with metadata:

```yaml
name: git-branch-code-review
description: Multi-persona code review for git branch changes
version: "1.0"
```

---

## Step 2: Define Inputs

Add inputs for flexibility:

```yaml
inputs:
  - name: base_branch
    type: string
    default: "main"
    description: Base branch to compare against

  - name: personas
    type: array
    default: ["security", "performance", "style"]
    description: Review personas to use

  - name: output_file
    type: string
    default: "code-review.md"
    description: Output file for the review report
```

**Key points:**

- `type: array` — Input can be a list of values
- `default:` — Provides sensible defaults
- Users can customize which personas run

---

## Step 3: Get Git Information

Add steps to extract git data:

```yaml
steps:
  - id: get_branch
    name: Get Current Branch
    shell.run:
      command: ["git", "rev-parse", "--abbrev-ref", "HEAD"]

  - id: get_diff
    name: Get Git Diff
    shell.run: "git diff {{.inputs.base_branch}}...HEAD --stat && echo '---DIFF---' && git diff {{.inputs.base_branch}}...HEAD"

  - id: get_commits
    name: Get Commit Messages
    shell.run:
      command: ["git", "log", "{{.inputs.base_branch}}..HEAD", "--oneline"]
```

**Breaking it down:**

- `shell.run:` — Shorthand for shell tool
- `command: [...]` — Array syntax for commands with arguments
- `shell.run: "string"` — String syntax for shell pipelines
- `{{.inputs.base_branch}}` — Template variable in shell command

:::caution[Shell Tool Security]
The shell tool executes commands on your system. Only use trusted inputs.
Template variables in commands are **not** automatically escaped.
:::


---

## Step 4: Set Up Parallel Reviews

Now the powerful part - parallel review execution:

```yaml
  - id: reviews
    name: Parallel Persona Reviews
    type: parallel
    max_concurrency: 3
    steps:
      - id: security_review
        name: Security Review
        type: llm
        model: strategic
        condition:
          expression: '"security" in inputs.personas'
        system: |
          You are a security engineer reviewing code changes for vulnerabilities.
          Focus on:
          - Input validation and sanitization
          - Authentication and authorization issues
          - Injection attacks (SQL, command, XSS)
          - Sensitive data exposure
          - Cryptography issues

          Rate each finding as: CRITICAL, HIGH, MEDIUM, or LOW severity.
          Be specific about file names and line context.
        prompt: |
          Review these code changes for security issues:

          **Branch:** {{.steps.get_branch.stdout}}
          **Commits:**
          {{.steps.get_commits.stdout}}

          **Changes:**
          {{.steps.get_diff.stdout}}

          Provide a structured security review with:
          1. Executive summary (1-2 sentences)
          2. Findings (if any) with severity ratings
          3. Recommendations
```

**Parallel step anatomy:**

- `type: parallel` — Parent step that runs children concurrently
- `max_concurrency: 3` — Run up to 3 reviews at once
- `steps:` — Child steps (nested under parallel)
- `condition:` — Only run if "security" in personas array

**Why `model: strategic`?** Security reviews need deep reasoning about vulnerabilities. Strategic tier (Claude Opus, GPT) provides better analysis.

---

## Step 5: Add Performance and Style Reviews

Add the other two review personas:

```yaml
      - id: performance_review
        name: Performance Review
        type: llm
        model: balanced
        condition:
          expression: '"performance" in inputs.personas'
        system: |
          You are a performance engineer reviewing code for efficiency issues.
          Focus on:
          - Algorithmic complexity (O(n²), unnecessary loops)
          - Memory allocation and leaks
          - Database query efficiency (N+1 queries, missing indexes)
          - Caching opportunities
          - Resource cleanup

          Rate each finding as: CRITICAL, HIGH, MEDIUM, or LOW impact.
        prompt: |
          Review these code changes for performance issues:

          **Branch:** {{.steps.get_branch.stdout}}
          **Commits:**
          {{.steps.get_commits.stdout}}

          **Changes:**
          {{.steps.get_diff.stdout}}

          Provide a structured performance review with:
          1. Executive summary (1-2 sentences)
          2. Findings (if any) with impact ratings
          3. Optimization recommendations

      - id: style_review
        name: Code Style Review
        type: llm
        model: fast
        condition:
          expression: '"style" in inputs.personas'
        system: |
          You are a code quality reviewer focused on readability and maintainability.
          Focus on:
          - Naming conventions (variables, functions, types)
          - Code organization and structure
          - Documentation and comments
          - Error handling patterns
          - Idiomatic code usage

          Rate each finding as: SUGGESTION, MINOR, or IMPORTANT.
          Be constructive and specific.
        prompt: |
          Review these code changes for style and maintainability:

          **Branch:** {{.steps.get_branch.stdout}}
          **Commits:**
          {{.steps.get_commits.stdout}}

          **Changes:**
          {{.steps.get_diff.stdout}}

          Provide a structured style review with:
          1. Executive summary (1-2 sentences)
          2. Findings (if any) with severity
          3. Improvement suggestions
```

**Model tier choices:**

- **Security:** `strategic` (critical issues need deep analysis)
- **Performance:** `balanced` (good trade-off)
- **Style:** `fast` (pattern matching, less reasoning needed)

---

## Step 6: Consolidate Results

After parallel reviews finish, consolidate findings:

```yaml
  - id: generate_report
    name: Generate Review Report
    type: llm
    model: balanced
    prompt: |
      Generate a comprehensive code review report in Markdown format.

      ## Context
      - **Branch:** {{.steps.get_branch.stdout}}
      - **Base:** {{.inputs.base_branch}}
      - **Commits:**
      {{.steps.get_commits.stdout}}

      ## Reviews

      {{if .steps.reviews.security_review}}
      ### Security Review
      {{.steps.reviews.security_review.response}}
      {{end}}

      {{if .steps.reviews.performance_review}}
      ### Performance Review
      {{.steps.reviews.performance_review.response}}
      {{end}}

      {{if .steps.reviews.style_review}}
      ### Style Review
      {{.steps.reviews.style_review.response}}
      {{end}}

      Create a well-formatted Markdown report with:
      1. A header with branch info and date
      2. An overall summary with a recommendation (APPROVE, REQUEST_CHANGES, or NEEDS_DISCUSSION)
      3. Each review section (only include sections that have content)
      4. A prioritized action items list combining all findings
      5. A conclusion

      Output ONLY the Markdown content, no code fences.
```

**Accessing parallel step outputs:**

- `{{.steps.reviews.security_review.response}}` — Access child step by ID
- `.steps.reviews` — The parent parallel step
- `.security_review` — Child step ID
- `.response` — LLM output

**Conditional rendering:**

```yaml
{{if .steps.reviews.security_review}}
...content...
{{end}}
```

This only includes sections when the persona was enabled.

---

## Step 7: Save the Report

Write the consolidated report to a file:

```yaml
  - id: write_report
    name: Write Report File
    file.write:
      path: "{{.inputs.output_file}}"
      content: "{{.steps.generate_report.response}}"
```

---

## Step 8: Add Outputs

Define workflow outputs:

```yaml
outputs:
  - name: report_path
    type: string
    value: "{{.inputs.output_file}}"
    description: Path to the generated review report

  - name: branch
    type: string
    value: "{{.steps.get_branch.stdout}}"
    description: Branch that was reviewed

  - name: summary
    type: string
    value: "{{.steps.generate_report.response}}"
    description: Full review report content
```

---

## Step 9: Run the Code Review

Make sure you have a git repository with changes on a branch:

```bash
# Create a feature branch if needed
git checkout -b feature/my-changes

# Make some changes
echo "console.log('test');" >> app.js
git add app.js
git commit -m "Add logging"

# Run the review
conductor run code-review.yaml
```

**Expected flow:**

```
[conductor] Starting workflow: git-branch-code-review
[conductor] Step 1/6: get_branch (shell.run)
[conductor] ✓ Completed in 0.1s

[conductor] Step 2/6: get_diff (shell.run)
[conductor] ✓ Completed in 0.2s

[conductor] Step 3/6: get_commits (shell.run)
[conductor] ✓ Completed in 0.1s

[conductor] Step 4/6: reviews (parallel)
[conductor]   ├─ security_review (llm, strategic) ...
[conductor]   ├─ performance_review (llm, balanced) ...
[conductor]   └─ style_review (llm, fast) ...
[conductor] ✓ All parallel steps completed in 5.2s

[conductor] Step 5/6: generate_report (llm)
[conductor] ✓ Completed in 2.1s

[conductor] Step 6/6: write_report (file.write)
[conductor] ✓ Completed in 0.01s

--- Output: report_path ---
code-review.md

--- Output: branch ---
feature/my-changes

[workflow complete]
```

Check the generated report:

```bash
cat code-review.md
```

---

## Complete Workflow

Here's the full `code-review.yaml`:

```yaml
name: git-branch-code-review
description: Multi-persona code review for git branch changes
version: "1.0"

inputs:
  - name: base_branch
    type: string
    default: "main"
    description: Base branch to compare against

  - name: personas
    type: array
    default: ["security", "performance", "style"]
    description: Review personas to use

  - name: output_file
    type: string
    default: "code-review.md"
    description: Output file for the review report

steps:
  - id: get_branch
    name: Get Current Branch
    shell.run:
      command: ["git", "rev-parse", "--abbrev-ref", "HEAD"]

  - id: get_diff
    name: Get Git Diff
    shell.run: "git diff {{.inputs.base_branch}}...HEAD --stat && echo '---DIFF---' && git diff {{.inputs.base_branch}}...HEAD"

  - id: get_commits
    name: Get Commit Messages
    shell.run:
      command: ["git", "log", "{{.inputs.base_branch}}..HEAD", "--oneline"]

  - id: reviews
    name: Parallel Persona Reviews
    type: parallel
    max_concurrency: 3
    steps:
      - id: security_review
        name: Security Review
        type: llm
        model: strategic
        condition:
          expression: '"security" in inputs.personas'
        system: |
          You are a security engineer reviewing code changes for vulnerabilities.
          Focus on:
          - Input validation and sanitization
          - Authentication and authorization issues
          - Injection attacks (SQL, command, XSS)
          - Sensitive data exposure
          - Cryptography issues

          Rate each finding as: CRITICAL, HIGH, MEDIUM, or LOW severity.
          Be specific about file names and line context.
        prompt: |
          Review these code changes for security issues:

          **Branch:** {{.steps.get_branch.stdout}}
          **Commits:**
          {{.steps.get_commits.stdout}}

          **Changes:**
          {{.steps.get_diff.stdout}}

          Provide a structured security review with:
          1. Executive summary (1-2 sentences)
          2. Findings (if any) with severity ratings
          3. Recommendations

      - id: performance_review
        name: Performance Review
        type: llm
        model: balanced
        condition:
          expression: '"performance" in inputs.personas'
        system: |
          You are a performance engineer reviewing code for efficiency issues.
          Focus on:
          - Algorithmic complexity (O(n²), unnecessary loops)
          - Memory allocation and leaks
          - Database query efficiency (N+1 queries, missing indexes)
          - Caching opportunities
          - Resource cleanup

          Rate each finding as: CRITICAL, HIGH, MEDIUM, or LOW impact.
        prompt: |
          Review these code changes for performance issues:

          **Branch:** {{.steps.get_branch.stdout}}
          **Commits:**
          {{.steps.get_commits.stdout}}

          **Changes:**
          {{.steps.get_diff.stdout}}

          Provide a structured performance review with:
          1. Executive summary (1-2 sentences)
          2. Findings (if any) with impact ratings
          3. Optimization recommendations

      - id: style_review
        name: Code Style Review
        type: llm
        model: fast
        condition:
          expression: '"style" in inputs.personas'
        system: |
          You are a code quality reviewer focused on readability and maintainability.
          Focus on:
          - Naming conventions (variables, functions, types)
          - Code organization and structure
          - Documentation and comments
          - Error handling patterns
          - Idiomatic code usage

          Rate each finding as: SUGGESTION, MINOR, or IMPORTANT.
          Be constructive and specific.
        prompt: |
          Review these code changes for style and maintainability:

          **Branch:** {{.steps.get_branch.stdout}}
          **Commits:**
          {{.steps.get_commits.stdout}}

          **Changes:**
          {{.steps.get_diff.stdout}}

          Provide a structured style review with:
          1. Executive summary (1-2 sentences)
          2. Findings (if any) with severity
          3. Improvement suggestions

  - id: generate_report
    name: Generate Review Report
    type: llm
    model: balanced
    prompt: |
      Generate a comprehensive code review report in Markdown format.

      ## Context
      - **Branch:** {{.steps.get_branch.stdout}}
      - **Base:** {{.inputs.base_branch}}
      - **Commits:**
      {{.steps.get_commits.stdout}}

      ## Reviews

      {{if .steps.reviews.security_review}}
      ### Security Review
      {{.steps.reviews.security_review.response}}
      {{end}}

      {{if .steps.reviews.performance_review}}
      ### Performance Review
      {{.steps.reviews.performance_review.response}}
      {{end}}

      {{if .steps.reviews.style_review}}
      ### Style Review
      {{.steps.reviews.style_review.response}}
      {{end}}

      Create a well-formatted Markdown report with:
      1. A header with branch info and date
      2. An overall summary with a recommendation (APPROVE, REQUEST_CHANGES, or NEEDS_DISCUSSION)
      3. Each review section (only include sections that have content)
      4. A prioritized action items list combining all findings
      5. A conclusion

      Output ONLY the Markdown content, no code fences.

  - id: write_report
    name: Write Report File
    file.write:
      path: "{{.inputs.output_file}}"
      content: "{{.steps.generate_report.response}}"

outputs:
  - name: report_path
    type: string
    value: "{{.inputs.output_file}}"
    description: Path to the generated review report

  - name: branch
    type: string
    value: "{{.steps.get_branch.stdout}}"
    description: Branch that was reviewed

  - name: summary
    type: string
    value: "{{.steps.generate_report.response}}"
    description: Full review report content
```

---

## Customization Ideas

### Compare Specific Commits

Instead of comparing branches, compare commit ranges:

```yaml
  - id: get_diff
    shell.run: "git diff {{.inputs.from_commit}}..{{.inputs.to_commit}}"
```

### Add More Personas

Create specialized reviewers:

```yaml
- id: accessibility_review
  name: Accessibility Review
  type: llm
  model: balanced
  condition:
    expression: '"accessibility" in inputs.personas'
  system: |
    You are an accessibility expert. Review for:
    - ARIA attributes and semantic HTML
    - Keyboard navigation support
    - Color contrast and visual indicators
    - Screen reader compatibility
```

### Review Specific Files

Filter the diff to specific patterns:

```yaml
  - id: get_diff
    shell.run: "git diff {{.inputs.base_branch}}...HEAD -- '*.go' '*.js'"
```

### Integration with CI/CD

Run in GitHub Actions:

```yaml
# .github/workflows/code-review.yml
name: AI Code Review
on: [pull_request]
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Run Conductor Review
        run: |
          conductor run code-review.yaml \
            -i base_branch="${{ github.base_ref }}" \
            -i output_file="review-${{ github.event.pull_request.number }}.md"
      - name: Comment on PR
        run: gh pr comment ${{ github.event.pull_request.number }} --body-file review-*.md
```

---

## Troubleshooting

### "command not found: git"

**Problem:** Git not in PATH

**Solution:** Ensure git is installed and accessible:

```bash
which git
# Should output: /usr/bin/git or similar
```

### "fatal: bad revision"

**Problem:** Base branch doesn't exist

**Solution:** Use the correct branch name:

```bash
# List branches
git branch -a

# Use the right base
conductor run code-review.yaml -i base_branch="develop"
```

### Parallel steps taking too long

**Problem:** Three strategic models are expensive and slow

**Solution:** Adjust model tiers:

```yaml
# Faster but less thorough
- id: security_review
  model: balanced  # Instead of strategic

- id: performance_review
  model: fast  # Instead of balanced
```

### Reviews are empty

**Problem:** No changes between branches

**Solution:** Verify diff is not empty:

```bash
git diff main...HEAD
# Should show changes
```

### "permission denied" when writing report

**Problem:** Output path not writable

**Solution:** Use current directory or absolute path:

```bash
conductor run code-review.yaml -i output_file="./review.md"
```

---

## Key Concepts Learned

!!! success "You now understand:"
    - **Parallel execution** — Running multiple steps concurrently with `type: parallel`
    - **Shell tool** — Executing git commands with `shell.run:`
    - **Conditional steps** — Using `condition:` to enable/disable steps
    - **Array inputs** — Accepting lists with `type: array`
    - **Nested step access** — Referencing parallel child outputs
    - **Template conditionals** — `{{if}}` statements in templates
    - **Multi-tier models** — Choosing the right model for the task
    - **File writing** — Saving outputs with `file.write:`

---

## What's Next?

### Related Guides

- **[Flow Control](../../guides/flow-control.md)** — Parallel execution and conditional logic
- **[Connectors](../../reference/connectors/index.md)** — Shell, file, and HTTP connectors
- **[Error Handling](../../guides/error-handling.md)** — Handle failures gracefully

### Next Tutorial

- **[Creating Slack Integrations](slack-integration.md)** — Send review reports to Slack

### Production Enhancements

To make this production-ready:

1. **Add error handling** — What if git commands fail?
2. **Implement retry logic** — Handle transient LLM failures
3. **Add tests** — Validate output format
4. **Cache results** — Don't re-review unchanged code
5. **Track costs** — Monitor LLM usage per review

See [Testing](../../guides/testing.md) for testing strategies.

---

## Additional Resources

- **[Workflow Schema: Parallel Steps](../../reference/workflow-schema.md#parallel-steps)**
- **[Shell Connector](../../reference/connectors/shell.md)**
- **[File Connector](../../reference/connectors/file.md)**
- **[Template Variables Guide](../concepts/template-variables.md)**
