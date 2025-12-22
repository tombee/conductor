# GitHub

The GitHub connector provides integration with the GitHub API for managing issues, pull requests, repositories, and releases. It works with both GitHub.com and GitHub Enterprise.

## Quick Start

```yaml
connectors:
  github:
    from: connectors/github
    auth:
      token: ${GITHUB_TOKEN}
```

For GitHub Enterprise, override the `base_url`:

```yaml
connectors:
  github:
    from: connectors/github
    base_url: https://github.mycompany.com/api/v3
    auth:
      token: ${GITHUB_TOKEN}
```

## Authentication

### Getting a GitHub Token

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Select scopes based on what you need:
   - `repo` - Full control of private repositories (includes issues, PRs)
   - `public_repo` - Access public repositories only
   - `workflow` - Update GitHub Actions workflows
4. Copy the token and store it securely

```bash
export GITHUB_TOKEN=ghp_your_token_here
```

### Required Scopes by Operation

| Operation | Required Scopes |
|-----------|----------------|
| `create_issue`, `update_issue`, `close_issue`, `add_comment`, `list_issues` | `repo` or `public_repo` |
| `create_pr`, `merge_pr`, `list_prs` | `repo` |
| `get_file` | `repo` or `public_repo` |
| `create_release` | `repo` |
| `list_repos` | `repo` or `public_repo` |
| `get_workflow_runs` | `repo` + `workflow` |

## Operations

### Issues

#### create_issue

Create a new issue in a repository.

```yaml
- id: create_bug_report
  type: connector
  connector: github.create_issue
  inputs:
    owner: my-org
    repo: my-repo
    title: "Bug: Login fails on Safari"
    body: |
      ## Description
      Users cannot log in when using Safari browser.

      ## Steps to Reproduce
      1. Open login page in Safari
      2. Enter credentials
      3. Click "Sign in"

      ## Expected
      User is logged in

      ## Actual
      Error message: "Invalid credentials"
    labels: ["bug", "needs-triage"]
    assignees: ["devops-team"]
```

**Inputs:**
- `owner` (required): Repository owner (username or organization)
- `repo` (required): Repository name
- `title` (required): Issue title
- `body`: Issue description (supports Markdown)
- `labels`: Array of label names
- `assignees`: Array of usernames to assign
- `milestone`: Milestone number

**Output:** `{number, html_url, state}`

#### update_issue

Update an existing issue.

```yaml
- id: update_issue_status
  type: connector
  connector: github.update_issue
  inputs:
    owner: my-org
    repo: my-repo
    issue_number: 42
    state: closed
    labels: ["bug", "fixed"]
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `issue_number` (required): Issue number
- `title`: New title
- `body`: New body
- `state`: `open` or `closed`
- `labels`: Array of label names
- `assignees`: Array of usernames

**Output:** `{number, html_url, state, updated_at}`

#### close_issue

Close an issue.

```yaml
- id: close_resolved_issue
  type: connector
  connector: github.close_issue
  inputs:
    owner: my-org
    repo: my-repo
    issue_number: 42
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `issue_number` (required): Issue number

**Output:** `{number, state}`

#### add_comment

Add a comment to an issue or pull request.

```yaml
- id: post_analysis
  type: connector
  connector: github.add_comment
  inputs:
    owner: my-org
    repo: my-repo
    issue_number: 42
    body: |
      Analysis complete. Root cause identified:
      - Database connection timeout
      - Recommendation: Increase timeout to 30s
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `issue_number` (required): Issue or PR number
- `body` (required): Comment text (supports Markdown)

**Output:** `{id, html_url}`

#### list_issues

List issues in a repository with filters.

```yaml
- id: get_open_bugs
  type: connector
  connector: github.list_issues
  inputs:
    owner: my-org
    repo: my-repo
    state: open
    labels: "bug"
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `state`: Filter by state (`open`, `closed`, `all`)
- `labels`: Comma-separated list of label names
- `sort`: Sort by (`created`, `updated`, `comments`)
- `direction`: Sort direction (`asc`, `desc`)
- `per_page`: Results per page (max 100)

**Output:** `[{number, title, state, html_url, labels, created_at}]`

### Pull Requests

#### create_pr

Create a pull request.

```yaml
- id: create_feature_pr
  type: connector
  connector: github.create_pr
  inputs:
    owner: my-org
    repo: my-repo
    title: "feat: Add user authentication"
    head: feature/auth
    base: main
    body: |
      ## Changes
      - Implement OAuth2 login
      - Add user session management

      ## Testing
      - Unit tests pass
      - Integration tests pass
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `title` (required): PR title
- `head` (required): Branch name containing changes
- `base` (required): Branch name to merge into
- `body`: PR description (supports Markdown)
- `draft`: Create as draft PR (boolean)

**Output:** `{number, html_url, state}`

#### merge_pr

Merge a pull request.

```yaml
- id: merge_approved_pr
  type: connector
  connector: github.merge_pr
  inputs:
    owner: my-org
    repo: my-repo
    pull_number: 123
    merge_method: squash
    commit_title: "feat: Add user authentication (#123)"
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `pull_number` (required): PR number
- `merge_method`: `merge`, `squash`, or `rebase` (default: `merge`)
- `commit_title`: Commit message title
- `commit_message`: Commit message body

**Output:** `{sha, merged, message}`

#### list_prs

List pull requests with filters.

```yaml
- id: get_open_prs
  type: connector
  connector: github.list_prs
  inputs:
    owner: my-org
    repo: my-repo
    state: open
    sort: updated
    direction: desc
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `state`: Filter by state (`open`, `closed`, `all`)
- `head`: Filter by head branch
- `base`: Filter by base branch
- `sort`: Sort by (`created`, `updated`, `popularity`, `long-running`)
- `direction`: Sort direction (`asc`, `desc`)

**Output:** `[{number, title, state, html_url, head, base, created_at}]`

#### get_pr

Get details of a specific pull request.

```yaml
- id: get_pr_details
  type: connector
  connector: github.get_pr
  inputs:
    owner: my-org
    repo: my-repo
    pull_number: 123
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `pull_number` (required): PR number

**Output:** `{number, title, state, html_url, body, head, base, user, created_at, updated_at, mergeable, merged, merged_at}`

#### get_pr_diff

Get the diff for a pull request.

```yaml
- id: get_pr_changes
  type: connector
  connector: github.get_pr_diff
  inputs:
    owner: my-org
    repo: my-repo
    pull_number: 123
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `pull_number` (required): PR number

**Output:** `{diff}` - The unified diff text showing all changes in the PR

#### create_review

Submit a review on a pull request.

```yaml
- id: approve_pr
  type: connector
  connector: github.create_review
  inputs:
    owner: my-org
    repo: my-repo
    pull_number: 123
    event: APPROVE
    body: |
      LGTM! The code looks good and all tests pass.

      Nice work on the refactoring.
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `pull_number` (required): PR number
- `event` (required): Review action (`APPROVE`, `REQUEST_CHANGES`, `COMMENT`)
- `body`: Review comment text (required for `REQUEST_CHANGES` and `COMMENT`)
- `comments`: Array of line-specific comments with `{path, position, body}`

**Output:** `{id, state, html_url}`

Example with inline comments:

```yaml
- id: request_changes
  type: connector
  connector: github.create_review
  inputs:
    owner: my-org
    repo: my-repo
    pull_number: 123
    event: REQUEST_CHANGES
    body: "Please address the comments below"
    comments:
      - path: src/main.go
        position: 15
        body: "This function needs error handling"
      - path: src/util.go
        position: 42
        body: "Consider using a constant here"
```

### Repositories

#### list_repos

List repositories for a user or organization.

```yaml
- id: list_org_repos
  type: connector
  connector: github.list_repos
  inputs:
    username: my-org
    type: all
    sort: updated
```

**Inputs:**
- `username` (required): Username or organization name
- `type`: Repository type (`all`, `owner`, `member`)
- `sort`: Sort by (`created`, `updated`, `pushed`, `full_name`)
- `direction`: Sort direction (`asc`, `desc`)

**Output:** `[{name, full_name, description, html_url, private}]`

#### get_file

Get file contents from a repository.

```yaml
- id: read_config
  type: connector
  connector: github.get_file
  inputs:
    owner: my-org
    repo: my-repo
    path: config/production.yaml
    ref: main
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `path` (required): File path in repository
- `ref`: Branch, tag, or commit SHA (default: default branch)

**Output:** `{content, encoding, sha}`

Note: Content is base64-encoded. Decode with:
```yaml
- id: decode_content
  type: action
  action: base64_decode
  inputs:
    data: "{{.steps.read_config.content}}"
```

### Releases

#### create_release

Create a release with assets.

```yaml
- id: publish_release
  type: connector
  connector: github.create_release
  inputs:
    owner: my-org
    repo: my-repo
    tag_name: v1.2.3
    name: "Release 1.2.3"
    body: |
      ## What's New
      - Feature A
      - Bug fix B
    draft: false
    prerelease: false
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `tag_name` (required): Git tag name
- `name`: Release name
- `body`: Release description (supports Markdown)
- `draft`: Create as draft (boolean)
- `prerelease`: Mark as prerelease (boolean)
- `target_commitish`: Branch or commit SHA to tag

**Output:** `{id, html_url, tag_name, name}`

### GitHub Actions

#### get_workflow_runs

Get workflow run history.

```yaml
- id: check_ci_status
  type: connector
  connector: github.get_workflow_runs
  inputs:
    owner: my-org
    repo: my-repo
    workflow_id: ci.yml
    status: completed
```

**Inputs:**
- `owner` (required): Repository owner
- `repo` (required): Repository name
- `workflow_id`: Workflow file name or ID
- `branch`: Filter by branch
- `status`: Filter by status (`completed`, `in_progress`, `queued`)
- `per_page`: Results per page (max 100)

**Output:** `[{id, status, conclusion, html_url, created_at}]`

## Examples

### Create Issue from Analysis

```yaml
steps:
  - id: analyze_logs
    type: llm
    model: balanced
    prompt: |
      Analyze these error logs and create an issue:
      {{.inputs.logs}}

      Provide: title, description, severity (low/medium/high)

  - id: create_issue
    type: connector
    connector: github.create_issue
    inputs:
      owner: my-org
      repo: my-repo
      title: "{{.steps.analyze_logs.title}}"
      body: "{{.steps.analyze_logs.description}}"
      labels: ["bug", "{{.steps.analyze_logs.severity}}"]
```

### PR Review Bot

```yaml
steps:
  - id: get_pr
    type: connector
    connector: github.get_pr
    inputs:
      owner: "{{.inputs.owner}}"
      repo: "{{.inputs.repo}}"
      pull_number: "{{.inputs.pr_number}}"

  - id: get_diff
    type: connector
    connector: github.get_pr_diff
    inputs:
      owner: "{{.inputs.owner}}"
      repo: "{{.inputs.repo}}"
      pull_number: "{{.inputs.pr_number}}"

  - id: review
    type: llm
    model: balanced
    prompt: |
      Review this pull request:
      Title: {{.steps.get_pr.title}}
      Description: {{.steps.get_pr.body}}
      Changes:
      {{.steps.get_diff.diff}}

      Provide constructive feedback on code quality, best practices, and potential issues.
      Return APPROVE, REQUEST_CHANGES, or COMMENT along with your review comments.

  - id: post_review
    type: connector
    connector: github.create_review
    inputs:
      owner: "{{.inputs.owner}}"
      repo: "{{.inputs.repo}}"
      pull_number: "{{.inputs.pr_number}}"
      event: "{{.steps.review.event}}"
      body: "{{.steps.review.comments}}"
```

### Auto-close Stale Issues

```yaml
steps:
  - id: find_stale
    type: connector
    connector: github.list_issues
    inputs:
      owner: my-org
      repo: my-repo
      state: open
      labels: "stale"

  - id: close_each
    type: parallel
    steps:
      - id: close
        type: connector
        connector: github.close_issue
        inputs:
          owner: my-org
          repo: my-repo
          issue_number: "{{.item.number}}"
    for_each: "{{.steps.find_stale}}"
```

## Rate Limiting

GitHub API has rate limits:
- **Authenticated requests**: 5,000 requests/hour
- **Unauthenticated requests**: 60 requests/hour

The connector includes conservative defaults:

```yaml
connectors:
  github:
    from: connectors/github
    auth:
      token: ${GITHUB_TOKEN}
    rate_limit:
      requests_per_second: 10
      requests_per_minute: 100  # Well under 5000/hour limit
```

Adjust based on your needs:

```yaml
# More aggressive rate limiting for high-volume workflows
rate_limit:
  requests_per_second: 5
  requests_per_minute: 50

# Faster for low-volume workflows
rate_limit:
  requests_per_second: 20
  requests_per_minute: 500
```

## Troubleshooting

### 401 Unauthorized

**Problem**: `401 Unauthorized` error

**Solutions**:
1. Check token is set: `echo $GITHUB_TOKEN`
2. Verify token hasn't expired (classic tokens don't expire, but fine-grained tokens do)
3. Check token has required scopes
4. For Enterprise: Ensure `base_url` points to your instance

### 404 Not Found

**Problem**: `404 Not Found` when accessing repository

**Solutions**:
1. Verify repository exists: `owner/repo` format is correct
2. Check token has access to the repository
3. For private repos, ensure token has `repo` scope
4. For Enterprise: Check `base_url` is correct

### 403 Forbidden / Rate Limited

**Problem**: `403 Forbidden` or rate limit errors

**Solutions**:
1. Check rate limit status in response headers
2. Reduce `requests_per_minute` in connector config
3. Add delays between high-volume operations
4. For secondary rate limits (abuse detection), implement exponential backoff

### Response Not What You Expected

**Problem**: Response doesn't match documentation

**Solutions**:
1. Check API version header: `X-GitHub-Api-Version: 2022-11-28`
2. Review GitHub API docs for changes
3. Use `--dry-run` to see the actual request
4. Enable debug logging: `--log-level debug`

## See Also

- [GitHub REST API Documentation](https://docs.github.com/en/rest)
- [GitHub Personal Access Tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token)
- [GitHub API Rate Limiting](https://docs.github.com/en/rest/overview/resources-in-the-rest-api#rate-limiting)
