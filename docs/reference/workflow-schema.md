# Workflow Schema Reference

Complete YAML schema reference for Conductor workflows.

## Overview

Conductor workflows are defined in YAML files with a declarative structure. This document provides a comprehensive reference for all available fields and options.

## Minimal Workflow

The most basic workflow requires only a name and steps:

```conductor
name: hello-world
steps:
  - id: greet
    type: llm
    prompt: "Say hello to the world"
```

## Schema Structure

### Top-Level Fields

#### name (required)
**Type:** `string`

Unique identifier for this workflow.

```conductor
name: code-review-workflow
```

#### description
**Type:** `string`

Human-readable description of what this workflow does.

```conductor
description: "Performs multi-step code review with security and performance checks"
```

#### version
**Type:** `string`
**Default:** `"1.0"`

Workflow schema version. Optional field that defaults to "1.0".

```conductor
version: "1.0"
```

#### inputs
**Type:** `array` of Input objects

Defines expected input parameters for the workflow.

```conductor
inputs:
  - name: code
    type: string
    required: true
    description: "Source code to review"
  - name: language
    type: string
    required: false
    default: "go"
    description: "Programming language"
```

See [Input Definition](#input-definition) for details.

#### steps (required)
**Type:** `array` of Step objects

Executable units of the workflow, run sequentially unless otherwise specified.

```conductor
steps:
  - id: analyze
    type: llm
    prompt: "Analyze this code: {{.code}}"
  - id: summarize
    type: llm
    prompt: "Summarize: {{.steps.analyze.response}}"
```

See [Step Definition](#step-definition) for details.

#### outputs
**Type:** `array` of Output objects

Defines what data is returned when the workflow completes.

```conductor
outputs:
  - name: summary
    type: string
    value: "$.summarize.response"
    description: "Final code review summary"
```

See [Output Definition](#output-definition) for details.

#### triggers
**Type:** `array` of Trigger objects

Defines how this workflow can be invoked (webhooks, schedules, manual).

```conductor
triggers:
  - type: webhook
    webhook:
      path: /webhooks/code-review
      source: github
      events: [pull_request]
  - type: schedule
    schedule:
      cron: "0 9 * * MON"
      timezone: "America/New_York"
```

See [Trigger Definition](#trigger-definition) for details.

#### agents
**Type:** `map[string]Agent`

Named agents with provider preferences and capability requirements.

```conductor
agents:
  vision-agent:
    prefers: anthropic
    capabilities: [vision, tool-use]
  fast-agent:
    capabilities: [streaming]
```

See [Agent Definition](#agent-definition) for details.

---

## Input Definition

Describes a workflow input parameter.

### Fields

#### name (required)
**Type:** `string`

Input parameter identifier.

```conductor
name: user_query
```

#### type (required)
**Type:** `string`
**Values:** `string`, `number`, `boolean`, `object`, `array`

Data type of the input.

```conductor
type: string
```

#### required
**Type:** `boolean`
**Default:** `false`

Whether this input must be provided.

```conductor
required: true
```

#### default
**Type:** `any`

Fallback value if input is not provided.

```conductor
default: "balanced"
```

#### description
**Type:** `string`

Explanation of what this input is for.

```conductor
description: "User's natural language query"
```

### Example

```conductor
inputs:
  - name: file_path
    type: string
    required: true
    description: "Path to file to analyze"
  - name: model_tier
    type: string
    required: false
    default: "balanced"
    description: "Model tier to use (fast, balanced, strategic)"
```

---

## Step Definition

Represents a single step in a workflow.

### Common Fields

#### id (required)
**Type:** `string`

Unique step identifier within this workflow.

```conductor
id: analyze_code
```

#### name
**Type:** `string`

Human-readable step name (optional).

```conductor
name: "Code Analysis"
```

#### type (required)
**Type:** `string`
**Values:** `llm`, `tool`, `action`, `condition`, `parallel`

Specifies the step type.

```conductor
type: llm
```

#### timeout
**Type:** `integer`
**Default:** `30`
**Unit:** seconds

Maximum execution time for this step.

```conductor
timeout: 60
```

#### retry
**Type:** Retry object

Configures retry behavior.

```conductor
retry:
  max_attempts: 3
  backoff_base: 2
  backoff_multiplier: 2.0
```

See [Retry Definition](#retry-definition) for details.

#### on_error
**Type:** ErrorHandling object

Specifies error handling behavior.

```conductor
on_error:
  strategy: fallback
  fallback_step: error_handler
```

See [Error Handling Definition](#error-handling-definition) for details.

---

### LLM Step (type: llm)

Makes an LLM API call for text generation, analysis, or transformation.

#### prompt (required)
**Type:** `string`

User prompt for the LLM. Supports template variables:
- `{{.input_name}}` - workflow inputs
- `{{.steps.step_id.response}}` - previous step outputs

```conductor
prompt: "Review this code: {{.code}}"
```

#### model
**Type:** `string`
**Values:** `fast`, `balanced`, `strategic`
**Default:** `balanced`

Model tier selection. Abstracts specific provider models.

- **fast**: Quick tasks, low cost (e.g., haiku, gpt-4o-mini)
- **balanced**: Most tasks, good quality/cost tradeoff (e.g., sonnet, gpt-4o)
- **strategic**: Complex reasoning, advanced capabilities (e.g., opus, o1)

```conductor
model: strategic
```

#### system
**Type:** `string`

System prompt to guide model behavior (optional).

```conductor
system: "You are a security expert reviewing code for vulnerabilities."
```

#### agent
**Type:** `string`

References an agent definition for provider resolution.

```conductor
agent: vision-agent
```

#### output_schema
**Type:** `object`

JSON Schema defining expected structured output.

```conductor
output_schema:
  type: object
  properties:
    severity:
      type: string
      enum: [low, medium, high, critical]
    issues:
      type: array
      items:
        type: string
  required: [severity, issues]
```

**Limits:**
- Maximum nesting depth: 10 levels
- Maximum properties: 100
- Maximum size: 64KB

#### output_type
**Type:** `string`
**Values:** `classification`, `decision`, `extraction`
**Mutually exclusive with:** `output_schema`

Built-in output type that expands to a schema.

**classification** - Categorizes input into predefined categories:
```conductor
output_type: classification
output_options:
  categories: [bug, feature, documentation, question]
```

**decision** - Makes a choice with optional reasoning:
```conductor
output_type: decision
output_options:
  choices: [approve, reject, needs-changes]
  require_reasoning: true
```

**extraction** - Extracts specific fields:
```conductor
output_type: extraction
output_options:
  fields: [name, email, phone]
```

#### output_options
**Type:** `object`

Configuration for built-in output types.

See examples under `output_type`.

### Example LLM Steps

**Simple prompt:**
```conductor
- id: summarize
  type: llm
  prompt: "Summarize this in 3 sentences: {{.text}}"
```

**With system prompt and model tier:**
```conductor
- id: security_review
  type: llm
  model: strategic
  system: "You are a security expert. Identify vulnerabilities."
  prompt: "Review: {{.code}}"
```

**With structured output:**
```conductor
- id: classify_issue
  type: llm
  prompt: "Classify this issue: {{.issue_text}}"
  output_type: classification
  output_options:
    categories: [bug, feature, question, documentation]
```

---

### Connector Steps (Shorthand Syntax)

Connector steps execute operations on builtin or external integrations. The preferred syntax is **shorthand** where you don't need to specify `type: connector` explicitly.

#### Shorthand Pattern

**Format:** `connector.operation: inputs`

The integration and operation are specified as a single key following the pattern `connector.operation:`. The value becomes the inputs for that operation.

```conductor
# Inline form (simple inputs)
- file.read: ./config.json
- shell.run: git status
- http.get: https://api.example.com/data

# Block form (complex inputs)
- github.create_issue:
    owner: myorg
    repo: myrepo
    title: "Bug Report"
    body: "{{.steps.analyze.findings}}"
```

#### Auto-Generated IDs

Steps using shorthand syntax automatically get IDs in the format `{connector}_{operation}_{N}`:

```conductor
steps:
  - file.read: ./config.json      # id: file_read_1
  - file.read: ./data.json        # id: file_read_2
  - github.create_issue: {...}    # id: github_create_issue_1
```

You can still provide an explicit `id:` when needed:

```conductor
- id: load_config
  file.read: ./config.json
```

#### Builtin Connectors

These connectors work without configuration:

**file** - File system operations
```conductor
# Read a file
- file.read: ./data.json

# Write a file
- file.write:
    path: ./output.txt
    content: "{{.steps.generate.response}}"

# List directory
- file.list:
    path: ./logs
    pattern: "*.log"
```

**shell** - Execute shell commands
```conductor
# Simple command (string form)
- shell.run: git status

# Command with arguments (array form - safer)
- shell.run:
    command: ["git", "commit", "-m", "{{.message}}"]
```

**http** - HTTP requests
```conductor
# GET request
- http.get: https://api.github.com/repos/owner/repo

# POST with body
- http.post:
    url: https://api.example.com/data
    headers:
      Content-Type: application/json
    body:
      query: "{{.search_term}}"
```

**transform** - Data transformations
```conductor
# Parse JSON
- transform.parse_json: "{{.steps.fetch.body}}"

# Filter array
- transform.filter:
    data: "{{.steps.list.items}}"
    expr: '.priority == "high"'
```

#### External Connectors

External connectors require configuration in the `integrations:` section:

```conductor
integrations:
  github:
    from: connectors/github
    auth:
      token: ${GITHUB_TOKEN}

steps:
  # Use the configured connector
  - github.create_issue:
      owner: myorg
      repo: myrepo
      title: "Automated Issue"
```

#### Verbose Connector Syntax (Optional)

For complex cases, you can use the verbose syntax:

```conductor
- id: create_issue
  type: connector
  connector: github
  operation: create_issue
  inputs:
    owner: myorg
    repo: myrepo
    title: "Bug Report"
  timeout: 60
  retry:
    max_attempts: 3
```

This is equivalent to the shorthand but allows step-level configuration like `timeout` and `retry`.

---

### Condition Step (type: condition)

Evaluates a condition and branches execution.

#### condition (required)
**Type:** Condition object

Defines the condition and branching logic.

```conductor
condition:
  expression: "$.analyze.severity == 'critical'"
  then_steps: [alert_team]
  else_steps: [log_result]
```

See [Condition Definition](#condition-definition) for details.

---

### Parallel Step (type: parallel)

Executes multiple steps concurrently.

#### steps (required)
**Type:** `array` of Step objects

Nested steps that run in parallel.

```conductor
steps:
  - id: check_style
    type: llm
    prompt: "Check code style"
  - id: check_security
    type: llm
    prompt: "Check security"
```

#### foreach (optional)
**Type:** `string` (template expression evaluating to array)

Enables iteration over an array, executing the nested steps once for each element. Each iteration has access to special context variables:

- `.item` - Current array element
- `.index` - Zero-based index of current element
- `.total` - Total number of elements in the array

**Requirements:**
- Input must be an array (returns `ErrorTypeTypeError` if not)
- Empty array produces empty results (0 iterations)
- Results maintain original array order (by index, not completion time)
- All-or-nothing error semantics (if any iteration fails, the entire step fails)
- Nested `foreach` is not allowed (parallel steps within `foreach` cannot use `foreach`)

**Example:**

```conductor
# Split array and process each element
- id: split_issues
  transform.split: '{{.steps.analyze.issues}}'

- id: process_each
  type: parallel
  foreach: '{{.steps.split_issues}}'
  steps:
    - id: fix_issue
      type: llm
      prompt: |
        Fix this issue:
        File: {{.item.file}}
        Line: {{.item.line}}
        Description: {{.item.description}}

        Processing item {{.index}} of {{.total}}
```

### Example Parallel Steps

**Fixed parallel execution:**

```conductor
- id: parallel_checks
  type: parallel
  steps:
    - id: lint
      shell.run: [golangci-lint, run]
    - id: test
      shell.run: [go, test, ./...]
```

**Dynamic array iteration with foreach:**

```conductor
- id: review_files
  type: parallel
  foreach: '{{.steps.changed_files}}'
  steps:
    - id: analyze_file
      type: llm
      model: claude-sonnet-4
      prompt: |
        Analyze this file for issues:
        Path: {{.item.path}}
        Changes: {{.item.diff}}

        File {{add .index 1}} of {{.total}}
```

---

## Output Definition

Describes a workflow output value.

### Fields

#### name (required)
**Type:** `string`

Output identifier.

```conductor
name: result
```

#### type (required)
**Type:** `string`
**Values:** `string`, `number`, `boolean`, `object`, `array`

Output data type.

```conductor
type: string
```

#### value (required)
**Type:** `string`

JSONPath expression that computes the output value.

- `$.step_id.response` - step's full response
- `$.step_id.field_name` - specific field from structured output

```conductor
value: "$.final_step.response"
```

#### description
**Type:** `string`

Explanation of what this output represents.

```conductor
description: "Final code review summary"
```

### Example

```conductor
outputs:
  - name: severity
    type: string
    value: "$.analyze.severity"
    description: "Issue severity level"
  - name: issues
    type: array
    value: "$.analyze.issues"
    description: "List of identified issues"
```

---

## Trigger Definition

Defines how a workflow can be triggered.

### Fields

#### type (required)
**Type:** `string`
**Values:** `webhook`, `schedule`, `poll`, `manual`

Trigger type.

```conductor
type: webhook
```

#### webhook
**Type:** Webhook object

Configuration for webhook triggers (when type is `webhook`).

```conductor
webhook:
  path: /webhooks/my-workflow
  source: github
  events: [push, pull_request]
  secret: ${WEBHOOK_SECRET}
```

See [Webhook Trigger](#webhook-trigger) for details.

#### schedule
**Type:** Schedule object

Configuration for schedule triggers (when type is `schedule`).

```conductor
schedule:
  cron: "0 9 * * *"
  timezone: "America/New_York"
  enabled: true
```

See [Schedule Trigger](#schedule-trigger) for details.

#### poll
**Type:** Poll object

Configuration for poll triggers (when type is `poll`).

```conductor
poll:
  integration: pagerduty
  query:
    user_id: "PUSER123"
    statuses: [triggered, acknowledged]
  interval: 30s
```

See [Poll Trigger](#poll-trigger) for details.

---

### Webhook Trigger

#### path (required)
**Type:** `string`

URL path for the webhook (e.g., `/webhooks/my-workflow`).

```conductor
path: /webhooks/code-review
```

#### source
**Type:** `string`
**Values:** `github`, `slack`, `generic`

Webhook source type.

```conductor
source: github
```

#### events
**Type:** `array` of strings

Limits which events trigger the workflow.

```conductor
events: [push, pull_request]
```

#### secret
**Type:** `string`

Secret for signature verification. Can be environment variable reference like `${SECRET_NAME}`.

```conductor
secret: ${GITHUB_WEBHOOK_SECRET}
```

#### input_mapping
**Type:** `object`

Maps webhook payload fields to workflow inputs.

```conductor
input_mapping:
  repo: "repository.name"
  pr_number: "pull_request.number"
```

---

### Schedule Trigger

#### cron (required)
**Type:** `string`

Cron expression for scheduling.

```conductor
cron: "0 9 * * MON"  # 9 AM every Monday
```

#### timezone
**Type:** `string`
**Default:** `"UTC"`

Timezone for cron evaluation.

```conductor
timezone: "America/New_York"
```

#### enabled
**Type:** `boolean`
**Default:** `true`

Controls if this schedule is active.

```conductor
enabled: false
```

#### inputs
**Type:** `object`

Static inputs to pass when scheduled.

```conductor
inputs:
  environment: production
  notify: true
```

---

### Poll Trigger

Poll triggers periodically query external service APIs for events and fire workflows for new events. This enables personal automation without webhooks or public endpoints.

#### integration (required)
**Type:** `string`
**Values:** `pagerduty`, `slack`, `jira`, `datadog`

Which integration to poll.

```conductor
integration: pagerduty
```

#### query (required)
**Type:** `object`

Integration-specific query parameters. The structure varies by integration.

**PagerDuty Example:**
```conductor
query:
  user_id: "PUSER123"           # Your PagerDuty user ID
  services: ["PSVC001", "PSVC002"]  # Services you care about
  statuses: [triggered, acknowledged]
  urgencies: [high]
```

**Slack Example:**
```conductor
query:
  mentions: "@jsmith"           # Your Slack username
  channels: [engineering, oncall]
  include_threads: true
  exclude_bots: true
```

**Jira Example:**
```conductor
query:
  assignee: "jsmith"            # Your Jira username
  project: MYTEAM
  issue_types: [Bug, Story]
  statuses: ["In Progress", "To Do"]
```

**Datadog Example:**
```conductor
query:
  tags: ["service:api", "team:platform"]  # Services you own
  monitor_ids: [12345, 67890]
  statuses: [triggered, warn]
```

#### interval
**Type:** `string` (duration)
**Default:** `"30s"`
**Minimum:** `"10s"`

How often to poll the integration.

```conductor
interval: 60s  # Poll every minute
```

#### startup
**Type:** `string`
**Values:** `since_last`, `ignore_historical`, `backfill`
**Default:** `"since_last"`

Behavior on controller start:
- `since_last`: Process events since last poll time (default)
- `ignore_historical`: Only process events from now forward
- `backfill`: Process events from specified duration ago

```conductor
startup: since_last
```

#### backfill
**Type:** `string` (duration)
**Max:** `"24h"`

When `startup: backfill`, how far back to process events.

```conductor
startup: backfill
backfill: 4h  # Process last 4 hours of events on startup
```

#### input_mapping
**Type:** `object`

Maps event fields to workflow inputs using template syntax.

```conductor
input_mapping:
  incident_id: "{{.trigger.event.id}}"
  incident_title: "{{.trigger.event.title}}"
  urgency: "{{.trigger.event.urgency}}"
```

**Available Template Context:**
- `{{.trigger.event.*}}` - Event data from the integration
- `{{.trigger.integration}}` - Integration name (e.g., "pagerduty")
- `{{.trigger.poll_time}}` - When the poll executed
- `{{.trigger.query.*}}` - Query parameters for debugging

---

## Agent Definition

Describes an agent with provider preferences and capability requirements.

### Fields

#### prefers
**Type:** `string`
**Values:** Provider names (e.g., `anthropic`, `openai`, `ollama`)

Hint about which provider family works best (not enforced).

```conductor
prefers: anthropic
```

#### capabilities
**Type:** `array` of strings
**Values:** `vision`, `long-context`, `tool-use`, `streaming`, `json-mode`

Required provider capabilities.

```conductor
capabilities: [vision, tool-use]
```

### Example

```conductor
agents:
  vision-agent:
    prefers: anthropic
    capabilities: [vision, tool-use]
  fast-responder:
    capabilities: [streaming]
  data-extractor:
    prefers: openai
    capabilities: [json-mode]
```

---

## Condition Definition

Defines a conditional expression for branching.

### Fields

#### expression (required)
**Type:** `string`

JSONPath condition to evaluate.

```conductor
expression: "$.previous_step.status == 'success'"
```

#### then_steps
**Type:** `array` of strings

Step IDs to execute if condition is true.

```conductor
then_steps: [success_handler, notify]
```

#### else_steps
**Type:** `array` of strings

Step IDs to execute if condition is false.

```conductor
else_steps: [error_handler]
```

### Example

```conductor
- id: check_severity
  type: condition
  condition:
    expression: "$.analyze.severity == 'critical'"
    then_steps: [alert_team, create_ticket]
    else_steps: [log_result]
```

---

## Error Handling Definition

Defines how to handle step errors.

### Fields

#### strategy (required)
**Type:** `string`
**Values:** `fail`, `ignore`, `retry`, `fallback`

Error handling approach.

- **fail**: Stop workflow execution on error (default)
- **ignore**: Continue workflow despite error
- **retry**: Retry according to retry configuration
- **fallback**: Execute fallback step on error

```conductor
strategy: fallback
```

#### fallback_step
**Type:** `string`

Step ID to execute on error (required when strategy is `fallback`).

```conductor
fallback_step: error_handler
```

### Example

```conductor
on_error:
  strategy: fallback
  fallback_step: use_cached_result
```

---

## Retry Definition

Configures retry behavior for a step.

### Fields

#### max_attempts
**Type:** `integer`
**Default:** `2`

Maximum number of retry attempts.

```conductor
max_attempts: 3
```

#### backoff_base
**Type:** `integer`
**Default:** `1`
**Unit:** seconds

Base duration for exponential backoff.

```conductor
backoff_base: 2
```

#### backoff_multiplier
**Type:** `float`
**Default:** `2.0`

Multiplier for exponential backoff.

```conductor
backoff_multiplier: 1.5
```

### Example

```conductor
retry:
  max_attempts: 5
  backoff_base: 1
  backoff_multiplier: 2.0
```

**Retry schedule:** 1s, 2s, 4s, 8s, 16s

---

## Template Variables

Template variables allow dynamic data flow between workflow inputs and step outputs.

### Syntax

Use double curly braces with dot notation:

```
{{.variable_name}}
{{.steps.step_id.field}}
```

### Workflow Inputs

Reference workflow inputs directly:

```conductor
prompt: "Analyze {{.language}} code: {{.code}}"
```

### Step Outputs

Reference previous step outputs:

```conductor
prompt: "Summarize this analysis: {{.steps.analyze.response}}"
```

For structured outputs, access specific fields:

```conductor
prompt: "The severity is {{.steps.analyze.severity}}"
```

### Example

```conductor
name: code-review
inputs:
  - name: code
    type: string
    required: true
  - name: language
    type: string
    default: "go"

steps:
  - id: analyze
    type: llm
    prompt: "Analyze this {{.language}} code for bugs: {{.code}}"
    output_type: extraction
    output_options:
      fields: [severity, issues]

  - id: summarize
    type: llm
    prompt: |
      Create a summary for these issues:
      Severity: {{.steps.analyze.severity}}
      Issues: {{.steps.analyze.issues}}

outputs:
  - name: summary
    type: string
    value: "$.summarize.response"
```

---

## Complete Example

```conductor
name: code-review-workflow
description: "Multi-step code review with security and style checks"
version: "1.0"

inputs:
  - name: code
    type: string
    required: true
    description: "Source code to review"
  - name: language
    type: string
    default: "go"
    description: "Programming language"

agents:
  security-expert:
    prefers: anthropic
    capabilities: [tool-use]

steps:
  - id: security_scan
    type: llm
    agent: security-expert
    model: strategic
    system: "You are a security expert. Identify vulnerabilities."
    prompt: "Review this {{.language}} code for security issues: {{.code}}"
    output_type: classification
    output_options:
      categories: [safe, low-risk, medium-risk, high-risk, critical]
    timeout: 60
    retry:
      max_attempts: 3
      backoff_base: 2
      backoff_multiplier: 2.0

  - id: check_critical
    type: condition
    condition:
      expression: "$.security_scan.category == 'critical'"
      then_steps: [detailed_analysis]
      else_steps: [style_check]

  - id: detailed_analysis
    type: llm
    model: strategic
    prompt: "Provide detailed analysis: {{.steps.security_scan.response}}"
    timeout: 120

  - id: style_check
    shell.run:
      command: ["golangci-lint", "run", "--timeout=5m"]
    on_error:
      strategy: ignore

  - id: summarize
    type: llm
    prompt: |
      Create a review summary:
      Security: {{.steps.security_scan.category}}
      {{#if .steps.detailed_analysis}}
      Details: {{.steps.detailed_analysis.response}}
      {{/if}}

outputs:
  - name: security_level
    type: string
    value: "$.security_scan.category"
    description: "Security assessment level"
  - name: summary
    type: string
    value: "$.summarize.response"
    description: "Complete review summary"

triggers:
  - type: webhook
    webhook:
      path: /webhooks/code-review
      source: github
      events: [pull_request]
      secret: ${GITHUB_WEBHOOK_SECRET}
```

---

## Security Best Practices

### Credential Management

**NEVER hardcode credentials in workflows.** Always use environment variables or secret references:

```conductor
# GOOD - Environment variable
integrations:
  github:
    from: connectors/github
    auth:
      token: ${GITHUB_TOKEN}

# GOOD - Secret reference (when secrets backend is configured)
integrations:
  slack:
    from: connectors/slack
    auth:
      token: $secret:slack_token

# BAD - Hardcoded credential (will cause validation error)
integrations:
  github:
    from: connectors/github
    auth:
      token: ghp_xxxxxxxxxxxxxxxxxxxx  # ERROR!
```

### Shell Command Injection Prevention

When using `shell.run`, prefer the array form to prevent command injection:

```conductor
# SAFE - Array form (arguments not shell-interpreted)
- shell.run:
    command: ["git", "commit", "-m", "{{.inputs.message}}"]

# RISKY - String form with user input
- shell.run: "git commit -m '{{.inputs.message}}'"  # Injection risk!

# SAFE - String form for static commands only
- shell.run: "git status"
```

The array form passes arguments directly to the command without shell interpretation, preventing injection attacks when using user-provided input.

### File Path Validation

Be specific with file paths to prevent unauthorized file access:

```conductor
# GOOD - Specific paths
- file.read: ./config/app.json
- file.list:
    path: ./logs
    pattern: "*.log"

# RISKY - Overly broad paths
- file.read: /  # WARNING: Too broad!
- file.list:
    path: ~  # WARNING: User home directory
```

### Connector Configuration

External connectors should always include authentication:

```conductor
integrations:
  github:
    from: connectors/github
    auth:  # Always configure auth for external services
      token: ${GITHUB_TOKEN}
```

---

## Validation Rules

Conductor validates workflow definitions before execution:

1. **Step types**: Must be `llm`, `parallel`, `condition`, or detected from connector shorthand
2. **Unique IDs**: All step IDs must be unique within a workflow (auto-generated if not provided for shorthand)
3. **LLM steps**: Must have `prompt` field and explicit `id`
4. **Connector steps**: Must have valid connector and operation names matching pattern `^[a-z][a-z0-9_]*$`
5. **Condition steps**: Must have `condition` field with valid expression
6. **Parallel steps**: Must have `steps` array with at least one nested step
7. **Model tiers**: Must be `fast`, `balanced`, or `strategic`
8. **Input types**: Must be `string`, `number`, `boolean`, `object`, or `array`
9. **Output schema**: Maximum depth 10, maximum 100 properties, maximum 64KB size
10. **Agent references**: Referenced agents must be defined in `agents:` section
11. **Mutually exclusive**: Cannot specify both `output_schema` and `output_type`
12. **Security checks**:
    - Plaintext credentials in connector auth trigger validation errors
    - Shell commands with string form + templates trigger warnings
    - Overly broad file paths trigger warnings

---

## Next Steps

- [CLI Reference](cli.md) - Command-line interface documentation
- [Configuration Reference](configuration.md) - All configuration options
- [API Reference](api.md) - Go package documentation
- [Workflows and Steps](../learn/concepts/workflows-steps.md) - Workflow fundamentals
