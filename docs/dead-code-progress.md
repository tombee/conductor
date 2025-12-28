# Dead Code Removal Progress

**Spec:** SPEC-163
**Started:** 2025-12-28

## Summary

### Metrics
| Metric | Baseline | Current | Removed |
|--------|----------|---------|---------|
| Total Go LOC | 168,595 | TBD | ~489 lines |
| Deadcode findings | 282 items | TBD | ~13 items |
| Files removed | 0 | 1 | 1 |

### Completed Removals

#### 1. Compilation Fix + Incomplete Workflow Tracer (Commit d5b828c, 73abdab)
**Lines removed:** ~447 lines (including baseline files creation)

**Files:**
- `internal/daemon/runner/tracing.go` - DELETED (178 lines)
- `internal/daemon/runner/executor.go` - Removed dead workflow/step span tracking (~90 lines)
- `internal/daemon/runner/observability_integration_test.go` - Cleaned up test code (~5 lines)

**Items from baseline:**
- safeStartSpan (line 29)
- safeEndSpan (line 44)
- safeSetAttributes (line 59)
- safeRecordError (line 74)
- safeSetStatus (line 90)
- safeStartWorkflowRun (line 105)
- safeStartStep (line 120)
- safeEndWorkflowSpan (line 135)
- safeSetWorkflowSpanAttributes (line 150)
- safeRecordWorkflowSpanError (line 165)

**Rationale:** These functions were part of an incomplete tracing implementation. The `workflowTracer` field and `SetWorkflowTracer` method were never added to the Runner struct, making all this code unreachable (tracer was always nil).

#### 2. Unused Test Helpers and Error Mapping (Commit 132767e)
**Lines removed:** ~42 lines

**Files:**
- `internal/commands/shared/exit_codes_test.go` - Removed mockUserVisibleError type (23 lines)
- `internal/commands/shared/error_codes.go` - Removed mapExitErrorToCode function (19 lines)

**Items from baseline:**
- mockUserVisibleError type and methods (lines 34, 38, 42, 46 of exit_codes_test.go)
- mapExitErrorToCode function (line 48 of error_codes.go)

**Rationale:** Test mock was defined but never instantiated; tests use real error types instead. The error mapping function was never called.

## Remaining Work

### High Priority (Low Risk)
These are safe removals with clear evidence of being unused:

1. **Other test mocks:**
   - `mockFlakyTool` in `pkg/workflow/executor_test.go`
   - `mockAdapter` in `internal/daemon/runner/drain_test.go`

2. **Runner helper functions:**
   - `saveCheckpoint` (line 58 in checkpoint.go) - appears to be superseded by LifecycleManager
   - `startMCPServers` (line 29 in mcp_tools.go)
   - `registerMCPTools` (line 66 in mcp_tools.go)
   - `snapshotRun` (line 518 in runner.go)

3. **Unused options pattern:**
   - All 6 option functions in `internal/daemon/runner/options.go`
   - Appears the Runner uses direct Config struct instead

4. **Unused CLI helpers:**
   - `SetFlags`, `GetVerbose` in `internal/commands/shared/flags.go`
   - `Execute`, `GetVerbose`, `GetQuiet`, `GetJSON`, `GetConfigPath` in `internal/cli/root.go`

### Medium Priority (Verify First)
These should be verified as truly unused before removal:

5. **Output formatters** (`internal/output/formatter.go`)
   - DefaultFormatter, JSONFormatter, TextFormatter and all methods (7 items)
   - Verify CLI doesn't use these

6. **Config helpers** (`internal/config/xdg.go`)
   - DataDir, CacheDir functions
   - Check if these are actually dead or just not called from Go code

7. **MCP features:**
   - Lockfile management (8 functions in `internal/mcp/lockfile.go`)
   - Version resolution (15+ items in `internal/mcp/version/`)
   - Event system (11 items in `internal/mcp/events.go`)
   - Log capture (5 items in `internal/mcp/logs.go`)
   - **Decision needed:** Are these planned features or dead infrastructure?

### Low Priority (Security/Architecture Review Required)

8. **Tracing infrastructure** (`internal/tracing/`)
   - Exporters (console, OTLP, OTLP HTTP) - 9+ items
   - Sampling strategies - 8 items
   - Retention management - 6 items
   - Audit middleware - 9 items
   - Propagation helpers - 5 items
   - **Decision needed:** Is observability WIP or dead?

9. **Security features** (`pkg/security/`)
   - DNS query monitoring subsystem - 9 items
   - Metrics collection and Prometheus export - 4 items
   - Override management system - 10 items
   - Audit log rotation - 9 items
   - **Requires security review before removal**

10. **LLM features:**
    - Cost tracking query/aggregation methods (12+ items in pkg/llm/cost/)
    - Failover provider methods (3 items)
    - Provider setLastUsage methods (3 items)
    - **Decision needed:** Planned for 0.0.1 or future?

11. **Transport registry:**
    - Registry creation and management (8 items in `internal/connector/transport/`)
    - AWS SigV4 transport
    - OAuth2 transport
    - **Decision needed:** Architectural prep or dead?

## Next Steps

1. Continue with high-priority removals (test mocks, runner helpers, options pattern, CLI helpers)
2. Verify medium-priority items are truly unused
3. Get product/architecture decisions on MCP, tracing, and LLM features
4. Get security review on security feature usage
5. Re-run deadcode analysis after each phase
6. Update this document with progress

## Testing Strategy

After each removal batch:
1. Run `go build ./...` to verify compilation
2. Run `go test ./...` to verify tests pass
3. Run specific package tests for affected areas
4. Commit with clear description of what was removed and why

## Files Modified

- Compilation fix: 2 files modified, 1 file deleted
- Test helpers: 2 files modified
- **Total:** 4 files modified, 1 file deleted

## References

- Baseline analysis: docs/dead-code-baseline.md
- SPEC-163: plan.md, spec.md
