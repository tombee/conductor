# Agent-Friendly CLI Design Principles

## Philosophy and Goals

The Conductor CLI is designed to be equally usable by both human operators and LLM coding agents (Claude Code, GitHub Copilot, Cursor, etc.). This dual-audience approach ensures that automated tooling can reliably discover, learn, and invoke Conductor commands without sacrificing human usability.

### Core Principles

1. **Discoverability**: Agents should be able to discover available commands, their purposes, and usage patterns through the CLI itself
2. **Predictability**: Command structure, flags, and output formats follow consistent patterns across all commands
3. **Machine-Readable Output**: All commands support JSON output for programmatic parsing
4. **Non-Interactive Operation**: Commands can run without prompts or confirmations
5. **Clear Error Messages**: Errors include actionable suggestions for recovery
6. **Comprehensive Documentation**: Help text includes examples, flag descriptions, and cross-references

## Required Patterns for All Commands

### 1. Command Structure

Every command MUST have:

```go
cmd := &cobra.Command{
    Use:   "command <required-arg> [optional-arg]",
    Short: "Brief description (< 50 characters)",
    Annotations: map[string]string{
        "group": "execution", // One of: execution, workflow, management, mcp, diagnostics, configuration, documentation, system
    },
    Long: `Detailed description of what the command does.

Include usage guidelines, when to use this command, and key concepts.

Related commands or cross-references to other functionality.`,
    Args: cobra.ExactArgs(1), // Validate argument count
    SilenceUsage: true,  // Don't print usage on errors
    SilenceErrors: true, // Handle errors ourselves for consistent formatting
    RunE: func(cmd *cobra.Command, args []string) error {
        // Implementation
    },
}
```

### 2. Help Text Examples

Every command MUST include at least 3 examples in the `Example` field:

```go
cmd.Example = `  # Example 1: Minimal usage with required args only
  conductor run workflow.yaml

  # Example 2: Common usage with typical flags (including --json)
  conductor run workflow.yaml -i task="review code" --json

  # Example 3: Advanced usage or integration scenario
  conductor run workflow.yaml --dry-run --json | jq '.steps[].name'`
```

**Example Quality Standards:**
- All examples must be copy-paste executable (only substitute obvious placeholders)
- No real API keys, tokens, or credentials (use placeholder patterns)
- Use generic hostnames: `example.com`, `api.example.org`
- Use example IP ranges: `192.0.2.x` (TEST-NET-1, RFC 5737)
- No email addresses except `@example.com`
- Include at least one example showing `--json` output integration

### 3. Command Groups

Commands are organized into logical groups via the `Annotations` map:

- `execution`: Run and validate workflows
- `workflow`: Create, discover, and manage workflow definitions
- `management`: Manage runs, connectors, and cache
- `mcp`: Model Context Protocol server management
- `diagnostics`: Health checks, ping, completion
- `configuration`: Config and secrets management
- `documentation`: Help and docs commands
- `system`: Daemon and version commands

### 4. JSON Output Support

All commands MUST support JSON output:

```go
var jsonOutput bool

cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

// In RunE function:
useJSON := shared.GetJSON() || jsonOutput  // Check both global and local flags

if useJSON {
    resp := shared.JSONResponse{
        Version: "1.0",
        Command: "command-name",
        Success: true,
        // ... command-specific fields
    }
    encoder := json.NewEncoder(cmd.OutOrStdout())
    encoder.SetIndent("", "  ")
    return encoder.Encode(resp)
}
```

**Standard JSON Response Envelope:**

```json
{
  "@version": "1.0",
  "command": "command-name",
  "success": true,
  ...command-specific fields
}
```

**Standard Error Envelope:**

```json
{
  "@version": "1.0",
  "command": "command-name",
  "success": false,
  "errors": [
    {
      "code": "ERROR_CODE",
      "message": "Human-readable error description",
      "location": {"line": 5, "column": 3},
      "suggestion": "Specific action to resolve the issue"
    }
  ]
}
```

### 5. Non-Interactive Mode

Commands with interactive prompts MUST support non-interactive operation:

```go
var noInteractive bool

cmd.Flags().BoolVar(&noInteractive, "non-interactive", false, "Disable interactive prompts")

// In RunE function:
if shared.GetJSON() {
    noInteractive = true  // --json implies --non-interactive
}

// Detection priority order (use shared.IsNonInteractive()):
// 1. --non-interactive flag (explicit)
// 2. CONDUCTOR_NON_INTERACTIVE=true env var
// 3. CI environment detection (CI=true, GITHUB_ACTIONS=true, etc.)
// 4. stdin is not a TTY (lowest priority)
```

**Error Handling in Non-Interactive Mode:**

When required input is missing in non-interactive mode:

```go
if noInteractive && missingRequired {
    return fmt.Errorf(`missing required inputs (non-interactive mode)

Required inputs:
  --input name=<string>    Workflow name (required)
  --input version=<int>    Version number (required)

Run 'conductor run --help-inputs workflow.yaml' to see all inputs.`)
}
```

### 6. Flag Descriptions

All flags MUST have clear descriptions that include:
- What the flag does
- Expected value format (if not boolean)
- Default value (if applicable)

```go
cmd.Flags().StringVar(&provider, "provider", "", "LLM provider (e.g., anthropic, openai)")
cmd.Flags().StringVar(&model, "model", "", "Model name (e.g., claude-3-5-sonnet-20241022)")
cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without executing")
cmd.Flags().StringVar(&timeout, "timeout", "5m", "Maximum execution time (e.g., 30s, 5m, 1h)")
```

### 7. Dry-Run Support

Commands with side effects SHOULD support `--dry-run`:

```go
var dryRun bool

cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without executing")

// In RunE function:
if dryRun {
    fmt.Fprintln(cmd.OutOrStdout(), "Dry run: The following actions would be performed:")
    fmt.Fprintf(cmd.OutOrStdout(), "\nCREATE: <config-dir>/config.yaml\n")
    fmt.Fprintf(cmd.OutOrStdout(), "MODIFY: <config-dir>/providers.yaml (add provider)\n")
    fmt.Fprintf(cmd.OutOrStdout(), "\nRun without --dry-run to execute.\n")
    return nil
}
```

**Dry-Run Output Format:**
- Use placeholders for paths: `<config-dir>`, `<workflow-dir>`, etc.
- Mask sensitive values as `[REDACTED]`
- Show action type: `CREATE`, `MODIFY`, `DELETE`
- Never make network calls or write files in dry-run mode

## Code Examples

### Example 1: Basic Command (from validate.go)

```go
func NewCommand() *cobra.Command {
    var (
        schemaPath string
        jsonOutput bool
        workspace  string
        profile    string
    )

    cmd := &cobra.Command{
        Use:   "validate <workflow>",
        Short: "Validate workflow YAML syntax and schema",
        Annotations: map[string]string{
            "group": "execution",
        },
        Long: `Validate checks that a workflow file has valid YAML syntax and conforms
to the Conductor workflow schema. This validation does not require provider
configuration and only checks the workflow structure itself.

Profile Validation (SPEC-130):
  --workspace, -w <name>   Workspace for profile resolution
  --profile, -p <name>     Profile to validate against workflow requirements

When --profile is specified, validates that all workflow requirements
are satisfied by the profile bindings.`,
        Args:          cobra.ExactArgs(1),
        SilenceUsage:  true,
        SilenceErrors: true,
        RunE: func(cmd *cobra.Command, args []string) error {
            return runValidate(cmd, args, schemaPath, jsonOutput, workspace, profile)
        },
    }

    cmd.Flags().StringVar(&schemaPath, "schema", "", "Path to custom schema (default: embedded schema)")
    cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output validation results as JSON")
    cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace for profile resolution")
    cmd.Flags().StringVarP(&profile, "profile", "p", "", "Profile to validate against workflow requirements")

    return cmd
}
```

### Example 2: Command with JSON Output (from docs.go)

```go
func NewDocsCommand() *cobra.Command {
    var jsonOutput bool

    cmd := &cobra.Command{
        Use:   "docs",
        Annotations: map[string]string{
            "group": "documentation",
        },
        Short: "Show documentation URLs",
        Long: `Display URLs to various documentation resources.

Use subcommands to get specific documentation sections:
  conductor docs cli        - CLI reference documentation
  conductor docs schema     - Workflow schema documentation
  conductor docs config     - Configuration file documentation
  conductor docs workflows  - Workflow examples and guides`,
        RunE: func(cmd *cobra.Command, args []string) error {
            useJSON := shared.GetJSON() || jsonOutput
            out := cmd.OutOrStdout()

            resources := []DocResource{
                {
                    Name:        "CLI Reference",
                    Description: "Complete command-line interface reference",
                    URL:         docsBaseURL + "/reference/cli/",
                },
                // ... more resources
            }

            if useJSON {
                resp := DocsResponse{
                    JSONResponse: shared.JSONResponse{
                        Version: "1.0",
                        Command: "docs",
                        Success: true,
                    },
                    Resources: resources,
                }
                encoder := json.NewEncoder(out)
                encoder.SetIndent("", "  ")
                return encoder.Encode(resp)
            }

            // Human-readable output
            for _, r := range resources {
                fmt.Fprintf(out, "  %s\n", r.Name)
                fmt.Fprintf(out, "    %s\n", r.Description)
                fmt.Fprintf(out, "    %s\n", r.URL)
            }

            return nil
        },
    }

    cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
    return cmd
}
```

### Example 3: Non-Interactive Detection (from run.go)

```go
func NewCommand() *cobra.Command {
    var noInteractive bool

    cmd := &cobra.Command{
        Use:   "run <workflow>",
        Short: "Execute a workflow",
        RunE: func(cmd *cobra.Command, args []string) error {
            // --json implies --non-interactive
            if shared.GetJSON() {
                noInteractive = true
            }

            // Use centralized detection
            if !noInteractive && shared.IsNonInteractive() {
                noInteractive = true
            }

            return runWorkflow(args[0], noInteractive)
        },
    }

    cmd.Flags().BoolVar(&noInteractive, "non-interactive", false, "Disable interactive prompts")
    return cmd
}
```

## Anti-Patterns to Avoid

### 1. Vague Error Messages

**Bad:**
```go
return fmt.Errorf("invalid input")
```

**Good:**
```go
return fmt.Errorf("invalid workflow path %q: file does not exist or is not readable\n\nSuggestion: Verify the file path and check file permissions", path)
```

### 2. Nested Interactive Prompts

**Bad:**
```go
provider := promptForProvider()
if provider == "custom" {
    customURL := promptForURL()
    apiKey := promptForAPIKey()
}
```

**Good:**
```go
// Require all inputs up-front via flags in non-interactive mode
if noInteractive {
    if provider == "" || apiKey == "" {
        return fmt.Errorf("--provider and --api-key required in non-interactive mode")
    }
}
```

### 3. Non-Deterministic Output

**Bad:**
```go
// Output includes timestamps, UUIDs, or random values
fmt.Printf("Request ID: %s\n", uuid.New())
```

**Good:**
```go
// Deterministic output; optional verbose mode for details
if verbose {
    fmt.Printf("Request ID: %s\n", requestID)
}
```

### 4. Exposing Internal Details in Errors

**Bad:**
```go
return fmt.Errorf("panic in /internal/workflow/executor.go:142: nil pointer dereference")
```

**Good:**
```go
return fmt.Errorf("workflow execution failed: step %q encountered an error\n\nSet CONDUCTOR_DEBUG=1 for detailed trace", stepName)
```

### 5. Missing Examples in Help Text

**Bad:**
```go
cmd.Example = ""  // No examples
```

**Good:**
```go
cmd.Example = `  # Validate a workflow file
  conductor validate workflow.yaml

  # Validate with JSON output
  conductor validate workflow.yaml --json

  # Validate and check specific profile requirements
  conductor validate workflow.yaml --profile production`
```

## Code Review Checklist

When reviewing new commands or changes to existing commands, verify:

- [ ] Command has a `group` annotation
- [ ] Short description is < 50 characters
- [ ] Long description explains what, when, and why to use the command
- [ ] At least 3 examples are provided in the `Example` field
- [ ] All flags have clear descriptions including value format
- [ ] Command supports `--json` output (checks both global and local flags)
- [ ] JSON output uses standard envelope structure (`@version`, `command`, `success`)
- [ ] Interactive prompts respect `--non-interactive` flag and `IsNonInteractive()` detection
- [ ] Error messages include actionable suggestions
- [ ] Dry-run mode (if applicable) masks sensitive values and doesn't make side effects
- [ ] No API keys, real emails, or real IPs in examples
- [ ] Cross-references to related commands in Long description
- [ ] `SilenceUsage` and `SilenceErrors` are set to true for consistent error formatting

## Testing Requirements

All commands must have tests that verify:

1. **Help text quality:**
   - Example count >= 3
   - Short description < 50 characters
   - No sensitive data patterns in examples

2. **JSON output:**
   - Valid JSON structure
   - Standard envelope fields present
   - Backward compatibility with previous versions

3. **Non-interactive mode:**
   - Command runs without prompts when `--non-interactive` is set
   - Error messages list missing required inputs
   - CI environment variables trigger non-interactive behavior

4. **Dry-run mode (if applicable):**
   - No files created or modified
   - No network calls made
   - Sensitive values are masked

## References

- Spec: [SPEC-71 - LLM Agent Discoverability](../../.config/octopus/projects/github.com-tombee-conductor/specs/SPEC-71/spec.md)
- JSON output utilities: `internal/commands/shared/json_output.go`
- Non-interactive detection: `internal/commands/shared/interactive.go`
- Security masking: `internal/connector/security.go`
- Example commands: `internal/commands/run/`, `internal/commands/validate/`, `internal/commands/docs/`

## Continuous Improvement

This document should evolve as we:
- Discover new patterns that improve agent usability
- Receive feedback from agent users or LLM vendors
- Identify common mistakes in command implementations
- Learn from user issues related to discoverability

When updating this document:
- Add new patterns to the "Required Patterns" section
- Document anti-patterns in the "Anti-Patterns" section
- Update the checklist if new requirements emerge
- Keep examples synchronized with actual command implementations
