# YAML Workflows

Load and run platform YAML workflows programmatically.

## What You'll Build

A tool that loads existing YAML workflows and runs them with the SDK.

## Loading a YAML Workflow

Given a workflow file `review.yaml`:

```yaml
name: code-review
description: Review code for issues

inputs:
  code:
    type: string
    description: Code to review

steps:
  - id: review
    model: balanced
    prompt: |
      Review this code and provide feedback:
      {{.inputs.code}}

output: "{{.steps.review.response}}"
```

Load and run it:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/tombee/conductor/sdk"
    "github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

func main() {
    s, err := newSDK()
    if err != nil {
        panic(err)
    }
    defer s.Close()

    wf, err := s.LoadWorkflowFile("./review.yaml")
    if err != nil {
        panic(err)
    }

    result, err := s.Run(context.Background(), wf, map[string]any{
        "code": "func add(a, b int) int { return a + b }",
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(result.Output)
}

func newSDK() (*sdk.SDK, error) {
    cc := claudecode.New()
    if found, _ := cc.Detect(); found {
        return sdk.New(sdk.WithProvider("claude-code", cc))
    }
    if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
        return sdk.New(sdk.WithAnthropicProvider(key))
    }
    return nil, fmt.Errorf("no provider available")
}
```

## Embedding Workflows

Compile workflows into your binary using `go:embed`:

```go
import (
    "context"
    _ "embed"

    "github.com/tombee/conductor/sdk"
    "github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

//go:embed workflows/review.yaml
var reviewWorkflow []byte

//go:embed workflows/summarize.yaml
var summarizeWorkflow []byte

func main() {
    s, _ := newSDK()
    defer s.Close()

    // Load from embedded bytes
    wf, _ := s.LoadWorkflow(reviewWorkflow)
    result, _ := s.Run(context.Background(), wf, inputs)
}
```

## Validating Workflows

Check workflows before running:

```go
wf, err := s.LoadWorkflowFile("./workflow.yaml")
if err != nil {
    var validErr *sdk.ValidationError
    if errors.As(err, &validErr) {
        fmt.Printf("Validation failed: %s at %s\n",
            validErr.Message, validErr.Field)
    }
    return err
}
```

## Multiple Workflow Files

Load a directory of workflows:

```go
//go:embed workflows/*.yaml
var workflowFS embed.FS

func loadWorkflows(s *sdk.SDK) (map[string]*sdk.Workflow, error) {
    workflows := make(map[string]*sdk.Workflow)

    entries, _ := workflowFS.ReadDir("workflows")
    for _, entry := range entries {
        data, _ := workflowFS.ReadFile("workflows/" + entry.Name())
        wf, err := s.LoadWorkflow(data)
        if err != nil {
            return nil, fmt.Errorf("load %s: %w", entry.Name(), err)
        }
        workflows[wf.Name] = wf
    }

    return workflows, nil
}
```

## Key Concepts

**File Loading**: `LoadWorkflowFile()` reads from disk; `LoadWorkflow()` parses bytes (useful with `go:embed`).

**Validation**: Loading validates the YAML structure and returns typed errors for invalid workflows.

**Portability**: The same YAML workflows work with both the SDK and the Conductor platform.

## Next Steps

- Combine YAML base workflows with programmatic extensions
- Build a workflow selection CLI
- Add event streaming with `s.OnEvent()` for progress tracking
