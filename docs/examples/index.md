# Examples

Example workflows demonstrating Conductor's capabilities.

This section showcases real-world workflows from the [examples/](https://github.com/tombee/conductor/tree/main/examples) directory. Each example includes complete workflow files and documentation.

## Available Examples

### Development Workflows

- **[Code Review](/examples/code-review/)** - Multi-persona AI code review analyzing security, performance, and style
- **[CI/CD Integration](/examples/templates/ci-cd/)** - Continuous integration workflows

### DevOps & Infrastructure

- **[IaC Review](/examples/devops/iac-review/)** - Infrastructure-as-Code review for Terraform and CloudFormation
- **[Security Audit](/examples/devops/security-audit/)** - Security vulnerability scanning and analysis

### Automation

- **[Issue Triage](/examples/automation/issue-triage/)** - Automated GitHub issue classification and labeling
- **[Slack Integration](/examples/automation/slack-integration/)** - Post messages and interact with Slack channels

### Creative

- **[Write Song](/examples/templates/write-song/)** - Generate songs with lyrics and chord symbols in various genres

## Example Structure

Each example includes:

- **README** - Overview, use cases, and prerequisites
- **Workflow YAML** - Complete workflow definition
- **Documentation** - How to run and customize

## Running Examples

1. Clone the repository:
   ```bash
   git clone https://github.com/tombee/conductor.git
   cd conductor/examples
   ```

2. Navigate to an example directory:
   ```bash
   cd code-review
   ```

3. Follow the README instructions to run the workflow

## Creating Your Own

Use these examples as templates for your own workflows:

1. Copy an example that's close to your use case
2. Modify the prompts and steps
3. Adjust inputs and outputs
4. Test with `conductor run`

See the [Building Workflows](../building-workflows/) guide for more details.
