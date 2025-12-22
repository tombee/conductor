# Remote Workflows

Conductor supports running workflows from GitHub repositories, enabling easy sharing and reuse of workflows.

## Basic Usage

Run a workflow from a GitHub repository:

```bash
# Run from repository root (uses workflow.yaml)
conductor run github:user/repo

# Run from subdirectory
conductor run github:user/repo/path/to/workflow

# Pin to specific version
conductor run github:user/repo@v1.0.0
conductor run github:user/repo@main
conductor run github:user/repo@abc123def

# Pass inputs
conductor run github:user/repo -i input1=value1 -i input2=value2
```

## Caching

Remote workflows are cached locally for faster subsequent runs. The cache is content-addressable by commit SHA.

```bash
# Force fresh download (bypass cache)
conductor run github:user/repo --no-cache

# Manage cache
conductor cache list --owner user --repo repo
conductor cache clear --owner user --repo repo
conductor cache clear  # Clear entire cache
```

## Authentication

For private repositories, provide a GitHub token:

```bash
# Option 1: Environment variable
export GITHUB_TOKEN=ghp_your_token_here
conductor run github:user/private-repo

# Option 2: Use GitHub CLI authentication
gh auth login
conductor run github:user/private-repo

# Option 3: Conductor-specific token
export CONDUCTOR_GITHUB_TOKEN=ghp_your_token_here
```

Token resolution order:
1. `GITHUB_TOKEN` environment variable
2. `CONDUCTOR_GITHUB_TOKEN` environment variable
3. GitHub CLI (`gh auth token`)

## Version Pinning

Always pin to specific versions in production:

```bash
# Development - use branch
conductor run github:user/repo@main

# Production - use tag or commit SHA
conductor run github:user/repo@v1.2.3
conductor run github:user/repo@abc123def456789
```

## Example: Shared Code Review Workflow

Create a reusable code review workflow in a GitHub repo:

**Repository: `company/conductor-workflows`**
**File: `code-review/workflow.yaml`**

```yaml
name: code-review
description: Automated code review workflow

inputs:
  pull_request_url:
    description: URL of the pull request to review
    required: true

steps:
  - name: review
    prompt: |
      Review this pull request: {{ inputs.pull_request_url }}

      Provide feedback on:
      - Code quality
      - Potential bugs
      - Security issues
      - Best practices

outputs:
  review: ${{ steps.review.output }}
```

**Usage across teams:**

```bash
# Team A uses it
conductor run github:company/conductor-workflows/code-review \
  -i pull_request_url=https://github.com/team-a/repo/pull/123

# Team B uses the same workflow
conductor run github:company/conductor-workflows/code-review \
  -i pull_request_url=https://github.com/team-b/repo/pull/456
```

## GitHub Enterprise

Configure GitHub Enterprise host:

```bash
export GITHUB_HOST=github.company.com
conductor run github:user/repo
```

## Provenance Tracking

Remote workflow runs include source URL in metadata:

```bash
conductor runs show abc123
# Output includes:
# {
#   "id": "abc123",
#   "source_url": "github:user/repo@v1.0.0",
#   ...
# }
```

## Security Considerations

- Remote workflows are executed with your credentials
- Review workflows before first run (future enhancement: security prompt)
- Pin to specific versions to prevent supply chain attacks
- Use private repositories for sensitive workflows
- Cache is per-user and stored locally

## Cache Location

Workflows are cached in:
- Linux/macOS: `~/.cache/conductor/remote-workflows/`
- Windows: `%LOCALAPPDATA%\conductor\remote-workflows\`

Cache structure:
```
~/.cache/conductor/remote-workflows/
  github.com/
    user/
      repo/
        abc123def456/  # commit SHA
          workflow.yaml
          metadata.json
```
