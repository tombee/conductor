# Infrastructure-as-Code Review Example

Analyze Infrastructure as Code changes (Terraform, Pulumi, CDK) to produce risk assessments and operator-friendly change summaries.

## Description

This workflow analyzes IaC plan outputs to identify dangerous changes, assess operational risks, and translate technical diffs into clear explanations for operators. It provides both risk-based recommendations (BLOCK, REQUIRES_APPROVAL, PROCEED) and plain-English summaries of what's actually changing in your infrastructure.

## Use Cases

- **Pre-deployment review** - Catch risky changes before applying to production
- **PR automation** - Add risk assessments to infrastructure PRs
- **Change approval workflows** - Route high-risk changes to appropriate approvers
- **Incident prevention** - Flag configuration changes that could cause outages

## Prerequisites

### Required

- Conductor installed ([Installation Guide](../getting-started/installation.md))
- LLM provider configured (Claude Code, Anthropic API, or OpenAI)
- IaC tool installed (Terraform, Pulumi, or AWS CDK)
- Infrastructure code with planned changes

### Optional

- CI/CD system for automation (GitHub Actions, GitLab CI, Jenkins)
- Approval system for routing high-risk changes

## How to Run It

### Basic Usage - Auto-Generate Plan

Let the workflow run the plan command:

```bash
# Terraform project
conductor run examples/iac-review \
  -i working_dir=./terraform \
  -i tool=terraform \
  -i environment=production

# Pulumi project
conductor run examples/iac-review \
  -i working_dir=./infra \
  -i tool=pulumi \
  -i environment=staging

# CDK project
conductor run examples/iac-review \
  -i working_dir=./cdk \
  -i tool=cdk \
  -i environment=dev
```

### Use Pre-Generated Plan

If you've already generated the plan, pass it directly:

```bash
# Terraform
terraform plan -no-color > plan.txt
conductor run examples/iac-review \
  -i plan_output="$(cat plan.txt)" \
  -i tool=terraform \
  -i environment=production

# Pulumi
pulumi preview --diff > preview.txt
conductor run examples/iac-review \
  -i plan_output="$(cat preview.txt)" \
  -i tool=pulumi \
  -i environment=production
```

### Strict Mode (Fail on High Risk)

Exit with non-zero code if high or critical risks are detected:

```bash
conductor run examples/iac-review \
  -i working_dir=./terraform \
  -i environment=production \
  -i strict_mode=true
```

### GitHub Actions Integration

```yaml
# .github/workflows/iac-review.yml
name: IaC Review
on:
  pull_request:
    paths: ['terraform/**']

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
        run: terraform plan -no-color > plan.txt
        working-directory: ./terraform
        continue-on-error: true

      - name: IaC Review
        run: |
          conductor run examples/iac-review \
            -i plan_output="$(cat terraform/plan.txt)" \
            -i tool=terraform \
            -i environment=${{ github.base_ref == 'main' && 'production' || 'staging' }} \
            --output-json > review.json

      - name: Comment on PR
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const review = JSON.parse(fs.readFileSync('review.json'));
            await github.rest.issues.createComment({
              issue_number: context.issue.number,
              body: review.report
            });

      - name: Block on Critical
        run: |
          if grep -q '"recommendation".*"BLOCK"' review.json; then
            echo "::error::Critical risks detected. Deployment blocked."
            exit 1
          fi
```

## Code Walkthrough

The workflow consists of four sequential steps that transform raw IaC plans into actionable reviews:

### 1. Generate Plan (Step 1)

```yaml
- id: generate_plan
  name: Generate IaC Plan
  condition: "{{not .inputs.plan_output}}"
  shell.run:
    command: |
      cd {{.inputs.working_dir}}
      {{if eq .inputs.tool "terraform"}}
      terraform plan -no-color -detailed-exitcode 2>&1 || true
      {{else if eq .inputs.tool "pulumi"}}
      pulumi preview --diff --non-interactive 2>&1 || true
      {{else if eq .inputs.tool "cdk"}}
      cdk diff 2>&1 || true
      {{end}}
```

**What it does**: Conditionally generates the IaC plan if one wasn't provided as input. Uses tool-specific commands with appropriate flags for machine-readable output.

**Conditional execution**: The `condition: "{{not .inputs.plan_output}}"` ensures this step only runs if the user didn't provide a pre-generated plan. This allows flexibility in usage—either let Conductor run the plan or provide your own.

**Error handling**: The `|| true` ensures the step doesn't fail on Terraform exit code 2 (changes present), which is expected behavior when analyzing changes.

### 2. Risk Analysis (Step 2)

```yaml
- id: risk_analysis
  name: Risk Analysis
  type: llm
  model: balanced
  system: |
    You are an infrastructure security expert analyzing IaC plan output.

    **CRITICAL Risks (block deployment):**
    - Destruction of production databases or persistent storage
    - Security group changes exposing sensitive ports to 0.0.0.0/0
    - IAM policy changes granting admin/* access
    - Disabling encryption or audit logging
    ...

    **HIGH Risks (requires approval):**
    - Any resource destruction in production
    - Network topology changes (VPC, subnets, routing)
    - Database configuration changes
    ...

    For each finding, provide: risk level, resource, what's changing, why risky, recommendation
  prompt: |
    Analyze this {{.inputs.tool}} plan for risks:
    **Environment:** {{.inputs.environment}}
    **Plan Output:** [plan content]

    Provide structured risk assessment with:
    - Risk score (0-100)
    - Deployment recommendation (PROCEED/PROCEED_WITH_CAUTION/REQUIRES_APPROVAL/BLOCK)
    - Grouped findings by severity
```

**What it does**: Performs deep analysis of the plan output to identify security and operational risks. Categorizes findings by severity and provides a numerical risk score.

**Environment awareness**: The workflow considers the target environment (`production` vs `staging` vs `dev`) when assessing risk. Deleting a database in dev is LOW risk; in production it's CRITICAL.

**Model tier choice**: Uses `balanced` tier because risk analysis requires good reasoning quality (to understand context and implications) but doesn't need the absolute highest tier. Typical response time is 5-8 seconds.

**Structured risk categories**: The system prompt defines clear criteria for each risk level, ensuring consistent assessments across different types of changes.

### 3. Operator-Friendly Summary (Step 3)

```yaml
- id: change_summary
  name: Change Summary for Operators
  type: llm
  model: fast
  system: |
    You are a technical writer creating infrastructure change summaries for
    network operators and on-call engineers.

    **What operators care about:**
    - What endpoints/services are affected?
    - Will there be downtime?
    - What ports/protocols are changing?
    - What monitoring might alert?
    - Rollback steps?

    **Avoid:**
    - Terraform resource type names
    - ARNs or resource IDs (use friendly names)
    - Implementation details (use outcomes)
  prompt: |
    Create operator-friendly summary of these changes:
    **Environment:** {{.inputs.environment}}
    **Raw Plan:** [plan content]

    Write a clear summary that a network operator can understand at 3 AM during an incident.
```

**What it does**: Translates technical IaC changes into plain-English explanations focused on operational impact. Removes jargon and focuses on what operators need to know.

**Target audience shift**: While the risk analysis is for technical reviewers and security teams, this summary targets on-call operators who may not be infrastructure experts. It answers "what services break if this fails?" not "which Terraform resources are changing?"

**Example transformation**:
- Technical: "aws_db_instance.main: instance_class db.r5.large → db.r5.xlarge"
- Operator-friendly: "Database will restart (5-10 min downtime) during instance resize"

**Model tier choice**: Uses `fast` tier since summarization and simplification don't require complex reasoning.

### 4. Consolidate Report (Step 4)

```yaml
- id: final_report
  name: Generate Final Report
  type: llm
  model: fast
  system: |
    Combine risk analysis and change summary into a single deployment review document.

    Add:
    1. Executive summary (2-3 sentences)
    2. Clear GO/NO-GO recommendation
    3. Required approvals based on risk level
  prompt: |
    Combine these analyses into final IaC change review:
    **Risk Analysis:** {{.steps.risk_analysis.response}}
    **Operator Summary:** {{.steps.change_summary.response}}

    For production + CRITICAL/HIGH risks, recommend approval from:
    - Platform team lead
    - Security team (for security changes)
    - Database team (for data changes)
```

**What it does**: Merges the technical risk analysis and operator summary into a cohesive report with clear recommendations and approval requirements.

**Approval routing logic**: Based on the environment and risk level, the LLM recommends which teams need to review. This could be extended to integrate with actual approval systems (PagerDuty, ServiceNow, etc.).

**Executive summary value**: The 2-3 sentence summary at the top allows leadership to quickly understand the scope without reading the full analysis.

## Customization Options

### 1. Add Organization-Specific Risk Criteria

Extend risk detection for your compliance requirements:

```yaml
system: |
  **CRITICAL Risks:**
  - Changes to PCI-scoped resources (add your resource tags/names)
  - Modifications to SOC2 audit logging
  - Changes to resources tagged "compliance:required"
```

### 2. Customize Team Routing

Define your team structure for approval routing:

```yaml
prompt: |
  For approval routing, use these teams:
  - Database changes → DBA team
  - Network changes → NetOps team
  - Security groups → InfoSec team
  - Kubernetes → Platform team
```

### 3. Add Cost Impact Analysis

Include a separate step for cost estimation:

```yaml
- id: cost_analysis
  type: llm
  model: fast
  prompt: |
    Estimate cost impact of these changes:
    {{if .inputs.plan_output}}{{.inputs.plan_output}}{{else}}{{.steps.generate_plan.stdout}}{{end}}

    Note:
    - New resources and their estimated monthly cost
    - Resources being scaled up/down
    - Resources being deleted (cost savings)
```

### 4. Integration with Approval Systems

Use the output to trigger approval workflows:

```bash
#!/bin/bash
# approval-workflow.sh

# Run review
conductor run examples/iac-review ... --output-json > review.json

# Extract recommendation
RECOMMENDATION=$(jq -r '.recommendation' review.json)
RISK_SCORE=$(jq -r '.risk_score' review.json)

case $RECOMMENDATION in
  *BLOCK*)
    echo "Deployment blocked - critical risks detected"
    # Create incident ticket
    create_incident "IaC deployment blocked" "$(jq -r '.report' review.json)"
    exit 1
    ;;
  *REQUIRES_APPROVAL*)
    # Create approval request in your system
    create_approval_request \
      --title "IaC Change Approval" \
      --description "$(jq -r '.report' review.json)" \
      --approvers "platform-team,security-team" \
      --risk-score "$RISK_SCORE"
    ;;
  *PROCEED*)
    echo "Safe to proceed with deployment"
    ;;
esac
```

### 5. Add Compliance Checks

Create additional validation steps:

```yaml
- id: compliance_check
  type: llm
  model: fast
  prompt: |
    Check if these changes comply with:
    - SOC2 requirements (encryption at rest, audit logging)
    - GDPR requirements (data residency, retention policies)
    - PCI-DSS requirements (network segmentation, access controls)

    Plan: {{if .inputs.plan_output}}{{.inputs.plan_output}}{{else}}{{.steps.generate_plan.stdout}}{{end}}
```

## Common Issues and Solutions

### Issue: Workflow fails to generate plan

**Symptom**: "terraform: command not found" or similar

**Solution**: Ensure the IaC tool is installed and in PATH:

```bash
# Test tool availability
which terraform
terraform version

# Or provide pre-generated plan
terraform plan -no-color > plan.txt
conductor run examples/iac-review -i plan_output="$(cat plan.txt)"
```

### Issue: Risk assessment is too conservative

**Symptom**: Everything flagged as HIGH or CRITICAL risk

**Solution**: Adjust risk thresholds in the system prompt:

```yaml
system: |
  **HIGH Risks (not CRITICAL):**
  - Resource destruction in PRODUCTION ONLY (dev/staging is MEDIUM)
  - Database changes that don't cause downtime (restarts are HIGH)
```

### Issue: Operator summary still too technical

**Symptom**: Summary includes resource names and IaC terminology

**Solution**: Add more explicit guidance in the system prompt:

```yaml
system: |
  **Translation examples:**
  - "aws_security_group ingress rule" → "Firewall rule allowing port X"
  - "aws_db_instance parameter_group" → "Database configuration change"
  - "aws_iam_policy" → "Permission changes for service Y"

  Always use friendly service names, not resource types.
```

### Issue: Plans are too large for context window

**Symptom**: Token limit errors with large plans

**Solution**: Pre-filter the plan to focus on changes:

```bash
# Terraform: Only show changes, not full plan
terraform show -no-color tfplan | grep -A5 -B5 "will be\|must be" > filtered-plan.txt

conductor run examples/iac-review -i plan_output="$(cat filtered-plan.txt)"
```

### Issue: False positives on drift

**Symptom**: Workflow flags changes that weren't in the commit

**Solution**: Run `terraform refresh` before planning to sync state:

```bash
terraform refresh
terraform plan -no-color > plan.txt
conductor run examples/iac-review -i plan_output="$(cat plan.txt)"
```

## Related Examples

- [Code Review](code-review.md) - Similar multi-perspective review pattern
- [Issue Triage](issue-triage.md) - Classification and routing patterns
- [Slack Integration](slack-integration.md) - Post review summaries to Slack

## Workflow Files

Full workflow definition: [examples/iac-review/workflow.yaml](https://github.com/tombee/conductor/blob/main/examples/iac-review/workflow.yaml)

## Further Reading

- [Sequential Processing Pattern](../building-workflows/patterns.md#sequential-processing)
- [Conditional Execution](../building-workflows/flow-control.md#conditional-execution)
- [Error Handling](../building-workflows/error-handling.md)
- [CI/CD Integration](../building-workflows/daemon-mode.md)
