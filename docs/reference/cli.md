# CLI Reference

Complete command-line interface reference for Conductor.

## Overview

Conductor provides a comprehensive CLI for managing workflows, providers, and daemon processes.

## Global Flags

These flags are available for all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Enable verbose output |
| `--quiet` | `-q` | Suppress non-error output |
| `--json` | | Output in JSON format |
| `--config` | | Path to config file (default: `~/.config/conductor/config.yaml`) |

## Commands

### conductor

Root command for Conductor CLI.

```bash
conductor [command]
```

**Description:**

Conductor is a command-line tool for orchestrating complex workflows with Large Language Models. It provides a simple, declarative way to define multi-step processes and execute them across different LLM providers.

**Available Commands:**
- `init` - Initialize Conductor or create a workflow
- `run` - Execute a workflow
- `validate` - Validate workflow syntax
- `providers` - Manage LLM provider configurations
- `daemon` - Manage the conductor daemon
- `runs` - View workflow run history
- `config` - Manage configuration
- `doctor` - Diagnose setup issues
- `quickstart` - Run an interactive quick start
- `examples` - Browse example workflows
- `version` - Show version information
- `completion` - Generate shell completion scripts
- `help` - Help about any command

---

### conductor init

Initialize Conductor or create a new workflow.

```bash
conductor init [name] [flags]
```

**Usage Modes:**

**1. Setup wizard (no arguments):**
```bash
conductor init
```

Runs the interactive setup wizard to configure LLM providers.

**2. Create workflow (with name):**
```bash
conductor init my-workflow
```

Creates `my-workflow/workflow.yaml` from a template.

**3. Create single file:**
```bash
conductor init --file review.yaml
```

Creates a single workflow file in the current directory.

**Flags:**

| Flag | Description |
|------|-------------|
| `--advanced` | Advanced setup with API key configuration |
| `--yes` | Accept defaults without prompts (non-interactive) |
| `--force` | Overwrite existing files |
| `--template`, `-t` | Template to use (default: `blank`) |
| `--file`, `-f` | Create single file instead of directory |
| `--list` | List available templates |

**Examples:**

```bash
# Run setup wizard
conductor init

# Create new workflow from blank template
conductor init my-workflow

# Use code-review template
conductor init --template code-review my-review

# List available templates
conductor init --list

# Non-interactive setup (CI/CD)
conductor init --yes
```

---

### conductor run

Execute a workflow.

```bash
conductor run <workflow> [flags]
```

**Arguments:**

- `<workflow>` - Path to workflow YAML file or workflow name

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--input` | `-i` | Workflow input in `key=value` format (repeatable) |
| `--input-file` | | JSON file with inputs (use `-` for stdin) |
| `--output` | `-o` | Write output to file |
| `--no-stats` | | Don't show cost/token statistics |
| `--provider` | | Override default provider |
| `--model` | | Override model tier |
| `--timeout` | | Override step timeout |
| `--dry-run` | | Show execution plan without running |
| `--quiet` | `-q` | Suppress all warnings |
| `--verbose` | `-v` | Show detailed execution logs |
| `--daemon` | `-d` | Submit to daemon for execution |
| `--background` | | Run asynchronously, return run ID immediately (implies `--daemon`) |

**Provider Resolution Order:**

1. Agent mapping lookup (if step specifies `agent`)
2. `CONDUCTOR_PROVIDER` environment variable
3. `default_provider` from config
4. Auto-detection fallback

**Execution Modes:**

- **Direct**: Execute workflow immediately in current process
- **Daemon** (`--daemon`): Submit to `conductord` daemon
- **Background** (`--background`): Asynchronous execution via daemon

**Examples:**

```bash
# Run workflow with inputs
conductor run workflow.yaml -i name=World -i greeting=Hello

# Run with JSON inputs from file
conductor run workflow.yaml --input-file inputs.json

# Save output to file
conductor run workflow.yaml -o output.json

# Dry run to see execution plan
conductor run workflow.yaml --dry-run

# Run via daemon in background
conductor run workflow.yaml --background

# Override provider
conductor run workflow.yaml --provider anthropic

# Verbose output
conductor run workflow.yaml --verbose
```

---

### conductor validate

Validate workflow YAML syntax and schema.

```bash
conductor validate <workflow>
```

**Arguments:**

- `<workflow>` - Path to workflow YAML file

**Description:**

Validates that a workflow file has valid YAML syntax and conforms to the Conductor workflow schema. This validation does not require provider configuration and only checks the workflow structure itself.

**Examples:**

```bash
# Validate workflow
conductor validate workflow.yaml
```

**Output:**

```
Validation Results:
  [OK] Syntax valid
  [OK] Schema valid
  [OK] All step references resolve correctly

Model tiers used: [balanced, strategic]
Note: Run with configured provider to validate model tier mappings
```

---

### conductor providers

Manage LLM provider configurations.

```bash
conductor providers [command]
```

**Description:**

Providers connect Conductor to Large Language Model APIs or CLIs. Each provider has a unique name and can be configured for different use cases.

**Subcommands:**

- `list` - List configured providers
- `add` - Add a new provider
- `remove` - Remove a provider
- `test` - Test provider connectivity
- `set-default` - Set default provider

Running `conductor providers` without a subcommand defaults to `list`.

---

#### conductor providers list

List configured providers.

```bash
conductor providers list [--json]
```

**Description:**

Display all configured providers with their types, status, and default indicator.

**Examples:**

```bash
# List providers
conductor providers list

# JSON output
conductor providers list --json
```

**Output:**

```
NAME         TYPE          STATUS      DEFAULT
claudecode   Claude Code   Available   *
anthropic    Anthropic     Not Tested
```

---

#### conductor providers add

Add a new provider.

```bash
conductor providers add <name> [flags]
```

**Arguments:**

- `<name>` - Provider name

**Flags:**

| Flag | Description |
|------|-------------|
| `--type` | Provider type (`claudecode`, `anthropic`, `openai`, `ollama`) |
| `--set-default` | Set as default provider |

**Examples:**

```bash
# Add Claude Code provider
conductor providers add claudecode --type claudecode

# Add Anthropic provider and set as default
conductor providers add anthropic --type anthropic --set-default
```

---

#### conductor providers remove

Remove a provider.

```bash
conductor providers remove <name>
```

**Arguments:**

- `<name>` - Provider name to remove

**Examples:**

```bash
# Remove provider
conductor providers remove openai
```

---

#### conductor providers test

Test provider connectivity.

```bash
conductor providers test <name>
```

**Arguments:**

- `<name>` - Provider name to test

**Description:**

Tests whether a provider is correctly configured and can be reached.

**Examples:**

```bash
# Test provider
conductor providers test claudecode
```

---

#### conductor providers set-default

Set default provider.

```bash
conductor providers set-default <name>
```

**Arguments:**

- `<name>` - Provider name to set as default

**Examples:**

```bash
# Set default provider
conductor providers set-default anthropic
```

---

### conductor daemon

Manage the conductor daemon.

```bash
conductor daemon [command]
```

**Description:**

Commands for managing the conductor daemon (`conductord`). The daemon is the central service that executes workflows. The CLI communicates with the daemon to run workflows, check status, and more.

**Subcommands:**

- `status` - Show daemon status and version
- `ping` - Check if daemon is reachable

---

#### conductor daemon status

Show daemon status and version.

```bash
conductor daemon status
```

**Description:**

Display the status, version, and health of the conductor daemon.

**Examples:**

```bash
# Check daemon status
conductor daemon status
```

**Output:**

```
Conductor Daemon Status
=======================

Status:     healthy
Version:    v0.1.0
Commit:     abc1234
Build Date: 2025-01-15
Go Version: go1.21.5
Platform:   darwin/arm64
Uptime:     2h15m30s

Health Checks:
  database: ok
  providers: ok
```

---

#### conductor daemon ping

Check if daemon is reachable.

```bash
conductor daemon ping
```

**Description:**

Quickly check if the conductor daemon is running and reachable.

**Examples:**

```bash
# Ping daemon
conductor daemon ping
```

**Output:**

```
Daemon is running (latency: 2ms)
```

**Exit Codes:**
- `0` - Daemon is running
- `1` - Daemon is not running

---

### conductor runs

View workflow run history.

```bash
conductor runs [command]
```

**Description:**

Manage and view workflow execution history.

**Subcommands:**

- `list` - List recent workflow runs
- `show` - Show details for a specific run
- `logs` - View logs for a run
- `cancel` - Cancel a running workflow

**Examples:**

```bash
# List recent runs
conductor runs list

# Show run details
conductor runs show <run-id>

# View logs
conductor runs logs <run-id>

# Cancel running workflow
conductor runs cancel <run-id>
```

---

### conductor config

Manage configuration.

```bash
conductor config [command]
```

**Description:**

View and edit Conductor configuration.

**Subcommands:**

- `show` - Display current configuration
- `path` - Show config file path
- `edit` - Open config in editor

**Examples:**

```bash
# Show config
conductor config show

# Show config path
conductor config path

# Edit config
conductor config edit
```

---

### conductor doctor

Diagnose setup issues.

```bash
conductor doctor
```

**Description:**

Runs diagnostic checks to identify and help resolve common setup issues:

- Provider configuration
- Provider connectivity
- Environment variables
- File permissions
- Daemon status

**Examples:**

```bash
# Run diagnostics
conductor doctor
```

**Output:**

```
Running Conductor Diagnostics...

[✓] Config file found: ~/.config/conductor/config.yaml
[✓] Providers configured: claudecode
[!] Default provider not set
[✓] Claude Code CLI detected
[!] ANTHROPIC_API_KEY not set

Recommendations:
  • Set a default provider: conductor providers set-default claudecode
  • Set ANTHROPIC_API_KEY if you plan to use Anthropic provider
```

---

### conductor quickstart

Run an interactive quick start.

```bash
conductor quickstart
```

**Description:**

Interactive tutorial that walks you through:

1. Creating your first workflow
2. Running it with an LLM provider
3. Understanding the output
4. Next steps

**Examples:**

```bash
# Start quick start tutorial
conductor quickstart
```

---

### conductor examples

Browse example workflows.

```bash
conductor examples [command]
```

**Description:**

Explore and run example workflows.

**Subcommands:**

- `list` - List available examples
- `show` - Display example code
- `run` - Run an example

**Examples:**

```bash
# List examples
conductor examples list

# Show example code
conductor examples show code-review

# Run example
conductor examples run hello-world
```

---

### conductor version

Show version information.

```bash
conductor version
```

**Description:**

Display Conductor CLI version, commit hash, and build information.

**Examples:**

```bash
# Show version
conductor version
```

**Output:**

```
Conductor v0.1.0
Commit: abc1234
Built: 2025-01-15T10:30:00Z
Go: go1.21.5
```

---

### conductor completion

Generate shell completion scripts.

```bash
conductor completion [bash|zsh|fish|powershell]
```

**Description:**

Generate shell completion scripts for various shells.

**Examples:**

```bash
# Bash completion
conductor completion bash > /etc/bash_completion.d/conductor

# Zsh completion
conductor completion zsh > "${fpath[1]}/_conductor"

# Fish completion
conductor completion fish > ~/.config/fish/completions/conductor.fish
```

---

## Environment Variables

Conductor respects the following environment variables:

| Variable | Description |
|----------|-------------|
| `CONDUCTOR_CONFIG` | Path to config file |
| `CONDUCTOR_PROVIDER` | Default provider name |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `CONDUCTOR_DAEMON_SOCKET` | Path to daemon socket |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) |
| `NO_COLOR` | Disable colored output |

---

## Exit Codes

Conductor uses consistent exit codes:

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Invalid workflow |
| `3` | Missing required input |
| `4` | Workflow execution failed |
| `5` | Provider error |
| `10` | Daemon not running |

---

## Configuration File

Default location: `~/.config/conductor/config.yaml`

See [Configuration Reference](configuration.md) for complete configuration options.

---

## Examples

### Basic Workflow Execution

```bash
# Run a workflow with inputs
conductor run workflow.yaml \
  -i code="package main" \
  -i language=go
```

### Background Execution

```bash
# Submit workflow to daemon and continue
RUN_ID=$(conductor run workflow.yaml --background --json | jq -r '.run_id')

# Check status later
conductor runs show $RUN_ID
```

### Pipeline Integration

```bash
# Validate workflow in CI
conductor validate workflow.yaml || exit 1

# Run workflow and save output
conductor run workflow.yaml \
  --input-file inputs.json \
  --output results.json
```

### Provider Management

```bash
# Initialize with provider
conductor init --advanced

# Add additional provider
conductor providers add openai --type openai

# Test providers
conductor providers test claudecode
conductor providers test openai

# Set default
conductor providers set-default openai
```

---

## Next Steps

- [Workflow Schema Reference](workflow-schema.md) - Complete YAML reference
- [Configuration Reference](configuration.md) - All configuration options
- [API Reference](api.md) - Go package documentation
- [Guides](../guides/index.md) - Practical guides
