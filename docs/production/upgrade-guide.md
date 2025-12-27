# Upgrade Guide

Guide for upgrading Conductor to newer versions, including breaking changes and migration procedures.

## General Upgrade Process

### 1. Check Current Version

```bash
conductor --version
```

### 2. Review Release Notes

Before upgrading, review the [CHANGELOG](https://github.com/tombee/conductor/blob/main/CHANGELOG.md) for:
- New features
- Breaking changes
- Deprecations
- Bug fixes

### 3. Backup Configuration

```bash
# Backup your config file
cp ~/.config/conductor/config.yaml ~/.config/conductor/config.yaml.backup

# Backup workflow files
tar -czf workflows-backup-$(date +%Y%m%d).tar.gz ./workflows/
```

### 4. Upgrade Conductor

=== "Homebrew"

    ```bash
    brew update
    brew upgrade conductor
    ```

=== "Go Install"

    ```bash
    go install github.com/tombee/conductor/cmd/conductor@latest
    ```

=== "Binary Download"

    Download the latest release from [GitHub Releases](https://github.com/tombee/conductor/releases/latest) and replace your existing binary.

=== "Docker"

    ```bash
    docker pull ghcr.io/tombee/conductor:latest
    ```

### 5. Verify Installation

```bash
conductor --version
conductor doctor  # Check for configuration issues
```

### 6. Test Workflows

```bash
# Validate workflow syntax
conductor validate workflow.yaml

# Test run with dry-run
conductor run workflow.yaml --dry-run

# Run with test inputs
conductor run workflow.yaml -i test_mode=true
```

## Version Migration Guides

### Upgrading to v2.0 (Future)

:::note[Not Yet Released]
This is a placeholder for future breaking changes. Current version is 1.x.
:::

**Breaking Changes:**
- TBD

**Migration Steps:**
1. TBD

### Upgrading to v1.5 (Future)

:::note[Not Yet Released]
This is a placeholder for future minor version changes.
:::

**New Features:**
- TBD

**Deprecations:**
- TBD

**Migration Steps:**
1. TBD

### Upgrading to v1.0 (Current Stable)

**What's New:**
- Stable API and workflow schema
- Production-ready daemon mode
- Comprehensive connector library
- Multi-provider support

**Migration from Beta (v0.x):**

If upgrading from pre-1.0 beta versions:

1. **Update workflow schema version:**

   ```yaml
   # Old (v0.x)
   schema_version: "0.9"

   # New (v1.0)
   version: "1.0"
   ```

2. **Update step syntax:**

   ```yaml
   # Old: action field
   - id: read
     action: file.read
     inputs:
       path: "file.txt"

   # New: type + action fields
   - id: read
     type: action
     action: file.read
     inputs:
       path: "file.txt"
   ```

3. **Update template syntax:**

   ```yaml
   # Old: Direct step reference
   prompt: "{{.read.content}}"

   # New: $ prefix for steps
   prompt: "{{$.read.content}}"
   ```

4. **Update configuration file location:**

   ```bash
   # Old location
   ~/.conductor/config.yaml

   # New location
   ~/.config/conductor/config.yaml

   # Move config
   mkdir -p ~/.config/conductor
   mv ~/.conductor/config.yaml ~/.config/conductor/config.yaml
   ```

5. **Update provider configuration:**

   ```yaml
   # Old format
   llm_provider: anthropic
   api_keys:
     anthropic: "key"

   # New format
   providers:
     anthropic:
       api_key: "key"
   default_provider: anthropic
   ```

## Breaking Changes by Version

### v1.0.0

**Changed:**
- Template variable syntax: Steps now require `$.` prefix
- Configuration file location: Moved to `~/.config/conductor/`
- Workflow schema: `schema_version` renamed to `version`
- Step syntax: Added explicit `type` field for all steps

**Removed:**
- Legacy `action` shorthand without `type` field
- Direct step references in templates (must use `$.step_id`)

**Action Required:**
- Update all workflow files to use `$.` prefix for step references
- Move configuration file to new location
- Add `type` field to all steps

### v0.9.0 (Beta)

**Changed:**
- Provider auto-detection for Claude Code
- Workflow validation strictness increased

**Deprecated:**
- Old configuration file location

## Deprecation Timeline

### Currently Deprecated

None. Version 1.0 is the stable baseline.

### Future Deprecations

:::caution[Subject to Change]
Future deprecations will be announced in release notes with migration guides.
:::

Planned deprecation policy:
1. **Announcement**: Deprecation notice in release notes
2. **Warning Period**: Minimum 2 minor versions with warnings
3. **Removal**: Breaking change in next major version

Example timeline:
- v1.5: Feature X deprecated (warnings shown)
- v1.6: Feature X still works (warnings continue)
- v2.0: Feature X removed (breaking change)

## Rollback Procedures

If you encounter issues after upgrading, you can roll back to the previous version.

### Homebrew

```bash
# View available versions
brew info conductor

# Install specific version
brew uninstall conductor
brew install conductor@1.4.0
```

### Go Install

```bash
# Install specific version
go install github.com/tombee/conductor/cmd/conductor@v1.4.0
```

### Binary

1. Download the previous version from [GitHub Releases](https://github.com/tombee/conductor/releases)
2. Replace the conductor binary
3. Verify version: `conductor --version`

### Docker

```bash
# Use specific version tag
docker pull ghcr.io/tombee/conductor:v1.4.0
```

### Restore Configuration

```bash
# Restore backed up config
cp ~/.config/conductor/config.yaml.backup ~/.config/conductor/config.yaml

# Restore workflows
tar -xzf workflows-backup-20240101.tar.gz
```

## Compatibility

### Workflow Compatibility

Conductor maintains **backward compatibility for workflows** within major versions:
- Workflows written for v1.0 will work in v1.x
- Breaking changes only occur in major version bumps (v1.x → v2.0)

### API Compatibility

Conductor follows [Semantic Versioning](https://semver.org/):
- **Major version (v1.0.0)**: Breaking changes
- **Minor version (v1.1.0)**: New features, backward compatible
- **Patch version (v1.0.1)**: Bug fixes, backward compatible

### Provider Compatibility

Conductor supports multiple LLM provider API versions:
- Anthropic: API versions 2023-06-01 and newer
- OpenAI: API v1
- Azure OpenAI: API version 2024-02-01 and newer

Provider API changes are handled internally; workflows remain unchanged.

## Testing Upgrades

### Development Environment

Test upgrades in a non-production environment first:

```bash
# Create test environment
mkdir -p ~/conductor-test
cd ~/conductor-test

# Copy workflows
cp -r ~/production/workflows ./

# Test with new version
conductor validate workflows/*.yaml
conductor run workflows/test.yaml --dry-run
```

### Staging Environment

If running in production:

1. **Deploy to staging** with new version
2. **Run test suite** of critical workflows
3. **Monitor logs** for warnings or errors
4. **Verify integrations** (APIs, webhooks, etc.)
5. **Load test** if applicable

### Gradual Rollout

For large deployments:

1. **Canary deployment**: Upgrade 10% of instances
2. **Monitor metrics**: Error rates, response times
3. **Expand gradually**: 25% → 50% → 100%
4. **Rollback if needed**: Revert to previous version

## Common Upgrade Issues

### Configuration File Not Found

**Symptom:**
```bash
Error: config file not found
```

**Solution:**
Configuration location changed in v1.0. Move your config:

```bash
mkdir -p ~/.config/conductor
cp ~/.conductor/config.yaml ~/.config/conductor/config.yaml
```

### Template Variable Errors

**Symptom:**
```bash
Error: template variable "step1" not found
```

**Solution:**
Update step references to use `$.` prefix:

```yaml
# Old
prompt: "{{.step1.response}}"

# New
prompt: "{{$.step1.response}}"
```

### Invalid Workflow Schema

**Symptom:**
```bash
Error: unknown field "schema_version"
```

**Solution:**
Update to new schema format:

```yaml
# Old
schema_version: "0.9"

# New
version: "1.0"
```

### Provider Not Found

**Symptom:**
```bash
Error: provider "anthropic" not configured
```

**Solution:**
Update provider configuration format:

```yaml
# Old format
llm_provider: anthropic
api_keys:
  anthropic: "sk-..."

# New format
providers:
  anthropic:
    api_key: "sk-..."
default_provider: anthropic
```

## Getting Help

If you encounter issues during upgrade:

1. **Check the troubleshooting guide**: [Troubleshooting](troubleshooting.md)
2. **Review release notes**: [CHANGELOG](https://github.com/tombee/conductor/blob/main/CHANGELOG.md)
3. **Ask the community**: [GitHub Discussions](https://github.com/tombee/conductor/discussions)
4. **Report bugs**: [GitHub Issues](https://github.com/tombee/conductor/issues)

When reporting upgrade issues, include:
- Previous version number
- New version number
- Full error message
- Relevant workflow snippet (sanitized)
- Operating system

## Best Practices

### Pin Versions in Production

For production deployments, pin to specific versions:

=== "Docker"

    ```yaml
    # docker-compose.yml
    services:
      conductor:
        image: ghcr.io/tombee/conductor:v1.4.0  # Specific version
    ```

=== "Kubernetes"

    ```yaml
    # deployment.yaml
    spec:
      containers:
        - name: conductor
          image: ghcr.io/tombee/conductor:v1.4.0
    ```

=== "Go Module"

    ```go
    // go.mod
    require github.com/tombee/conductor v1.4.0
    ```

### Automated Testing

Set up automated tests for critical workflows:

```bash
#!/bin/bash
# test-workflows.sh

for workflow in workflows/*.yaml; do
  echo "Testing $workflow..."
  conductor validate "$workflow" || exit 1
  conductor run "$workflow" --dry-run || exit 1
done

echo "All workflows passed validation"
```

Run this in CI/CD before deploying upgrades.

### Monitoring After Upgrade

Monitor key metrics after upgrading:
- Workflow success/failure rates
- Execution times
- API error rates
- Resource usage (CPU, memory)

Set up alerts for anomalies that may indicate upgrade issues.

## Related Resources

- [CHANGELOG](https://github.com/tombee/conductor/blob/main/CHANGELOG.md) - All version changes
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
- [Configuration Reference](../reference/configuration.md) - Configuration file format
- [GitHub Releases](https://github.com/tombee/conductor/releases) - Download specific versions
