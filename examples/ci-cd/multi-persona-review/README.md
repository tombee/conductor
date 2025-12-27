# Multi-Persona PR Review

Comprehensive code review using multiple specialized reviewer personas (security, performance, architecture, style) running in parallel.

## Why Use This?

Single reviewers often miss issues outside their expertise:
- Security engineers catch vulnerabilities but may miss performance issues
- Performance experts focus on speed but may miss security concerns
- Style reviewers nitpick formatting but overlook architectural problems

Multi-persona review ensures consistent coverage across all dimensions.

## What It Does

```
PR Opened
        ↓
[Fetch PR diff and context]        ← GitHub API
        ↓
┌─────────────────────────────────────────────┐
│  [Security]  [Performance]  [Architecture]  │  ← Parallel LLM personas
│  [Style]     [Testing]      [Accessibility] │
└─────────────────────────────────────────────┘
        ↓
[Aggregate and dedupe findings]    ← LLM
        ↓
[Post unified review]              ← GitHub API
```

## Example Output

```markdown
## 🔍 Conductor Code Review

**PR:** #234 - Add user authentication endpoints
**Reviewed by:** Security, Performance, Architecture, Style

### Summary
Found **2 blocking issues**, **3 suggestions**

**Recommendation:** ⚠️ REQUEST CHANGES

---

### 🔴 Critical Issues

#### [Security] SQL Injection Risk
**File:** `auth/handlers.go:45`
```go
query := fmt.Sprintf("SELECT * FROM users WHERE id = '%s'", userID)
```

**Issue:** User input directly interpolated into SQL query.

**Recommendation:** Use parameterized queries:
```go
query := "SELECT * FROM users WHERE id = ?"
row := db.QueryRow(query, userID)
```

---

#### [Security] Hardcoded Secret
**File:** `auth/config.go:12`
```go
const jwtSecret = "super-secret-key-123"
```

**Issue:** Secrets should not be hardcoded in source code.

**Recommendation:** Use environment variable or secrets manager.

---

### 🟡 Suggestions

#### [Performance] N+1 Query Pattern
**File:** `auth/handlers.go:78`

The loop fetches user roles individually. Consider eager loading.

#### [Architecture] Consider Dependency Injection
**File:** `auth/service.go:23`

Direct database connection in service. Consider injecting.

#### [Style] Function Length
**File:** `auth/handlers.go:30-120`

`HandleLogin` is 90 lines. Consider extracting validation.

---

### ✅ What Looks Good
- Proper password hashing with bcrypt
- Input validation on email format
- Good test coverage for happy paths
- Clear error messages for API responses

---
*Reviewed by Conductor 🎼 | Security • Performance • Architecture • Style*
```

## Usage

### Manual Review

```bash
conductor run examples/ci-cd/multi-persona-review/workflow.yaml \
  --input repo=owner/repo \
  --input pr_number=234
```

### Specific Personas Only

```bash
conductor run examples/ci-cd/multi-persona-review/workflow.yaml \
  --input repo=owner/repo \
  --input pr_number=234 \
  --input personas='["security", "performance"]'
```

### Auto-Review All PRs

```yaml
# .github/workflows/code-review.yml
name: Automated Code Review
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Code Review
        run: |
          conductor run examples/ci-cd/multi-persona-review/workflow.yaml \
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
| `personas` | array | No | All enabled | Which personas to run |

### Available Personas

| Persona | Focus | Model |
|---------|-------|-------|
| `security` | Auth, injection, data exposure, crypto | `strategic` |
| `performance` | Complexity, queries, memory, concurrency | `balanced` |
| `architecture` | SOLID, patterns, coupling, interfaces | `balanced` |
| `style` | Naming, docs, organization, error messages | `fast` |
| `testing` | Coverage, assertions, edge cases | `balanced` |
| `accessibility` | A11y concerns (frontend) | `balanced` |

### Customizing Personas

Modify the system prompt for each persona in the workflow:

```yaml
- id: security_review
  type: llm
  system: |
    You are a senior security engineer reviewing code changes.

    Focus on:
    - Authentication/Authorization
    - Input Validation
    - Your additional concerns here...

    Also check for:
    - Company-specific security requirements
    - Compliance requirements (PCI, HIPAA)
```

## How It Works

1. **Fetch PR**: Get diff, commits, and description
2. **Parallel Reviews**: Run enabled personas concurrently
3. **Aggregate**: Merge findings, deduplicate overlaps
4. **Determine Recommendation**: APPROVE, REQUEST_CHANGES, or COMMENT
5. **Post Review**: Submit as GitHub review with appropriate action

### Recommendation Logic

- **REQUEST_CHANGES**: Any CRITICAL or HIGH security issues
- **COMMENT**: Only MEDIUM or architectural concerns
- **APPROVE**: Only minor style suggestions (rare for automation)

### Model Selection

Each persona uses an appropriate model tier:
- **Security**: `strategic` - Best accuracy for vulnerabilities
- **Performance/Architecture**: `balanced` - Good tradeoff
- **Style**: `fast` - Quick analysis, lower cost

## Cost Considerations

With default 4 personas:
- Security (strategic): ~$0.03
- Performance (balanced): ~$0.02
- Architecture (balanced): ~$0.02
- Style (fast): ~$0.005

**Total per PR: ~$0.07-0.10**

Reduce cost by using fewer personas or `fast` model for all.

## Limitations

- Cannot approve/merge automatically (posts as comment or review)
- May miss context-dependent issues
- Best used alongside human review, not as replacement
