# Dead Code Baseline Report

**Generated:** 2025-12-28
**Spec:** SPEC-163 - Unused and Dead Code Removal

## Executive Summary

This document establishes the baseline state of dead code in the conductor codebase before systematic removal. The analysis uses automated tools and pattern searches to identify unused functions, types, constants, and other dead code patterns.

## Baseline Metrics

| Metric | Value |
|--------|-------|
| Total Go LOC | 168,595 lines |
| Deadcode findings | 282 unreachable functions/types/constants |
| golangci-lint issues (all) | 1,535 issues |
| golangci-lint unused issues | ~34 issues (sampled) |
| Deprecated code patterns | 1 instance |
| TODO: implement patterns | 0 instances |
| panic("not implemented") patterns | 0 instances |

## Tool Analysis Results

### Deadcode Tool Output

The `deadcode -test ./...` command identified **282 unreachable functions, types, and constants** across the codebase.

**Top Categories by Package:**
- `internal/mcp/*` - 52 unreachable items (version resolution, lockfile management, event system, log capture)
- `internal/tracing/*` - 49 unreachable items (exporters, sampling, retention, audit middleware, propagation)
- `pkg/security/*` - 26 unreachable items (DNS monitoring, metrics, override management, audit rotation)
- `pkg/llm/*` - 17 unreachable items (cost tracking, failover, retry, provider methods)
- `internal/controller/runner/*` - 15 unreachable items (checkpoint, MCP tools, tracing helpers, options)
- `internal/operation/*` - 14 unreachable items (package validation, transport registry, builtins)
- `pkg/workflow/*` - 8 unreachable items (cost limits, environment, executor test mocks)
- `internal/output/*` - 7 unreachable items (formatter implementations)
- Other packages - ~94 unreachable items

### golangci-lint Unused Analysis

Sample of unused code detected by golangci-lint's `unused` linter:

**Variables:**
- `internal/secrets/validation.go:42` - `malformedSchemePattern` variable unused

**Fields:**
- `pkg/security/context.go:59` - `sandboxConfig` field unused
- `pkg/llm/providers/ollama.go:19` - `baseURL` field unused
- `pkg/llm/providers/openai.go:19` - `apiKey` field unused

**Functions:**
- Multiple `setLastUsage` methods in LLM providers (unused)
- Test helper mocks in `*_test.go` files
- Tracing helpers in `internal/controller/runner/tracing.go`
- Permission helpers in `internal/permissions/*.go`

**Types:**
- `pkg/security/audit/rotation.go:349` - `Alias` type unused
- Test mock types in various `*_test.go` files

## Pattern-Based Analysis

### Deprecated Code
**Count:** 1 instance

```
./pkg/security/file.go:105:	// Deprecated: filepath.EvalSymlinks has its own internal limit for symlink resolution
```

**Analysis:** This is a comment explaining why code was removed, not deprecated code itself. No action needed.

### Placeholder/Unimplemented Code
- **"TODO: implement":** 0 instances
- **panic("not implemented"):** 0 instances

**Analysis:** Excellent - no placeholder implementations found in the codebase.

### Migration Code
Manual review required to identify migration/upgrade/convert functions.

### Feature Flags
Manual review required to identify dead conditional branches.

## Key Findings

### Compilation Errors Fixed (Pre-Analysis)
Before running dead code analysis, the following compilation errors were discovered and fixed:
- `internal/controller/runner/executor.go:149` - Reference to non-existent `r.workflowTracer` field
- `internal/controller/runner/observability_integration_test.go` - Calls to non-existent `SetWorkflowTracer` method

These were incomplete tracing implementations that were never finished. They have been commented out with TODO markers for removal during this spec.

### Security-Critical Code (Excluded from Removal)
The following directories are explicitly excluded from automated removal per AD1:
- `internal/controller/auth/` (14 files)
- `internal/secrets/` (21 files)
- `pkg/security/` (20+ files)
- `pkg/secrets/masker.go`

Any findings in these directories require manual security review before removal.

### High-Confidence Dead Code Candidates

Based on tool convergence (both deadcode and golangci-lint flagging the same items):

1. **MCP Version Resolution System** (`internal/mcp/version/`) - 15+ unreachable items
   - Semver parsing and constraint matching
   - Local resolver
   - Resolver registry
   - **Rationale:** May be for future feature; verify intended use

2. **MCP Lockfile Management** (`internal/mcp/lockfile.go`) - 8 unreachable items
   - Load/save lockfile functions
   - Lockfile manipulation methods
   - **Rationale:** May be for future feature; verify intended use

3. **MCP Event System** (`internal/mcp/events.go`) - 11 unreachable items
   - EventEmitter and all emit methods
   - **Rationale:** May be for future feature or dead

4. **Tracing Exporters** (`internal/tracing/export/`) - 9+ unreachable items
   - Console, OTLP, OTLP HTTP exporters
   - **Rationale:** Observability features may be WIP or dead

5. **Tracing Advanced Features** (`internal/tracing/`) - 20+ unreachable items
   - Sampling strategies (deterministic, random)
   - Retention management
   - Propagation helpers
   - Audit middleware
   - **Rationale:** Observability features may be WIP or dead

6. **Security Advanced Features** (`pkg/security/`) - Multiple items
   - DNS query monitoring (entire subsystem)
   - Metrics collection and Prometheus export
   - Override management system
   - Audit log rotation
   - **Rationale:** Security-critical; requires manual review per AD1

7. **LLM Cost Tracking** (`pkg/llm/cost/` and `pkg/llm/cost.go`) - 12+ unreachable items
   - Memory store query methods
   - Aggregation functions
   - **Rationale:** Feature may be planned but unfinished

8. **Transport Registry** (`internal/operation/transport/registry.go`) - 8 unreachable items
   - Registry creation and management
   - AWS SigV4 transport
   - OAuth2 transport
   - **Rationale:** May be architectural prep for future features

9. **Output Formatters** (`internal/output/formatter.go`) - 7 unreachable items
   - JSON and Text formatter implementations
   - **Rationale:** CLI output system may be dead or replaced

10. **Runner Options Pattern** (`internal/controller/runner/options.go`) - 6 unreachable items
    - Functional options for Runner construction
    - **Rationale:** Options pattern implemented but unused; check if intended

### Test Code
Multiple test helper types and mocks flagged as unused:
- `mockUserVisibleError` in `internal/commands/shared/exit_codes_test.go`
- `mockFlakyTool` in `pkg/workflow/executor_test.go`
- `mockAdapter` in `internal/controller/runner/drain_test.go`

**Analysis:** These are test code - verify they're actually used in tests or remove.

## Decisions Required

Before proceeding with removal, the following questions need answers:

1. **MCP features** (lockfile, version resolution, events): Are these planned features or dead code?
2. **Tracing exporters and sampling**: Is observability a WIP feature or can it be removed?
3. **Security advanced features**: Which security features are actively used vs planned?
4. **LLM cost tracking**: Is this a planned feature for the 0.0.1 release or future work?
5. **Transport registry**: Is this architectural prep or dead infrastructure?
6. **Runner options pattern**: Was this replaced by a different configuration approach?

## Next Steps

1. Review high-confidence candidates with product/architecture team
2. Verify security-critical code usage before any removal
3. Begin removal starting with lowest-risk items (test mocks, internal helpers)
4. Proceed phase-by-phase per plan.md
5. Re-run tools after each phase to measure progress

## References

- Baseline files:
  - `baseline-deadcode.txt` - Full deadcode tool output (282 items)
  - `baseline-lint.json` - Full golangci-lint output (1,535 issues, ~34 unused)
- SPEC-163: docs/specs/SPEC-163-spec.md
- Plan: docs/specs/SPEC-163-plan.md
