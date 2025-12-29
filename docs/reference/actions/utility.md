# Utility

The `utility` action provides operations for generating IDs, random values, and math operations within workflows.

## Overview

Utility operations enable workflows to generate unique identifiers, make random selections, and perform mathematical calculations without additional dependencies. Key capabilities:

- **ID Generation** - UUIDs, NanoIDs, and custom-alphabet IDs
- **Random Operations** - Integer ranges, array selections, weighted choices, sampling, shuffling
- **Math Operations** - Clamping, rounding, min/max

## Security

Utility operations use secure randomness:

- **Cryptographic source** - All random operations use `crypto/rand` for unpredictable outputs
- **Deterministic seeding** - Optional seed parameter for reproducible testing
- **Input validation** - Parameter ranges and types validated before execution
- **Alphabet validation** - Custom ID alphabets validated for printable ASCII

!!! warning "Not for cryptographic keys"
    While `utility` operations use cryptographically secure randomness, they are designed for generating identifiers and workflow randomization—not for generating encryption keys, secrets, or tokens. Use a dedicated cryptographic library for those purposes.

## ID Generation Operations

### id_uuid

Generate a UUID v4 (random) identifier.

**Parameters:** None

**Returns:** String (36 characters, format: `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`)

**Examples:**

```conductor
# Generate a unique run ID
- id: generate_id
  utility.id_uuid:

# Access: {{.steps.generate_id.result}}
# Output: "550e8400-e29b-41d4-a716-446655440000"
```

**Longhand form:**

```conductor
- id: generate_id
  type: action
  action: utility
  operation: id_uuid
```

---

### id_nanoid

Generate a NanoID—a URL-friendly, compact identifier.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `length` | integer | No | 21 | Length of generated ID |

**Returns:** String (default 21 characters, URL-safe alphabet)

**Examples:**

```conductor
# Default 21-character NanoID
- id: short_id
  utility.id_nanoid:

# Access: {{.steps.short_id.result}}
# Output: "V1StGXR8_Z5jdHi6B-myT"

# Custom length
- id: shorter_id
  utility.id_nanoid:
    length: 12

# Output: "V1StGXR8_Z5j"
```

**When to use NanoID vs UUID:**

- **NanoID** — Shorter, URL-safe, good for user-facing IDs
- **UUID** — Standard format, better for system integration

---

### id_custom

Generate an ID with a custom alphabet and length.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `alphabet` | string | Yes | — | Characters to use (printable ASCII only) |
| `length` | integer | Yes | — | Length of generated ID |

**Returns:** String of specified length using characters from alphabet

**Examples:**

```conductor
# Numeric-only ID
- id: numeric_id
  utility.id_custom:
    alphabet: "0123456789"
    length: 8

# Output: "47382916"

# Lowercase alphanumeric
- id: lowercase_id
  utility.id_custom:
    alphabet: "abcdefghijklmnopqrstuvwxyz0123456789"
    length: 10

# Output: "a7bc3f9k2m"

# Hex ID
- id: hex_id
  utility.id_custom:
    alphabet: "0123456789abcdef"
    length: 16

# Output: "3f7a9b2c8e1d4f0a"
```

**Errors:**

- `ValidationError` — Empty alphabet, non-printable characters, or length < 1

---

## Random Operations

### random_int

Generate a random integer in a range.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `min` | integer | Yes | — | Minimum value (inclusive) |
| `max` | integer | Yes | — | Maximum value (inclusive) |

**Returns:** Integer in range [min, max]

**Examples:**

```conductor
# Roll a die
- id: roll
  utility.random_int:
    min: 1
    max: 6

# Access: {{.steps.roll.result}}
# Output: 4

# Generate percentage
- id: percent
  utility.random_int:
    min: 0
    max: 100

# Output: 73
```

**Errors:**

- `ValidationError` — min > max

---

### random_choose

Select one item randomly from an array.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `items` | array | Yes | — | Array of items to choose from |

**Returns:** Single item from the array (same type as array elements)

**Examples:**

```conductor
# Choose a random reviewer
- id: pick_reviewer
  utility.random_choose:
    items: ["alice", "bob", "charlie", "diana"]

# Output: "charlie"

# Choose from dynamic list
- id: pick_issue
  utility.random_choose:
    items: '{{.steps.get_issues.result}}'
```

**Errors:**

- `ValidationError` — Empty array

---

### random_weighted

Select an item with weighted probabilities.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `choices` | array | Yes | — | Array of `{item, weight}` objects |

**Returns:** Single item based on weighted probability

**Examples:**

```conductor
# Weighted priority assignment
- id: assign_priority
  utility.random_weighted:
    choices:
      - item: "low"
        weight: 50
      - item: "medium"
        weight: 35
      - item: "high"
        weight: 15

# 50% chance: "low", 35% chance: "medium", 15% chance: "high"

# A/B test assignment
- id: ab_group
  utility.random_weighted:
    choices:
      - item: "control"
        weight: 80
      - item: "treatment"
        weight: 20
```

**Errors:**

- `ValidationError` — Empty choices, negative weights, or all weights zero

---

### random_sample

Select N items randomly from an array (without replacement).

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `items` | array | Yes | — | Array of items to sample from |
| `count` | integer | Yes | — | Number of items to select |

**Returns:** Array of selected items (order is random)

**Examples:**

```conductor
# Select 3 random reviewers
- id: reviewers
  utility.random_sample:
    items: ["alice", "bob", "charlie", "diana", "eve"]
    count: 3

# Output: ["diana", "alice", "eve"]

# Select subset of issues
- id: sample_issues
  utility.random_sample:
    items: '{{.steps.all_issues.result}}'
    count: 5
```

**Errors:**

- `ValidationError` — count > length of items, count < 1, or empty items

---

### random_shuffle

Randomly reorder an array.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `items` | array | Yes | — | Array to shuffle |

**Returns:** New array with elements in random order

**Examples:**

```conductor
# Shuffle review order
- id: review_order
  utility.random_shuffle:
    items: ["security", "performance", "style", "correctness"]

# Output: ["style", "security", "correctness", "performance"]

# Randomize iteration order
- id: shuffled_items
  utility.random_shuffle:
    items: '{{.steps.get_items.result}}'

- id: process_random
  type: parallel
  foreach: '{{.steps.shuffled_items.result}}'
  steps:
    - id: process
      type: llm
      prompt: "Process: {{.item}}"
```

---

## Math Operations

### math_clamp

Constrain a value to a range.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `value` | number | Yes | — | Value to clamp |
| `min` | number | Yes | — | Minimum bound |
| `max` | number | Yes | — | Maximum bound |

**Returns:** Number in range [min, max]

**Examples:**

```conductor
# Ensure score is 0-100
- id: bounded_score
  utility.math_clamp:
    value: '{{.steps.calculate.result}}'
    min: 0
    max: 100

# Input: 125 → Output: 100
# Input: -5 → Output: 0
# Input: 75 → Output: 75

# Limit concurrency
- id: concurrency
  utility.math_clamp:
    value: '{{.inputs.requested_workers}}'
    min: 1
    max: 10
```

**Errors:**

- `ValidationError` — min > max

---

### math_round

Round a number to specified decimal places.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `value` | number | Yes | — | Value to round |
| `places` | integer | No | 0 | Decimal places (0 = integer) |

**Returns:** Rounded number

**Examples:**

```conductor
# Round to integer
- id: whole
  utility.math_round:
    value: 3.7

# Output: 4

# Round to 2 decimal places
- id: currency
  utility.math_round:
    value: 19.999
    places: 2

# Output: 20.00

# Round percentage
- id: percent
  utility.math_round:
    value: '{{.steps.calculate_ratio.result}}'
    places: 1
```

---

### math_min

Return the minimum of multiple values.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `values` | array | Yes | — | Array of numbers |

**Returns:** Smallest number from the array

**Examples:**

```conductor
# Find minimum score
- id: lowest
  utility.math_min:
    values: [85, 92, 78, 95, 88]

# Output: 78

# Dynamic comparison
- id: min_cost
  utility.math_min:
    values:
      - '{{.steps.provider_a.cost}}'
      - '{{.steps.provider_b.cost}}'
      - '{{.steps.provider_c.cost}}'
```

**Errors:**

- `ValidationError` — Empty array or non-numeric values

---

### math_max

Return the maximum of multiple values.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `values` | array | Yes | — | Array of numbers |

**Returns:** Largest number from the array

**Examples:**

```conductor
# Find highest score
- id: highest
  utility.math_max:
    values: [85, 92, 78, 95, 88]

# Output: 95

# Ensure minimum threshold
- id: threshold
  utility.math_max:
    values:
      - '{{.steps.calculated.result}}'
      - 10

# Output: at least 10
```

---

## Error Handling

All utility operations return structured errors:

| Error Type | Cause | Example |
|------------|-------|---------|
| `ValidationError` | Invalid parameters | Empty array, min > max |
| `TypeError` | Wrong parameter type | String where number expected |

**Example error:**

```
Operation: utility.random_int
Error: Validation failed
Type: ValidationError
Cause: min (10) must be less than or equal to max (5)
```

## Best Practices

1. **Use UUIDs for system identifiers** — Standard format, easy to debug
2. **Use NanoIDs for user-facing IDs** — Shorter, URL-safe
3. **Validate array lengths before sampling** — Check count <= length
4. **Use weighted selection for A/B tests** — Clear probability control
5. **Clamp user inputs** — Prevent out-of-range values
6. **Round costs for display** — Use `places: 2` for currency

## See Also

- [Transform Action](transform.md) - Data manipulation with jq
- [File Action](file.md) - Read/write data
- [Workflow Schema](../workflow-schema.md) - Step definitions
