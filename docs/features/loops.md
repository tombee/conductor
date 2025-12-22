# Loops

Iterate over lists with `foreach` to process multiple items.

## Basic Loop

Iterate over a static list:

```yaml
steps:
  - id: greet_all
    foreach:
      items:
        - Alice
        - Bob
        - Carol
      steps:
        - id: greet
          llm:
            prompt: "Say hello to ${item}"
```

Access the current item with `${item}`.

## Dynamic Lists

Iterate over step outputs:

```yaml
steps:
  - id: fetch_users
    http:
      method: GET
      url: https://api.example.com/users
  - id: process_users
    foreach:
      items: ${steps.fetch_users.output.users}
      steps:
        - id: analyze
          llm:
            prompt: "Analyze user: ${item.name}"
```

## Nested Loops

Loops can contain other loops:

```yaml
steps:
  - id: process_teams
    foreach:
      items: ${inputs.teams}
      steps:
        - id: process_members
          foreach:
            items: ${item.members}
            steps:
              - id: greet
                llm:
                  prompt: "Welcome ${item.name} from team ${parent.item.name}"
```

## Access Loop Results

Reference all outputs from a loop:

```yaml
steps:
  - id: generate_recipes
    foreach:
      items:
        - breakfast
        - lunch
        - dinner
      steps:
        - id: recipe
          llm:
            prompt: "Generate a ${item} recipe"
  - id: combine
    llm:
      prompt: "Create a meal plan from these recipes: ${steps.generate_recipes.outputs}"
```

`${steps.loopId.outputs}` contains an array of all iteration outputs.

## Current Iteration

Access the current iteration index:

```yaml
steps:
  - id: number_items
    foreach:
      items: ${inputs.items}
      steps:
        - id: label
          llm:
            prompt: "Item ${index}: ${item}"
```

Use `${index}` for the zero-based position.

## Loop Context

Within a loop, access:
- `${item}` - Current item
- `${index}` - Current index (0-based)
- `${steps.loopId.outputs}` - Previous iteration outputs

## Parallel Loop Execution

Loop iterations run sequentially by default. For parallel execution:

```yaml
steps:
  - id: process_all
    foreach:
      items: ${inputs.items}
      parallel: true
      steps:
        - id: process
          llm:
            prompt: "Process ${item}"
```

Use `parallel: true` when iterations are independent.

## Breaking Early

Loops run all iterations. To exit early, use conditions in subsequent steps:

```yaml
steps:
  - id: search
    foreach:
      items: ${inputs.candidates}
      steps:
        - id: check
          llm:
            prompt: "Is ${item} suitable? Answer yes or no."
  - id: select
    condition: ${steps.search.outputs | contains("yes")}
    llm:
      prompt: "Select the first suitable item"
```

## Performance

Sequential loops process one item at a time. For large lists, use `parallel: true` to process multiple items concurrently.
