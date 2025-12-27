# Execution Flow Diagrams

Sequence diagrams showing how workflows execute at runtime.

## CLI Workflow Execution

When a user runs `conductor run workflow.yaml`:

```mermaid
sequenceDiagram
    participant User
    participant CLI as conductor CLI
    participant Daemon as conductord
    participant Executor as Workflow Executor
    participant Provider as LLM Provider
    participant Tool as Tool Registry

    User->>CLI: conductor run workflow.yaml
    CLI->>Daemon: POST /v1/runs (socket)
    Daemon->>Daemon: Load & validate workflow
    Daemon->>Executor: Execute(workflow, inputs)

    loop For each step
        Executor->>Provider: Complete(prompt, tools)
        Provider-->>Executor: Response (may include tool_calls)

        opt If tool calls
            loop For each tool call
                Executor->>Tool: Execute(tool_name, params)
                Tool-->>Executor: Result
            end
            Executor->>Provider: Continue with tool results
            Provider-->>Executor: Final response
        end

        Executor->>Daemon: Checkpoint step result
    end

    Executor-->>Daemon: Workflow complete
    Daemon-->>CLI: Run result
    CLI-->>User: Output
```

## Daemon API Workflow

When an external client triggers a workflow via API:

```mermaid
sequenceDiagram
    participant Client as API Client
    participant API as Daemon API
    participant Queue as Job Queue
    participant Runner as Runner
    participant Executor as Executor
    participant State as State Backend

    Client->>API: POST /v1/runs
    API->>Queue: Enqueue job
    API-->>Client: 202 Accepted (run_id)

    Queue->>Runner: Dequeue job
    Runner->>Executor: Execute workflow

    loop For each step
        Executor->>State: Save checkpoint
        Executor->>Executor: Execute step
        State-->>Executor: Confirm saved
    end

    Runner->>State: Save final result

    Note over Client,State: Client polls for status
    Client->>API: GET /v1/runs/{id}
    API->>State: Get run status
    State-->>API: Run result
    API-->>Client: 200 OK (result)
```

## LLM Step Execution

Detailed view of a single LLM step with tool use:

```mermaid
sequenceDiagram
    participant Executor
    participant Provider as LLM Provider
    participant Tools as Tool Registry
    participant HTTP as HTTP Tool
    participant Shell as Shell Tool

    Executor->>Provider: Complete(prompt, available_tools)
    Provider->>Provider: Generate response
    Provider-->>Executor: Response with tool_calls

    Note over Executor: Parse tool calls from response

    par Execute tools in parallel
        Executor->>HTTP: Execute(http.get, {url: "..."})
        HTTP-->>Executor: {"status": 200, "body": "..."}
    and
        Executor->>Shell: Execute(shell.run, {cmd: "..."})
        Shell-->>Executor: {"stdout": "...", "exit_code": 0}
    end

    Executor->>Provider: Continue with tool_results
    Provider-->>Executor: Final text response

    Note over Executor: Apply output schema if defined
    Executor->>Executor: Validate/parse output
```

## Webhook Trigger Flow

When a webhook triggers a workflow:

```mermaid
sequenceDiagram
    participant Source as Webhook Source
    participant Handler as Webhook Handler
    participant Router as Trigger Router
    participant Daemon as Daemon
    participant Executor as Executor

    Source->>Handler: POST /webhooks/{source}
    Handler->>Handler: Verify signature
    Handler->>Router: Match triggers
    Router->>Router: Find matching workflows

    loop For each matched workflow
        Router->>Daemon: Enqueue run
        Daemon->>Executor: Execute async
    end

    Handler-->>Source: 200 OK

    Note over Executor: Workflows execute in background
```

## Scheduled Workflow Flow

When a cron schedule triggers a workflow:

```mermaid
sequenceDiagram
    participant Scheduler as Scheduler
    participant State as State Backend
    participant Daemon as Daemon
    participant Executor as Executor

    loop Every minute
        Scheduler->>State: Get due schedules
        State-->>Scheduler: List of workflows

        loop For each due workflow
            Scheduler->>State: Mark as triggered
            Scheduler->>Daemon: Enqueue run
            Daemon->>Executor: Execute async
        end
    end
```

## Error Recovery Flow

How the system recovers from failures:

```mermaid
sequenceDiagram
    participant Executor as Executor
    participant State as State Backend
    participant Provider as LLM Provider

    Executor->>State: Load checkpoint (if exists)

    alt Has checkpoint
        State-->>Executor: Resume from step N
    else No checkpoint
        State-->>Executor: Start from beginning
    end

    loop Execute remaining steps
        Executor->>Provider: Complete(prompt)

        alt Success
            Provider-->>Executor: Response
            Executor->>State: Save checkpoint
        else Failure (retryable)
            Provider-->>Executor: Error
            Executor->>Executor: Wait (exponential backoff)
            Executor->>Provider: Retry
        else Failure (non-retryable)
            Provider-->>Executor: Error
            Executor->>State: Save error state
            Note over Executor: Workflow failed
        end
    end
```

## Key Concepts

### Checkpointing

- State saved after each step
- On crash, resume from last checkpoint
- No duplicate LLM calls on recovery

### Tool Execution

- Tools run in parallel when independent
- Sandboxed based on security profile
- Results passed back to LLM in tool_results

### Streaming

For real-time output, clients can use SSE:

```
GET /v1/runs/{id}/logs
Accept: text/event-stream

data: {"type": "step_start", "step_id": "analyze"}
data: {"type": "token", "content": "The"}
data: {"type": "token", "content": " code"}
data: {"type": "step_end", "step_id": "analyze"}
```

---
*See [Deployment Modes](deployment-modes.md) for infrastructure options.*
