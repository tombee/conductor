# PR Size & Complexity Gate

Analyze PR size and complexity. Warn on oversized changes and suggest logical ways to split them.

## Why Use This?

Research shows PR review quality degrades with size:
- PRs over 400 lines get superficial reviews
- PRs over 1000 lines are often approved without thorough review
- Large PRs hide bugs in the noise

But simple line counts don't tell the full story - 500 lines of generated code is different from 500 lines of business logic.

## What It Does

```
PR Opened/Updated
        ↓
[Get files and metrics]       ← GitHub API
        ↓
[Classify change type]        ← LLM reasoning
[Assess reviewability]
[Suggest splits if needed]
        ↓
[Post analysis comment]       ← GitHub API
```

## Example Output

```markdown
## 📊 PR Size Analysis

**Lines Changed:** 847 (+612, -235)
**Files Changed:** 23
**Classification:** Feature (cross-cutting)

### Assessment: ⚠️ LARGE

This PR is larger than recommended for thorough review. Consider splitting.

**Estimated Review Time:** 90-120 minutes
**Review Confidence:** Moderate (size may cause fatigue)

### Suggested Splits

1. **Database Schema Changes** (first)
   - `migrations/*`, `models/*`
   - ~180 lines, independent foundation
   - Can be reviewed and merged first

2. **API Endpoints**
   - `handlers/*`, `routes/*`
   - ~320 lines, depends on #1
   - Well-scoped unit

3. **Frontend Integration**
   - `ui/*`, `components/*`
   - ~280 lines, depends on #2
   - Separate review expertise

### High-Risk Areas
- `auth/permissions.go` - Security-sensitive changes
- `db/queries.go` - Complex SQL modifications

---
*Size analysis by Conductor 🎼*
```

## Usage

### Manual Analysis

```bash
conductor run examples/ci-cd/pr-size-gate/workflow.yaml \
  --input repo=owner/repo \
  --input pr_number=123
```

### Auto-Analyze All PRs

```yaml
# .github/workflows/pr-size.yml
name: PR Size Analysis
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Analyze PR Size
        run: |
          conductor run examples/ci-cd/pr-size-gate/workflow.yaml \
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
| `pr_number` | number | Yes | - | PR number to analyze |
| `thresholds` | object | No | See below | Line count thresholds |
| `ignored_patterns` | array | No | See below | Files to exclude |

### Default Thresholds

```yaml
thresholds:
  good: 200        # Great - quick to review
  acceptable: 400  # Fine - reasonable review
  large: 800       # Warning - may get superficial review
  oversized: 1500  # Alert - split recommended
```

### Default Ignored Patterns

```yaml
ignored_patterns:
  - "*.generated.*"
  - "*.lock"
  - "go.sum"
  - "package-lock.json"
  - "yarn.lock"
```

## How It Works

1. **Get PR Files**: Fetch list of changed files with diffs
2. **Compute Metrics**: Calculate lines, files, test ratio
3. **Classify Change**: Determine if feature, bugfix, refactor, etc.
4. **Assess Reviewability**: Consider complexity, not just size
5. **Suggest Splits**: If large, propose logical breakpoints
6. **Post Comment**: Share analysis with the team

### Reviewability Factors

Beyond line count, the LLM considers:
- **Change Type**: Refactors are often safer than new features
- **File Types**: Generated code, configs vs business logic
- **Test Coverage**: PRs with tests are easier to review
- **Scope**: Single component vs cross-cutting changes
- **Risk Areas**: Security, data model, performance-critical code

### Split Suggestions

Good splits are:
- Independently reviewable and mergeable
- Have clear boundaries (by feature, layer, or file group)
- Can be ordered logically (foundation first)

## Customization

### Stricter Thresholds

For critical codebases:

```yaml
thresholds:
  good: 100
  acceptable: 200
  large: 400
  oversized: 800
```

### Additional Ignored Patterns

```yaml
ignored_patterns:
  - "*.generated.*"
  - "*.lock"
  - "*.snap"           # Jest snapshots
  - "**/testdata/**"   # Test fixtures
  - "*.pb.go"          # Protobuf generated
```

## Cost Considerations

- Uses `balanced` model for classification and analysis
- Uses `strategic` model only for split suggestions
- Cost per PR: ~$0.02-0.05
