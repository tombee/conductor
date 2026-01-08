# Conditions

Execute steps conditionally based on expressions.

## The `if` Field

The recommended way to conditionally execute steps is with the `if` field:

```yaml
steps:
  - id: check
    llm:
      prompt: "Is it raining? Answer yes or no."
  - id: bring_umbrella
    if: "{{.steps.check.output}} == 'yes'"
    llm:
      prompt: "Remind me to bring an umbrella"
```

The `bring_umbrella` step only runs if the condition evaluates to `true`. If `false`, the step is skipped (not errored).

### Basic Syntax

The `if` field accepts a string expression that must evaluate to boolean:

```yaml
# Template syntax (recommended)
if: "{{.steps.check.stdout}} == 'value'"

# Bare syntax (also supported)
if: "steps.check.stdout == 'value'"

# Workflow inputs
if: "{{.inputs.mode}} == 'strict'"
if: "inputs.mode == 'strict'"

# Boolean values
if: "{{.inputs.enabled}}"
if: "inputs.enabled == true"
```

### Relationship to `condition.expression`

The `if` field is shorthand for `condition.expression`. These are equivalent:

```yaml
# Using if (recommended)
- id: step1
  if: "inputs.enabled == true"

# Using condition
- id: step2
  condition:
    expression: "inputs.enabled == true"
```

You cannot use both `if` and `condition` on the same step.

## Basic Conditions (Legacy Syntax)

The older `condition` syntax is still supported:

```yaml
steps:
  - id: check
    llm:
      prompt: "Is it raining? Answer yes or no."
  - id: bring_umbrella
    condition: ${steps.check.output == "yes"}
    llm:
      prompt: "Remind me to bring an umbrella"
```

## Expression Operators

The `if` field supports standard comparison and boolean operators:

```yaml
# Equality
if: "{{.steps.step1.output}} == 'expected'"

# Inequality
if: "{{.steps.step1.output}} != 'skip'"

# Numeric comparison
if: "{{.steps.count.output}} > 10"

# Boolean
if: "{{.inputs.enabled}} == true"
if: "inputs.enabled"

# String contains
if: "contains({{.steps.check.output}}, 'keyword')"
```

### Comparison Operators
- `==` - Equal
- `!=` - Not equal
- `<` - Less than
- `>` - Greater than
- `<=` - Less than or equal
- `>=` - Greater than or equal

### Boolean Operators

Combine conditions with logical operators:

```yaml
# AND - All conditions must be true
if: "inputs.env == 'prod' && inputs.region == 'us'"

# OR - At least one condition must be true
if: "inputs.region == 'us' || inputs.region == 'eu'"

# NOT - Negate a condition
if: "!inputs.dry_run"

# Complex expressions
if: "inputs.env == 'prod' && (inputs.region == 'us' || inputs.region == 'eu') && !inputs.dry_run"
```

### Array Membership

Check if a value exists in an array:

```yaml
# Using 'in' operator
if: "'feature' in inputs.features"

# Check multiple values
if: "'security' in inputs.personas || 'compliance' in inputs.personas"
```

## Common Patterns

### Input-Based Conditions

Branch based on workflow inputs:

```yaml
inputs:
  - name: environment
    type: string

steps:
  - id: production_deploy
    if: "inputs.environment == 'production'"
    shell:
      command: ./deploy-production.sh

  - id: staging_deploy
    if: "inputs.environment == 'staging'"
    shell:
      command: ./deploy-staging.sh
```

### Conditional Based on Previous Step Output

Execute steps only if previous steps produce specific output:

```yaml
steps:
  - id: check_tests
    shell:
      command: test -d tests && echo 'has_tests' || echo 'no_tests'

  - id: run_tests
    if: "{{.steps.check_tests.stdout}} == 'has_tests'"
    shell:
      command: npm test

  - id: skip_message
    if: "{{.steps.check_tests.stdout}} == 'no_tests'"
    llm:
      prompt: "No tests directory found, skipping test execution"
```

### Chained Conditions

Reference multiple previous steps:

```yaml
steps:
  - id: validate
    shell:
      command: ./validate.sh

  - id: build
    if: "{{.steps.validate.exit_code}} == 0"
    shell:
      command: ./build.sh

  - id: deploy
    if: "steps.validate.status == 'success' && steps.build.status == 'success'"
    shell:
      command: ./deploy.sh
```

### Available Functions

The `if` field supports these expression functions:

- `contains(string, substring)` - Check if string contains substring
- `startsWith(string, prefix)` - Check if string starts with prefix
- `endsWith(string, suffix)` - Check if string ends with suffix
- `length(array)` - Get array/string length

```yaml
# Check substring
if: "contains({{.steps.result.output}}, 'success')"

# Check array length
if: "length(inputs.features) > 0"

# Check prefix
if: "startsWith({{.steps.check.output}}, 'ERROR')"
```

## Skipped Step Behavior

When a step is skipped because its `if` condition evaluates to `false`:

- The step has status `skipped` (not `failed` or `success`)
- Output fields are set to zero values: strings → `""`, numbers → `0`, booleans → `false`
- Downstream steps can safely reference the skipped step's outputs (they return empty values)
- Skipped steps don't trigger error handlers
- The workflow continues executing subsequent steps

Example:

```yaml
steps:
  - id: optional_check
    if: "inputs.run_check == true"
    shell:
      command: ./check.sh

  - id: continue
    # This runs even if optional_check was skipped
    llm:
      prompt: "Continuing workflow..."
```

## Type Safety

The `if` field requires boolean expressions. Non-boolean results cause an error:

```yaml
# ✓ Valid - evaluates to boolean
if: "inputs.enabled == true"
if: "{{.steps.count.output}} > 5"
if: "inputs.enabled"

# ✗ Invalid - evaluates to non-boolean
if: "{{.steps.check.stdout}}"           # Error: returns string
if: "{{.steps.count.output}}"           # Error: returns number
if: "42"                                # Error: not a boolean
```

Use explicit comparisons:

```yaml
# Check if output is non-empty
if: "{{.steps.check.stdout}} != ''"

# Check if count is non-zero
if: "{{.steps.count.output}} > 0"
```

### Nil Handling

When a step is skipped, its outputs return nil. Use explicit nil checks:

```yaml
# Safe nil handling
if: "steps.previous.result != nil && steps.previous.result > 5"

# Without nil check (may cause error if previous was skipped)
if: "steps.previous.result > 5"
```

## Error Handling

Use conditions for error recovery:

```yaml
steps:
  - id: try_api
    http:
      method: GET
      url: https://api.example.com/data

  - id: fallback
    if: "steps.try_api.status != 'success'"
    file:
      action: read
      path: cached-data.json
```

## Best Practices

1. **Use template syntax** - `{{.steps.id.field}}` is more consistent with the rest of Conductor
2. **Check for success explicitly** - `if: "steps.validate.status == 'success'"` is clearer than relying on truthy values
3. **Handle nil safely** - Always check for nil when referencing potentially skipped steps
4. **Keep expressions simple** - Complex logic should be in steps, not conditions
5. **Document intent** - Use step names that explain why something is conditional

## Performance

Conditions are evaluated before step execution. Skipped steps don't consume resources or call external services.
