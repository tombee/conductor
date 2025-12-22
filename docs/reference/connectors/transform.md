# Transform

The `transform` builtin connector provides operations for parsing, extracting, and reshaping step outputs for downstream consumption.

## Overview

Transform operations enable workflows to manipulate data between steps without additional LLM calls or shell scripts. Key capabilities:

- **Parse** - Extract JSON/XML from LLM text output (handles markdown code blocks)
- **Extract** - Pull nested fields using jq expressions
- **Split** - Break arrays into items for parallel fan-out with `foreach`
- **Reshape** - Map/filter/transform data structures with jq
- **Combine** - Merge objects and concatenate arrays

## Security

Transform operations execute in a sandboxed jq runtime with:

- **Function restrictions** - Dangerous functions (env, input, debug) disabled
- **Timeout controls** - 1 second execution limit per expression
- **Input validation** - Null checks, type validation before execution
- **XXE prevention** - XML parsing blocks DOCTYPE, ENTITY, SYSTEM/PUBLIC patterns
- **Data redaction** - Error messages scrub sensitive field values
- **Size limits** - 10MB max input/output, 10,000 item arrays

## Parse Operations

### parse_json

Extract and parse JSON from text, handling markdown code blocks.

**Parameters:**
- `data` (string, required) - JSON string or text containing JSON

**Algorithm:**
1. If already object/array JSON, return as-is
2. If starts with `{` or `[`, parse directly
3. If contains ` ```json ` fence, extract from fence
4. If contains any ` ``` `, try each block
5. Locate first `{` or `[` and extract to matching bracket

**Examples:**

```yaml
# Parse JSON from LLM response
- id: parse_response
  type: connector
  connector: transform
  operation: parse_json
  inputs:
    data: '{{.steps.analyze.output}}'

# Shorthand form
- id: parse
  transform.parse_json: '{{.steps.analyze.output}}'
```

**Error handling:**
- Returns `ErrorTypeParseError` for invalid JSON syntax
- Shows position, context (redacted), and suggestions
- Pass-through for already-parsed objects/arrays

### parse_xml

Parse XML to JSON-like structure with XXE prevention.

**Parameters:**
- `data` (string, required) - XML string to parse
- `attribute_prefix` (string, optional) - Prefix for attributes (default: `@`)
- `strip_namespaces` (boolean, optional) - Remove namespace prefixes (default: `true`)

**Structure mapping:**
- Attributes: `<tag attr="val">` → `{"@attr": "val", "#text": ...}`
- Empty elements: `<tag/>` → `""`
- Multiple children: `<root><item>a</item><item>b</item></root>` → `{"root": {"item": ["a", "b"]}}`
- Namespaces: Stripped by default, or preserved as URI (not prefix)

**Security:**
- Pre-scan blocks DOCTYPE, ENTITY, SYSTEM/PUBLIC patterns
- All entity expansion disabled
- Audit logging for parsing attempts and XXE detection

**Examples:**

```yaml
# Parse XML API response
- id: parse_xml
  transform.parse_xml:
    data: '{{.steps.legacy_api.response}}'

# Custom attribute prefix
- id: parse_xml_custom
  transform.parse_xml:
    data: '{{.steps.soap_response.body}}'
    attribute_prefix: $
    strip_namespaces: false
```

## Extract Operation

### extract

Extract nested fields using jq expressions.

**Parameters:**
- `data` (any, required) - Data to extract from
- `expr` (string, required) - jq expression for extraction

**jq syntax examples:**
- `.field` - Extract field
- `.nested.field` - Nested extraction
- `.[0]` - Array index
- `.[] | .name` - Map to field
- `map(.name)` - Same as above
- `select(.active)` - Filter
- `{a: .x, b: .y}` - Reshape

**Examples:**

```yaml
# Extract nested field
- id: get_title
  transform.extract:
    data: '{{.steps.analyze.issues}}'
    expr: '.[0].title'

# Extract and reshape
- id: get_names
  transform.extract:
    data: '{{.steps.users}}'
    expr: 'map({id: .id, name: .name})'
```

**Allowed jq functions:**
```
map, select, filter, keys, values, length, type, has, in, add, unique,
flatten, reverse, sort_by, group_by, min_by, max_by, contains, inside,
startswith, endswith, ascii_downcase, ascii_upcase, split, join, ltrimstr,
rtrimstr, tonumber, tostring, arrays, objects, iterables, booleans,
numbers, strings, nulls, scalars, empty, error, first, last, nth, range,
floor, ceil, round, sqrt, min, max, add, any, all, not, and, or,
if-then-else, try-catch, @base64, @uri, @json, @text, @html
```

**Disabled functions (security):**
```
env, $ENV, input, inputs, debug, modulemeta, @base64d, @sh, getpath,
setpath, delpaths, path, leaf_paths, builtins, now, strftime, strptime,
gmtime, mktime, limit, until, while, recurse, recurse_down, walk, $__loc__
```

## Array Operations

### split

Pass-through array for `foreach` iteration in parallel steps.

**Parameters:**
- `data` (array, required) - Array to pass through

**Usage with foreach:**

```yaml
# Split array for parallel processing
- id: split_issues
  transform.split: '{{.steps.analyze.issues}}'

- id: process_each
  type: parallel
  foreach: '{{.steps.split_issues}}'
  steps:
    - id: fix_issue
      type: llm
      prompt: |
        Fix this issue:
        File: {{.item.file}}
        Line: {{.item.line}}
        Description: {{.item.description}}

        Item {{.index}} of {{.total}}
```

**Context variables in foreach:**
- `.item` - Current array element
- `.index` - Zero-based index
- `.total` - Total number of elements

**Error handling:**
- Returns `ErrorTypeTypeError` if input is not an array
- Empty array produces empty results (0 iterations)
- Results maintain original array order (by index, not completion time)

### filter

Filter array elements using jq predicate expression.

**Parameters:**
- `data` (array, required) - Array to filter
- `expr` (string, required) - jq predicate expression

**Examples:**

```yaml
# Filter by boolean field
- id: active_users
  transform.filter:
    data: '{{.steps.users}}'
    expr: '.status == "active"'

# Filter by comparison
- id: adults
  transform.filter:
    data: '{{.steps.users}}'
    expr: '.age >= 18'

# Complex predicate
- id: recent_high_priority
  transform.filter:
    data: '{{.steps.issues}}'
    expr: '.priority == "high" and .created > "2024-01-01"'
```

### map

Transform array elements using jq expression.

**Parameters:**
- `data` (array, required) - Array to transform
- `expr` (string, required) - jq transformation expression

**Examples:**

```yaml
# Extract field values
- id: names
  transform.map:
    data: '{{.steps.users}}'
    expr: '.name'

# Reshape objects
- id: simplified
  transform.map:
    data: '{{.steps.users}}'
    expr: '{id: .id, email: .email}'

# Calculate values
- id: with_age
  transform.map:
    data: '{{.steps.users}}'
    expr: '. + {age_category: (if .age >= 18 then "adult" else "minor" end)}'
```

### flatten

Flatten nested arrays.

**Parameters:**
- `data` (array, required) - Nested array to flatten

**Examples:**

```yaml
# Flatten one level
- id: flattened
  transform.flatten: '{{.steps.nested_arrays}}'

# Input: [[1, 2], [3, 4]]
# Output: [1, 2, 3, 4]
```

### sort

Sort array by value or expression.

**Parameters:**
- `data` (array, required) - Array to sort
- `expr` (string, optional) - jq expression for sort key

**Examples:**

```yaml
# Sort numbers/strings directly
- id: sorted
  transform.sort: '{{.steps.items}}'

# Sort by field
- id: sorted_by_name
  transform.sort:
    data: '{{.steps.users}}'
    expr: '.name'

# Sort by numeric field
- id: sorted_by_priority
  transform.sort:
    data: '{{.steps.issues}}'
    expr: '.priority_score'
```

**Limits:**
- Default max: 10,000 items
- Configurable via `max_items` parameter
- Returns `ErrorTypeLimitExceeded` if exceeded

### group

Group array by key expression.

**Parameters:**
- `data` (array, required) - Array to group
- `expr` (string, required) - jq expression for grouping key

**Examples:**

```yaml
# Group by category
- id: by_category
  transform.group:
    data: '{{.steps.items}}'
    expr: '.category'

# Group by status
- id: by_status
  transform.group:
    data: '{{.steps.issues}}'
    expr: '.status'

# Output structure: {"key1": [items...], "key2": [items...]}
```

## Combine Operations

### merge

Merge multiple objects.

**Parameters:**
- `data` (object, optional) - First object (or use `sources`)
- `sources` (array, optional) - Array of objects to merge
- `strategy` (string, optional) - Merge strategy: `shallow` (default) or `deep`

**Merge strategies:**
- `shallow`: Merge top-level keys only, rightmost wins on conflicts
- `deep`: Recursive merge, nested objects merged, arrays concatenated

**Examples:**

```yaml
# Merge two objects (shallow)
- id: merged
  transform.merge:
    sources:
      - '{{.steps.config}}'
      - '{{.steps.overrides}}'

# Deep merge with nesting
- id: deep_merged
  transform.merge:
    sources:
      - '{{.steps.base_config}}'
      - '{{.steps.custom_config}}'
    strategy: deep

# Merge arrays of objects
- id: combined
  transform.merge:
    data: [{a: 1}, {b: 2}, {c: 3}]
```

**Conflict resolution:**
- Shallow: Rightmost value wins
- Deep: Nested objects merge, arrays concatenate

### concat

Concatenate multiple arrays.

**Parameters:**
- `data` (array, optional) - First array (or use `sources`)
- `sources` (array, optional) - Arrays to concatenate

**Examples:**

```yaml
# Concatenate two arrays
- id: combined
  transform.concat:
    sources:
      - '{{.steps.list1}}'
      - '{{.steps.list2}}'

# Concatenate array of arrays
- id: flattened_lists
  transform.concat:
    data: [[1, 2], [3, 4], [5, 6]]
# Output: [1, 2, 3, 4, 5, 6]
```

## Error Handling

All transform operations return structured errors with:

- **Error type**: `parse_error`, `expression_error`, `type_error`, `empty_input`, `limit_exceeded`, `validation`
- **Context**: Position info, preview (redacted), suggestion
- **Sensitive data redaction**: Values of fields matching `password`, `token`, `key`, `secret`, `api_key`, `auth`, `credential` are redacted

Example error:

```
Operation: transform.parse_json
Error: Invalid JSON syntax
Type: parse_error
Cause: Unexpected token at position 42

Input preview (first 100 chars, sensitive values redacted):
  Here is the JSON you requested:
  {"items": [{"name": [REDACTED]...

Possible Solutions:
  - Check if the LLM included extra text around the JSON
  - Use output_schema on the LLM step for reliable structured output
  - Try transform.extract with a jq expression to find the JSON
```

## Limits and Safety

| Limit | Default | On Exceed |
|-------|---------|-----------|
| Expression timeout | 1 second | `ErrorTypeLimitExceeded` |
| Max input size | 10MB | `ErrorTypeLimitExceeded` |
| Max output size | 10MB | `ErrorTypeLimitExceeded` |
| Max array length | 10,000 items | `ErrorTypeLimitExceeded` |
| Max recursion depth | 100 | `ErrorTypeLimitExceeded` |

## Best Practices

1. **Use output_schema for LLMs** when possible instead of relying on parse_json
2. **Prefer shorthand syntax** for simple operations: `transform.split: '{{.steps.array}}'`
3. **Chain operations** via separate steps for clarity and debuggability
4. **Validate inputs** before expensive operations (check array length before foreach)
5. **Handle errors** with workflow error handlers for robustness
6. **Tune max_items** based on memory constraints and context size for foreach
7. **Use deep merge sparingly** - it's more expensive than shallow merge
8. **Test jq expressions** in isolation before embedding in workflows

## See Also

- [File Connector](file.md) - Read/write JSON, YAML, CSV
- [Workflow Schema](../architecture/workflow-schema.md) - Parallel steps and foreach
- [Template Syntax](../advanced/templates.md) - Accessing step outputs
- [Error Handling](../guides/error-handling.md) - Workflow error patterns
