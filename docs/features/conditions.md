# Conditions

Execute steps conditionally based on expressions.

## Basic Conditions

Skip steps based on expressions:

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

The `bring_umbrella` step only runs if the condition is true.

## Condition Syntax

Conditions use simple expressions:

```yaml
# Equality
condition: ${steps.step1.output == "expected"}

# Inequality
condition: ${steps.step1.output != "skip"}

# Numeric comparison
condition: ${steps.count.output > 10}

# Boolean
condition: ${inputs.enabled == true}

# Contains
condition: ${steps.check.output | contains("keyword")}
```

## Multiple Conditions

Combine conditions with logical operators:

```yaml
# AND
condition: ${steps.check1.output == "yes" && steps.check2.output == "yes"}

# OR
condition: ${steps.check1.output == "yes" || steps.check2.output == "yes"}

# NOT
condition: ${!(steps.check.output == "skip")}
```

## Input-Based Conditions

Conditionally execute based on inputs:

```yaml
steps:
  - id: production_deploy
    condition: ${inputs.environment == "production"}
    shell:
      command: ./deploy-production.sh
  - id: staging_deploy
    condition: ${inputs.environment == "staging"}
    shell:
      command: ./deploy-staging.sh
```

## Default Values

Handle missing data with defaults:

```yaml
steps:
  - id: optional_step
    condition: ${inputs.runOptional | default(false)}
    llm:
      prompt: "This runs if runOptional is true"
```

## Condition Functions

Available functions:

- `contains(substring)` - Check if string contains substring
- `startsWith(prefix)` - Check if string starts with prefix
- `endsWith(suffix)` - Check if string ends with suffix
- `default(value)` - Use default if undefined
- `length()` - Get array/string length
- `isEmpty()` - Check if empty

```yaml
# Length check
condition: ${steps.items.output | length() > 0}

# Empty check
condition: ${!(steps.result.output | isEmpty())}
```

## Skipped Steps

When a step is skipped by a condition:
- Its output is undefined
- Steps depending on it are also skipped
- The workflow continues with other steps

## Error Handling

Use conditions for error recovery:

```yaml
steps:
  - id: try_api
    http:
      method: GET
      url: https://api.example.com/data
  - id: fallback
    condition: ${steps.try_api.error != null}
    file:
      action: read
      path: cached-data.json
```

## Performance

Conditions are evaluated before step execution. Skipped steps don't consume resources.
