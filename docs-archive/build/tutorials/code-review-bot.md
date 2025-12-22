# Code Review Bot

Build a multi-step workflow that analyzes code and generates structured feedback.

## What You'll Build

A code review bot that:
- Analyzes code for issues in multiple categories
- Generates structured JSON feedback
- Uses different model tiers for cost efficiency

## The Workflow

```go
package main

import (
    "context"
    "encoding/json"
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

    wf, _ := s.NewWorkflow("code-review").
        Input("code", sdk.TypeString).
        Input("language", sdk.TypeString).

        // Step 1: Analyze for bugs (use balanced model for accuracy)
        Step("bugs").LLM().
            Model("balanced").
            System("You are a code reviewer focused on finding bugs.").
            Prompt(`Analyze this {{.inputs.language}} code for bugs:

{{.inputs.code}}

Return JSON: {"bugs": [{"line": N, "issue": "...", "severity": "high|medium|low"}]}`).
            Done().

        // Step 2: Analyze for style issues (fast model is sufficient)
        Step("style").LLM().
            Model("fast").
            System("You are a code style reviewer.").
            Prompt(`Review this {{.inputs.language}} code for style issues:

{{.inputs.code}}

Return JSON: {"issues": [{"line": N, "suggestion": "..."}]}`).
            Done().

        // Step 3: Generate summary from previous steps
        Step("summary").LLM().
            Model("fast").
            Prompt(`Summarize this code review:

Bugs found: {{.steps.bugs.response}}
Style issues: {{.steps.style.response}}

Write a brief 2-3 sentence summary.`).
            Done().

        Build()

    code := `func calculate(x int) int {
    if x = 0 {  // bug: assignment instead of comparison
        return 0
    }
    result := x*2
    return result
}`

    result, err := s.Run(context.Background(), wf, map[string]any{
        "code":     code,
        "language": "Go",
    })
    if err != nil {
        panic(err)
    }

    printReview(result)
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

## Processing the Results

```go
type BugReport struct {
    Bugs []struct {
        Line     int    `json:"line"`
        Issue    string `json:"issue"`
        Severity string `json:"severity"`
    } `json:"bugs"`
}

func printReview(result *sdk.Result) {
    var bugs BugReport
    bugsJSON := result.Steps["bugs"].Output.(string)
    json.Unmarshal([]byte(bugsJSON), &bugs)

    fmt.Println("=== Code Review ===\n")

    fmt.Println("Bugs Found:")
    for _, bug := range bugs.Bugs {
        fmt.Printf("  Line %d [%s]: %s\n", bug.Line, bug.Severity, bug.Issue)
    }

    fmt.Printf("\nSummary:\n%s\n", result.Steps["summary"].Output)
}
```

## Run It

```bash
go run main.go
```

```
=== Code Review ===

Bugs Found:
  Line 2 [high]: Assignment operator '=' used instead of comparison '=='

Summary:
The code contains a critical bug on line 2 where an assignment operator is
used instead of a comparison in the if statement. Style-wise, the code is
reasonably clean but could benefit from more descriptive variable names.
```

## Key Concepts

**Multi-Step Workflows**: Each step runs in sequence. Later steps reference earlier outputs via `{{.steps.stepId.response}}`.

**Model Tiers**: Use `"balanced"` for complex analysis and `"fast"` for simpler tasks.

**Structured Output**: Prompt the LLM to return JSON, then parse it in Go for programmatic use.

## Next Steps

- Add security analysis as a parallel step
- Integrate with GitHub using `sdk.WithBuiltinIntegrations()`
- Add file reading with `sdk.WithBuiltinActions()` to review local files
