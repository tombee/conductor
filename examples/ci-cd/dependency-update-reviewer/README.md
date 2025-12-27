# Dependency Update Reviewer

Review dependency update PRs from Dependabot or Renovate. Analyze changelogs, identify breaking changes, and assess upgrade risk.

## Why Use This?

Dependency updates are frequent but reviewing them is tedious:
- Changelogs are inconsistent or missing
- Breaking changes are buried in prose
- Patch versions sometimes contain breaking changes
- Developers rubber-stamp or ignore updates

This workflow reads the changelog and provides a risk assessment.

## What It Does

```
Dependabot/Renovate PR
        ↓
[Parse dependency + versions]  ← PR analysis
[Fetch changelog/releases]     ← GitHub API
        ↓
[Identify breaking changes]    ← LLM reasoning
[Assess risk level]
        ↓
[Post review comment]          ← GitHub API
```

## Example Output

```markdown
## 🤖 Dependency Update Review

### 📦 express: 4.18.2 → 5.0.0

**Risk Level:** ⚠️ REVIEW RECOMMENDED

**Version Jump:** Major (4.x → 5.x)

### Breaking Changes
- `req.host` now returns only hostname, not port
- Removed `app.del()` (use `app.delete()`)
- Path route matching stricter with trailing slashes

### Security
- No security fixes in this release

### Notable Changes
- Async middleware support (no more wrapper needed)
- Built-in body parsing (json, urlencoded)
- Improved TypeScript types

### Recommendation
This is a major version bump with breaking changes. Review the
migration guide and test thoroughly before merging.

**Migration Guide:** https://expressjs.com/en/guide/migrating-5.html

---
*Analyzed by Conductor 🎼*
```

## Usage

### Manual Review

```bash
conductor run examples/ci-cd/dependency-update-reviewer/workflow.yaml \
  --input repo=owner/repo \
  --input pr_number=123
```

### Auto-Review Dependabot PRs

```yaml
# .github/workflows/review-deps.yml
name: Review Dependency Updates
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    if: ${{ github.actor == 'dependabot[bot]' || github.actor == 'renovate[bot]' }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Review Update
        run: |
          conductor run examples/ci-cd/dependency-update-reviewer/workflow.yaml \
            --input repo=${{ github.repository }} \
            --input pr_number=${{ github.event.pull_request.number }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Configuration

### Inputs

| Input | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `repo` | string | Yes | - | Repository in `owner/repo` format |
| `pr_number` | number | Yes | - | PR number to review |

### Risk Levels

| Level | Meaning | Action |
|-------|---------|--------|
| SAFE | Patch, no breaking changes | Safe to merge |
| REVIEW | Minor or notable changes | Worth reviewing |
| RISKY | Major or potential breaking | Test thoroughly |
| BREAKING | Confirmed breaking changes | Migration required |

## How It Works

1. **Get PR**: Fetch PR details and changed files
2. **Parse Updates**: Extract package names and version changes
3. **Fetch Changelogs**: Search GitHub Releases, CHANGELOG.md
4. **Analyze**: LLM identifies breaking changes and assesses risk
5. **Comment**: Post structured review to PR

### Changelog Discovery

The workflow tries multiple sources:
1. GitHub Releases API
2. `CHANGELOG.md` in the package repo
3. Commit messages between tags
4. Package registry metadata (npm, pkg.go.dev)

## Tips

- **Grouped Updates**: Works with PRs containing multiple updates
- **Security Fixes**: Highlights CVE fixes in the changelog
- **Migration Guides**: Links to migration docs when available

## Handling Edge Cases

### Missing Changelog

```markdown
### 📦 obscure-lib: 1.0.0 → 1.1.0

**Risk Level:** ⚠️ REVIEW RECOMMENDED

**Note:** No changelog found. Review commits manually.

**Commits:** https://github.com/owner/obscure-lib/compare/v1.0.0...v1.1.0
```

### False Breaking Changes

If the LLM flags something incorrectly, the review is just a comment - developers make the final call.

## Cost Considerations

- Uses `strategic` model for changelog analysis
- Uses `fast` model for PR parsing
- Cost per PR: ~$0.03-0.08
