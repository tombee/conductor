# Security Audit Example

Comprehensive security analysis workflow that examines code, dependencies, and configurations for vulnerabilities and compliance issues.

## Quick Start

```bash
# Full codebase audit
conductor run examples/security-audit \
  --input code_path="./src" \
  --input check_dependencies=true \
  --input compliance_framework="OWASP" \
  --output-json > security-audit.json

# PR security review
git diff main..feature > changes.diff
conductor run examples/security-audit \
  --input code_diff="$(cat changes.diff)" \
  --input check_dependencies=false

# Block deployment on critical issues
AUDIT=$(conductor run examples/security-audit --input code_path="./src" --output-json)
if [ $(echo "$AUDIT" | jq -r '.has_critical_issues') = "true" ]; then
  echo "Critical security issues found. Deployment blocked."
  exit 1
fi
```

## Prerequisites

1. Conductor CLI installed
2. Anthropic API key configured: `export ANTHROPIC_API_KEY="your-key"`
3. Code to audit (directory path or git diff)

## Features

- Multi-aspect security analysis (code, dependencies, configuration)
- CWE/CVE vulnerability identification with severity ratings
- Compliance framework mapping (OWASP, SOC2, PCI-DSS, HIPAA)
- Prioritized remediation roadmap
- Executive-ready security reports

## Use Cases

- Pre-deployment security checks
- Pull request security reviews
- Compliance audits and reporting
- Vulnerability assessments
- Security posture tracking

## Expected Output

The workflow generates:
- Comprehensive security audit report (markdown)
- Detailed vulnerability findings (JSON)
- Compliance assessment and score
- Remediation roadmap with effort estimates

### Sample Report Structure

```markdown
# Security Audit Report

## Executive Summary
- Overall security posture: MODERATE RISK
- Critical findings: 2 (SQL injection, hardcoded credentials)
- Compliance score: 73/100 (OWASP Top 10)

## CRITICAL Findings
1. SQL Injection in login.js:42
   - Fix: Use parameterized queries
   - Effort: 2 hours
   - Priority: P0

## Remediation Roadmap
Phase 1 (This Week): Fix critical issues (9 hours, 70% risk reduction)
Phase 2 (Next 2 Weeks): Address high priority (20 hours, 20% risk reduction)
```

## Compliance Frameworks

- **OWASP Top 10** - Web application security
- **SOC2** - Trust service principles
- **PCI-DSS** - Payment card security
- **HIPAA** - Healthcare data protection

## Integration

### CI/CD Pipeline

```yaml
- name: Security Audit
  run: |
    conductor run examples/security-audit \
      --input code_path="./src" \
      --output-json > audit.json

    if [ $(jq -r '.has_critical_issues' audit.json) = "true" ]; then
      exit 1
    fi
```

### Pre-commit Hook

```bash
#!/bin/bash
git diff --cached > /tmp/changes.diff
conductor run examples/security-audit \
  --input code_diff="$(cat /tmp/changes.diff)" \
  --input check_dependencies=false
```

## Documentation

For detailed usage, customization options, and best practices, see:
[Security Audit Documentation](../../docs/examples/security-audit.md)
