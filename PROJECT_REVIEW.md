# Project Review: Public Release Readiness

This document outlines all areas requiring review before Conductor can be released publicly. Each section contains specific review criteria and prompts that can be used for focused review sessions.

---

## Table of Contents

1. [Code Quality & Completeness](#1-code-quality--completeness)
2. [Testing & Quality Assurance](#2-testing--quality-assurance)
3. [Security](#3-security)
4. [Documentation](#4-documentation)
5. [CLI & API User Experience](#5-cli--api-user-experience)
6. [Operations & Observability](#6-operations--observability)
7. [Build, CI & Release](#7-build-ci--release)
8. [Dependencies](#8-dependencies)
9. [Performance](#9-performance)
10. [Compliance & Legal](#10-compliance--legal)
11. [Architecture & Design](#11-architecture--design)
12. [Pre-Release Cleanup](#12-pre-release-cleanup)
13. [Naming Convention Consistency](#13-naming-convention-consistency)
14. [Complexity & Simplification](#14-complexity--simplification)
15. [Feature Integration Validation](#15-feature-integration-validation)
16. [LLM-Assisted Development Improvements](#16-llm-assisted-development-improvements)

---

## ⚠️ Pre-Release Policy: No Backward Compatibility Required

> **CRITICAL**: This project has NOT been released publicly yet. There are NO existing users to support.

### What This Means

**DO:**
- Remove all legacy syntax support (e.g., `${VAR}` vs `env:VAR` - pick one and delete the other)
- Delete all backward compatibility shims and migration code
- Rename things freely to use canonical terminology (daemon→controller, connector→action/integration)
- Remove deprecated types, fields, and code paths entirely
- Break APIs, config formats, and CLI interfaces as needed for a clean design
- Delete all "for backward compatibility" comments and the code they describe

**DON'T:**
- Keep dual-syntax support "just in case"
- Add deprecation warnings - just remove the old thing
- Maintain migration paths - there's nothing to migrate from
- Preserve old naming alongside new naming
- Keep type aliases or wrapper functions for "compatibility"

### Reasoning

Backward compatibility is a **future feature** that will only matter AFTER we release v1.0. Building it now:
- Adds complexity that makes the codebase harder to understand
- Creates tech debt before we even have users
- Makes it harder for AI agents to understand what the "right" pattern is
- Wastes time maintaining code paths no one uses

### Review Implication

When reviewing any section of this document, if you find code patterns like:
- "Legacy syntax support"
- "Backward compatibility shim"
- "Deprecated but still supported"
- "Kept for migration"
- Dual implementations of the same feature

**The action is always: DELETE IT.** Don't document how to migrate - just remove the old code.

---

## 1. Code Quality & Completeness

### 1.1 Dead Code & Unused Exports

**Review Prompt:**
> Identify dead code, unused functions, unexported types that could be removed, and unused package-level variables across the codebase.

**Specific Checks:**
- [x] Unexported functions never called within their package *(ran deadcode, removed dead code)*
- [x] Exported functions/types never used outside their package *(removed replay.go, inspector.go, simplified init.go)*
- [ ] Unused struct fields
- [x] Commented-out code blocks *(grep found none in non-test code)*
- [x] TODO/FIXME comments indicating incomplete work *(0 in non-test code)*
- [x] Placeholder implementations (functions that panic or return nil) *(removed OpenAI/Ollama stubs, cleaned test runner)*

**Tools:** `deadcode`, `staticcheck`, grep for `TODO|FIXME|XXX|HACK`

---

### 1.2 Feature Completeness & Wiring

**Review Prompt:**
> Identify features that are implemented but not exposed through CLI commands or API endpoints. Find code paths that are unreachable from user entry points.

**Specific Checks:**
- [x] CLI commands defined but not registered *(verified all commands registered)*
- [x] API endpoints implemented but not routed *(reviewed, handlers wired)*
- [ ] Configuration options defined but never read
- [x] Feature flags/toggles that are always off *(featureflags package removed)*
- [ ] Interfaces with no implementations
- [ ] Implementations not wired into dependency injection

**Areas to Check:**
- `internal/commands/` - all commands registered in root?
- `internal/daemon/api/` - all handlers routed?
- `internal/config/` - all config fields consumed?
- Connectors/integrations - all registered?

---

### 1.3 Error Handling Consistency

**Review Prompt:**
> Review error handling patterns across the codebase for consistency. Identify swallowed errors, inconsistent error wrapping, and missing error context.

**Specific Checks:**
- [ ] Errors silently ignored (assigned to `_`)
- [ ] Errors logged but not returned
- [ ] Missing error context/wrapping
- [ ] Inconsistent error types (string errors vs typed errors)
- [x] Panic usage outside of truly unrecoverable situations *(no panics in non-test production code)*
- [ ] defer statements that ignore Close() errors inappropriately

---

### 1.4 Code Consistency & Style

**Review Prompt:**
> Check for code style inconsistencies, naming convention violations, and patterns that deviate from established codebase conventions.

**Specific Checks:**
- [ ] Inconsistent naming (camelCase vs snake_case in configs)
- [ ] Mixed patterns for similar operations
- [ ] Package organization inconsistencies
- [ ] Import organization (stdlib, external, internal)
- [ ] Comment style consistency

---

## 2. Testing & Quality Assurance

### 2.1 Test Coverage

**Review Prompt:**
> Analyze test coverage across all packages. Identify packages with low or no coverage, and critical paths that lack tests.

**Specific Checks:**
- [ ] Overall coverage percentage by package
- [ ] Packages with 0% coverage
- [ ] Critical paths without tests (auth, security, payments)
- [ ] Public API surface coverage
- [ ] Error paths tested (not just happy paths)

**Coverage Targets:**
- Core packages (`pkg/`): 80%+
- Security-critical code: 90%+
- CLI commands: 70%+
- Integration points: 60%+

---

### 2.2 Test Quality

**Review Prompt:**
> Review test quality beyond coverage. Identify tests that don't actually verify behavior, flaky tests, and tests with poor assertions.

**Specific Checks:**
- [ ] Tests that never fail (no real assertions)
- [ ] Tests that test implementation not behavior
- [x] Hardcoded sleep/timing dependencies (flaky) *(converted some sleep waits to polling)*
- [ ] Tests that depend on external services without mocks
- [ ] Missing edge case tests
- [ ] Missing negative/error case tests
- [ ] Table-driven tests where appropriate

---

### 2.3 Integration & E2E Tests

**Review Prompt:**
> Review integration test coverage for component interactions and end-to-end workflow execution.

**Specific Checks:**
- [ ] CLI command integration tests
- [ ] API endpoint integration tests
- [ ] Workflow execution E2E tests
- [ ] Controller lifecycle tests
- [ ] Database/backend integration tests
- [ ] External service integration tests (with mocks)

---

### 2.4 Test Infrastructure

**Review Prompt:**
> Review test helpers, fixtures, and infrastructure for maintainability and correctness.

**Specific Checks:**
- [ ] Test helpers that hide failures
- [ ] Shared test fixtures that create coupling
- [ ] Missing test cleanup (leaked resources)
- [x] Race conditions in tests (`go test -race`) *(fixed in publicapi, runner/logs, httpclient, foreach)*
- [ ] Parallel test safety

---

## 3. Security

### 3.1 Secrets & Credentials

**Review Prompt:**
> Scan for hardcoded secrets, API keys, credentials, and sensitive data in code, configs, tests, and git history.

**Specific Checks:**
- [x] Hardcoded API keys or tokens *(all matches are tests/examples with fake values)*
- [x] Test credentials that look real *(all labeled as FAKE/TEST/NOT_REAL)*
- [x] Secrets in example configs *(examples use placeholders like sk-ant-..., ghp_your_token_here)*
- [x] Sensitive data in error messages *(redaction implemented in multiple places)*
- [ ] Credentials in git history
- [x] .env files committed *(no .env files found)*

**Patterns to Search:**
```
sk-ant-, sk-, gsk_, xai-, ghp_, glpat-, xoxb-, xoxp-
password, secret, token, api_key, apikey, credential
BEGIN RSA PRIVATE KEY, BEGIN OPENSSH PRIVATE KEY
```

---

### 3.2 Input Validation

**Review Prompt:**
> Review all user input handling for proper validation. Check CLI arguments, API request bodies, file inputs, and environment variables.

**Specific Checks:**
- [ ] CLI argument validation
- [ ] API request body validation
- [x] File path validation (traversal prevention) *(filepath.Clean/Join used in 182 files)*
- [x] URL validation (SSRF prevention) *(DenyPrivate/BlockPrivate implemented in http tools)*
- [ ] Template injection prevention
- [ ] Command injection prevention
- [ ] SQL/NoSQL injection prevention (if applicable)

---

### 3.3 Authentication & Authorization

**Review Prompt:**
> Review authentication and authorization implementation for the controller API and any protected resources.

**Specific Checks:**
- [ ] Auth bypass possibilities
- [ ] Token validation completeness
- [ ] Session management security
- [ ] Privilege escalation paths
- [ ] Missing auth on endpoints
- [x] Timing attacks on auth comparisons *(subtle.ConstantTimeCompare used in auth.go:260 and bearer_auth.go:59)*

---

### 3.4 Cryptography

**Review Prompt:**
> Review cryptographic implementations for correctness and security.

**Specific Checks:**
- [x] Use of crypto/rand vs math/rand *(crypto/rand for secrets/auth/encryption; math/rand only for jitter/sampling)*
- [ ] Secure hash algorithms (no MD5/SHA1 for security)
- [ ] Proper key derivation
- [ ] Secure defaults for TLS
- [x] No custom crypto implementations *(uses standard library crypto throughout)*

---

### 3.5 Dependency Vulnerabilities

**Review Prompt:**
> Scan dependencies for known vulnerabilities.

**Tools:** `govulncheck`, `nancy`, Dependabot alerts

**Specific Checks:**
- [x] Direct dependency vulnerabilities *(govulncheck: "No vulnerabilities found")*
- [x] Transitive dependency vulnerabilities *(govulncheck: "No vulnerabilities found")*
- [ ] Outdated dependencies with security fixes

---

### 3.6 OWASP Top 10 Review

**Review Prompt:**
> Systematic review against OWASP Top 10 categories relevant to this application.

**Applicable Categories:**
- [ ] A01: Broken Access Control
- [ ] A02: Cryptographic Failures
- [ ] A03: Injection
- [ ] A04: Insecure Design
- [ ] A05: Security Misconfiguration
- [x] A06: Vulnerable Components *(govulncheck shows no vulnerabilities)*
- [ ] A07: Auth Failures
- [ ] A08: Data Integrity Failures
- [ ] A09: Logging Failures
- [x] A10: SSRF *(DenyPrivate/BlockPrivate implemented in http tools, tested)*

---

## 4. Documentation

### 4.1 Accuracy & Correctness

**Review Prompt:**
> Verify documentation accuracy against actual implementation. Identify docs that describe features differently than they work.

**Specific Checks:**
- [ ] CLI help text matches actual behavior
- [ ] API docs match actual endpoints/parameters
- [ ] Config docs match actual options
- [ ] Example workflows actually work
- [ ] Example commands produce expected output
- [ ] Version numbers are current

---

### 4.2 Completeness

**Review Prompt:**
> Identify undocumented features, missing how-to guides, and gaps in reference documentation.

**Specific Checks:**
- [ ] All CLI commands documented
- [ ] All config options documented
- [ ] All API endpoints documented
- [ ] Installation instructions complete
- [ ] Getting started guide works end-to-end
- [ ] Troubleshooting/FAQ coverage
- [ ] Error code documentation

---

### 4.3 Broken Links & References

**Review Prompt:**
> Find broken internal links, dead external links, and references to non-existent files or sections.

**Specific Checks:**
- [ ] Internal documentation links
- [ ] External website links
- [ ] Code/file references
- [ ] Image/asset links
- [ ] Anchor links within pages

---

### 4.4 Code Examples

**Review Prompt:**
> Verify all code examples in documentation are syntactically correct and actually work when run.

**Specific Checks:**
- [ ] YAML workflow examples validate
- [ ] Shell commands execute successfully
- [ ] Code snippets compile/run
- [ ] Output examples match actual output
- [ ] Examples use current API/syntax

---

### 4.5 README & First Impressions

**Review Prompt:**
> Review the main README and landing pages for clarity, accuracy, and appeal to new users.

**Specific Checks:**
- [ ] Clear value proposition
- [ ] Accurate feature list
- [ ] Working quickstart
- [ ] Installation instructions
- [ ] Links to further documentation
- [ ] License information
- [ ] Contribution guidelines link

---

## 5. CLI & API User Experience

### 5.1 CLI Consistency

**Review Prompt:**
> Review CLI commands for consistency in naming, flags, output format, and behavior patterns.

**Specific Checks:**
- [ ] Consistent verb usage (get/list/show/describe)
- [ ] Consistent flag names across commands
- [ ] Consistent output formats
- [ ] Consistent exit codes
- [ ] Subcommand organization logic

---

### 5.2 Help Text Quality

**Review Prompt:**
> Review all CLI help text for clarity, completeness, and usefulness.

**Specific Checks:**
- [ ] Command descriptions are clear
- [ ] Flag descriptions explain purpose
- [ ] Examples provided where helpful
- [ ] Default values documented
- [ ] Required vs optional clear

---

### 5.3 Error Messages

**Review Prompt:**
> Review user-facing error messages for clarity and actionability. Users should understand what went wrong and how to fix it.

**Specific Checks:**
- [ ] Errors explain what went wrong
- [ ] Errors suggest how to fix
- [ ] No internal jargon in user errors
- [ ] No stack traces in normal errors
- [ ] Consistent error formatting
- [ ] Sensitive info not leaked in errors

---

### 5.4 API Design

**Review Prompt:**
> Review REST API design for consistency, RESTful conventions, and usability.

**Specific Checks:**
- [ ] Consistent URL patterns
- [ ] Appropriate HTTP methods
- [ ] Consistent response formats
- [ ] Proper status codes
- [ ] Pagination consistency
- [ ] Error response format

---

### 5.5 Discoverability

**Review Prompt:**
> Review how easily users can discover available features and commands.

**Specific Checks:**
- [x] Help command coverage *(--help on all commands, help subcommand)*
- [x] Tab completion support *(14 files in internal/commands/completion/)*
- [ ] Suggestions for typos
- [ ] Related command hints
- [ ] Progressive disclosure

---

## 6. Operations & Observability

### 6.1 Logging

**Review Prompt:**
> Review logging implementation for consistency, appropriate levels, and operational usefulness.

**Specific Checks:**
- [ ] Consistent log levels usage
- [x] Structured logging format *(slog used in 21+ files)*
- [x] Request/correlation ID propagation *(35 files with correlation ID support)*
- [ ] No sensitive data in logs
- [ ] Appropriate verbosity at each level
- [ ] Log rotation considerations

---

### 6.2 Metrics & Monitoring

**Review Prompt:**
> Review metrics exposure and monitoring capabilities.

**Specific Checks:**
- [ ] Key metrics exposed (latency, errors, throughput)
- [x] Prometheus endpoint working *(/metrics endpoint in router.go:108)*
- [ ] Metric naming conventions
- [ ] Cardinality concerns (high-cardinality labels)
- [ ] Dashboard/alerting documentation

---

### 6.3 Health Checks

**Review Prompt:**
> Review health check endpoints and their accuracy.

**Specific Checks:**
- [x] Liveness endpoint *(/health, /v1/health endpoints implemented)*
- [ ] Readiness endpoint
- [ ] Dependency health inclusion
- [ ] Appropriate timeouts
- [ ] No false positives/negatives

---

### 6.4 Configuration Management

**Review Prompt:**
> Review configuration handling for operational correctness.

**Specific Checks:**
- [ ] All options documented
- [ ] Sensible defaults
- [ ] Environment variable support
- [ ] Config validation on startup
- [ ] Config reload capability (if claimed)
- [ ] Example/template configs

---

### 6.5 Graceful Shutdown

**Review Prompt:**
> Review shutdown behavior for graceful handling of in-flight work.

**Specific Checks:**
- [x] Signal handling (SIGTERM, SIGINT) *(signal.Notify in 8 files including controller/run.go)*
- [ ] In-flight request completion
- [ ] Resource cleanup
- [ ] Timeout on shutdown
- [ ] Status reporting during shutdown

---

### 6.6 Error Recovery

**Review Prompt:**
> Review error recovery and resilience patterns.

**Specific Checks:**
- [x] Retry logic with backoff *(RetryPolicy/BackoffConfig in 17 files)*
- [ ] Circuit breaker implementation
- [ ] Partial failure handling
- [ ] State recovery after crash
- [ ] Data durability guarantees

---

## 7. Build, CI & Release

### 7.1 Build Reproducibility

**Review Prompt:**
> Verify builds are reproducible and properly versioned.

**Specific Checks:**
- [x] Version embedding in binary *(.goreleaser.yaml sets version/commit/date via ldflags)*
- [ ] Reproducible builds
- [ ] Build instructions documented
- [x] Cross-platform builds *(.goreleaser.yaml: linux/darwin, amd64/arm64)*
- [ ] Build dependencies documented

---

### 7.2 CI Pipeline

**Review Prompt:**
> Review CI pipeline for completeness and correctness.

**Specific Checks:**
- [x] All tests run in CI *(.github/workflows/ci.yml: test, integration jobs)*
- [x] Linting enforced *(.github/workflows/ci.yml: golangci-lint)*
- [ ] Security scanning
- [ ] Coverage reporting
- [ ] Build matrix (OS/arch)
- [ ] CI passes on main branch

---

### 7.3 Release Process

**Review Prompt:**
> Review release automation and artifact generation.

**Specific Checks:**
- [x] Release automation (goreleaser) *(.goreleaser.yaml configured)*
- [ ] Changelog generation
- [ ] Binary signing
- [ ] Checksum files
- [ ] Container image builds
- [ ] Package manager support (Homebrew)

---

### 7.4 Version Management

**Review Prompt:**
> Review versioning strategy and implementation.

**Specific Checks:**
- [ ] Semantic versioning compliance
- [x] Version in `--version` output *(conductor version command shows version/commit/date)*
- [ ] Version in API responses
- [ ] Breaking change documentation
- [ ] Deprecation policy

---

## 8. Dependencies

### 8.1 Dependency Audit

**Review Prompt:**
> Audit dependencies for necessity, maintenance status, and license compatibility.

**Specific Checks:**
- [ ] Unused dependencies
- [ ] Abandoned/unmaintained dependencies
- [ ] Duplicate functionality dependencies
- [ ] Heavy dependencies for simple tasks
- [ ] Version pinning strategy

---

### 8.2 License Compliance

**Review Prompt:**
> Verify all dependency licenses are compatible with project license.

**Specific Checks:**
- [ ] Direct dependency licenses
- [ ] Transitive dependency licenses
- [ ] License compatibility with Apache 2.0
- [ ] Attribution requirements
- [ ] Copyleft contamination

**Tools:** `go-licenses`, `license-checker`

---

### 8.3 Dependency Updates

**Review Prompt:**
> Review dependency freshness and update process.

**Specific Checks:**
- [ ] Outdated dependencies
- [ ] Security updates pending
- [ ] Major version updates available
- [ ] Update automation (Dependabot)

---

## 9. Performance

### 9.1 Resource Usage

**Review Prompt:**
> Review code for resource leaks and inefficient resource usage.

**Specific Checks:**
- [ ] Unclosed file handles
- [ ] Unclosed HTTP response bodies
- [ ] Unclosed database connections
- [ ] Memory leaks (especially in long-running processes)
- [ ] Goroutine leaks
- [ ] Buffer pool usage where appropriate

---

### 9.2 Concurrency

**Review Prompt:**
> Review concurrent code for correctness and efficiency.

**Specific Checks:**
- [ ] Race conditions (`go test -race`)
- [ ] Deadlock possibilities
- [ ] Mutex usage correctness
- [ ] Channel usage patterns
- [ ] Context cancellation propagation

---

### 9.3 Scalability Concerns

**Review Prompt:**
> Identify potential scalability bottlenecks.

**Specific Checks:**
- [ ] O(n^2) or worse algorithms
- [ ] Unbounded memory growth
- [ ] Global locks/bottlenecks
- [ ] Database query efficiency
- [ ] Connection pool sizing

---

## 10. Compliance & Legal

### 10.1 License

**Review Prompt:**
> Verify license is properly applied throughout the project.

**Specific Checks:**
- [x] LICENSE file present and correct *(Apache 2.0)*
- [x] License headers in source files *(Apache 2.0 header in all .go files)*
- [ ] License in package metadata
- [ ] Third-party license notices

---

### 10.2 Privacy

**Review Prompt:**
> Review data handling for privacy concerns.

**Specific Checks:**
- [ ] Data collection disclosure
- [ ] Telemetry opt-out
- [ ] PII handling
- [ ] Data retention
- [ ] GDPR considerations (if applicable)

---

### 10.3 Export Control

**Review Prompt:**
> Review for export control considerations (cryptography).

**Specific Checks:**
- [ ] Cryptography usage documented
- [ ] Export classification (if applicable)

---

## 11. Architecture & Design

### 11.1 API Stability

**Review Prompt:**
> Review public API surface for stability and future compatibility.

**Specific Checks:**
- [x] Public API clearly defined *(pkg/ for public, internal/ for private)*
- [x] Internal packages properly marked *(internal/ directory structure)*
- [ ] Breaking change risks identified
- [ ] Deprecation paths available
- [ ] Versioning strategy for APIs

---

### 11.2 Extension Points

**Review Prompt:**
> Review extensibility mechanisms for completeness and usability.

**Specific Checks:**
- [ ] Plugin/extension documentation
- [ ] Extension API stability
- [ ] Example extensions
- [ ] Extension testing guidance

---

### 11.3 Configuration vs Code

**Review Prompt:**
> Review balance between configuration and hardcoded values.

**Specific Checks:**
- [ ] Hardcoded values that should be configurable
- [ ] Over-configuration (too many options)
- [ ] Default value appropriateness
- [ ] Configuration documentation

---

## Review Execution Plan

### Recommended Order

1. **Security** - Blockers must be fixed before public release
2. **Documentation Accuracy** - Users will try what docs say
3. **CLI/API UX** - First impressions matter
4. **Test Coverage** - Confidence in stability
5. **Code Completeness** - Remove dead weight
6. **Operations** - Production readiness
7. **Dependencies** - Legal and security
8. **Build/Release** - Distribution readiness
9. **Performance** - Can optimize later
10. **Architecture** - Design review for future

### Tracking Progress

For each section, track:
- [ ] Review completed
- [ ] Issues documented
- [ ] Issues prioritized (Blocker/High/Medium/Low)
- [ ] Fixes implemented
- [ ] Fixes verified

---

## Issue Priority Definitions

| Priority | Definition | Timeline |
|----------|------------|----------|
| **Blocker** | Prevents public release (security, legal, critical bugs) | Must fix |
| **High** | Significantly impacts user experience or trust | Should fix |
| **Medium** | Noticeable issues but workarounds exist | Nice to fix |
| **Low** | Minor issues, polish items | Backlog |

---

## 12. Pre-Release Cleanup

This section identifies code patterns that should be removed or addressed before a clean initial public release (v0.0.1 or v1.0). These patterns accumulate during development but create technical debt and confusion for new users and contributors.

### 12.1 Legacy Syntax Support

**Review Prompt:**
> Identify legacy syntax patterns being maintained for backward compatibility that can be removed before v1.0 since there are no existing users to migrate.

**Specific Locations Found:**

| Location | Pattern | Decision Needed |
|----------|---------|-----------------|
| `internal/secrets/registry.go:40-41` | `legacyEnvVarRegex` for `${VAR_NAME}` syntax | Remove or standardize on one syntax |
| `internal/secrets/cache.go:203` | "Try legacy ${VAR} syntax" fallback | Remove if standardizing |
| `internal/secrets/env_provider.go:26-30` | Supports both `env:` and `${VAR}` formats | Pick one format for v1 |
| `pkg/profile/provider.go:28,73,78` | Legacy `${VAR}` syntax references | Align with secrets module |

**Specific Checks:**
- [ ] Decide: Keep `${VAR}` or `env:VAR` as the canonical syntax
- [ ] Remove support for the deprecated syntax
- [ ] Update all documentation and examples to use canonical syntax
- [ ] Search: `grep -r "legacy.*syntax\|legacyEnvVar" --include="*.go"`

---

### 12.2 Deprecated Types and Schema Fields

**Review Prompt:**
> Identify deprecated types and schema definitions that are documented as deprecated but still exist in the codebase.

**Specific Locations Found:**

| Location | Deprecated Item | Replacement |
|----------|-----------------|-------------|
| `pkg/workflow/definition.go:105-119` | `TriggerDefinition` type | `TriggerConfig` via `listen:` |
| `schemas/workflow.schema.json:128` | `triggers` field in schema | `listen:` field |
| `pkg/workflow/definition.go:1290-1292` | Validation for deprecated `triggers:` key | Can remove post-release |

**Specific Checks:**
- [x] Remove `TriggerDefinition` type entirely (no users to migrate) *(already removed)*
- [ ] Remove `triggers` from JSON schema or mark clearly as invalid
- [x] Update workflow parsing to error on `triggers:` instead of warning *(removed UnmarshalYAML check entirely)*
- [ ] Search: `grep -r "DEPRECATED\|deprecated" --include="*.go" | grep -v "test"`

---

### 12.3 Placeholder Implementations

**Review Prompt:**
> Identify placeholder implementations that either need to be completed or removed before release.

**Critical Placeholder Code Found:**

| Location | Description | Action |
|----------|-------------|--------|
| ~~`pkg/llm/providers/openai.go`~~ | ~~Entire file is placeholder~~ | ✅ Removed |
| ~~`pkg/llm/providers/ollama.go`~~ | ~~Entire file is placeholder~~ | ✅ Removed |
| ~~`internal/commands/test/runner.go:116,163`~~ | ~~Test runner marked as "Phase 1 placeholder"~~ | ✅ Comments cleaned |
| ~~`sdk/mcp.go:46,60`~~ | ~~"TODO: Implement in Phase 2" comments~~ | ✅ Cleaned up |
| ~~`sdk/options.go:276-278`~~ | ~~`WithConductorMCP` returns "not implemented yet"~~ | ✅ Removed |
| `examples/slack-integration/workflow.yaml:94-100` | Step 3 is placeholder for connectors | Replace with real integration |

**Specific Checks:**
- [x] Decide: Include OpenAI/Ollama in v1 or remove placeholders? *(removed - not shipping)*
- [x] Search: `grep -r "placeholder\|Placeholder\|PLACEHOLDER" --include="*.go"` *(cleaned up)*
- [x] Search: `grep -r "Phase [12]\|Phase[12]" --include="*.go"` *(cleaned misleading ones)*
- [x] Verify no functions return `StatusNotImplemented` in production paths *(only in error types)*

---

### 12.4 Backward Compatibility Shims

**Review Prompt:**
> Identify backward compatibility code patterns that exist for migration purposes but have no existing users to migrate from.

**Specific Locations Found:**

| Location | Pattern | Notes |
|----------|---------|-------|
| ~~`internal/binding/resolver.go:183`~~ | ~~"backward compatibility" comment~~ | ✅ Comment removed |
| `internal/controller/trigger/scanner.go:134` | "Convert listen config to legacy trigger format" | Converts new to old format internally |
| `internal/operation/transport_config.go:21` | "backward compatibility" for plain auth values | Decide on auth format |
| `internal/operation/executor.go:189` | "backward compatibility with integrations" | Review if needed |
| `internal/config/config.go:725-736` | "backward-compatible" default workspace | No existing workspaces to compat |
| ~~`internal/permissions/context.go:56`~~ | ~~"Permissive defaults for backward compatibility"~~ | ✅ Comment cleaned |
| `pkg/tools/builtin/file.go:44,349` | "backward compatibility" notes | Review if needed |
| `pkg/llm/providers/anthropic.go:93` | Connection pool param "kept for backward compatibility" | Can simplify API |
| ~~`pkg/workflow/types.go:282`~~ | ~~Response alias "for backward compatibility"~~ | ✅ Comment clarified |

**Specific Checks:**
- [x] Search: `grep -r "backward.*compat\|compat.*backward" --include="*.go"` *(reviewed, cleaned misleading ones)*
- [x] Remove or simplify patterns that don't serve actual users *(cleaned 3 comments)*
- [ ] Document any intentional dual-support that should remain

---

### 12.5 TODO/FIXME Comments

**Review Prompt:**
> Audit TODO/FIXME comments to determine which represent incomplete work that must be addressed versus intentional future enhancements.

**High-Priority TODOs Found:**

| Location | TODO | Priority |
|----------|------|----------|
| `internal/controller/runner/checkpoint.go:46` | "Implement actual resume logic" | High - Feature incomplete |
| `internal/controller/runner/lifecycle.go:222` | "Implement actual resume logic" | High - Feature incomplete |
| `internal/commands/security/generate.go:258,281` | "Implement permission persistence (P4-T7)" | Medium |
| `pkg/workflow/definition.go:2113-2115` | Validation TODOs for JSON Schema and jq | Medium |
| ~~`internal/controller/runner/replay.go:390,395`~~ | ~~User authorization TODOs~~ | ✅ File deleted |
| `internal/commands/integrations/test.go:73` | "Implement actual connectivity testing" | Medium |
| `internal/commands/triggers/helpers.go:71,78` | "Get actual host from controller config" | Medium |

**Low-Priority TODOs (Enhancements):**
- `sdk/run.go:338` - Add temperature to StepDefinition
- `sdk/adapters.go:123` - Properly handle multi-turn conversation
- `sdk/adapters.go:187` - Implement streaming

**Specific Checks:**
- [ ] Run: `grep -rn "TODO\|FIXME\|XXX\|HACK" --include="*.go" | wc -l` (currently ~50)
- [ ] Categorize each as: Must Fix, Can Defer, Remove
- [ ] Remove TODOs for decisions already made

---

### 12.6 Feature Flags for Gradual Rollout

**Review Prompt:**
> Identify feature flags used for gradual rollout that can be simplified since all features ship together in v1.

**Feature Flags Found:**

| Location | Flag | Notes |
|----------|------|-------|
| ~~`internal/featureflags/flags.go`~~ | ~~Entire package with 4 debug flags~~ | ✅ Package removed |
| ~~`DEBUG_TIMELINE_ENABLED`~~ | ~~Timeline visualization~~ | ✅ Removed |
| ~~`DEBUG_DRYRUN_DEEP_ENABLED`~~ | ~~Deep dry-run mode~~ | ✅ Removed |
| ~~`DEBUG_REPLAY_ENABLED`~~ | ~~Workflow replay~~ | ✅ Removed |
| ~~`DEBUG_SSE_ENABLED`~~ | ~~SSE debugging~~ | ✅ Removed |
| `FOREMAN_GO_BACKEND=1` (CHANGELOG.md:109) | Legacy Electron feature flag | Remove if not shipping Electron |

**Specific Checks:**
- [x] Evaluate if feature flag package needed for v1 *(removed entirely)*
- [x] Consider removing flags where features are always-on *(all removed)*
- [x] Document any flags that should remain for debugging *(none needed)*
- [x] Search: `grep -r "featureflags\|FeatureFlag\|feature.flag" --include="*.go"` *(no results)*

---

### 12.7 Undocumented Environment Variables

**Review Prompt:**
> Identify environment variables read by the code that are not documented for users.

**Environment Variables Found (need documentation review):**

| Variable | Location | Documented? |
|----------|----------|-------------|
| `CONDUCTOR_TRACE_KEY` | `internal/tracing/storage/encryption.go` | ? |
| `CONDUCTOR_MASTER_KEY` | `internal/secrets/file.go:377` | ? |
| `CONDUCTOR_ALL_PROVIDERS` | `internal/config/supported.go:50` | ? |
| `CONDUCTOR_ALLOWED_PATHS` | `internal/mcp/server/pathutil.go` | ? |
| `CONDUCTOR_AUTO_STARTED` | `internal/controller/controller.go` | Internal only? |
| `CONDUCTOR_GITHUB_TOKEN` | `internal/controller/github/client.go` | ? |
| `CONDUCTOR_DEBUG` | `internal/log/logger.go` | ? |
| `DEBUG_TIMELINE_ENABLED` | `internal/featureflags/flags.go` | ? |
| `DEBUG_DRYRUN_DEEP_ENABLED` | `internal/featureflags/flags.go` | ? |
| `DEBUG_REPLAY_ENABLED` | `internal/featureflags/flags.go` | ? |
| `DEBUG_SSE_ENABLED` | `internal/featureflags/flags.go` | ? |

**Specific Checks:**
- [ ] Create comprehensive env var documentation
- [ ] Search: `grep -rn "os\.Getenv" --include="*.go" | grep -v "_test.go"`
- [ ] Decide which are internal vs. user-configurable
- [ ] Add to configuration reference docs

---

### 12.8 Terminology Inconsistencies

**Review Prompt:**
> Identify remaining uses of deprecated terminology (per CLAUDE.md) that should be updated.

**Deprecated Term Occurrences:**

| Term | Correct Term | Locations |
|------|--------------|-----------|
| "connector" | "action" or "integration" | ~100+ occurrences (docs, examples, code comments) |
| "daemon" | "controller" | Multiple locations |
| "foreman" | Product-specific, remove | CHANGELOG.md, docs/production/startup.md |

**Key Files to Update:**
- `docs-site/grammars/conductor.tmLanguage.json` - References "connector-shorthand"
- `docs/reference/workflow-schema.md` - Multiple "connector" references
- `examples/slack-integration/workflow.yaml:94,100` - "connector" in comments
- `baseline-deadcode.txt` - References `internal/connector/*`

**Specific Checks:**
- [ ] Search: `grep -r "connector" --include="*.go" --include="*.md" --include="*.yaml"`
- [x] Search: `grep -r "daemon" --include="*.go" | grep -v "controller"` *(cleaned daemonTimeout→controllerTimeout)*
- [ ] Update or remove references per CLAUDE.md terminology guide

---

### 12.9 CHANGELOG Cleanup

**Review Prompt:**
> Review CHANGELOG.md for pre-release cleanup before public release.

**Issues Found:**

| Issue | Location | Action |
|-------|----------|--------|
| References "Phase 1a/1b/1c/1d" | Lines 37-134 | Consider simplifying for public changelog |
| References "foreman" product | Lines 76,84,109,130,134 | Remove or clarify relationship |
| "[Unreleased]" section | Lines 8-170 | Convert to version number |
| Internal task references (T012, T013) | Comments in code | Remove from public docs |
| "Phase 1b placeholder never implemented" | Lines 18-20 | Already removed, update changelog |

**Specific Checks:**
- [ ] Rewrite CHANGELOG for public consumption
- [ ] Remove internal phase/task references
- [ ] Add proper v0.0.1 or v1.0.0 version section
- [ ] Remove references to pre-release features that were removed

---

### 12.10 Example Workflows with Placeholders

**Review Prompt:**
> Identify example workflows that contain placeholder steps or incomplete implementations.

**Examples Needing Updates:**

| Example | Issue |
|---------|-------|
| `examples/slack-integration/workflow.yaml` | Step 3 is placeholder, not real Slack integration |
| Any example using deprecated `triggers:` | Should use `listen:` |
| Examples with "connector" terminology | Update to "action" or "integration" |

**Specific Checks:**
- [ ] Verify all examples in `examples/` directory work end-to-end
- [ ] Remove or complete placeholder steps
- [ ] Update terminology to match canonical terms

---

### 12.11 Dead Code Referenced in Baselines

**Review Prompt:**
> Review dead code baselines for items that should be removed before release.

**Files Referencing Dead Code:**

| File | Content |
|------|---------|
| `baseline-deadcode.txt` | Lists 150+ unreachable functions |
| `baseline-lint.json` | Large file with lint suppressions |
| `docs/dead-code-baseline.md` | Documents known dead code |
| `.golangci.yml:51` | Excludes `internal/connector` (doesn't exist) |

**Specific Checks:**
- [ ] Remove dead code instead of baseline suppressing
- [ ] Delete `internal/connector/*` references (directory doesn't exist)
- [ ] Review baseline files and remove entries for code that should be deleted
- [ ] Run `deadcode ./...` and address findings

---

### 12.12 Security: SHA-1 Signature Support

**Review Prompt:**
> Review security-related backward compatibility code.

**Found:**
- `internal/controller/webhook/github.go:34-40` - Checks for SHA-1 signatures but rejects them

**Current Behavior:** Code checks for legacy SHA-1 signature header but returns error saying "SHA-1 signatures not supported, please use SHA-256"

**Decision Needed:**
- [ ] Remove SHA-1 check entirely (simplify code)
- [ ] Or keep check with clear error (better UX for misconfigured webhooks)

---

### Pre-Release Cleanup Priority Matrix

| Category | Priority | Effort | Impact |
|----------|----------|--------|--------|
| 12.2 Deprecated Types | High | Low | Clean API surface |
| 12.3 Placeholder Implementations | High | High | No broken features |
| 12.5 Critical TODOs | High | Medium | Feature completeness |
| 12.8 Terminology | High | Medium | User confusion |
| 12.1 Legacy Syntax | Medium | Low | API simplicity |
| 12.4 Compat Shims | Medium | Medium | Code simplicity |
| 12.7 Env Var Docs | Medium | Low | User documentation |
| 12.9 CHANGELOG | Medium | Low | Public perception |
| 12.6 Feature Flags | Low | Low | Code simplicity |
| 12.10 Examples | Low | Medium | Documentation quality |
| 12.11 Dead Code | Low | High | Code cleanliness |
| 12.12 SHA-1 Check | Low | Low | Code simplicity |

---

### Cleanup Commands Reference

```bash
# Find all backward compatibility patterns
grep -rn "backward.*compat\|compat.*backward" --include="*.go"

# Find all legacy patterns
grep -rn "legacy\|Legacy" --include="*.go"

# Find all TODOs
grep -rn "TODO\|FIXME\|XXX\|HACK" --include="*.go" | grep -v "_test.go"

# Find all placeholders
grep -rn "placeholder\|Placeholder" --include="*.go"

# Find all Phase references
grep -rn "Phase [12]" --include="*.go"

# Find deprecated terminology
grep -rn "connector" --include="*.go" --include="*.yaml" --include="*.md"

# Find undocumented env vars
grep -rn "os\.Getenv" --include="*.go" | grep -v "_test.go" | cut -d'"' -f2 | sort -u

# Run dead code analysis
deadcode ./...
```

---

## 13. Naming Convention Consistency

This section provides a comprehensive methodology for identifying and fixing terminology inconsistencies before public release. Consistent terminology is critical for user experience, documentation clarity, and contributor onboarding.

### 13.1 Canonical Terminology Reference

Per CLAUDE.md, these are the canonical terms that MUST be used consistently:

| Canonical Term | Old/Incorrect Term | Usage |
|----------------|-------------------|-------|
| **controller** | daemon | The long-running service process |
| **action** | connector | Local operations (file, shell, http, utility, transform) |
| **integration** | connector | External service APIs (GitHub, Slack, Jira) |
| **trigger** | listen | Workflow invocation configuration in YAML |
| **executor** | engine | The component that executes workflow steps |

---

### 13.2 Directory/Package Name Violations

**Review Prompt:**
> Identify directories and packages using old terminology that should be renamed.

**Priority: HIGH (User-facing, affects import paths)**

**Current Violations Found:**

| Location | Current Name | Should Be | Impact |
|----------|--------------|-----------|--------|
| ~~`internal/commands/daemon/`~~ | ~~daemon~~ | ~~controller~~ | ✅ Already named `controller/` |

**Search Commands:**
```bash
# Find daemon directories
find . -type d -name "*daemon*" -not -path "*/.octopus/*"

# Find connector directories
find . -type d -name "*connector*" -not -path "*/.octopus/*"

# Find engine directories
find . -type d -name "*engine*" -not -path "*/.octopus/*"
```

**Note:** Both the main controller code (`internal/controller/`) and CLI commands (`internal/commands/controller/`) use the correct "controller" terminology.

**Specific Checks:**
- [x] Rename `internal/commands/daemon/` to `internal/commands/controller/` *(already using controller/)*
- [x] Verify no `internal/connector/` directory exists *(confirmed - no such directory)*
- [x] Verify no `internal/engine/` directory exists *(confirmed - no such directory)*

---

### 13.3 CLI Command and Flag Violations

**Review Prompt:**
> Identify CLI commands, subcommands, and flags using old terminology.

**Priority: HIGH (User-facing, affects user muscle memory and documentation)**

**Current CLI Structure:**
```
conductor daemon start
conductor daemon stop
conductor daemon status
conductor daemon ping
```

**Decision Required:**
The CLI exposes `conductor daemon` as the command group. Options:
1. **Keep `daemon`**: Users understand "daemon" as a background service concept
2. **Rename to `controller`**: Align with internal terminology

**Flags Using Old Terminology:**

| Location | Flag | Issue | Suggestion |
|----------|------|-------|------------|
| `internal/commands/run/command.go` | `--daemon` | Historical flag (removed per SPEC-147) | Document removal |
| `internal/commands/daemon/start.go:287` | `--daemon-child` | Internal flag for spawning | Internal-only, low priority |
| `internal/commands/validate/command.go:361` | `--daemon` | References removed flag | Update help text |

**Search Commands:**
```bash
# Find daemon flags
grep -rn "\-\-daemon" --include="*.go" | grep -v "_test.go"

# Find daemon in cobra commands
grep -rn 'Use:.*"daemon' --include="*.go"

# Find connector flags
grep -rn "\-\-connector" --include="*.go"
```

**Specific Checks:**
- [ ] Decide: Keep `conductor daemon` or rename to `conductor controller`
- [ ] Update all references to deprecated `--daemon` flag in help text
- [ ] Verify no `--connector` flags exist

---

### 13.4 Type and Function Name Violations

**Review Prompt:**
> Identify exported types, structs, functions, and variables using old terminology.

**Priority: MEDIUM (API surface, affects SDK users)**

**Current Violations Found:**

| Location | Name | Type | Should Be |
|----------|------|------|-----------|
| `internal/client/dial.go:96` | `DaemonNotRunningError` | struct | `ControllerNotRunningError` |
| `internal/client/dial.go:123` | `IsDaemonNotRunning` | func | `IsControllerNotRunning` |
| `internal/client/autostart.go:26` | `AutoStartConfig` | struct | Comments reference "daemon" |
| `internal/client/autostart.go:38` | `StartDaemon` | func | `StartController` |
| `internal/client/autostart.go:110` | `EnsureDaemon` | func | `EnsureController` |
| `internal/lifecycle/doc.go:16` | package doc | comment | References "daemon" |
| `internal/lifecycle/log.go:40` | `LifecycleLogger` | doc | References "daemon lifecycle" |
| `internal/lifecycle/process.go:30` | `ErrNotConductorProcess` | var | Message says "conductor daemon" |
| `internal/commands/daemon/start.go:285` | `buildDaemonArgs` | func | `buildControllerArgs` |
| `internal/commands/daemon/start.go:339` | `getDaemonLogPath` | func | `getControllerLogPath` |

**Search Commands:**
```bash
# Find Daemon in type/func names
grep -rn "Daemon[A-Z]" --include="*.go" | grep -v "_test.go"

# Find daemon in variable names
grep -rn "daemon[A-Z]" --include="*.go" | grep -v "_test.go"

# Find Connector types
grep -rn "Connector[A-Z]" --include="*.go" | grep -v "_test.go"

# Find Engine types
grep -rn "Engine[A-Z]" --include="*.go" | grep -v "_test.go"
```

**Specific Checks:**
- [ ] Rename `DaemonNotRunningError` to `ControllerNotRunningError`
- [ ] Rename `IsDaemonNotRunning` to `IsControllerNotRunning`
- [ ] Update function names in `internal/client/autostart.go`
- [ ] Verify no `Connector` or `Engine` types exist (unless appropriate)

---

### 13.5 Configuration Key Violations

**Review Prompt:**
> Identify YAML configuration keys using old terminology.

**Priority: HIGH (User-facing, affects config files)**

**Current Configuration Structure:**

```yaml
# Example from deploy/exe.dev/examples/config.yaml
daemon:                    # Should this be 'controller:'?
  listen:                  # Note: 'listen' in config context is OK (network listener)
    tcp: ":8374"
```

**Workflow YAML Structure:**
```yaml
# listen: is used for triggers (per CLAUDE.md, should be 'trigger:')
name: my-workflow
listen:                    # VIOLATION: Should be 'trigger:'
  api:
    path: /webhook
```

**Configuration Keys Found:**

| File | Key | Should Be | Notes |
|------|-----|-----------|-------|
| `deploy/exe.dev/examples/config.yaml:11` | `daemon:` | `controller:` (decision needed) | Top-level config section |
| `pkg/workflow/definition.go:38` | `yaml:"listen,omitempty"` | `yaml:"trigger,omitempty"` | Workflow trigger definition |
| `schemas/workflow.schema.json` | `listen` | `trigger` | JSON Schema field name |

**Search Commands:**
```bash
# Find daemon config keys in YAML files
grep -rn "^daemon:" --include="*.yaml"
grep -rn "^  daemon:" --include="*.yaml"

# Find listen config keys (context-dependent)
grep -rn "^listen:" --include="*.yaml"

# Find connector config keys
grep -rn "^connector:" --include="*.yaml"
```

**Note on `listen:` Key:**
The `listen:` key is used in TWO contexts:
1. **Network configuration** (`controller.listen:`): This is appropriate - describes what to listen on
2. **Workflow triggers** (`listen:` in workflow YAML): Per CLAUDE.md, should be `trigger:`

**Specific Checks:**
- [ ] Decide: Rename config key `daemon:` to `controller:`
- [ ] Update workflow YAML key from `listen:` to `trigger:` (breaking change)
- [ ] Update JSON schema to use `trigger:` instead of `listen:`
- [ ] Update all example workflows

---

### 13.6 Documentation Violations

**Review Prompt:**
> Identify documentation files using old terminology.

**Priority: HIGH (User-facing, affects first impressions)**

**Major Documentation Files with "daemon" References:**

| File | Occurrences | Sample Violations |
|------|-------------|-------------------|
| `docs/reference/cli.md` | 30+ | "conductor daemon", "--daemon flag" |
| `docs/reference/configuration.md` | 15+ | "daemon.auto_start", "daemon.socket_path" |
| `docs/production/startup.md` | 15+ | "conductor daemon start", "daemon.log" |
| `docs/production/deployment.md` | 5+ | "daemon" references |
| `docs/production/security.md` | 5+ | "conductor daemon start" |
| `docs/building-workflows/controller.md` | 5+ | Mixed terminology |
| `docs/architecture/overview.md` | 5+ | "daemon-first", "daemon" |
| `internal/mcp/README.md` | 3+ | "daemon-level" |
| `deploy/exe.dev/README.md` | 15+ | "daemon" throughout |

**Documentation Files with "connector" References:**

| File | Occurrences | Context |
|------|-------------|---------|
| `docs/reference/workflow-schema.md` | 20+ | "connector.operation:" syntax |
| `docs/reference/error-codes.md` | 10+ | "connector.Error" |
| `docs/reference/integrations/runbooks.md` | 5+ | "connector" references |
| `docs-site/GRAMMAR.md` | 5+ | "connector shorthand" |
| `CONTRIBUTING.md` | 2 | Error type references |

**Search Commands:**
```bash
# Count daemon occurrences in docs
grep -rc "daemon" docs/ --include="*.md" | grep -v ":0$" | sort -t: -k2 -rn

# Count connector occurrences in docs
grep -rc "connector" docs/ --include="*.md" | grep -v ":0$" | sort -t: -k2 -rn

# Find listen used as trigger
grep -rn "^listen:" docs/ --include="*.md"
```

**Specific Checks:**
- [ ] Create terminology migration script for docs
- [ ] Update docs/reference/cli.md for controller terminology
- [ ] Update docs/reference/configuration.md config key names
- [ ] Update all startup/deployment guides
- [ ] Replace "connector" with "action" or "integration" per context

---

### 13.7 Error Message Violations

**Review Prompt:**
> Identify user-facing error messages using old terminology.

**Priority: HIGH (User experience during troubleshooting)**

**Error Messages Found:**

| Location | Message | Suggested Fix |
|----------|---------|---------------|
| `internal/client/dial.go:103` | "conductor daemon is not running" | "conductor controller is not running" |
| `internal/client/dial.go:112` | "Conductor daemon is not running" (guidance) | "Conductor controller is not running" |
| `internal/commands/run/executor_controller.go:171` | "Hint: Ensure 'conductord' is in your PATH" | Update hint |
| `internal/commands/workflow/quickstart.go:74` | "Hint: Start with 'conductor daemon start'" | Use controller terminology |
| `internal/controller/webhook/router.go:115` | "daemon is shutting down gracefully" | "controller is shutting down" |
| `internal/controller/api/*.go` | Multiple "daemon is shutting down" | "controller is shutting down" |

**Search Commands:**
```bash
# Find daemon in error messages
grep -rn 'fmt\.\(Errorf\|Sprintf\).*daemon' --include="*.go"
grep -rn '".*daemon.*"' --include="*.go" | grep -v "_test.go"

# Find connector in error messages
grep -rn 'fmt\.\(Errorf\|Sprintf\).*connector' --include="*.go"
```

**Specific Checks:**
- [ ] Update all "daemon" references in error messages to "controller"
- [ ] Update guidance messages in `DaemonNotRunningError`
- [ ] Update shutdown messages in API handlers

---

### 13.8 Log Message Violations

**Review Prompt:**
> Identify log messages using old terminology.

**Priority: MEDIUM (Operations, affects monitoring and debugging)**

**Log Messages Found:**

| Location | Message | Type |
|----------|---------|------|
| `internal/controller/controller.go:129` | `logger := internallog.WithComponent(..., "daemon")` | Component name |
| `internal/controller/controller.go:346` | "daemon auto-started by CLI" | Info log |
| `internal/controller/controller.go:503` | "daemon already started" | Error |
| `internal/controller/controller.go:678` | "conductord starting" | Info log |
| `internal/controller/controller.go:882` | (Shutdown comment) "daemon" | Comment |
| `internal/controller/controller.go:1062` | "daemon stopped" | Info log |
| `internal/controller/run.go:133` | "Daemon error" | Error log |
| `internal/lifecycle/log.go` | Multiple daemon lifecycle messages | All lifecycle events |

**Search Commands:**
```bash
# Find daemon in logger calls
grep -rn 'logger\.\(Info\|Warn\|Error\|Debug\).*daemon' --include="*.go"
grep -rn 'slog\.\(Info\|Warn\|Error\|Debug\).*daemon' --include="*.go"

# Find daemon component tags
grep -rn 'WithComponent.*daemon' --include="*.go"
```

**Specific Checks:**
- [ ] Update logger component name from "daemon" to "controller"
- [ ] Update lifecycle log messages
- [ ] Update all info/error/warn log messages

---

### 13.9 Comment Violations

**Review Prompt:**
> Identify code comments using old terminology.

**Priority: LOW (Internal, affects contributors)**

**Comment Pattern Violations:**

| Pattern | Count (approx) | Example Locations |
|---------|----------------|-------------------|
| "daemon" in comments | 100+ | Throughout controller/, lifecycle/, client/ |
| "connector" in comments | 50+ | workflow-schema.md, error-codes.md |
| "engine" in comments | 3 | sdk/doc.go, pkg/workflow/template_test.go |

**Search Commands:**
```bash
# Count daemon in Go comments
grep -rn "// .*daemon\|/\* .*daemon" --include="*.go" | wc -l

# Find specific patterns
grep -rn "// .*daemon" --include="*.go" | head -50
```

**Specific Checks:**
- [ ] Low priority but should be fixed for consistency
- [ ] Focus on package documentation (doc.go files) first
- [ ] Update during related code changes

---

### 13.10 Binary and Process Name Considerations

**Review Prompt:**
> Review binary and process naming for consistency.

**Priority: MEDIUM (User-facing, affects scripts and deployment)**

**Current State:**

| Name | Location | Usage |
|------|----------|-------|
| `conductor` | Main binary | CLI and controller combined |
| `conductord` | Historical reference | Legacy daemon binary name |

**References to `conductord`:**

| Location | Context |
|----------|---------|
| `internal/client/autostart.go:48` | LookPath for "conductord" first |
| `internal/commands/daemon/group.go:37` | "conductor daemon (conductord)" |
| `internal/commands/run/executor_controller.go:171` | Hint mentions "conductord" |
| `deploy/exe.dev/README.md:321` | Installation instructions |

**Specific Checks:**
- [ ] Decide: Keep `conductord` as alias or remove references
- [ ] Update autostart.go to not look for conductord
- [ ] Update all documentation to use `conductor` only

---

### 13.11 Systematic Remediation Methodology

**Phase 1: Audit (1-2 hours)**
```bash
# Run comprehensive search for all old terms
./scripts/terminology-audit.sh  # Create this script

# Or manually:
echo "=== DAEMON occurrences ===" && grep -rc "daemon" . --include="*.go" --include="*.md" --include="*.yaml" | grep -v ":0$" | wc -l
echo "=== CONNECTOR occurrences ===" && grep -rc "connector" . --include="*.go" --include="*.md" --include="*.yaml" | grep -v ":0$" | wc -l
echo "=== ENGINE occurrences ===" && grep -rc "engine" . --include="*.go" --include="*.md" --include="*.yaml" | grep -v ":0$" | wc -l
```

**Phase 2: Prioritized Fixes**

| Priority | Category | Effort | Files Affected |
|----------|----------|--------|----------------|
| P0 | Error messages | Low | ~10 files |
| P1 | CLI help text | Medium | ~5 files |
| P1 | Configuration docs | Medium | ~10 files |
| P2 | Type/function names | High | ~15 files (refactor) |
| P2 | Documentation | Medium | ~30 files |
| P3 | Log messages | Low | ~10 files |
| P4 | Comments | Low | ~50+ files |

**Phase 3: Verification**
```bash
# After fixes, verify no old terms remain in high-priority areas
grep -rn "daemon" --include="*.go" | grep -E "(Error|fmt\.|slog\.)" | grep -v "systemctl daemon"
grep -rn "connector" --include="*.go" | grep -E "(Error|fmt\.|slog\.)"
```

---

### 13.12 Breaking Changes Consideration

⚠️ **Per Pre-Release Policy: No backward compatibility needed. Just delete the old terms.**

| Change | Action |
|--------|--------|
| `daemon:` config key → `controller:` | Delete `daemon:` support, use `controller:` only |
| `listen:` workflow key → `trigger:` | Delete `listen:` support, use `trigger:` only |
| CLI `conductor daemon` → `conductor controller` | Delete `daemon` subcommand, use `controller` only |
| `DaemonNotRunningError` type → `ControllerNotRunningError` | Rename directly, no type alias |
| `connector` package → `action`/`integration` packages | Rename directly, no re-exports |

**No migration code. No deprecation warnings. No dual support. Just use the canonical term.**

---

### 13.13 Terminology Audit Checklist

**User-Facing (Must Fix):**
- [ ] All error messages use canonical terms
- [ ] All CLI help text uses canonical terms
- [ ] All configuration keys use canonical terms (or documented decision to keep)
- [ ] README and getting started docs use canonical terms
- [ ] API documentation uses canonical terms

**Developer-Facing (Should Fix):**
- [ ] Exported type names use canonical terms
- [ ] Exported function names use canonical terms
- [ ] Package documentation uses canonical terms
- [ ] Log messages use canonical terms

**Internal (Nice to Fix):**
- [ ] Internal function names use canonical terms
- [ ] Code comments use canonical terms
- [ ] Test helper names use canonical terms

---

### Terminology Search Commands Reference

```bash
# Comprehensive daemon search
grep -rn "\bdaemon\b" --include="*.go" --include="*.md" --include="*.yaml" \
  | grep -v "systemctl daemon" \
  | grep -v "_test.go" \
  | grep -v ".octopus"

# Comprehensive connector search
grep -rn "\bconnector\b" --include="*.go" --include="*.md" --include="*.yaml" \
  | grep -v "_test.go" \
  | grep -v ".octopus"

# Find workflow YAML files using listen instead of trigger
grep -rl "^listen:" --include="*.yaml" | xargs grep -l "^name:"

# Find config files with daemon section
grep -rl "^daemon:" --include="*.yaml"

# Count violations by category
echo "Go files with 'daemon':" && grep -rl "daemon" --include="*.go" | wc -l
echo "Markdown files with 'daemon':" && grep -rl "daemon" --include="*.md" | wc -l
echo "YAML files with 'daemon':" && grep -rl "daemon" --include="*.yaml" | wc -l

# Find exported symbols with old terms
grep -rn "^func [A-Z].*[Dd]aemon" --include="*.go"
grep -rn "^type [A-Z].*[Dd]aemon" --include="*.go"
grep -rn "^var [A-Z].*[Dd]aemon" --include="*.go"
```

---

## 14. Complexity & Simplification

This section focuses on identifying overengineering, premature abstraction, and opportunities for simplification before public release. Reducing complexity improves maintainability, reduces bugs, and makes onboarding easier for new contributors and users.

### 14.1 Interface Proliferation

**Review Prompt:**
> Identify excessive interface usage where concrete types would suffice. In Go, interfaces should be discovered through need, not designed upfront. Look for interfaces with single implementations, interfaces defined alongside their only implementation, and adapter/wrapper layers that only exist to satisfy an interface.

**Specific Checks:**
- [ ] Interfaces with only one implementation (consider inlining)
- [ ] Interfaces defined in the same package as their only implementation
- [ ] `Adapter`, `Wrapper`, `Proxy` types that add no functionality
- [ ] Interface parameters where concrete types would work (premature flexibility)
- [ ] Interfaces that mirror a concrete type 1:1 (no abstraction value)
- [ ] Mock-only interfaces (interfaces created solely for testing)

**Current Indicators Found:**
- 75+ interfaces defined across `internal/` and `pkg/`
- Multiple adapter types: `ExecutorAdapter`, `ProviderAdapter`, `workflowRegistryAdapter`
- Registry patterns duplicated: `operation/Registry`, `secrets/Registry`, `mcp/Registry`
- Interface segregation in `backend.go`: `RunStore`, `RunLister`, `CheckpointStore`, `StepResultStore`, `Backend`

**Questions to Ask:**
- Does this interface have more than one implementation?
- Could this be a function type instead of a single-method interface?
- Is this interface defined where it's consumed (good) or where it's implemented (often premature)?
- Would removing this interface make the code simpler to understand?

**Search Commands:**
```bash
# Find all interfaces
grep -rn "^type.*interface {" --include="*.go" | wc -l

# Find adapter/wrapper types
grep -rn "Adapter\|Wrapper\|Proxy" --include="*.go" | grep "^type"

# Find interfaces with potentially single implementations
for iface in $(grep -rho "^type \w\+ interface" --include="*.go" | awk '{print $2}'); do
  count=$(grep -rc "$iface" --include="*.go" | grep -v ":0$" | wc -l)
  if [ "$count" -lt 3 ]; then echo "$iface: $count files"; fi
done
```

---

### 14.2 Configuration Explosion

**Review Prompt:**
> Review the configuration surface area for unnecessary options. Configuration that is never changed from defaults, options that require deep domain knowledge to tune, and features that could be hardcoded to sensible defaults are candidates for simplification.

**Specific Checks:**
- [ ] Config fields that are never overridden (grep for field usage)
- [ ] Config options with no user-facing documentation
- [ ] Nested configuration more than 3 levels deep
- [ ] Boolean flags that are always true or always false
- [ ] Timeout/retry values that users wouldn't know how to tune
- [ ] Options that conflict or interact in complex ways
- [ ] Config structs that mirror other config structs

**Current Indicators Found:**
- 100+ configuration struct types across the codebase
- `ControllerConfig` has 25+ fields with 8+ nested config types
- `ObservabilityConfig` contains `SamplingConfig`, `StorageConfig`, `RetentionConfig`, `ExporterConfig`, `TLSConfig`, `RedactionConfig`
- `SecurityConfig` embeds `PolicyConfig`, `AuditConfig`, `MetricsConfig`, `OverrideConfig`
- Duplicate config patterns: `TLSConfig` defined multiple times

**Simplification Candidates:**

| Config Area | Current State | Simplification Option |
|-------------|---------------|----------------------|
| `ControllerLogConfig` | Separate from `LogConfig` | Merge into single log config |
| `Policy.MinimumProfile` | Organization policy feature | Remove if not used |
| `ObservabilityConfig` | 6 nested config types | Flatten for common cases |
| `AuditRotationConfig` | Complex rotation settings | Simplify to single retention duration |
| `TLSConfig` | Defined multiple times | Consolidate to single shared type |

**Search Commands:**
```bash
# Count config structs
grep -rn "^type.*Config struct" --include="*.go" | wc -l

# Find deepest nesting
grep -rn "^\s\+\w\+Config\s" --include="*.go" | head -20

# Find unused config fields
# (Requires manual analysis - grep for field name usage)
```

---

### 14.3 Abstraction Layer Depth

**Review Prompt:**
> Measure abstraction depth and identify areas where code must pass through too many layers. Deep call stacks indicate overengineering and make debugging difficult.

**Specific Checks:**
- [ ] More than 3 package hops to reach actual implementation
- [ ] Wrapper functions that just call another function
- [ ] Factory functions that don't add configuration value
- [ ] Builder patterns where simple constructors would work
- [ ] Chain-of-responsibility patterns with 1-2 handlers

**Current Patterns to Evaluate:**

1. **LLM Call Path:**
   ```
   SDK.Run -> executeWorkflow -> Executor.Execute -> LLMProvider.Complete
   -> ProviderAdapter.Complete -> RetryableProvider.Complete -> Provider.Complete
   ```
   - Question: Is `ProviderAdapter` necessary? Does `RetryableProvider` add value?

2. **Registry Layers:**
   ```
   Controller uses:
   - operation/Registry (for integrations)
   - secrets/Registry (for secret resolution)
   - mcp/Registry (for MCP servers)
   - tools/Registry (for CLI tools)
   ```
   - Question: Could these be unified or simplified?

3. **Backend Abstraction:**
   ```
   Backend interface -> memory.Backend or postgres.Backend
   ```
   - This is appropriate - two implementations justify the interface

**Depth Analysis:**
```bash
# Find packages with many imports (high coupling)
for pkg in $(find internal -type d); do
  if [ -f "$pkg"/*.go 2>/dev/null ]; then
    imports=$(grep -h "^import\|^\t\"" "$pkg"/*.go 2>/dev/null | grep "conductor" | wc -l)
    echo "$pkg: $imports imports"
  fi
done | sort -t: -k2 -rn | head -20
```

---

### 14.4 Duplicate Patterns

**Review Prompt:**
> Identify repeated patterns that could be consolidated. Duplicate implementations diverge over time and create maintenance burden.

**Specific Checks:**
- [ ] Similar registry implementations across packages
- [ ] Repeated error wrapping patterns
- [ ] Duplicated validation logic
- [ ] Copy-paste configuration loading
- [ ] Multiple implementations of the same concept

**Current Indicators Found:**

1. **Registry Pattern Duplication:**
   - `internal/operation/registry.go`
   - `internal/secrets/registry.go`
   - `internal/mcp/registry.go`
   - `internal/integration/registry.go`
   - Each implements: Register, Get, List with minor variations

2. **Error Type Duplication:**
   - `internal/operation/errors.go`
   - `internal/action/*/errors.go` (file, http, transform, utility)
   - `internal/integration/*/errors.go` (per integration)
   - Each integration defines similar error types

3. **Transport/HTTP Client Patterns:**
   - `internal/operation/transport/*.go`
   - `pkg/httpclient/*.go`
   - Potentially overlapping HTTP client implementations

**Simplification Options:**
- [ ] Create generic `Registry[T]` type using Go generics
- [ ] Consolidate error types into `pkg/errors`
- [ ] Create shared HTTP client wrapper

---

### 14.5 Nice-to-Have Features for Post-v1

**Review Prompt:**
> Identify features that add complexity without providing core value. These could be deferred to post-v1 to reduce surface area and maintenance burden.

**Candidate Features to Evaluate:**

| Feature | Location | Complexity | Essential for v1? |
|---------|----------|------------|-------------------|
| Distributed Mode | `ControllerConfig.Distributed` | High - leader election, Postgres | Single-user CLI doesn't need |
| OTLP Export | `ObservabilityConfig.Exporters` | Medium - TLS, multiple types | Local SQLite may suffice |
| Security Override System | `OverrideConfig` | Medium - TTL, emergency bypass | Audit logging may suffice |
| Poll Triggers | `internal/controller/polltrigger/` | High - 16 files, 4 integrations | Webhooks may suffice |
| Multiple Secret Backends | `internal/secrets/` | Medium - keychain, file, env | Env-only may suffice |
| Workflow Replay | `internal/controller/runner/replay.go` | Medium - 400+ lines, TODOs | Debug feature incomplete |
| Circuit Breaker | `LLMConfig.CircuitBreaker*` | Low - 2 fields | Simple retry may suffice |

**Review Criteria:**
- [ ] Has anyone used this feature in production?
- [ ] Is the feature complete and tested?
- [ ] Does removing it simplify configuration?
- [ ] Can it be added later without breaking changes?

**File Count by Feature Area:**
```bash
# Poll triggers complexity
find internal/controller/polltrigger -name "*.go" | wc -l  # Currently 16 files

# MCP complexity
find internal/mcp -name "*.go" | wc -l  # Currently 28+ files

# Tracing complexity
find internal/tracing -name "*.go" | wc -l  # Currently 24+ files
```

---

### 14.6 SDK vs CLI Code Duplication

**Review Prompt:**
> The SDK (`/sdk`) and CLI use separate execution paths. Identify where logic is duplicated and whether unification is possible.

**Specific Checks:**
- [ ] Workflow execution logic duplicated between SDK and CLI
- [ ] Tool registration duplicated
- [ ] LLM provider initialization duplicated
- [ ] Event handling duplicated

**Current Structure:**
- `sdk/` - 17 Go files, provides Go SDK for programmatic use
- `internal/controller/runner/` - provides CLI/daemon execution
- `pkg/workflow/executor.go` - shared executor

**Duplication Analysis:**

| Capability | SDK Location | CLI Location | Shared? |
|------------|--------------|--------------|---------|
| Workflow parsing | `sdk/workflow.go` | `pkg/workflow/definition.go` | Partially |
| Step execution | `sdk/run.go` | `internal/controller/runner/adapter.go` | Uses same Executor |
| LLM provider init | `sdk/options.go` | `internal/llm/adapter.go` | Different paths |
| Tool registration | `sdk/tool.go` | `pkg/tools/registry.go` | Different registries |
| Event emission | `sdk/events.go` | `internal/tracing/` | Different systems |

**Questions:**
- [ ] Should SDK use the same executor as CLI?
- [ ] Are there features available in one path but not the other?
- [ ] Is SDK adequately tested against the same code paths as CLI?

---

### 14.7 Integration Completeness Audit

**Review Prompt:**
> Review each integration to ensure consistent quality. Incomplete integrations should be removed or marked experimental.

**Integrations Present:**
```
internal/integration/
  cloudwatch/  datadog/  discord/  elasticsearch/  github/
  jenkins/     jira/     loki/     pagerduty/      slack/
  splunk/
```

**Per-Integration Checklist:**

| Integration | Files | Has Tests | Has Docs | Has Example | Poll Support |
|-------------|-------|-----------|----------|-------------|--------------|
| cloudwatch | 3 | ? | ? | ? | No |
| datadog | 4 | ? | ? | ? | Yes |
| discord | 7 | ? | ? | ? | No |
| elasticsearch | 3 | ? | ? | ? | No |
| github | 9 | ? | ? | ? | No |
| jenkins | 5 | ? | ? | ? | No |
| jira | 5 | ? | ? | ? | Yes |
| loki | 2 | ? | ? | ? | No |
| pagerduty | 5 | ? | ? | ? | Yes |
| slack | 8 | ? | ? | ? | Yes |
| splunk | 3 | ? | ? | ? | No |

**Red Flags to Check:**
- [ ] Integrations with only `errors.go` defined
- [ ] Integrations without tests
- [ ] Integrations not documented in user guides
- [ ] Integrations with placeholder implementations

---

### 14.8 Package Size & Cohesion

**Review Prompt:**
> Review package organization for appropriate size and cohesion. Packages that are too large indicate god objects; packages that are too small indicate over-modularization.

**Current Package Statistics:**
- `internal/` - 686 Go files
- `pkg/` - 162 Go files
- `sdk/` - 17 Go files

**Large Files Requiring Review:**

| File | Size | Lines | Concern |
|------|------|-------|---------|
| `internal/controller/controller.go` | 45KB+ | 1000+ | God object - manages too many concerns |
| `internal/config/config.go` | 30KB+ | 1300+ | Configuration explosion |
| `pkg/workflow/definition.go` | Large | 2100+ | Many types in one file |

**Controller.go Responsibilities (too many):**
- Backend initialization
- Scheduler management
- MCP registry management
- File watcher management
- Poll trigger management
- Auth middleware
- API server
- Tracing/observability
- Security management

**Specific Checks:**
- [ ] Split `controller.go` into focused components
- [ ] Move trigger management to separate service
- [ ] Extract MCP management from controller
- [ ] Identify single-file packages that could be merged

---

### 14.9 Resume-Driven Development Signs

**Review Prompt:**
> Look for patterns that suggest features were added for impressiveness rather than user need. These often add maintenance burden without proportional value.

**Warning Signs:**
- [ ] Complex patterns with simple requirements
- [ ] Buzzword-heavy code without matching complexity needs
- [ ] Features that work but no one uses
- [ ] Over-specified error types (10+ error types in one package)
- [ ] Generic abstractions before second use case
- [ ] Plugin systems with no plugins

**Current Observations:**

1. **Circuit Breaker in LLM Config:**
   ```go
   CircuitBreakerThreshold int
   CircuitBreakerTimeout time.Duration
   ```
   - Question: Is this actually needed for single-user CLI tool?
   - Simpler retry with backoff may suffice

2. **Interface Segregation in Backend:**
   ```go
   type RunStore interface { ... }
   type RunLister interface { ... }
   type CheckpointStore interface { ... }
   type StepResultStore interface { ... }
   type Backend interface { /* embeds all above */ }
   ```
   - Good pattern in theory - verify each segregated interface is used independently

3. **Transport Registry Pattern:**
   - `operation/transport/registry.go` for HTTP transport types
   - Question: Are there actually multiple transport types used?

4. **Security Override System:**
   - Emergency override with TTL
   - Complex for edge case - is this needed for v1?

---

### 14.10 Dead Extension Points

**Review Prompt:**
> Identify extension points, plugin mechanisms, or hook systems that have no actual extensions. These represent complexity without value.

**Specific Checks:**
- [ ] Hook interfaces with no registered hooks
- [ ] Event systems with no subscribers
- [ ] Plugin registries with no plugins
- [ ] Factory functions that always return same type
- [ ] Strategy patterns with single strategy

**Search Commands:**
```bash
# Find registries and count registrations
grep -rn "\.Register(" --include="*.go" | grep -v "_test.go" | wc -l

# Find factory functions that might be over-abstracted
grep -rn "func New.*Factory" --include="*.go"

# Find event emitters and check for subscribers
grep -rn "\.Emit\|OnEvent\|AddListener" --include="*.go" | wc -l
```

---

### 14.11 Simplification Action Items

**Priority Order for Simplification:**

**High Impact, Low Effort:**
- [ ] Remove feature flags that are always enabled (`internal/featureflags/`)
- [ ] Delete incomplete TODO code that won't ship (see Section 12.5)
- [ ] Consolidate duplicate error types
- [ ] Remove unused configuration options

**High Impact, Medium Effort:**
- [ ] Split `controller.go` into smaller components (<500 lines each)
- [ ] Simplify configuration for common use cases (provide presets)
- [ ] Unify registry implementations with generic type
- [ ] Remove or complete replay feature

**Medium Impact, Documented:**
- [ ] Mark distributed mode as experimental/optional
- [ ] Document which integrations are production-ready
- [ ] Create "minimal config" example showing just required settings
- [ ] List post-v1 features in ROADMAP.md

**Tech Debt Backlog:**
- [ ] Create generic `Registry[T]` once patterns stabilize
- [ ] Consolidate SDK and CLI execution paths where sensible
- [ ] Implement proper checkpoint resume or remove the feature

---

### 14.12 Complexity Metrics

**Ongoing Measurements:**

```bash
# Count interfaces vs implementations
echo "Interfaces:" && grep -rn "^type.*interface {" --include="*.go" | wc -l

# Count config structs
echo "Config structs:" && grep -rn "^type.*Config struct" --include="*.go" | wc -l

# Find largest files
find . -name "*.go" -not -path "*/.octopus/*" -exec wc -l {} + | sort -n | tail -20

# Count lines per package (top 10)
for dir in $(find internal pkg sdk -type d -not -path "*/.octopus/*"); do
  if ls "$dir"/*.go 1>/dev/null 2>&1; then
    lines=$(wc -l "$dir"/*.go 2>/dev/null | tail -1 | awk '{print $1}')
    echo "$lines $dir"
  fi
done | sort -rn | head -10

# Cyclomatic complexity (requires gocyclo tool)
# go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
gocyclo -top 20 ./...
```

**Targets for v1:**

| Metric | Current | Target |
|--------|---------|--------|
| Interface count | 75+ | Reduce by 20% |
| Config struct count | 100+ | Reduce by 30% |
| Largest file (lines) | 2100+ | < 1000 |
| Max function complexity | ? | < 15 |

---

## 15. Feature Integration Validation

This section addresses a critical gap: features get implemented with unit tests, but there's insufficient validation that features actually work when integrated together. This section provides comprehensive guidance for validating that implemented features work end-to-end.

---

### 15.1 Current State of Integration Testing

**Review Prompt:**
> Assess the current integration test coverage and identify gaps between unit test coverage and actual feature functionality validation.

**Existing Integration Test Infrastructure:**

| Component | Location | Purpose | Coverage Gap |
|-----------|----------|---------|--------------|
| Integration test helpers | `internal/testing/integration/` | Cost tracking, retry logic, cleanup | Infrastructure only, no feature tests |
| LLM provider tests | `pkg/llm/providers/*_integration_test.go` | Real API calls to Anthropic/OpenAI | Individual provider tests only |
| Controller lifecycle | `internal/controller/controller_test.go` | Start/stop/health check | Basic lifecycle, no workflow execution |
| MCP integration | `internal/controller/runner/mcp_integration_test.go` | MCP server lifecycle | Uses mocks, not real MCP servers |
| Replay flow | `internal/controller/runner/replay_integration_test.go` | Workflow replay from failure | Uses shell commands only |
| Poll trigger | `internal/controller/polltrigger/service_test.go` | Poll trigger lifecycle | Uses mock poller |

**What's Missing:**
- No true end-to-end workflow execution tests
- No CLI command integration tests
- No multi-step workflow integration tests
- No real trigger (webhook/schedule/poll) tests with actual HTTP traffic
- No integration tests with real external services (GitHub, Slack)

---

### 15.2 CLI Command Validation

**Review Prompt:**
> Identify CLI commands that require end-to-end testing to validate they work as documented.

**Critical CLI Commands Requiring E2E Testing:**

| Command | Priority | Current Test Status | E2E Test Required |
|---------|----------|---------------------|-------------------|
| `conductor run <workflow>` | P0 | Unit tests only | Full workflow execution |
| `conductor daemon start/stop` | P0 | Basic lifecycle test | Full lifecycle with workflow runs |
| `conductor validate <workflow>` | P1 | Unit tests | Should validate all example workflows |
| `conductor test <tests>` | P1 | Unit tests | Should run test discovery/execution |
| `conductor run show <run-id>` | P1 | Unit tests | Requires successful run first |
| `conductor run replay <run-id>` | P1 | Integration test exists | Need real failed run scenario |
| `conductor run diff <id1> <id2>` | P2 | None | Compare two real runs |
| `conductor triggers add` | P2 | None | Add and verify trigger fires |
| `conductor triggers list` | P2 | None | List after adding triggers |
| `conductor workspace create/use` | P2 | Unit tests | Multi-workspace isolation |
| `conductor mcp list/status` | P2 | Unit tests | Real MCP server integration |
| `conductor integrations test` | P2 | Unit tests | Real connectivity tests |
| `conductor secrets` | P2 | None | Secret storage/retrieval |
| `conductor config` | P3 | None | Config read/write cycle |

**Specific E2E Test Scenarios:**

```bash
# P0: Workflow execution E2E
conductor run examples/workflows/minimal.yaml --input input="test"
# Verify: Output contains expected response, exit code 0

# P0: Controller lifecycle E2E
conductor daemon start
conductor daemon status  # Verify: Shows "running"
conductor run examples/workflows/minimal.yaml --input input="test"
conductor daemon stop
# Verify: Clean startup, execution, shutdown

# P1: Validate all examples
for f in examples/**/*.yaml; do conductor validate "$f"; done
# Verify: All examples pass validation

# P1: Workflow test execution
conductor test examples/tests/  # Assuming test directory exists
# Verify: Tests discovered and run
```

---

### 15.3 Workflow Execution Validation

**Review Prompt:**
> Identify workflow types and step types that require integration validation.

**Step Types Requiring Validation:**

| Step Type | Current Test | Validation Needed |
|-----------|--------------|-------------------|
| `type: llm` | Provider unit tests | Full step with templating |
| `type: action` (file) | `internal/action/file/observability_test.go` | Multi-file workflow |
| `type: action` (shell) | `internal/action/shell/action_test.go` | Command output in templates |
| `type: action` (http) | `internal/action/http/integration_test.go` | Real HTTP with mocked server |
| `type: action` (transform) | Unit tests | Transform in workflow context |
| `type: action` (utility) | Unit tests | Utility in workflow context |
| `type: integration` | Mocked tests | Real GitHub/Slack APIs |
| `type: parallel` | Unit tests | Parallel execution with real steps |
| `type: condition` | Unit tests | Condition evaluation in workflow |
| `type: loop` | Unit tests | Loop with real step execution |

**Workflow Pattern Validation Matrix:**

| Pattern | Example Location | Validation Status |
|---------|------------------|-------------------|
| Simple LLM step | `examples/workflows/minimal.yaml` | No E2E test |
| Multi-step LLM | `examples/write-song/workflow.yaml` | No E2E test |
| Shell + LLM | `examples/code-review/workflow.yaml` | No E2E test |
| Parallel execution | `examples/ci-cd/multi-persona-review/workflow.yaml` | No E2E test |
| Conditional steps | `examples/code-review/workflow.yaml` | No E2E test |
| File read/write | `examples/code-review/workflow.yaml` | No E2E test |
| HTTP action | `examples/recipes/hello-api/workflow.yaml` | No E2E test |
| Integration (GitHub) | `examples/ci-cd/nightly-build-summary/workflow.yaml` | No E2E test |
| Integration (Slack) | `examples/slack-integration/workflow.yaml` | No E2E test |
| Template expressions | Various | No E2E test |
| Input validation | Various | No E2E test |
| Output mapping | Various | No E2E test |

---

### 15.4 Action Validation Checklist

**Review Prompt:**
> Create a validation checklist for each action type to ensure they work in real workflows.

**File Action (`file.*`):**
- [ ] `file.read`: Read file from workflow directory
- [ ] `file.read`: Handle file not found error gracefully
- [ ] `file.write`: Write to `$out/` directory
- [ ] `file.write`: Quota enforcement (if configured)
- [ ] `file.glob`: Pattern matching in workflow directory
- [ ] `file.append`: Append to existing file
- [ ] Template variable interpolation in path
- [ ] Audit logging captures operations

**Shell Action (`shell.run`):**
- [ ] Simple command execution
- [ ] Command with arguments (array form)
- [ ] Command output captured in `$.step.stdout`
- [ ] Working directory setting
- [ ] Environment variable injection
- [ ] Timeout enforcement
- [ ] Non-zero exit code handling
- [ ] Output in subsequent template expressions

**HTTP Action (`http.*`):**
- [ ] `http.get`: Basic GET request
- [ ] `http.post`: POST with JSON body
- [ ] `http.get`: Response parsed as JSON
- [ ] Custom headers
- [ ] Authentication (Bearer token)
- [ ] Error response handling (4xx, 5xx)
- [ ] Timeout handling
- [ ] Response in template expressions

**Transform Action (`transform.*`):**
- [ ] JSON parse/stringify
- [ ] JQ expressions
- [ ] Array operations (filter, map)
- [ ] Object operations (pick, omit)
- [ ] XML parse (if implemented)

**Utility Action (`utility.*`):**
- [ ] `utility.sleep`: Delay execution
- [ ] `utility.uuid`: Generate unique IDs
- [ ] `utility.timestamp`: Current time

---

### 15.5 Integration Validation Checklist

**Review Prompt:**
> Create validation requirements for each external integration.

**GitHub Integration:**
- [ ] `github.get_repo`: Fetch repository info (requires GITHUB_TOKEN)
- [ ] `github.list_pulls`: List pull requests
- [ ] `github.get_pull`: Get specific PR details
- [ ] `github.create_issue`: Create issue (read-write test)
- [ ] `github.add_comment`: Add PR comment
- [ ] Authentication via token
- [ ] Rate limit handling
- [ ] Pagination handling

**Slack Integration:**
- [ ] `slack.post_message`: Post to channel (requires SLACK_TOKEN)
- [ ] `slack.update_message`: Update existing message
- [ ] `slack.upload_file`: Upload file to channel
- [ ] `slack.list_channels`: List available channels
- [ ] Authentication via token
- [ ] Error handling for invalid channel

**Jira Integration:**
- [ ] `jira.get_issue`: Fetch issue details
- [ ] `jira.create_issue`: Create new issue
- [ ] `jira.add_comment`: Add comment to issue
- [ ] `jira.transition_issue`: Move issue status
- [ ] Authentication via API token

**Other Integrations to Validate:**
- [ ] Discord: Message posting
- [ ] Jenkins: Job triggering
- [ ] PagerDuty: Incident creation
- [ ] Datadog: Metric submission
- [ ] Elasticsearch: Document indexing

---

### 15.6 Trigger Validation Checklist

**Review Prompt:**
> Create validation requirements for each trigger type.

**Webhook Triggers:**
- [ ] Register webhook trigger via CLI
- [ ] Send HTTP POST to webhook endpoint
- [ ] Verify workflow executes with payload data
- [ ] GitHub webhook signature validation (SHA-256)
- [ ] Slack webhook signature validation
- [ ] Generic webhook (no signature)
- [ ] Trigger appears in `conductor triggers list`
- [ ] Remove trigger via CLI

**Schedule Triggers:**
- [ ] Register schedule trigger via CLI
- [ ] Verify cron expression parsing
- [ ] Wait for scheduled execution (requires time)
- [ ] Schedule appears in `conductor triggers list`
- [ ] Remove schedule trigger

**Poll Triggers:**
- [ ] Register poll trigger via CLI
- [ ] Mock poll source returns new data
- [ ] Verify workflow fires with poll data
- [ ] Poll state persisted (no duplicate fires)
- [ ] Poll trigger appears in list

**File Watcher Triggers:**
- [ ] Register file watcher trigger
- [ ] Create/modify watched file
- [ ] Verify workflow fires with file info
- [ ] Debounce works (rapid changes)

**API Endpoint Triggers:**
- [ ] Register API endpoint
- [ ] Call endpoint with parameters
- [ ] Verify workflow executes with inputs
- [ ] Endpoint appears in trigger list

---

### 15.7 Controller Lifecycle Validation

**Review Prompt:**
> Create validation checklist for controller lifecycle scenarios.

**Startup Scenarios:**
- [ ] Fresh start (no socket exists)
- [ ] Start when already running (idempotent)
- [ ] Start with invalid config (error handling)
- [ ] Start with foreground flag
- [ ] Start with custom socket path
- [ ] Start with TCP listener

**Running Scenarios:**
- [ ] Health endpoint returns healthy
- [ ] Version endpoint returns version info
- [ ] Metrics endpoint returns Prometheus metrics
- [ ] Execute workflow via CLI
- [ ] Execute workflow via API
- [ ] Execute multiple concurrent workflows
- [ ] Handle workflow failure gracefully

**Shutdown Scenarios:**
- [ ] Graceful shutdown via CLI stop
- [ ] Graceful shutdown via SIGTERM
- [ ] Shutdown with in-flight workflows (drain)
- [ ] Forced shutdown via SIGKILL behavior
- [ ] Socket file cleaned up

**Recovery Scenarios:**
- [ ] Crash recovery (unclean shutdown)
- [ ] Resume workflow from checkpoint
- [ ] Replay failed workflow
- [ ] Handle corrupted state

---

### 15.8 Smoke Test Suite

**Review Prompt:**
> Define minimum smoke tests that should pass before any release.

**Pre-Release Smoke Test Checklist:**

```bash
# === TIER 1: No external dependencies ===

# Build and install
make build
./bin/conductor version

# Validate all bundled examples
for f in examples/**/*.yaml; do
  echo "Validating: $f"
  ./bin/conductor validate "$f" || exit 1
done

# Run minimal workflow (dry-run mode if no API key)
./bin/conductor run examples/workflows/minimal.yaml \
  --input input="smoke test" \
  --dry-run

# Controller lifecycle
./bin/conductor daemon start
sleep 2
./bin/conductor daemon status
./bin/conductor daemon stop

# === TIER 2: Requires LLM API key ===

# Run actual LLM workflow
export ANTHROPIC_API_KEY="..."
./bin/conductor daemon start --foreground &
DAEMON_PID=$!

./bin/conductor run examples/workflows/minimal.yaml \
  --input input="Hello, smoke test"

./bin/conductor run show $(conductor runs list --limit 1 --format json | jq -r '.[0].id')

kill $DAEMON_PID

# === TIER 3: Requires external service tokens ===

# GitHub integration
export GITHUB_TOKEN="..."
./bin/conductor run examples/ci-cd/pr-size-gate/workflow.yaml \
  --input pr_url="https://github.com/owner/repo/pull/1"

# Slack integration (if configured)
export SLACK_TOKEN="..."
./bin/conductor integrations test slack
```

**Smoke Test Automation:**
```yaml
# .github/workflows/smoke.yml
name: Smoke Tests
on: [push, pull_request]
jobs:
  smoke:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build
        run: make build

      - name: Validate examples
        run: |
          for f in examples/**/*.yaml; do
            ./bin/conductor validate "$f"
          done

      - name: Controller lifecycle
        run: |
          ./bin/conductor daemon start
          ./bin/conductor daemon status
          ./bin/conductor daemon stop
```

---

### 15.9 Manual Validation Checklist

**Review Prompt:**
> Create a manual validation checklist for features that are difficult to automate.

**Pre-Release Manual Testing:**

| Category | Test | Verified | Notes |
|----------|------|----------|-------|
| **Installation** | | | |
| | `go install` works | [ ] | |
| | Homebrew formula installs | [ ] | |
| | Binary runs on macOS ARM64 | [ ] | |
| | Binary runs on macOS AMD64 | [ ] | |
| | Binary runs on Linux AMD64 | [ ] | |
| **First Run Experience** | | | |
| | `conductor --help` is clear | [ ] | |
| | `conductor init` creates valid workflow | [ ] | |
| | `conductor run` with missing API key shows helpful error | [ ] | |
| | `conductor daemon start` works without config | [ ] | |
| **Workflow Authoring** | | | |
| | YAML syntax errors show line numbers | [ ] | |
| | Template expression errors are actionable | [ ] | |
| | Invalid step type shows valid options | [ ] | |
| | Missing required input shows which input | [ ] | |
| **Workflow Execution** | | | |
| | Progress output is readable | [ ] | |
| | Step timing is accurate | [ ] | |
| | Output formatting is correct | [ ] | |
| | `--json` output is valid JSON | [ ] | |
| | Large output doesn't truncate | [ ] | |
| **Error Scenarios** | | | |
| | Network timeout shows helpful message | [ ] | |
| | API key invalid shows helpful message | [ ] | |
| | Rate limit shows retry guidance | [ ] | |
| | Workflow timeout shows which step | [ ] | |
| **Controller Operations** | | | |
| | `conductor daemon start` is fast (<2s) | [ ] | |
| | `conductor daemon stop` is clean | [ ] | |
| | Controller survives terminal close | [ ] | |
| | Controller auto-starts (if configured) | [ ] | |
| **Observability** | | | |
| | Logs are structured JSON | [ ] | |
| | Metrics endpoint works | [ ] | |
| | Tracing captures all steps | [ ] | |

---

### 15.10 Example Workflow Validation Matrix

**Review Prompt:**
> Track validation status of all bundled example workflows.

**Example Validation Status:**

| Example | Path | Validated | Issues | Test Method |
|---------|------|-----------|--------|-------------|
| Minimal | `examples/workflows/minimal.yaml` | [ ] | | LLM required |
| Code Review | `examples/code-review/workflow.yaml` | [ ] | | LLM + git diff |
| Write Song | `examples/write-song/workflow.yaml` | [ ] | | LLM only |
| Issue Triage | `examples/issue-triage/workflow.yaml` | [ ] | | LLM only |
| IaC Review | `examples/iac-review/workflow.yaml` | [ ] | | LLM only |
| Security Audit | `examples/security-audit/workflow.yaml` | [ ] | | LLM + file |
| Custom Tools | `examples/custom-tools-workflow.yaml` | [ ] | | MCP server |
| Tool Workflow | `examples/workflows/tool-workflow.yaml` | [ ] | | LLM + tools |
| Observability Demo | `examples/workflows/observability-demo.yaml` | [ ] | | LLM |
| Hello API | `examples/recipes/hello-api/workflow.yaml` | [ ] | | HTTP mock |
| Webhook Handler | `examples/recipes/webhook-handler/workflow.yaml` | [ ] | | Webhook |
| Nightly Build | `examples/ci-cd/nightly-build-summary/workflow.yaml` | [ ] | | GitHub + Slack |
| PR Size Gate | `examples/ci-cd/pr-size-gate/workflow.yaml` | [ ] | | GitHub |
| Multi-Persona Review | `examples/ci-cd/multi-persona-review/workflow.yaml` | [ ] | | LLM parallel |
| Release Notes | `examples/ci-cd/release-notes/workflow.yaml` | [ ] | | GitHub |
| Build Failure Triage | `examples/ci-cd/build-failure-triage/workflow.yaml` | [ ] | | GitHub |
| Security Scan | `examples/ci-cd/security-scan-interpreter/workflow.yaml` | [ ] | | LLM |
| Dependency Reviewer | `examples/ci-cd/dependency-update-reviewer/workflow.yaml` | [ ] | | GitHub |
| Slack Integration | `examples/slack-integration/workflow.yaml` | [ ] | Contains placeholder | Slack |
| Remote Workflow | `examples/remote-workflows/example-workflow.yaml` | [ ] | | Remote fetch |

---

### 15.11 Integration Test Infrastructure Gaps

**Review Prompt:**
> Identify gaps in the integration test infrastructure.

**Current Infrastructure:**
- Cost tracking for LLM API calls
- Environment-based test skipping
- Retry with exponential backoff
- Cleanup management for resources
- Build tag separation (`//go:build integration`)

**Missing Infrastructure:**

| Gap | Priority | Effort | Solution |
|-----|----------|--------|----------|
| No CLI test harness | P0 | Medium | Create CLI test runner that spawns conductor binary |
| No workflow execution test helper | P0 | Medium | Helper to run workflow and verify outputs |
| No mock HTTP server for integrations | P1 | Low | Use `httptest` package, create fixtures |
| No GitHub API test fixtures | P1 | Medium | Record/playback with go-vcr |
| No Slack API test fixtures | P1 | Medium | Record/playback with go-vcr |
| No controller test harness | P1 | Medium | Helper to start/stop controller for tests |
| No trigger test infrastructure | P2 | High | Webhook simulator, schedule accelerator |
| No test workflow generator | P2 | Low | Helper to create minimal test workflows |

**Proposed Test Helpers:**

```go
// CLI test helper
func RunCLI(t *testing.T, args ...string) (stdout, stderr string, exitCode int)

// Workflow execution helper
func RunWorkflow(t *testing.T, workflowPath string, inputs map[string]any) *WorkflowResult

// Controller lifecycle helper
func WithController(t *testing.T, fn func(client *Client))

// Integration mock helper
func WithMockGitHub(t *testing.T, fixtures string, fn func(serverURL string))
```

---

### 15.12 CI Integration Test Tiers

**Review Prompt:**
> Define tiered integration test strategy for CI.

**Current CI Configuration:**
- Tier 1 (always): `make test` - Unit tests
- Tier 2 (secrets required): `make test-integration` - LLM integration tests

**Proposed Tier Structure:**

| Tier | Trigger | Cost | Duration | Tests |
|------|---------|------|----------|-------|
| Tier 0 | Every commit | Free | <1min | Example validation, dry-run |
| Tier 1 | Every commit | Free | <3min | Unit tests, CLI smoke tests |
| Tier 2 | PR + main | ~$0.10 | <5min | Single LLM call tests |
| Tier 3 | Main only | ~$1.00 | <15min | Full workflow tests |
| Tier 4 | Weekly/nightly | ~$5.00 | <30min | All integrations |

**CI Workflow Updates Needed:**

```yaml
# Tier 0: Free, always runs
tier0:
  runs-on: ubuntu-latest
  steps:
    - name: Validate examples
      run: |
        for f in examples/**/*.yaml; do
          conductor validate "$f"
        done

    - name: Dry-run test
      run: |
        conductor run examples/workflows/minimal.yaml \
          --input input="test" --dry-run

# Tier 2: Requires secrets, single API call
tier2:
  if: github.event_name != 'fork'
  needs: tier1
  steps:
    - name: Single LLM test
      run: make test-integration
      env:
        ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        INTEGRATION_TEST_BUDGET: "0.50"

# Tier 3: Main only, full workflows
tier3:
  if: github.ref == 'refs/heads/main'
  needs: tier2
  steps:
    - name: Full workflow tests
      run: |
        conductor run examples/workflows/minimal.yaml \
          --input input="integration test"
```

---

### 15.13 Feature-by-Feature Validation Checklist

**Review Prompt:**
> Create comprehensive checklist for validating each major feature works end-to-end.

**Workflow Engine:**
- [ ] Simple single-step workflow executes
- [ ] Multi-step workflow executes in order
- [ ] Step output available in subsequent step templates
- [ ] Parallel steps execute concurrently
- [ ] Conditional steps skip correctly
- [ ] Loop steps iterate correctly
- [ ] Workflow inputs validate correctly
- [ ] Workflow outputs map correctly
- [ ] Workflow timeout enforced
- [ ] Step timeout enforced

**Template Engine:**
- [ ] Input variable substitution: `{{.inputs.foo}}`
- [ ] Step output access: `{{.steps.stepId.content}}`
- [ ] Nested field access: `{{.steps.stepId.response.field}}`
- [ ] Conditional rendering: `{{if .foo}}...{{end}}`
- [ ] Range iteration: `{{range .items}}...{{end}}`
- [ ] Built-in functions: `now`, `date`, `json`
- [ ] JQ expressions in templates
- [ ] Error messages show template location

**LLM Integration:**
- [ ] Model tier resolution (fast/balanced/strategic)
- [ ] System prompt injection
- [ ] User prompt templating
- [ ] Response streaming (if supported)
- [ ] Token usage tracking
- [ ] Cost calculation
- [ ] Error recovery on API failure
- [ ] Rate limit handling

**MCP Integration:**
- [ ] MCP servers start with workflow
- [ ] MCP tools available to LLM steps
- [ ] MCP tool calls executed
- [ ] MCP servers stop after workflow
- [ ] MCP server error handling

**Controller:**
- [ ] Starts on Unix socket
- [ ] Starts on TCP (if configured)
- [ ] Health check endpoint works
- [ ] Version endpoint works
- [ ] Metrics endpoint works
- [ ] Run history stored
- [ ] Run history queryable
- [ ] Graceful shutdown with drain
- [ ] Auto-start from CLI (if configured)

**Secrets:**
- [ ] Environment variable resolution: `env:VAR_NAME`
- [ ] File secret resolution: `file:/path/to/secret`
- [ ] Keychain resolution (macOS)
- [ ] Secret not exposed in logs
- [ ] Secret not exposed in traces

**Observability:**
- [ ] Structured logging works
- [ ] Log level configurable
- [ ] Prometheus metrics exposed
- [ ] OpenTelemetry traces generated
- [ ] Audit logging captures operations
- [ ] Sensitive data redacted

---

### 15.14 Validation Command Reference

**Quick Validation Commands:**

```bash
# Validate all examples compile
find examples -name "*.yaml" -exec conductor validate {} \;

# Run minimal smoke test
conductor run examples/workflows/minimal.yaml --input input="test" --dry-run

# Check controller lifecycle
conductor daemon start && \
  sleep 2 && \
  conductor daemon status && \
  conductor daemon stop

# Validate all integration configs
conductor integrations list

# Test specific integration connectivity
conductor integrations test github
conductor integrations test slack

# Run integration test suite
make test-integration

# Run with verbose output
CONDUCTOR_DEBUG=1 conductor run examples/workflows/minimal.yaml --input input="test"

# Check workflow execution trace
conductor run examples/workflows/minimal.yaml --input input="test" && \
  conductor traces list --limit 1
```

---

### 15.15 Gap Summary and Prioritization

**Critical Gaps (Must Fix for Release):**

| Gap | Impact | Effort | Status |
|-----|--------|--------|--------|
| No CLI E2E test harness | Users may hit CLI bugs | Medium | Not started |
| No example validation in CI | Broken examples ship | Low | Not started |
| No controller lifecycle E2E | Controller bugs in production | Medium | Partial |
| No workflow execution E2E | Core feature untested | High | Not started |

**High Priority Gaps (Should Fix):**

| Gap | Impact | Effort | Status |
|-----|--------|--------|--------|
| No trigger E2E tests | Webhooks may not work | High | Not started |
| No integration mock fixtures | Can't test GitHub/Slack | Medium | Not started |
| No smoke test suite | No release gate | Low | Not started |

**Medium Priority Gaps (Nice to Have):**

| Gap | Impact | Effort | Status |
|-----|--------|--------|--------|
| No performance baseline | Can't detect regressions | Medium | Not started |
| No cross-platform validation | May not work on Windows | Medium | Not started |
| No multi-workflow stress test | Concurrency issues | High | Not started |

---

### Validation Priority Matrix

| Priority | Category | Effort | Files Affected |
|----------|----------|--------|----------------|
| P0 | CLI smoke tests | Low | New test files |
| P0 | Example validation in CI | Low | CI workflow |
| P1 | Workflow E2E test harness | Medium | New package |
| P1 | Controller E2E tests | Medium | Existing tests |
| P2 | Integration mock fixtures | Medium | New fixtures |
| P2 | Trigger E2E tests | High | New tests |
| P3 | Full smoke test suite | Low | Scripts + CI |
| P3 | Manual validation checklist | Low | Documentation |

---

## 16. LLM-Assisted Development Improvements

This section focuses on making the codebase more "AI-friendly" - easier for LLMs/AI coding agents to understand, navigate, and modify correctly. Since AI-assisted development is increasingly common, investing in LLM-ergonomics pays dividends in productivity.

### 16.1 Eliminate Ambiguity

**Review Prompt:**
> Identify code patterns that would confuse an AI agent trying to understand "the right way" to do something. Multiple ways to accomplish the same thing, inconsistent patterns across packages, and naming variations create cognitive overhead for both humans and AI.

**Ambiguity Patterns to Remove:**

| Pattern | Problem | Solution |
|---------|---------|----------|
| Multiple syntax for same thing | `${VAR}`, `$VAR`, `env:VAR` all work | Pick one, delete others |
| Parallel implementations | `connector` and `action` packages doing similar things | Consolidate into one |
| Inconsistent naming | `controller` in some places, `daemon` in others | Use canonical term only |
| Multiple entry points | Several ways to initialize the same component | Single factory function |
| Legacy + modern patterns | Old pattern preserved "for compatibility" | Delete old pattern entirely |

**Specific Checks:**
- [ ] Only one way to reference environment variables in workflows
- [ ] Only one way to reference secrets in workflows
- [ ] Only one package for each concept (no `connector` AND `action` packages)
- [ ] Consistent constructor patterns (`New*` vs factory functions)
- [ ] No deprecated code paths that "still work"

### 16.2 Single Source of Truth

**Review Prompt:**
> Identify information that exists in multiple places where one could become stale. AI agents work best when each piece of information has exactly one authoritative location.

**Duplicate Information to Consolidate:**

| Information | Locations | Authoritative Source |
|-------------|-----------|---------------------|
| CLI command definitions | Multiple cobra files | Should be one per command |
| Error codes | Scattered across packages | Single `errors` or `errorcodes` package |
| Configuration defaults | In code + in docs | Code is authoritative, docs generated |
| Workflow schema | JSON schema + Go types | Go types generate JSON schema |
| Terminology mapping | CLAUDE.md + code comments | CLAUDE.md is authoritative |

**Specific Checks:**
- [ ] README/docs can be generated from code where possible
- [ ] Configuration defaults documented via code extraction
- [ ] API examples in docs are tested against actual API
- [ ] Version numbers come from single source (e.g., `go generate`)

### 16.3 Navigable Code Structure

**Review Prompt:**
> Assess whether an AI agent could easily navigate from a user question to the relevant code. Clear package boundaries, predictable file naming, and well-organized imports reduce token waste.

**Navigation Improvements:**

| Pattern | Current | Improved |
|---------|---------|----------|
| Package per feature | `internal/daemon/*` contains everything | Subpackages by concern |
| Predictable file names | Mixed naming patterns | `{feature}.go`, `{feature}_test.go` |
| Index documentation | None | `doc.go` in each package explaining purpose |
| Import organization | Mixed | stdlib / external / internal grouping |

**Specific Checks:**
- [ ] Each package has `doc.go` with purpose statement
- [ ] Package names match directory names
- [ ] No circular import workarounds (indicates bad boundaries)
- [ ] `internal/` structure mirrors feature boundaries
- [ ] README.md files in complex directories explain contents

### 16.4 Self-Documenting Code

**Review Prompt:**
> Identify code that requires external context to understand. AI agents work best when code intent is clear from the code itself, not from tribal knowledge or external docs.

**Self-Documentation Patterns:**

| Pattern | Bad | Good |
|---------|-----|------|
| Magic numbers | `timeout: 30` | `timeout: defaultStepTimeout` (const) |
| Cryptic names | `proc`, `mgr`, `svc` | `processor`, `manager`, `service` |
| Implicit state | Multiple bools tracking state | State machine with named states |
| Silent defaults | Zero value means "use default" | Explicit `WithDefault()` option |
| Opaque errors | `return err` | `return fmt.Errorf("doing X: %w", err)` |

**Specific Checks:**
- [ ] No unexported magic numbers (all constants named)
- [ ] No single-letter variable names outside tiny scopes
- [ ] Error messages include context about what was attempted
- [ ] Function names describe what they do, not how
- [ ] Type names are nouns, method names are verbs

### 16.5 Minimal CLAUDE.md

**Review Prompt:**
> Review the CLAUDE.md file for effectiveness. It should contain project-specific guidance that isn't obvious from the code, not general Go practices or information easily discoverable from the codebase.

**CLAUDE.md Best Practices:**

**Include:**
- Canonical terminology choices (controller not daemon)
- Non-obvious architectural decisions and why
- Testing patterns specific to this project
- Commands to build/test/run that work
- Known limitations and footguns

**Exclude:**
- Standard Go conventions (gofmt, testing, etc.)
- Information that exists in code (types, interfaces)
- Stale build commands
- Historical context about removed features

**Specific Checks:**
- [ ] Every instruction in CLAUDE.md is actionable
- [ ] CLAUDE.md is under 100 lines (brevity = signal)
- [ ] Build/test commands actually work when run
- [ ] No instructions that contradict code patterns
- [ ] Terminology mapping is complete and current

### 16.6 Reduce Cognitive Load

**Review Prompt:**
> Identify code patterns that require holding too much context in mind. AI agents have context limits; code that requires understanding many files simultaneously is harder to work with.

**Cognitive Load Reducers:**

| High Load | Low Load |
|-----------|----------|
| Feature spread across 10 packages | Feature in 1-2 packages |
| Deep inheritance/embedding | Flat composition |
| Global state | Passed dependencies |
| Implicit initialization order | Explicit construction |
| Cross-package type assertions | Interface at point of use |

**Specific Checks:**
- [ ] No god objects (types with 20+ methods)
- [ ] Functions under 100 lines (prefer 30-50)
- [ ] Max 3-4 levels of package nesting
- [ ] Interfaces defined where they're used, not where implemented
- [ ] Dependencies injected, not discovered via globals

### 16.7 Test as Specification

**Review Prompt:**
> Review test files for their documentation value. Well-written tests serve as executable specifications that AI agents can reference to understand expected behavior.

**Tests as Documentation:**

| Test Style | Documentation Value |
|------------|---------------------|
| `TestFoo` with assertions | Low - just checks something |
| `TestFoo_WhenX_ShouldY` | High - describes behavior |
| Table-driven with case names | High - enumerates scenarios |
| Comments before each case | Medium - explains intent |
| Example functions (`Example*`) | Very high - shows usage |

**Specific Checks:**
- [ ] Test function names describe the scenario being tested
- [ ] Table-driven tests have descriptive `name` fields
- [ ] Example functions exist for key public APIs
- [ ] Tests can be read as behavior specification
- [ ] No tests that only check "doesn't crash"

### 16.8 Consistent Patterns

**Review Prompt:**
> Identify where different packages/files use different patterns for the same thing. Consistency helps AI agents learn "how we do X" once and apply it everywhere.

**Pattern Consistency Checks:**

| Concern | Pattern Should Be |
|---------|-------------------|
| Constructors | `New{Type}(ctx, deps) (*Type, error)` |
| Options | `With{Option}(value) Option` functional options |
| Errors | Wrapped with `fmt.Errorf("context: %w", err)` |
| Logging | `slog.{Level}(msg, "key", value)` structured |
| Context | First parameter to functions that do I/O |
| Cleanup | `defer` or `t.Cleanup()` in tests |

**Specific Checks:**
- [ ] All packages use same constructor pattern
- [ ] All packages use same error wrapping pattern
- [ ] All packages use same logging pattern
- [ ] Interface patterns consistent (accept interfaces, return structs)
- [ ] Option patterns consistent (functional options OR config struct, not both)

### 16.9 LLM Verification Commands

**Commands to verify LLM-friendliness:**

```bash
# Check for ambiguous patterns
grep -rn "legacy\|deprecated\|backward" --include="*.go" | grep -v "_test.go"

# Check for magic numbers
grep -rn "[0-9]\{2,\}" --include="*.go" | grep -v "const\|test\|_test"

# Check for inconsistent naming
grep -rn "daemon\|controller" --include="*.go" | sort | uniq -c | sort -rn

# Check package doc coverage
find . -name "*.go" -path "./internal/*" -exec dirname {} \; | sort -u | \
  while read dir; do
    if [ ! -f "$dir/doc.go" ]; then echo "Missing doc.go: $dir"; fi
  done

# Check for god objects (types with many methods)
grep -rn "^func.*\*.*)" --include="*.go" | \
  sed 's/.*func.*(\([^)]*\)).*/\1/' | \
  sort | uniq -c | sort -rn | head -20
```

### 16.10 LLM-Friendly Documentation

**Review Prompt:**
> Assess documentation for AI agent consumption. AI agents need quick access to authoritative information, not prose-heavy explanations.

**Documentation Improvements:**

| Current | Improved |
|---------|----------|
| Long prose paragraphs | Structured tables and lists |
| Buried configuration examples | Configuration at top with examples |
| Narrative API descriptions | Endpoint tables with parameters |
| Inline code comments | Package-level overview + focused comments |

**Specific Checks:**
- [ ] Each package README fits in ~500 tokens
- [ ] API reference uses tables, not paragraphs
- [ ] Configuration has complete example, not fragments
- [ ] Common operations listed as bullet points
- [ ] "See also" links between related docs

---

## Output Artifacts

Each review session should produce:

1. **Issues List** - Specific problems found with locations
2. **Priority Assessment** - Categorization of each issue
3. **Remediation Tasks** - Actionable fix descriptions
4. **Verification Criteria** - How to confirm fix worked
