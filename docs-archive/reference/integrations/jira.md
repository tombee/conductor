# Jira

The Jira integration provides integration with Jira Cloud and Jira Server for issue tracking and project management.

## Quick Start

```conductor
integrations:
  jira:
    from: integrations/jira
    base_url: https://your-company.atlassian.net
    auth:
      type: basic
      username: ${JIRA_EMAIL}
      password: ${JIRA_API_TOKEN}
```

## Authentication

Jira Cloud uses API tokens with basic auth. Generate at: [id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens)

```bash
export JIRA_EMAIL=your.email@company.com
export JIRA_API_TOKEN=your-api-token
```

## Operations

### Issue Management

#### create_issue

Create a new issue in a project.

```conductor
- id: create_bug
  type: integration
  integration: jira.create_issue
  inputs:
    project: PROJ
    summary: "Bug: Login fails on Safari"
    description: |
      Users cannot log in when using Safari browser.

      Steps to reproduce:
      1. Open login page
      2. Enter credentials
      3. Click sign in
    issue_type: Bug
    priority: High
    labels: ["security", "browser-compatibility"]
```

**Inputs:**
- `project` (required): Project key (e.g., "PROJ")
- `summary` (required): Issue title
- `description`: Issue description (supports Jira markdown)
- `issue_type` (required): Issue type (e.g., "Bug", "Task", "Story")
- `priority`: Priority name (e.g., "High", "Medium", "Low")
- `labels`: Array of label strings
- `assignee`: Username or account ID to assign
- `components`: Array of component names
- `custom_fields`: Map of custom field IDs to values

**Output:** `{id, key, self}`

#### update_issue

Update an existing issue.

```conductor
- id: update_priority
  type: integration
  integration: jira.update_issue
  inputs:
    issue_key: PROJ-123
    summary: "Updated: Bug: Login fails on Safari and Chrome"
    priority: Critical
    labels: ["security", "browser-compatibility", "urgent"]
```

**Inputs:**
- `issue_key` (required): Issue key (e.g., "PROJ-123")
- `summary`: New summary
- `description`: New description
- `priority`: New priority
- `labels`: New labels (replaces existing)
- `assignee`: New assignee
- `components`: New components
- `custom_fields`: Custom field updates

**Output:** `{id, key}`

#### transition_issue

Transition an issue to a different status.

```conductor
- id: close_issue
  type: integration
  integration: jira.transition_issue
  inputs:
    issue_key: PROJ-123
    transition: "Done"
    resolution: "Fixed"
    comment: "Issue resolved in v1.2.3"
```

**Inputs:**
- `issue_key` (required): Issue key
- `transition` (required): Transition name or ID (e.g., "Done", "In Progress")
- `resolution`: Resolution name (e.g., "Fixed", "Won't Fix")
- `comment`: Comment to add during transition
- `fields`: Additional fields to update during transition

**Output:** `{id, key}`

Note: Use `get_transitions` to see available transitions for an issue.

#### get_issue

Get details of a specific issue.

```conductor
- id: fetch_issue
  type: integration
  integration: jira.get_issue
  inputs:
    issue_key: PROJ-123
    fields: ["summary", "status", "assignee", "description"]
```

**Inputs:**
- `issue_key` (required): Issue key
- `fields`: Array of field names to include (default: all)
- `expand`: Array of entities to expand (e.g., ["changelog", "renderedFields"])

**Output:** `{id, key, fields: {summary, status, assignee, description, ...}, changelog}`

#### search_issues

Search for issues using JQL (Jira Query Language).

```conductor
- id: find_open_bugs
  type: integration
  integration: jira.search_issues
  inputs:
    jql: 'project = PROJ AND status = "Open" AND type = Bug'
    fields: ["key", "summary", "status", "assignee"]
    max_results: 50
```

**Inputs:**
- `jql` (required): JQL query string
- `fields`: Array of field names to include
- `start_at`: Starting index for pagination (default: 0)
- `max_results`: Maximum results to return (default: 50, max: 100)

**Output:** `{total, issues: [{key, fields}]}`

Common JQL examples:
- `project = PROJ AND assignee = currentUser()`
- `status = "In Progress" AND updated >= -7d`
- `priority in (High, Critical) AND resolution = Unresolved`

#### assign_issue

Assign an issue to a user.

```conductor
- id: assign_to_dev
  type: integration
  integration: jira.assign_issue
  inputs:
    issue_key: PROJ-123
    assignee: john.doe
```

**Inputs:**
- `issue_key` (required): Issue key
- `assignee` (required): Username or account ID (use "-1" for automatic, "null" for unassigned)

**Output:** `{id, key}`

#### add_comment

Add a comment to an issue.

```conductor
- id: post_update
  type: integration
  integration: jira.add_comment
  inputs:
    issue_key: PROJ-123
    body: |
      Analysis complete. Root cause identified:
      * Database connection timeout
      * Recommendation: Increase timeout to 30s
```

**Inputs:**
- `issue_key` (required): Issue key
- `body` (required): Comment text (supports Jira markdown)
- `visibility`: Visibility restriction `{type: "role", value: "Developers"}`

**Output:** `{id, self, body, author, created}`

#### add_attachment

Attach a file to an issue.

```conductor
- id: attach_logs
  type: integration
  integration: jira.add_attachment
  inputs:
    issue_key: PROJ-123
    file_content: "{{.steps.get_logs.content}}"
    filename: "error-logs.txt"
```

**Inputs:**
- `issue_key` (required): Issue key
- `file_content` (required): File content (string or base64)
- `filename` (required): Filename

**Output:** `{id, filename, size, mimeType, self}`

### Project Management

#### list_projects

List all projects accessible to the user.

```conductor
- id: get_projects
  type: integration
  integration: jira.list_projects
  inputs:
    recent: 10
```

**Inputs:**
- `recent`: Number of recent projects to return (optional)
- `expand`: Array of entities to expand (e.g., ["description", "lead"])

**Output:** `[{id, key, name, projectTypeKey, style}]`

### Workflow

#### get_transitions

Get available transitions for an issue.

```conductor
- id: check_transitions
  type: integration
  integration: jira.get_transitions
  inputs:
    issue_key: PROJ-123
```

**Inputs:**
- `issue_key` (required): Issue key

**Output:** `[{id, name, to: {name, id}, hasScreen}]`

Use this to discover valid transition names/IDs before calling `transition_issue`.

#### link_issues

Create a link between two issues.

```conductor
- id: link_blocker
  type: integration
  integration: jira.link_issues
  inputs:
    inward_issue: PROJ-123
    outward_issue: PROJ-456
    link_type: "Blocks"
    comment: "This issue blocks the other"
```

**Inputs:**
- `inward_issue` (required): Source issue key
- `outward_issue` (required): Target issue key
- `link_type` (required): Link type name (e.g., "Blocks", "Relates", "Duplicates")
- `comment`: Comment to add with the link

**Output:** `{id, type, inwardIssue, outwardIssue}`

Common link types:
- "Blocks" / "is blocked by"
- "Duplicates" / "is duplicated by"
- "Relates to"
- "Causes" / "is caused by"

## Examples

### Automated Bug Triage

```conductor
steps:
  - id: search_untriaged
    type: integration
    integration: jira.search_issues
    inputs:
      jql: 'project = PROJ AND labels = "needs-triage" AND status = "Open"'
      max_results: 20

  - id: analyze_each
    type: parallel
    for_each: "{{.steps.search_untriaged.issues}}"
    steps:
      - id: analyze
        type: llm
        model: fast
        prompt: |
          Analyze this bug report and suggest priority:
          {{.item.fields.summary}}
          {{.item.fields.description}}

      - id: update_priority
        type: integration
        integration: jira.update_issue
        inputs:
          issue_key: "{{.item.key}}"
          priority: "{{.steps.analyze.priority}}"
          labels: ["triaged"]
```

### Issue Creation from Monitoring

```conductor
steps:
  - id: analyze_alert
    type: llm
    model: balanced
    prompt: |
      Create a Jira issue from this alert:
      {{.inputs.alert_data}}

      Provide: summary, description, severity

  - id: create_issue
    type: integration
    integration: jira.create_issue
    inputs:
      project: OPS
      summary: "{{.steps.analyze_alert.summary}}"
      description: "{{.steps.analyze_alert.description}}"
      issue_type: Incident
      priority: "{{.steps.analyze_alert.severity}}"
      labels: ["automated", "monitoring"]

  - id: assign_oncall
    type: integration
    integration: jira.assign_issue
    inputs:
      issue_key: "{{.steps.create_issue.key}}"
      assignee: "{{.inputs.oncall_engineer}}"
```

## Troubleshooting

### 401 Unauthorized

**Problem**: Authentication fails

**Solutions**:
1. Verify API token is correct and not expired
2. Check email matches the Jira account
3. For Jira Server, use password instead of API token
4. Ensure `base_url` is correct (e.g., `https://company.atlassian.net`)

### 404 Issue Not Found

**Problem**: Cannot find issue

**Solutions**:
1. Verify issue key format (PROJECT-123)
2. Check user has permission to view the issue
3. Confirm project exists and is accessible

### 400 Field Validation Error

**Problem**: Field value rejected

**Solutions**:
1. Use `get_issue` to see required/available fields
2. Check custom field IDs match your Jira instance
3. Verify enum values (priority, status) match exactly
4. Use `get_transitions` to see valid transition names

## See Also

- [Jira Cloud REST API v3](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
- [JQL Reference](https://support.atlassian.com/jira-service-management-cloud/docs/use-advanced-search-with-jira-query-language-jql/)
- [Jira API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
