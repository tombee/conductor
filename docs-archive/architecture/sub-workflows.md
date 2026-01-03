# Sub-workflow Architecture

## Overview

Sub-workflows enable modular workflow composition by allowing workflows to invoke other workflow YAML files as reusable components. This architecture document explains the design decisions and implementation of sub-workflow execution in Conductor.

## Design Principles

### 1. Workflow Universality

Any workflow YAML can run as either a standalone workflow OR as a sub-workflow with no special syntax required. This ensures maximum reusability and simplifies the mental model.

```yaml
# This workflow works both standalone and as a sub-workflow
name: sentiment-analyzer
inputs:
  - name: text
    type: string
    required: true
outputs:
  - name: sentiment
    type: string
    value: "{{.steps.analyze.outputs.response}}"
steps:
  - id: analyze
    type: llm
    model: balanced
    prompt: "Analyze sentiment of: {{.inputs.text}}"
```

### 2. Runtime Invocation Model

Sub-workflows execute as separate runtime invocations (not compile-time inlining). This provides:

- Clean isolation between parent and child contexts
- Better debugging with nested traces
- Independent error handling per sub-workflow
- Matches the mental model of "calling a function"

### 3. Strict Context Isolation

Sub-workflows only see their declared inputs - they cannot access parent workflow context. All data must be explicitly passed as inputs.

**Why?** This ensures sub-workflows are truly reusable across different parents with no hidden dependencies.

### 4. Black Box Execution

Sub-workflows are opaque to their callers. Parent workflows can only access a sub-workflow's declared outputs via `{{.steps.<step_id>.outputs.*}}`. Internal steps are not visible to the parent.

**Why?** This allows sub-workflow authors to refactor internals without breaking callers.

### 5. Atomic Retry Semantics

From the parent's perspective, a sub-workflow invocation is a single atomic step. The parent can configure retries on the sub-workflow step, but cannot influence the sub-workflow's internal retry behavior.

- Parent retry = retry the entire sub-workflow invocation from scratch
- Sub-workflow internal retries = encapsulated, invisible to parent
- Retries do NOT multiply (no 3×3×3 = 27 retries)

## Implementation Details

### File Resolution

Sub-workflow paths are always relative to the parent workflow file:

```yaml
# In /projects/my-app/workflows/main.yaml
steps:
  - id: analyze
    type: workflow
    workflow: ./helpers/sentiment.yaml  # Resolves to /projects/my-app/workflows/helpers/sentiment.yaml
```

The loader validates paths to prevent:
- Path traversal attacks (`../../../etc/passwd`)
- Symlink following
- Absolute path references

### Recursion Detection

The loader tracks the call stack and detects cycles:

```
main.yaml → helper.yaml → main.yaml
```

This is rejected with an error showing the cycle. Maximum nesting depth is 5 levels.

### Caching

The loader caches parsed workflow definitions keyed by absolute path + modification time. This provides performance benefits when the same sub-workflow is referenced multiple times.

Cache invalidation happens automatically when file modification time changes.

### Observability

Each sub-workflow execution generates a unique `child_trace_id` for correlation:

```json
{
  "step_id": "analyze_sentiment",
  "child_trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "duration": "1.2s",
  "status": "success"
}
```

Logs include breadcrumb trails showing the execution path:

```
[INFO] executing sub-workflow: main → analyze_sentiment → classify
```

### Error Handling

Errors from sub-workflows include breadcrumb trails for debugging:

```
main → analyze_sentiment → classify (trace: 550e8400-...): LLM API call failed: rate limit exceeded
```

The parent workflow can handle sub-workflow failures using standard error handling:

```yaml
- id: analyze
  type: workflow
  workflow: ./sentiment.yaml
  inputs:
    text: "{{.inputs.feedback}}"
  on_error:
    strategy: fallback
    fallback_step: default_sentiment
  retry:
    max_attempts: 3
    backoff_base: 2
    backoff_multiplier: 2.0
```

### Input/Output Mapping

Sub-workflows define typed inputs and outputs:

```yaml
# sentiment.yaml
name: sentiment-analyzer
inputs:
  - name: text
    type: string
    required: true
  - name: language
    type: string
    default: "en"
outputs:
  - name: sentiment
    type: string
    value: "{{.steps.analyze.outputs.category}}"
  - name: confidence
    type: number
    value: "{{.steps.analyze.outputs.score}}"
```

Parent workflows map inputs and access outputs:

```yaml
# main.yaml
steps:
  - id: analyze_feedback
    type: workflow
    workflow: ./sentiment.yaml
    inputs:
      text: "{{.inputs.user_feedback}}"
      language: "en"

  - id: respond
    type: llm
    prompt: |
      Sentiment: {{.steps.analyze_feedback.outputs.sentiment}}
      Confidence: {{.steps.analyze_feedback.outputs.confidence}}
      Draft a response to the user.
```

## Integration with Flow Control

Sub-workflow steps work with all existing flow control features:

### Parallel Execution

```yaml
- id: multi_review
  type: parallel
  steps:
    - id: security
      type: workflow
      workflow: ./reviews/security.yaml
      inputs: { code: "{{.inputs.code}}" }
    - id: performance
      type: workflow
      workflow: ./reviews/performance.yaml
      inputs: { code: "{{.inputs.code}}" }
```

### Foreach Iteration

```yaml
- id: review_files
  type: parallel
  foreach: "{{.steps.get_files.outputs.files}}"
  steps:
    - id: review
      type: workflow
      workflow: ./review-file.yaml
      inputs: { file: "{{.item}}" }
```

### Conditional Execution

```yaml
- id: deep_review
  condition:
    expression: 'steps.triage.outputs.priority == "high"'
  type: workflow
  workflow: ./reviews/deep-analysis.yaml
  inputs: { code: "{{.inputs.code}}" }
```

## Security Considerations

### Path Validation

All workflow paths are validated to prevent:
- Directory traversal: `../../sensitive/file.yaml` ❌
- Absolute paths: `/etc/workflows/evil.yaml` ❌
- Symlink following: symlinks in the path are rejected ❌

Only relative paths within the workflow directory are allowed:
- `./helpers/util.yaml` ✅
- `../shared/common.yaml` ✅ (if within workspace)

### Context Isolation

Sub-workflows cannot access:
- Parent workflow environment variables (unless passed as inputs)
- Parent workflow secrets (unless passed as inputs)
- Parent workflow step outputs (unless passed as inputs)

This "explicit everything" approach prevents accidental data leaks.

### Audit Trail

All sub-workflow invocations are logged with:
- Timestamp
- Parent workflow ID
- Sub-workflow path
- Child trace ID for correlation

This provides a complete audit trail for security and debugging.

## Performance Characteristics

### Overhead

Sub-workflow invocation adds minimal overhead:
- Path validation: <1ms
- Definition loading (cached): <5ms
- Definition loading (uncached): <50ms (warm disk cache)
- Executor creation: <1ms
- Context building: <1ms

**Total overhead per invocation: <60ms** (median, warm cache)

### Caching Strategy

The loader uses a two-level caching strategy:
1. In-memory cache keyed by (absolute path, modification time)
2. OS filesystem cache for file reads

This means repeated invocations of the same sub-workflow are very fast (<5ms).

### Recursion Depth Limit

Maximum nesting depth is 5 levels. This prevents stack overflow and performance degradation from deeply nested workflows.

Example of maximum depth:
```
main.yaml
  → level1.yaml
    → level2.yaml
      → level3.yaml
        → level4.yaml
          → level5.yaml  ← Maximum depth reached
```

## Validation

The `conductor validate` command recursively validates sub-workflow references:

```bash
$ conductor validate main.yaml

Validation Results:
  [OK] Syntax valid
  [OK] Schema valid
  [OK] All step references resolve correctly
  [OK] All sub-workflow references valid (3 sub-workflow(s))
```

Validation checks:
- Sub-workflow files exist
- Sub-workflow YAML is valid
- Sub-workflow inputs match parent's provided inputs
- No circular dependencies
- Depth limits not exceeded

## Future Enhancements (v2)

The current implementation (v1) supports local file composition only. Future versions will add:

- **Remote workflows**: Reference workflows from GitHub repos or registries
- **Version pinning**: Pin sub-workflows to specific versions
- **Workflow bundling**: `conductor bundle` to create self-contained artifacts
- **Dependency lockfiles**: Lock transitive dependencies for reproducibility
- **Registry publishing**: Share sub-workflows via central registry

## See Also

- [Sub-workflow Guide](../guides/sub-workflows.md) - User-facing tutorial
- [Examples](../examples/sub-workflows.md) - Working examples
- [API Reference](../reference/workflow-schema.md#sub-workflows) - Schema documentation
