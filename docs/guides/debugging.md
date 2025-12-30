# Debugging Workflows

Conductor provides comprehensive debugging capabilities to help you inspect, troubleshoot, and optimize your workflows during development and in production.

## Overview

Debugging tools available in Conductor:

- **Breakpoints** - Pause execution at specific steps
- **Step Mode** - Step through workflow execution one step at a time
- **State Inspection** - View inputs, outputs, and context at any step
- **Interactive Debugger** - Interactive shell for debugging commands
- **Verbose Logging** - Detailed execution logs with multiple verbosity levels
- **Run Inspection** - View details of past workflow runs

## Quick Start

Run a workflow with a breakpoint:

```bash
conductor run workflow.yaml --breakpoint step2
```

Step through a workflow one step at a time:

```bash
conductor run workflow.yaml --step
```

View details of a completed run:

```bash
conductor run show <run-id>
```

## Breakpoints

Breakpoints allow you to pause workflow execution at specific steps to inspect state, inputs, and context.

### Setting Breakpoints

Pause at a single step:

```bash
conductor run workflow.yaml --breakpoint process_data
```

Pause at multiple steps:

```bash
conductor run workflow.yaml --breakpoint step1,step3,step5
```

### Using the Debug Shell

When execution pauses at a breakpoint, you enter an interactive debug shell:

```
═══════════════════════════════════════════════════════════
Paused at step: process_data (index: 2)
───────────────────────────────────────────────────────────
Context:
  - input
  - step1_output
═══════════════════════════════════════════════════════════
Commands: continue, next, skip, abort, inspect <expr>, context, help

debug>
```

### Debug Commands

Available commands in the debug shell:

| Command | Shortcut | Description |
|---------|----------|-------------|
| `continue` | `c` | Resume execution until next breakpoint or completion |
| `next` | `n` | Execute current step and pause at the next step |
| `skip` | `s` | Skip the current step without executing it |
| `abort` | `a` | Cancel execution immediately |
| `inspect <key>` | `i <key>` | Show the value of a context variable |
| `context` | `ctx` | Dump the full workflow context as JSON |
| `help` | `h`, `?` | Show help message |

### Inspecting State

View a specific context variable:

```
debug> inspect step1_output
step1_output = {
  "data": "processed value",
  "count": 42
}
```

View the entire workflow context:

```
debug> context
Full Context:
{
  "input": {
    "filename": "data.json"
  },
  "step1_output": {
    "data": "processed value",
    "count": 42
  }
}
```

### Non-Interactive Mode

For CI/CD integration, breakpoints work in non-interactive mode. The workflow will pause and output state, then exit with a specific code:

```bash
conductor run workflow.yaml --breakpoint step2 --non-interactive
```

Exit codes:
- `0` - Completed successfully
- `1` - Error occurred
- `2` - Breakpoint/pause required (needs manual intervention)
- `3` - Validation failed

## Step Mode

Step mode pauses at every step in the workflow, allowing you to step through execution:

```bash
conductor run workflow.yaml --step
```

In step mode, you'll be prompted after each step. Use the `next` command to advance to the next step, or `continue` to resume normal execution.

## Logging and Verbosity

### Log Levels

Control the verbosity of execution logs with the `--log-level` flag:

```bash
# Debug level - shows step entry/exit, template expansion, condition evaluation
conductor run workflow.yaml --log-level debug

# Trace level - includes HTTP request/response bodies, LLM prompts/responses
conductor run workflow.yaml --log-level trace
```

Log levels (in order of increasing verbosity):
- `error` - Errors only
- `warn` - Warnings and errors
- `info` - General information (default)
- `debug` - Detailed debugging information
- `trace` - Very detailed trace information

### Trace Logs

Trace-level logs can be very large. By default, they're written to a file to avoid cluttering stdout:

```bash
conductor run workflow.yaml --log-level trace --log-file workflow-trace.log
```

### Structured Logging

For integration with log aggregation tools, use JSON format:

```bash
conductor run workflow.yaml --log-format json
```

### Filter Logs by Step

Limit verbose output to specific steps:

```bash
conductor run workflow.yaml --log-level debug --log-filter step:process_data
```

## Run Inspection

View details of completed workflow runs using the `conductor run show` command.

### Basic Usage

Show run details:

```bash
conductor run show run-abc123
```

Output:
```
Run: run-abc123
Workflow: example.yaml
Status: completed
Started: 2025-12-30T10:00:00Z
Completed: 2025-12-30T10:05:23Z
Duration: 5m23s
Progress: 5/5 steps

Inputs:
  filename: data.json
  mode: production

Output:
  {
    "processed": 150,
    "status": "success"
  }
```

### Step Details

View details for a specific step:

```bash
conductor run show run-abc123 --step process_data
```

Output:
```
Step: process_data
Index: 2
Status: completed
Duration: 2.3s

Inputs:
  {
    "data": "...",
    "config": {...}
  }

Outputs:
  {
    "result": "processed",
    "count": 150
  }
```

### JSON Output

Export run details as JSON for programmatic analysis:

```bash
conductor run show run-abc123 --json > run-details.json
```

## LLM Debugging

When debugging workflows with LLM steps, trace logs include the full prompt and response:

```bash
conductor run workflow.yaml --log-level trace
```

Trace output for LLM steps includes:
- Fully-rendered prompt after template expansion
- Complete response text (not just extracted fields)
- Token counts (prompt and completion)
- Model parameters (temperature, max_tokens, etc.)

Filter to show only LLM interactions:

```bash
conductor traces show <trace-id> --llm
```

## HTTP Action Debugging

For workflows with HTTP actions, trace logs show:
- Request URL, headers, and body
- Response status, headers, and body
- Request/response timing

```bash
conductor run workflow.yaml --log-level trace
```

Filter to show only HTTP spans:

```bash
conductor traces show <trace-id> --http
```

## Best Practices

### During Development

1. Use `--dry-run` first to validate syntax and see execution plan:
   ```bash
   conductor run workflow.yaml --dry-run
   ```

2. Set breakpoints at critical steps to inspect state:
   ```bash
   conductor run workflow.yaml --breakpoint process_data,finalize
   ```

3. Use debug log level to understand template expansion:
   ```bash
   conductor run workflow.yaml --log-level debug
   ```

### For Production Troubleshooting

1. Use `conductor run show` to inspect failed runs:
   ```bash
   conductor run show <run-id> --step <failed-step>
   ```

2. Export run details for offline analysis:
   ```bash
   conductor run show <run-id> --json > incident-report.json
   ```

3. Review traces to identify bottlenecks:
   ```bash
   conductor traces timeline <trace-id>
   ```

### Security Considerations

All debug commands respect the same permissions as workflow execution. Debug output automatically redacts:

- All `.secrets` references
- API keys and tokens in HTTP headers
- Password fields in step inputs/outputs
- Patterns matching common credential formats

Secrets are never output, even in masked form. They are indicated with `[REDACTED]`.

## Troubleshooting

### Breakpoint Not Hit

If your breakpoint isn't being triggered:

1. Verify the step ID matches exactly (case-sensitive)
2. Check if the step is skipped due to a condition
3. Ensure the workflow reaches that step

### Debug Shell Not Appearing

For the interactive debug shell to work:

1. Ensure you're not using `--quiet` or `--non-interactive`
2. Verify stdin is connected (not running in background)
3. Check that the workflow hasn't already completed

### Missing Step Details

If `conductor run show` doesn't show step details:

1. Verify the run ID is correct
2. Check that the step executed (wasn't skipped)
3. Ensure the step ID matches exactly

## Related Commands

- `conductor validate` - Syntax validation before running
- `conductor traces` - View OpenTelemetry spans for past executions
- `conductor traces timeline` - ASCII timeline visualization
- `conductor traces show --failed` - Show only failed spans

## Future Enhancements

Planned debugging features (see SPEC-195):

- **Replay** - Resume failed workflows from a specific step
- **Enhanced Dry-Run** - Deep inspection with template expansion
- **Trace Visualization** - Timeline/waterfall view of execution
- **Watch Expressions** - Auto-evaluate expressions on each step

For workflow testing capabilities (mocks, fixtures, assertions), see the [Testing Guide](testing.md).
