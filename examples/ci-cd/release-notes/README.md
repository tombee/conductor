# Release Notes Generator

Automatically generate professional release notes by analyzing commits, PRs, and their context since the last release.

## Why Use This?

Writing release notes is tedious and often:
- Delayed or skipped entirely
- Inconsistent in format and tone
- Missing important changes
- Full of technical jargon users don't understand

This workflow generates user-friendly, categorized release notes automatically.

## What It Does

```
Git Tags + Commits
        ↓
[Get commits since last tag]  ← Deterministic (git)
[Fetch associated PRs]        ← GitHub API
        ↓
[Categorize changes]          ← LLM reasoning
[Write user-friendly prose]
        ↓
[Save RELEASE_NOTES.md]       ← File output
```

## Example Output

```markdown
# v2.1.0 (2025-12-27)

## Highlights
- Parallel step execution for faster workflows
- New Slack connector for team notifications
- Fixed critical race condition in retry logic

## ✨ Features
- **Parallel step execution**: Workflows can now run steps concurrently
  using the `type: parallel` step type (#234)
- **Slack connector**: New built-in connector for Slack API integration (#228)

## 🐛 Bug Fixes
- Fixed race condition in step retry logic (#241)
- Corrected timeout handling for long-running LLM calls (#239)

## ⚠️ Breaking Changes
- `action` step type removed; use connectors instead (#230)
  - **Migration**: Replace `action: http.get` with `http.get:` shorthand

## 📦 Dependencies
- Upgraded expr-lang/expr to v1.16.0

## 👥 Contributors
Thanks to @alice, @bob, and @charlie for their contributions!
```

## Usage

### Generate Notes for Next Release

```bash
# From latest tag to HEAD
conductor run examples/ci-cd/release-notes/workflow.yaml \
  --input repo=owner/repo

# Between specific versions
conductor run examples/ci-cd/release-notes/workflow.yaml \
  --input repo=owner/repo \
  --input from_tag=v2.0.0 \
  --input to_ref=v2.1.0
```

### Create GitHub Release

After generating notes:

```bash
# Generate notes
conductor run examples/ci-cd/release-notes/workflow.yaml \
  --input repo=owner/repo \
  --input output_file=RELEASE_NOTES.md

# Create release with notes
gh release create v2.1.0 --notes-file RELEASE_NOTES.md
```

### Pre-Release Workflow

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags: ['v*']

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Full history for git log

      - name: Generate Release Notes
        run: |
          conductor run examples/ci-cd/release-notes/workflow.yaml \
            --input repo=${{ github.repository }} \
            --input to_ref=${{ github.ref_name }}

      - name: Create GitHub Release
        run: gh release create ${{ github.ref_name }} --notes-file RELEASE_NOTES.md
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Configuration

### Inputs

| Input | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `repo` | string | Yes | - | Repository in `owner/repo` format |
| `from_tag` | string | No | Latest tag | Starting point for changelog |
| `to_ref` | string | No | `HEAD` | Ending point (tag, branch, SHA) |
| `include_contributors` | boolean | No | `true` | Include contributor acknowledgments |
| `output_file` | string | No | `RELEASE_NOTES.md` | Output file path |

### Customization

**Change categories** by modifying the `categorize_changes` system prompt:

```yaml
system: |
  Categorize each change into ONE of these categories:
  - Features: New functionality
  - Improvements: Enhancements to existing features
  - Bug Fixes: ...
  - Security: Security-related changes
  ...
```

**Adjust tone** by modifying the `generate_notes` system prompt:

```yaml
system: |
  Write clear, user-focused release notes.
  Use casual, friendly tone.  # or "Use formal, technical tone"
  ...
```

## How It Works

1. **Detect Range**: Find starting tag if not specified
2. **Get Commits**: List all commits in the range
3. **Fetch PRs**: Get associated PR metadata for context
4. **Categorize**: LLM classifies each change by type
5. **Generate**: LLM writes formatted, user-friendly notes
6. **Save**: Write to output file

### Categorization Logic

The LLM uses:
- Commit message prefixes (`feat:`, `fix:`, `BREAKING:`)
- PR labels (`enhancement`, `bug`, `breaking-change`)
- Actual content of changes
- Context from PR descriptions

## Tips

- **Conventional Commits**: Using [Conventional Commits](https://conventionalcommits.org/) improves categorization accuracy
- **PR Labels**: Add labels to PRs for better classification
- **Breaking Changes**: Mark with `BREAKING:` prefix or `breaking-change` label

## Cost Considerations

- Uses `balanced` model for categorization and writing
- Two LLM calls per release
- Cost scales with number of commits

Estimated cost: ~$0.05-0.15 per release
