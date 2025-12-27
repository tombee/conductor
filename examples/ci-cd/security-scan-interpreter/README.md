# Security Scan Interpreter

Interpret security scan results from tools like Snyk, Trivy, or CodeQL. Assess actual risk in context and prioritize fixes.

## Why Use This?

Security scanners produce high volumes of alerts:
- Many are false positives in your context
- Same vulnerability reported multiple times
- CVSS scores don't reflect your deployment
- Developers get overwhelmed by noise

This workflow applies contextual reasoning to prioritize what actually matters.

## What It Does

```
Security Scan Results (SARIF/JSON)
        ↓
[Parse findings]              ← Deterministic (file parsing)
        ↓
[Assess exploitability]       ← LLM reasoning
[Check reachability]
[Consider deployment context]
        ↓
[Generate prioritized report] ← LLM
[Alert on critical findings]  ← Slack
```

## Example Output

```markdown
# Security Scan Analysis - 2025-12-27

**Scan:** Snyk dependency scan
**Context:** public deployment
**Total Findings:** 47 → **Actionable:** 8

## 🔴 Critical (Fix Immediately)

### CVE-2025-1234: Remote Code Execution in yaml-parser
- **Risk:** CRITICAL (reachable, public endpoint)
- **Location:** `pkg/config/loader.go:45`
- **Analysis:** This dep is used to parse user-uploaded YAML configs
  in the public API. The vulnerable `parse()` function is directly called.
- **Fix:** Upgrade yaml-parser to v2.1.0
- **Effort:** ~30 minutes

## 🟠 High (Fix This Sprint)

### CVE-2025-5678: SQL Injection in db-driver
- **Risk:** MEDIUM (originally HIGH, but mitigated)
- **Analysis:** While the CVE is severe, our use is behind parameterized
  queries. However, upgrade recommended.
- **Fix:** Upgrade db-driver to v3.2.1

## ⚪ False Positives (No Action)

### CVE-2025-9999: XSS in frontend-lib (39 instances)
- **Reason:** This dependency is only used in build tooling,
  not shipped to production.
```

## Usage

### After Running a Security Scan

```bash
# Run Trivy and save results
trivy fs --format sarif -o scan-results.sarif .

# Interpret results
conductor run examples/ci-cd/security-scan-interpreter/workflow.yaml \
  --input scan_file=scan-results.sarif \
  --input scan_format=sarif \
  --input deployment_context=public
```

### CI Integration

```yaml
# .github/workflows/security.yml
name: Security Scan
on: [push, pull_request]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Trivy
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'fs'
          format: 'sarif'
          output: 'trivy-results.sarif'

      - name: Interpret Results
        run: |
          conductor run examples/ci-cd/security-scan-interpreter/workflow.yaml \
            --input scan_file=trivy-results.sarif \
            --input deployment_context=public \
            --input slack_channel="#security-alerts"
        env:
          SLACK_TOKEN: ${{ secrets.SLACK_TOKEN }}

      - name: Upload Report
        uses: actions/upload-artifact@v4
        with:
          name: security-report
          path: security-report.md
```

## Configuration

### Inputs

| Input | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `scan_file` | string | Yes | - | Path to scan results file |
| `scan_format` | string | No | `sarif` | Format: `sarif`, `snyk`, `trivy` |
| `deployment_context` | string | No | `public` | Context: `internal`, `public`, `sensitive` |
| `slack_channel` | string | No | - | Channel for critical alerts |

### Deployment Contexts

- **internal**: Internal tool, not exposed to internet
- **public**: Public-facing service, higher risk
- **sensitive**: Handles PII, financial data, etc.

The LLM adjusts risk assessment based on context.

### Supported Scan Formats

- **SARIF**: GitHub CodeQL, Semgrep, many others
- **Snyk**: Snyk JSON output
- **Trivy**: Trivy JSON output

## How It Works

1. **Read Scan**: Load scan results from file
2. **Parse**: Extract structured findings
3. **Analyze**: For each finding, LLM assesses:
   - Is the vulnerable code reachable?
   - What's the real-world impact?
   - Does deployment context change the risk?
4. **Group**: Consolidate related findings (same root cause)
5. **Report**: Generate prioritized markdown report
6. **Alert**: Slack notification for critical findings

### Risk Assessment Logic

The LLM considers:
- **Reachability**: Can external input reach the vulnerable code?
- **Authentication**: Is the path behind auth?
- **Data sensitivity**: What data could be exposed?
- **Deployment**: Internal vs public vs sensitive

## Benefits Over Raw Scanner Output

| Scanner Output | Interpreted Output |
|----------------|-------------------|
| 47 findings, all HIGH | 8 actionable, prioritized |
| Same CVE 39 times | Grouped by root cause |
| CVSS 9.8 | "MEDIUM - behind auth" |
| No context | "This is used for..." |
| "Upgrade X" | "Upgrade X to 2.1.0, ~30 min" |

## Cost Considerations

- Uses `strategic` model for risk assessment (best accuracy)
- Uses `fast` model for parsing
- Cost scales with number of findings

Estimated cost: ~$0.05-0.20 per scan
