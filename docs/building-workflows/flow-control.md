# Flow Control

Control how steps execute in your workflows.

## Sequential Execution

By default, steps run in order:

```conductor
steps:
  - id: step1
    model: fast
    prompt: "Generate topic for: {{.inputs.subject}}"

  - id: step2
    model: balanced
    prompt: "Write outline for: {{.steps.step1.response}}"

  - id: step3
    model: balanced
    prompt: "Write full post from: {{.steps.step2.response}}"
```

## Parallel Execution

Run independent steps concurrently:

```conductor
steps:
  - id: fetch_data
    file.read: "{{.inputs.data_file}}"

  - id: parallel_analysis
    type: parallel
    max_concurrency: 4
    steps:
      - id: sentiment
        model: fast
        prompt: "Analyze sentiment: {{.steps.fetch_data.content}}"

      - id: keywords
        model: fast
        prompt: "Extract keywords: {{.steps.fetch_data.content}}"

      - id: summary
        model: balanced
        prompt: "Summarize: {{.steps.fetch_data.content}}"
```

Access parallel results:

```conductor
  - id: consolidate
    model: balanced
    prompt: |
      Combine results:
      Sentiment: {{.steps.parallel_analysis.sentiment.response}}
      Keywords: {{.steps.parallel_analysis.keywords.response}}
      Summary: {{.steps.parallel_analysis.summary.response}}
```

## Conditional Execution

Run steps only when conditions are met:

```conductor
  - id: security_scan
    condition: 'inputs.include_security == true'
    model: strategic
    prompt: "Security analysis: {{.inputs.code}}"

  - id: deep_analysis
    condition: 'steps.classify.response contains "complex"'
    model: strategic
    prompt: "Deep analysis: {{.inputs.code}}"
```

### Decision Routing

Route to different steps based on classification:

```conductor
  - id: classify
    model: fast
    prompt: |
      Classify ticket: {{.inputs.ticket}}
      Output ONLY: bug, feature, or question

  - id: handle_bug
    condition: 'steps.classify.response contains "bug"'
    model: balanced
    prompt: "Bug triage: {{.inputs.ticket}}"

  - id: handle_feature
    condition: 'steps.classify.response contains "feature"'
    model: fast
    prompt: "Feature analysis: {{.inputs.ticket}}"
```

## Workflow Composition

Break complex workflows into reusable components:

```conductor
# analyze-sentiment.yaml
name: analyze-sentiment
inputs:
  - name: text
    type: string
steps:
  - id: sentiment
    model: fast
    prompt: "Sentiment analysis: {{.inputs.text}}"
outputs:
  - name: result
    value: "{{.steps.sentiment.response}}"
```

Call from another workflow:

```conductor
# main.yaml
steps:
  - id: analyze
    workflow.run:
      workflow: "analyze-sentiment.yaml"
      inputs:
        text: "{{.inputs.feedback}}"

  - id: respond
    model: balanced
    prompt: "Based on sentiment {{.steps.analyze.result}}, generate response."
```

## Common Patterns

### Read-Process-Write

```conductor
steps:
  - id: read
    file.read: "{{.inputs.file}}"

  - id: process
    model: balanced
    prompt: "Process: {{.steps.read.content}}"

  - id: write
    file.write:
      path: "output.txt"
      content: "{{.steps.process.response}}"
```

### Early Termination

Validate before expensive operations:

```conductor
  - id: validate
    model: fast
    prompt: "Is this valid code? Output: VALID or INVALID"

  - id: expensive_review
    condition: 'steps.validate.response contains "VALID"'
    model: strategic
    prompt: "Full code review: {{.inputs.code}}"
```

## See Also

- [Workflows and Steps](../learn/concepts/workflows-steps.md) - Step fundamentals
- [Error Handling](error-handling.md) - Retries and fallbacks
