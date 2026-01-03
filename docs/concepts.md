# Core Concepts

## Workflows

A workflow is a YAML file that defines a sequence of steps. Each workflow has a name and one or more steps that execute in order.

```yaml
name: my-workflow
steps:
  - id: step1
    llm:
      prompt: Generate a recipe
  - id: step2
    shell:
      command: echo "Done"
```

## Steps

Steps are the building blocks of workflows. Each step has:

- **id** - Unique identifier for the step
- **type** - What the step does (llm, shell, http, file, etc.)
- **inputs** - Data the step receives
- **outputs** - Data the step produces

## Inputs and Outputs

Workflows can accept inputs and produce outputs:

```yaml
name: greet
inputs:
  name:
    type: string
steps:
  - id: greet
    llm:
      prompt: "Say hello to ${inputs.name}"
outputs:
  greeting: ${steps.greet.output}
```

Reference step outputs using `${steps.<step-id>.output}` syntax.

## Actions

Actions are local operations:

- **llm** - Call an LLM with a prompt
- **shell** - Execute shell commands
- **http** - Make HTTP requests
- **file** - Read/write files
- **transform** - Transform data (JSON, YAML, text)
- **utility** - Utility operations (sleep, random, etc.)

## Integrations

Integrations connect to external services:

- **GitHub** - Create issues, comment on PRs
- **Slack** - Send messages, read channels
- **Jira** - Create/update tickets
- **Discord** - Post to channels
- **Notion** - Create/update pages and databases

## Triggers

Triggers define how workflows are invoked:

- **cron** - Run on a schedule
- **webhook** - Trigger via HTTP POST
- **poll** - Check for changes periodically

## Controller

The controller is the long-running service that executes workflows with triggers. Run workflows directly with `conductor run` or deploy them to a controller with `conductor deploy`.
