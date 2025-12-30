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

## Current Limitations

**Phase 1 MVP limitations:**

- **Interactive debugging requires terminal access** - The interactive debugger shell runs in the controller process. When using a background controller (daemon mode), breakpoints are detected but interactive commands won't work. For interactive debugging, start the controller in the foreground or use `conductor run` directly with the controller inline.

- **No remote debugging** - Currently debugging only works when running on the same machine as the controller. Remote debugging will be added in a future release.

- **LLM step details limited** - Step inspection shows inputs and outputs but doesn't yet display full LLM prompts and responses. Use `--log-level trace` for LLM visibility.

These limitations will be addressed in Phase 2 with a proper event streaming protocol between CLI and controller.

## Workflow Replay

Replay allows you to resume a failed workflow from a specific step, optionally overriding inputs or step configuration. This saves time and cost by reusing successful step outputs.

### Basic Replay

Replay from the failure point:

```bash
conductor run replay <run-id>
```

Replay from a specific step:

```bash
conductor run replay <run-id> --from-step process_data
```

### Overriding Inputs

Fix problematic inputs when replaying:

```bash
conductor run replay <run-id> --override-input "user_id=user123" --override-input "batch_size=100"
```

### Overriding Steps

Replace a failing step's configuration:

```bash
conductor run replay <run-id> --override-step "validate:skip=true"
```

### Cost Estimation

Preview estimated cost savings before replaying:

```bash
conductor run replay <run-id> --estimate
```

Output shows:
- Steps that will be skipped (cached outputs reused)
- Steps that will be re-executed
- Estimated cost in USD
- Estimated cost savings from skipping steps

### Cost Limits

Set a maximum cost for the replay:

```bash
conductor run replay <run-id> --max-cost 1.50
```

The replay will abort if estimated cost exceeds the limit.

### Replay Metrics

Replay operations are tracked with Prometheus metrics:
- `conductor_replay_total` - Counter of replay executions (labeled by workflow, status)
- `conductor_replay_cost_saved_usd` - Total cost saved through replay

### Replay Limitations

- **Structural changes block replay** - If the workflow definition has changed structurally (steps added, removed, or reordered), replay is blocked. Internal changes (prompt text, model parameters) are allowed.
- **Cached output validation** - Cached outputs are validated against current step schemas. Invalid outputs will cause replay to fail.
- **No audit of override values** - For security, audit logs record override *keys* but not the actual override *values*.

## Timeline Visualization

View workflow execution as a visual timeline showing step durations, parallelism, and costs.

### ASCII Timeline

Generate an ASCII timeline in your terminal:

```bash
conductor traces timeline <run-id>
```

Example output:
```
Workflow Timeline: example-workflow (run-abc123)
Duration: 5.2s | Total Cost: $0.042

  0s           1s           2s           3s           4s           5s
  |------------|------------|------------|------------|------------|
  ████████████ fetch_data (1.2s) $0.001
               ████████████ transform (1.1s) $0.000
               ████████ analyze (0.8s) $0.035
                        ██████ validate (0.6s) $0.001
                              ████████████ store (1.2s) $0.005

Legend: █ = 100ms
```

The timeline shows:
- Parallel step execution on separate lanes
- Step duration as bar length
- Cost per step
- Overlapping execution (concurrent steps)

### HTML Export

Export timeline as a standalone HTML file:

```bash
conductor traces export <run-id> --format html --output timeline.html
```

The HTML export includes:
- Interactive timeline with zoom and pan
- Hover details showing full step information
- Cost breakdown by step
- Filterable by step type (llm, http, shell, etc.)

### Terminal Width

The ASCII timeline auto-detects terminal width. For narrow terminals, step names are truncated with ellipsis.

## Trace Filtering

Filter traces to focus on specific span types or failures.

Show only LLM requests:
```bash
conductor traces show <trace-id> --llm
```

Show only HTTP requests:
```bash
conductor traces show <trace-id> --http
```

Show only failed spans:
```bash
conductor traces show <trace-id> --failed
```

Combine filters:
```bash
conductor traces show <trace-id> --llm --failed
```

## Trace Comparison

Compare two workflow runs to identify differences in execution:

```bash
conductor traces diff <run-id-1> <run-id-2>
```

Output shows:
- Status differences (success vs. failure)
- Duration deltas (>5% or >100ms highlighted)
- Output differences (step-by-step comparison)
- Missing or added steps

This is useful for:
- Comparing successful vs. failed runs
- Regression testing (before/after changes)
- Performance analysis

## Enhanced Dry-Run

Deep dry-run mode expands templates, evaluates conditions, and validates references without executing actions.

### Deep Template Expansion

Preview fully-expanded templates with actual inputs:

```bash
conductor run workflow.yaml --dry-run --deep
```

Dynamic values that can't be determined at dry-run time are shown with placeholders:
```
prompt: "Analyze this data: [DYNAMIC: step1_output.data]"
```

Secrets are masked:
```
api_key: "[REDACTED-OPENAI-KEY]"
```

### Condition Evaluation

See which steps would execute vs. skip:

```bash
conductor run workflow.yaml --dry-run --show-conditions
```

Output shows:
```
✓ step1 (condition: true)
✗ step2 (condition: false - skipped)
✓ step3 (no condition)
```

### Reference Validation

Validate file paths and URLs before execution:

```bash
conductor run workflow.yaml --dry-run --validate-refs
```

Checks:
- File paths exist and are readable
- URLs are accessible (HEAD request)
- Integration configurations exist

### Cost Estimation

Estimate workflow cost before running:

```bash
conductor run workflow.yaml --dry-run --estimate-cost
```

Uses historical cost data from previous runs to estimate per-step and total costs.

## Controller Mode Debugging

When running workflows via a background controller, debugging works over Server-Sent Events (SSE).

### Debug Session Creation

Run a workflow with breakpoints against a background controller:

```bash
# Start controller in background
conductor daemon start

# Run workflow with breakpoints
conductor run workflow.yaml --breakpoint step2
```

The CLI automatically:
1. Creates a debug session on the controller
2. Connects to the SSE event stream
3. Presents the interactive debug shell
4. Sends commands to the controller over HTTP

### Attaching to Sessions

If the CLI disconnects, reattach to the debug session:

```bash
conductor debug attach <session-id>
```

The session persists for 15 minutes after disconnect, allowing reconnection.

### Listing Sessions

View active debug sessions:

```bash
conductor debug sessions
```

Output:
```
SESSION ID              RUN ID          STATUS    CURRENT STEP    LAST ACTIVITY
debug-abc123-001        run-abc123      PAUSED    step2           2m ago
debug-def456-002        run-def456      RUNNING   step5           30s ago
```

### Killing Sessions

Terminate a debug session and cancel the associated run:

```bash
conductor debug sessions kill <session-id>
```

### Observer Mode

Multiple clients can observe a debug session (max 10 concurrent observers):

```bash
# Terminal 1 - Session owner
conductor run workflow.yaml --breakpoint step2

# Terminal 2 - Observer (read-only)
conductor debug attach <session-id> --observe
```

Observers receive all debug events but cannot send commands.

### Debug Session Metrics

Debug sessions are tracked with Prometheus metrics:
- `conductor_debug_sessions_active` - Gauge of active debug sessions
- `conductor_debug_events_total` - Counter of debug events (labeled by event_type)

### Session Cleanup

Completed debug sessions are automatically cleaned up 24 hours after creation to reclaim storage.

### Debug Event Types

Debug events streamed over SSE:
- `heartbeat` - Keep-alive ping (every 30 seconds)
- `step_start` - Step execution started
- `step_complete` - Step execution completed
- `paused` - Execution paused at breakpoint
- `resumed` - Execution resumed
- `completed` - Workflow completed
- `failed` - Workflow failed
- `command_error` - Invalid command error

## Feature Flags

Debug features can be controlled via environment variables:

```bash
# Disable timeline visualization
export DEBUG_TIMELINE_ENABLED=false

# Disable replay functionality
export DEBUG_REPLAY_ENABLED=false

# Disable deep dry-run mode
export DEBUG_DRYRUN_DEEP_ENABLED=false

# Disable SSE debugging
export DEBUG_SSE_ENABLED=false
```

All flags default to `true` (enabled).

## Observability and Audit

### Audit Logging

Replay operations are logged to the audit log (if enabled):

```yaml
controller:
  observability:
    audit:
      enabled: true
      destination: file
      file_path: /var/log/conductor/audit.log
```

Audit log entries include:
- User ID (if authentication enabled)
- Parent run ID
- From step
- Override keys (not values, for security)
- Cost and cost savings
- Result (success, failure, unauthorized)

### Metrics

Debug-related metrics are exported via Prometheus:

```
conductor_replay_total{workflow="example",status="success"} 12
conductor_replay_cost_saved_usd{workflow="example",status="success"} 2.34
conductor_debug_sessions_active 3
conductor_debug_events_total{event_type="step_start"} 156
```

## Related Commands

- `conductor validate` - Syntax validation before running
- `conductor run replay` - Resume failed workflows
- `conductor traces timeline` - ASCII timeline visualization
- `conductor traces export` - Export timeline as HTML
- `conductor traces diff` - Compare two workflow runs
- `conductor debug attach` - Reattach to debug session
- `conductor debug sessions` - List active debug sessions

For workflow testing capabilities (mocks, fixtures, assertions), see the [Testing Guide](testing.md).
