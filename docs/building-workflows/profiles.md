# Profile Configuration Guide

This guide explains how to use profiles to separate workflow definitions from execution configurations, enabling safe workflow sharing and multi-environment deployments.

## Overview

Conductor supports two approaches to credential management:

1. **Inline credentials** (default) - Credentials embedded directly in workflow files
2. **Profiles** (recommended for teams) - Credentials managed separately in daemon configuration

Profiles enable:
- Safe workflow sharing within teams and organizations
- Multi-environment deployments (dev/staging/prod) without workflow changes
- Clean separation of workflow logic from deployment details

## When to Use Profiles

**Use inline credentials when:**
- Working alone on a single machine
- Developing and testing locally
- Prototyping workflows quickly

**Use profiles when:**
- Sharing workflows with team members
- Running workflows across multiple environments
- Publishing workflows to a registry
- Enforcing security best practices

## Basic Concepts

### Workflow Definition
The reusable execution logic: steps, prompts, flow control, and abstract references to external services.

```conductor
# workflow.yaml - Portable, shareable
name: deploy-service
version: "1.0"

# Abstract service requirements
requires:
  integrations:
    - name: github
    - name: slack
      optional: true

steps:
  - id: create-pr
    type: connector
    connector: github.create_pull_request
    inputs:
      title: "Deploy {{.input.version}}"
```

### Execution Profile
The runtime bindings: credentials, MCP server configurations, secrets, provider settings.

```conductor
# conductor.yaml - Contains credentials
workspaces:
  default:
    profiles:
      prod:
        inherit_env: false
        bindings:
          integrations:
            github:
              auth:
                token: ${GITHUB_TOKEN}
            slack:
              auth:
                webhook_url: ${SLACK_WEBHOOK}
```

## Configuration

### Daemon Configuration

Profiles are defined in `conductor.yaml`:

```conductor
workspaces:
  # Default workspace for single-user setups
  default:
    profiles:
      default:
        # Allow access to process environment variables
        inherit_env: true

      prod:
        # Strict isolation for production
        inherit_env: false
        bindings:
          integrations:
            github:
              auth:
                token: ${PROD_GITHUB_TOKEN}

  # Team-specific workspace
  frontend:
    profiles:
      dev:
        description: "Frontend team dev environment"
        inherit_env: false
        bindings:
          integrations:
            github:
              auth:
                token: ${FRONTEND_GITHUB_TOKEN}

      prod:
        inherit_env: false
        bindings:
          integrations:
            github:
              auth:
                token: file:/etc/conductor/secrets/frontend-github-token
```

### Secret References

Profiles support multiple secret providers:

```conductor
# Environment variable (default)
token: ${GITHUB_TOKEN}

# Explicit environment reference
token: env:GITHUB_TOKEN

# File reference (requires allowlist configuration)
token: file:/etc/conductor/secrets/github-token

# Future: External secret stores
# token: 1password:vault/item/field
# token: vault:secret/data/github#token
```

### File Provider Security

File references require explicit configuration in the controller config:

```conductor
secret_providers:
  file:
    enabled: true  # Disabled by default
    allowed_paths:
      - /etc/conductor/secrets/
      - /var/conductor/secrets/
    max_size_bytes: 65536
    follow_symlinks: false
```

Security constraints:
- Paths must be absolute
- Paths must start with an allowed prefix
- Symlinks resolved before validation
- Max file size: 64KB
- Regular files only (not directories or devices)

## Usage

### CLI Usage

```bash
# Use default profile
conductor run workflow.yaml --daemon

# Specify profile
conductor run workflow.yaml --daemon --profile prod

# Full workspace/profile selection
conductor run workflow.yaml --daemon --workspace frontend --profile dev

# Short forms
conductor run workflow.yaml -d -w frontend -p prod

# Environment variable override
CONDUCTOR_WORKSPACE=frontend CONDUCTOR_PROFILE=prod conductor run workflow.yaml -d
```

### API Usage

```bash
# POST with query parameters
curl -X POST http://localhost:9090/v1/runs?workspace=frontend&profile=prod \
  -H "Content-Type: application/x-yaml" \
  --data-binary @workflow.yaml

# POST with JSON body
curl -X POST http://localhost:9090/v1/runs \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": "deploy-service",
    "workspace": "frontend",
    "profile": "prod",
    "inputs": {"version": "1.2.3"}
  }'
```

### Profile Validation

Validate workflows against profile requirements:

```bash
# Validate workflow structure only
conductor validate workflow.yaml

# Show profile that would be used
conductor validate workflow.yaml --workspace frontend --profile prod

# Output shows profile context
Validation Results:
  [OK] Syntax valid
  [OK] Schema valid
  [OK] All step references resolve correctly

Profile Configuration:
  Workspace: frontend
  Profile: prod

  Note: Profile binding validation requires daemon connection
  Run with --controller to validate actual bindings
```

## Profile Selection Precedence

When multiple sources specify a profile, higher precedence wins:

1. CLI flag: `--profile <name>` (highest)
2. API field: `{"profile": "<name>"}`
3. Environment variable: `CONDUCTOR_PROFILE=<name>`
4. Workspace default profile
5. Global default profile (lowest)

Same precedence applies to workspace selection.

## Migration Guide

### Step 1: Assess Current State

Identify workflows with embedded credentials:

```bash
# Scan for credential patterns
grep -r "ghp_\|sk-ant-\|xoxb-" workflows/
```

### Step 2: Create First Profile

Add a named profile to `conductor.yaml`:

```conductor
workspaces:
  default:
    profiles:
      default:
        inherit_env: true  # Existing behavior

      prod:  # New profile
        inherit_env: false
        bindings:
          integrations:
            github:
              auth:
                token: ${GITHUB_TOKEN}
```

### Step 3: Test with Profile

Run existing workflow with the new profile:

```bash
conductor run workflow.yaml --daemon --profile prod
```

Verify credentials resolve correctly.

### Step 4: Remove Inline Credentials

Update workflow to use abstract references:

```conductor
# Before
integrations:
  github:
    type: github
    auth:
      token: ${GITHUB_TOKEN}  # Inline credential

# After
requires:
  integrations:
    - name: github  # Abstract reference
```

### Step 5: Validate

Ensure workflows work without inline credentials:

```bash
conductor validate workflow.yaml --profile prod
conductor run workflow.yaml --daemon --profile prod
```

## Best Practices

### Security

1. **Never commit credentials** to version control
   - Use `.gitignore` for `conductor.yaml` or
   - Use environment-specific config files

2. **Use `inherit_env: false` for production**
   - Prevents accidental environment variable leakage
   - Explicit allowlist if needed:
     ```yaml
     inherit_env:
       allowlist: [CONDUCTOR_*, CI, GITHUB_ACTIONS]
     ```

3. **Enable file provider only when needed**
   - Disabled by default
   - Strict path allowlist
   - Verify symlink policy

4. **Scan for plaintext credentials**
   - Daemon warns on startup if credentials detected
   - Use secret references instead

### Organization

1. **One workspace per team/environment**
   ```yaml
   workspaces:
     frontend:
       profiles: {dev, staging, prod}
     backend:
       profiles: {dev, staging, prod}
   ```

2. **Consistent naming conventions**
   - Profile names: lowercase, alphanumeric, `-` or `_`
   - Max 64 characters
   - Reserved: `default`, `system`

3. **Document profile purpose**
   ```yaml
   prod:
     description: "Production environment - requires approval"
     bindings: {...}
   ```

### Development Workflow

1. **Local development**: Use inline credentials or default profile
2. **CI/CD**: Use dedicated profiles with scoped credentials
3. **Production**: Use strict profiles with file or external secret providers

## Examples

### Multi-Environment Deployment

```conductor
# conductor.yaml
workspaces:
  default:
    profiles:
      dev:
        bindings:
          integrations:
            github:
              auth:
                token: ${DEV_GITHUB_TOKEN}
            kubernetes:
              auth:
                kubeconfig: file:/home/user/.kube/config-dev

      staging:
        bindings:
          integrations:
            github:
              auth:
                token: ${STAGING_GITHUB_TOKEN}
            kubernetes:
              auth:
                kubeconfig: file:/etc/conductor/kube/staging.config

      prod:
        inherit_env: false
        bindings:
          integrations:
            github:
              auth:
                token: file:/etc/conductor/secrets/github-prod
            kubernetes:
              auth:
                kubeconfig: file:/etc/conductor/kube/prod.config
```

Usage:

```bash
# Deploy to dev
conductor run deploy.yaml -d -p dev -i version=1.2.3

# Deploy to staging
conductor run deploy.yaml -d -p staging -i version=1.2.3

# Deploy to prod
conductor run deploy.yaml -d -p prod -i version=1.2.3
```

### Team Isolation

```conductor
# conductor.yaml
workspaces:
  frontend:
    profiles:
      default:
        bindings:
          integrations:
            github:
              auth:
                token: ${FRONTEND_GITHUB_TOKEN}

  backend:
    profiles:
      default:
        bindings:
          integrations:
            github:
              auth:
                token: ${BACKEND_GITHUB_TOKEN}
```

Usage:

```bash
# Frontend team
conductor run frontend-workflow.yaml -d -w frontend

# Backend team
conductor run backend-workflow.yaml -d -w backend
```

### Optional Services

```conductor
# workflow.yaml
requires:
  integrations:
    - name: github
    - name: slack
      optional: true  # Won't fail if not bound
```

```conductor
# conductor.yaml
workspaces:
  default:
    profiles:
      minimal:
        bindings:
          integrations:
            github:
              auth:
                token: ${GITHUB_TOKEN}
            # No slack - optional, won't error

      full:
        bindings:
          integrations:
            github:
              auth:
                token: ${GITHUB_TOKEN}
            slack:
              auth:
                webhook_url: ${SLACK_WEBHOOK}
```

## Troubleshooting

### Profile Not Found

```
Error: profile not found: frontend/prod
```

**Solution:** Check `conductor.yaml` has the workspace and profile defined:

```conductor
workspaces:
  frontend:
    profiles:
      prod: {...}
```

### Binding Resolution Failed

```
Error: binding resolution failed for profile default/prod: required binding not found: integrations.github
```

**Solution:** Add the required binding to the profile:

```conductor
profiles:
  prod:
    bindings:
      integrations:
        github:
          auth:
            token: ${GITHUB_TOKEN}
```

### Secret Resolution Failed

```
Error: secret resolution failed: file provider access denied
```

**Solutions:**
1. Check file provider is enabled in daemon config
2. Verify path is in allowed_paths list
3. Check file permissions
4. Ensure file is under 64KB

### Plaintext Credential Warning

```
Warning: Profile 'default/prod' contains plaintext credential patterns
```

**Solution:** Use secret references instead of literal values:

```conductor
# Bad
token: ghp_abc123def456

# Good
token: ${GITHUB_TOKEN}
token: file:/etc/conductor/secrets/github-token
```

## Reference

### Profile Schema

```conductor
workspace_name:
  profiles:
    profile_name:
      description: string (optional)
      inherit_env: bool | {allowlist: [string]}
      bindings:
        integrations:
          connector_name:
            auth: {...}
        mcp_servers:
          server_name:
            command: string
            args: [string]
            env: {key: value}
```

### Secret Provider Configuration

```conductor
secret_providers:
  file:
    enabled: bool (default: false)
    allowed_paths: [string]
    max_size_bytes: int (default: 65536)
    follow_symlinks: bool (default: false)
```

### Environment Variables

- `CONDUCTOR_WORKSPACE` - Default workspace for profile resolution
- `CONDUCTOR_PROFILE` - Default profile for binding resolution

### CLI Flags

- `--workspace, -w <name>` - Workspace for profile resolution
- `--profile, -p <name>` - Profile for binding resolution

## See Also

- [Daemon Mode Guide](controller.md) - Running workflows via controller
- [Error Handling](error-handling.md) - Troubleshooting profile errors
- [Security](../operations/security.md) - Security best practices
