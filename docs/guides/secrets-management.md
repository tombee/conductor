# Secrets Management

Conductor provides secure storage and retrieval of sensitive data like API keys and credentials using multiple backend systems.

## Overview

Conductor supports two secret backends:

| Backend | Storage | Access | Best For |
|---------|---------|--------|----------|
| **keychain** | System keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager) | Read/Write | Local development |
| **env** | Environment variables (`CONDUCTOR_SECRET_*`) | Read-only | CI/CD, production |

## Quick Start

### Check Available Backends

```bash
conductor secrets status
```

Output:
```
Secret Backends
───────────────────────────────────────────────────────
keychain     ✓ available     read/write    3 keys
env          ✓ available     read-only     2 keys

Default: keychain
```

### Store a Secret

```bash
conductor secrets set providers/anthropic/api_key
```

You'll be prompted for the value (hidden input). Alternatively, pipe from stdin:

```bash
echo "sk-ant-..." | conductor secrets set providers/anthropic/api_key
```

### Retrieve a Secret

```bash
conductor secrets get providers/anthropic/api_key
```

Output (masked by default):
```
sk-a...1234 (use --unmask to show full value)
```

### List All Secrets

```bash
conductor secrets list
```

## Configuration

Configure secret backends in your config file:

```yaml
# ~/.config/conductor/config.yaml
secrets:
  default_backend: keychain  # or "env"
  sources:
    - pattern: "shared/*"
      backend: vault  # Reserved for future use
    - pattern: "providers/*"
      backend: keychain
```

### default_backend

The backend to use for write operations. Valid values: `keychain`, `env`.

### sources

Pattern-based routing for secret resolution. Patterns use `filepath.Match` syntax (e.g., `providers/*`, `shared/db/*`).

Resolution order:
1. If key matches a pattern, try that backend first
2. Fall back to `default_backend`
3. Fall back to environment variables (always available)

## Secret References

Reference secrets in your config using `$secret:` prefix:

```yaml
providers:
  anthropic:
    type: anthropic
    api_key: $secret:providers/anthropic/api_key
```

At runtime, Conductor resolves `$secret:providers/anthropic/api_key` by querying configured backends.

## Environment Variables

The `env` backend reads from environment variables with the `CONDUCTOR_SECRET_` prefix:

| Secret Key | Environment Variable |
|------------|---------------------|
| `providers/anthropic/api_key` | `CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY` |
| `webhooks/github/secret` | `CONDUCTOR_SECRET_WEBHOOKS_GITHUB_SECRET` |

The env backend also recognizes provider-specific aliases:

| Secret Key | Alternative Variable |
|------------|---------------------|
| `providers/anthropic/api_key` | `ANTHROPIC_API_KEY` |
| `providers/openai/api_key` | `OPENAI_API_KEY` |

## Migrating Existing Keys

If you have plaintext API keys in your config, migrate them to secure storage:

```bash
# Preview what would be migrated
conductor secrets migrate --dry-run

# Run migration
conductor secrets migrate
```

This will:
1. Scan your config for plaintext API keys
2. Store them in the keychain
3. Update config to use `$secret:` references
4. Create a backup of the original config

## Usage Scenarios

### Solo Developer (Recommended Setup)

After running `conductor init`, your config will be:

```yaml
default_provider: claude
providers:
  claude:
    type: claude-code
secrets:
  default_backend: keychain
```

This setup:
- Uses Claude Code CLI (no API key needed)
- Stores any future secrets in your system keychain
- Works offline with no external dependencies

### CI/CD Pipeline

In CI/CD environments where keychain isn't available, use environment variables:

```yaml
# .github/workflows/ci.yml
jobs:
  test:
    env:
      CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

Or use the standard provider aliases:

```yaml
jobs:
  test:
    env:
      ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

### Team with Shared Secrets

For teams with shared secrets, you can configure pattern-based routing:

```yaml
secrets:
  default_backend: keychain
  sources:
    - pattern: "shared/*"
      backend: vault  # Reserved for future use
    - pattern: "team/*"
      backend: vault
```

Note: Vault backend is not yet implemented. This configuration prepares for future support.

## Troubleshooting

### Keychain Not Available

**macOS:**
```bash
security unlock-keychain
```

**Linux:**
```bash
# Start the keyring daemon
gnome-keyring-daemon --unlock

# Or via systemd
systemctl --user start gnome-keyring
```

### Secret Not Found

1. Check the key exists:
   ```bash
   conductor secrets list
   ```

2. Verify the backend has the key:
   ```bash
   conductor secrets status
   ```

3. Check environment variables:
   ```bash
   env | grep CONDUCTOR_SECRET
   env | grep ANTHROPIC
   ```

### Permission Denied

The keychain may require authentication:
- macOS: System may prompt for keychain password
- Linux: Ensure Secret Service (gnome-keyring or KDE Wallet) is running

## Command Reference

| Command | Description |
|---------|-------------|
| `conductor secrets status` | Show backend availability and key counts |
| `conductor secrets list` | List all secret keys |
| `conductor secrets get <key>` | Retrieve a secret (masked by default) |
| `conductor secrets get <key> --unmask` | Show full secret value |
| `conductor secrets set <key>` | Store a secret interactively |
| `conductor secrets delete <key>` | Remove a secret |
| `conductor secrets migrate` | Migrate plaintext keys to secure storage |

## Security Best Practices

1. **Use keychain for local development** - Secrets are encrypted at rest
2. **Use env variables for CI/CD** - Inject from your CI system's secrets manager
3. **Never commit secrets** - Use `$secret:` references in config files
4. **Rotate keys regularly** - Update both the secret backend and provider dashboard
5. **Check config before commits** - Run `conductor secrets migrate --dry-run` to find plaintext keys
6. **Limit secret scope** - Create separate keys for different environments

## Next Steps

- [Configuration Reference](../reference/configuration.md) - Full configuration options
- [Debugging Guide](debugging.md) - Troubleshoot workflow issues
