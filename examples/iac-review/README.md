# IaC Change Review

Analyze Infrastructure as Code changes from Terraform, Pulumi, or CDK to produce a **Risk Summary** and **Operator-Friendly Change Summary**.

## Why Use an LLM for This?

IaC plan outputs are verbose and technical. Important changes hide in walls of text:
- A security group rule change is just one line among hundreds
- Resource destruction looks the same as resource creation at a glance
- The operational impact of changes isn't obvious from resource diffs

This workflow uses LLM reasoning to:
1. **Identify risks** - Flag dangerous changes that humans might miss
2. **Assess context** - Understand that deleting a database in production is worse than in dev
3. **Translate for operators** - Convert technical diffs into "what does this mean at 3 AM?"

## Features

- **Multi-tool support**: Terraform, Pulumi, CDK
- **Risk scoring**: 0-100 score with GO/NO-GO recommendation
- **Anomaly detection**: Flags unexpected changes and drift
- **Operator summaries**: Plain English explanations for on-call engineers
- **Environment awareness**: Different thresholds for dev vs production
- **Approval routing**: Recommends who needs to approve based on risk

## Usage

### Basic Usage - Run Plan Inline

```bash
# Terraform project
conductor run examples/iac-review \
  --input working_dir=./infrastructure \
  --input tool=terraform \
  --input environment=production

# Pulumi project
conductor run examples/iac-review \
  --input working_dir=./infra \
  --input tool=pulumi \
  --input environment=staging

# CDK project
conductor run examples/iac-review \
  --input working_dir=./cdk \
  --input tool=cdk \
  --input environment=dev
```

### Provide Pre-Generated Plan

If you've already run the plan, pass it directly:

```bash
# Terraform
terraform plan -no-color > plan.txt
conductor run examples/iac-review \
  --input plan_output="$(cat plan.txt)" \
  --input tool=terraform \
  --input environment=production

# Pulumi
pulumi preview --diff > preview.txt
conductor run examples/iac-review \
  --input plan_output="$(cat preview.txt)" \
  --input tool=pulumi \
  --input environment=production
```

### CI/CD Integration

#### GitHub Actions

```yaml
name: IaC Review
on:
  pull_request:
    paths:
      - 'terraform/**'
      - 'infrastructure/**'

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Terraform
        uses: hashicorp/setup-terraform@v3

      - name: Terraform Init
        run: terraform init
        working-directory: ./terraform

      - name: Generate Plan
        id: plan
        run: terraform plan -no-color -out=tfplan 2>&1 | tee plan.txt
        working-directory: ./terraform
        continue-on-error: true

      - name: IaC Review
        id: review
        run: |
          conductor run examples/iac-review \
            --input plan_output="$(cat terraform/plan.txt)" \
            --input tool=terraform \
            --input environment=${{ github.base_ref == 'main' && 'production' || 'staging' }} \
            --output-json > review.json

      - name: Comment on PR
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const review = JSON.parse(fs.readFileSync('review.json', 'utf8'));
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: review.report
            });

      - name: Block on Critical
        run: |
          if grep -q "BLOCK" review.json; then
            echo "::error::Critical risks detected. Deployment blocked."
            exit 1
          fi
```

#### GitLab CI

```yaml
iac-review:
  stage: validate
  script:
    - terraform init
    - terraform plan -no-color > plan.txt
    - |
      conductor run examples/iac-review \
        --input plan_output="$(cat plan.txt)" \
        --input tool=terraform \
        --input environment=${CI_ENVIRONMENT_NAME:-staging} \
        --output-json > review.json
    - cat review.json | jq -r '.report'
  artifacts:
    reports:
      dotenv: review.env
    paths:
      - review.json
  rules:
    - changes:
        - "terraform/**/*"
```

#### Jenkins Pipeline

```groovy
pipeline {
    agent any
    stages {
        stage('IaC Review') {
            steps {
                dir('terraform') {
                    sh 'terraform init'
                    sh 'terraform plan -no-color > plan.txt'
                }
                sh '''
                    conductor run examples/iac-review \
                        --input plan_output="$(cat terraform/plan.txt)" \
                        --input tool=terraform \
                        --input environment=${ENVIRONMENT} \
                        --output-json > review.json
                '''
                script {
                    def review = readJSON file: 'review.json'
                    if (review.report.contains('BLOCK')) {
                        error('Critical risks detected. Deployment blocked.')
                    }
                }
            }
        }
    }
}
```

## Example Output

```markdown
## Executive Summary

This change modifies the production RDS instance configuration and updates
security group rules. Risk score: 72/100. Requires platform team approval
before proceeding.

## Recommendation: PROCEED_WITH_CAUTION

### Required Approvals
- Platform team lead (database configuration change)
- Security team (security group modification)

---

## Risk Score: 72/100

## Critical Risks
None detected.

## High Risks

### 1. RDS Instance Modification
- **Resource:** aws_db_instance.production
- **Change:** Modifying instance class from db.r5.large to db.r5.xlarge
- **Risk:** Database restart required, potential 5-10 minute downtime
- **Recommendation:** Schedule during maintenance window

### 2. Security Group Update
- **Resource:** aws_security_group.api_servers
- **Change:** Adding ingress rule for port 443 from 10.0.0.0/8
- **Risk:** Expands network access to internal CIDR
- **Recommendation:** Verify source CIDR is expected

## Medium Risks

### 1. New IAM Role
- **Resource:** aws_iam_role.lambda_execution
- **Change:** Creating new role with S3 read access
- **Risk:** New permissions being introduced
- **Recommendation:** Review policy document

---

## Services Affected
- **API Service**: Security group rules changing, may affect connectivity
- **Database**: Configuration update, expect brief restart

## Network Changes
- New ingress rule allowing 10.0.0.0/8 to reach API servers on port 443
- No DNS changes
- No routing changes

## Potential Impact
- 5-10 minute database restart during RDS modification
- No expected API downtime (security group changes are immediate)
- CloudWatch may alert on RDS metric gaps during restart

## Action Required
- Schedule change during maintenance window (recommended: 2-4 AM)
- Notify on-call that RDS restart is expected
- No manual intervention needed

## Rollback
- RDS instance class change is reversible (another restart required)
- Security group rule can be removed immediately
- IAM role can be deleted if unused
```

## Configuration Options

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `working_dir` | string | `.` | Path to IaC project directory |
| `tool` | string | `terraform` | IaC tool: `terraform`, `pulumi`, `cdk` |
| `plan_output` | string | - | Pre-generated plan (skips running plan command) |
| `environment` | string | `unknown` | Target environment: `dev`, `staging`, `production` |
| `strict_mode` | boolean | `false` | Exit non-zero on HIGH or CRITICAL risks |

## Customization

### Adjust Risk Thresholds

Modify the `risk_analysis` step's system prompt to adjust what's considered CRITICAL vs HIGH:

```yaml
# In workflow.yaml, adjust the system prompt:
system: |
  # Add your organization's specific risk criteria
  **CRITICAL Risks:**
  - Changes to PCI-scoped resources
  - Modifications to SOC2 audit logging
  - Your custom criteria...
```

### Add Team-Specific Routing

Extend the `final_report` step to route to your teams:

```yaml
prompt: |
  ...
  For database changes, require DBA approval.
  For network changes, require NetOps approval.
  For IAM changes, require Security approval.
```

### Integrate with Approval Systems

Use the JSON output to drive approval workflows:

```bash
# Get recommendation
recommendation=$(conductor run examples/iac-review ... --output-json | jq -r '.recommendation')

case $recommendation in
  *BLOCK*)
    echo "Deployment blocked"
    exit 1
    ;;
  *REQUIRES_APPROVAL*)
    # Create approval request in your system
    create_approval_request "$recommendation"
    ;;
  *PROCEED*)
    echo "Safe to proceed"
    ;;
esac
```

## Related Examples

- [Security Audit](../security-audit/) - Code security analysis
- [Code Review](../code-review/) - Multi-persona code review
