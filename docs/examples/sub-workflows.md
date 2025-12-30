# Sub-workflow Examples

This document provides complete, working examples of sub-workflow patterns.

## Example 1: Simple Extraction

Break a large workflow into focused sub-workflows.

### Directory Structure

```
sentiment-analysis/
├── main.yaml
└── helpers/
    ├── fetch-reviews.yaml
    ├── analyze-sentiment.yaml
    └── generate-report.yaml
```

### helpers/fetch-reviews.yaml

```yaml
name: fetch-reviews
description: Fetches product reviews from a data source

inputs:
  - name: product_id
    type: string
    required: true

outputs:
  - name: reviews
    type: array
    value: "{{.steps.fetch.outputs.data}}"

steps:
  - id: fetch
    type: llm
    model: fast
    prompt: |
      Generate 5 sample product reviews for product ID: {{.inputs.product_id}}

      Return as JSON array of review objects with "text" and "rating" fields.
    output_schema:
      type: object
      properties:
        data:
          type: array
          items:
            type: object
            properties:
              text: { type: string }
              rating: { type: number }
```

### helpers/analyze-sentiment.yaml

```yaml
name: analyze-sentiment
description: Analyzes sentiment of a review

inputs:
  - name: review_text
    type: string
    required: true

outputs:
  - name: sentiment
    type: string
    value: "{{.steps.classify.outputs.category}}"
  - name: confidence
    type: number
    value: "{{.steps.classify.outputs.confidence}}"

steps:
  - id: classify
    type: llm
    model: balanced
    prompt: |
      Analyze the sentiment of this review:

      "{{.inputs.review_text}}"

      Classify as positive, negative, or neutral.
    output_type: classification
    output_options:
      categories: [positive, negative, neutral]
```

### helpers/generate-report.yaml

```yaml
name: generate-report
description: Generates a summary report from sentiment analysis results

inputs:
  - name: product_id
    type: string
    required: true
  - name: results
    type: array
    required: true

outputs:
  - name: report
    type: string
    value: "{{.steps.summarize.outputs.response}}"

steps:
  - id: summarize
    type: llm
    model: balanced
    prompt: |
      Create a summary report for product {{.inputs.product_id}}:

      Analysis results:
      {{.inputs.results}}

      Provide:
      - Overall sentiment trend
      - Key themes from reviews
      - Recommendations for product team
```

### main.yaml

```yaml
name: product-review-analysis
description: Complete product review analysis pipeline

inputs:
  - name: product_id
    type: string
    required: true

steps:
  - id: fetch
    type: workflow
    workflow: ./helpers/fetch-reviews.yaml
    inputs:
      product_id: "{{.inputs.product_id}}"

  - id: analyze_all
    type: parallel
    foreach: "{{.steps.fetch.outputs.reviews}}"
    steps:
      - id: analyze_review
        type: workflow
        workflow: ./helpers/analyze-sentiment.yaml
        inputs:
          review_text: "{{.item.text}}"

  - id: generate_report
    type: workflow
    workflow: ./helpers/generate-report.yaml
    inputs:
      product_id: "{{.inputs.product_id}}"
      results: "{{.steps.analyze_all.outputs}}"

outputs:
  - name: report
    type: string
    value: "{{.steps.generate_report.outputs.report}}"
```

### Running the Example

```bash
$ conductor run main.yaml -i product_id="PROD-123"

Workflow: product-review-analysis
  ✓ fetch (1.5s)
    └─ 5 reviews fetched
  ✓ analyze_all (3.2s)
    └─ 5 parallel analyses completed
  ✓ generate_report (1.8s)

Result: Product PROD-123 Analysis Report
Overall sentiment: Mostly positive (80% positive, 15% neutral, 5% negative)

Key themes:
- Users love the ease of use
- Quality is consistently praised
- Some concerns about pricing

Recommendations:
- Consider value-tier pricing option
- Expand documentation for advanced features
- Continue focus on user experience
```

## Example 2: Reusable Library Pattern

Create a shared library of workflow components.

### Directory Structure

```
workflows/
├── library/
│   ├── github/
│   │   ├── fetch-pr.yaml
│   │   ├── create-comment.yaml
│   │   └── merge-pr.yaml
│   ├── analysis/
│   │   ├── code-quality.yaml
│   │   ├── security-scan.yaml
│   │   └── test-coverage.yaml
│   └── notifications/
│       ├── slack-notify.yaml
│       └── email-notify.yaml
└── apps/
    ├── pr-review-bot.yaml
    └── release-checklist.yaml
```

### library/github/fetch-pr.yaml

```yaml
name: fetch-pr
description: Fetches pull request details from GitHub

inputs:
  - name: pr_number
    type: number
    required: true
  - name: repo
    type: string
    required: true

outputs:
  - name: title
    type: string
    value: "{{.steps.get_pr.outputs.title}}"
  - name: description
    type: string
    value: "{{.steps.get_pr.outputs.description}}"
  - name: files_changed
    type: array
    value: "{{.steps.get_pr.outputs.files}}"

steps:
  - id: get_pr
    github.get_pull_request:
      repo: "{{.inputs.repo}}"
      pull_number: "{{.inputs.pr_number}}"
```

### library/analysis/code-quality.yaml

```yaml
name: code-quality-check
description: Analyzes code quality and provides suggestions

inputs:
  - name: files
    type: array
    required: true
    description: "Array of file objects with path and content"

outputs:
  - name: quality_score
    type: number
    value: "{{.steps.analyze.outputs.score}}"
  - name: suggestions
    type: array
    value: "{{.steps.analyze.outputs.suggestions}}"
  - name: issues
    type: array
    value: "{{.steps.analyze.outputs.issues}}"

steps:
  - id: analyze
    type: llm
    model: balanced
    prompt: |
      Analyze these code files for quality issues:

      {{.inputs.files}}

      Provide:
      1. Quality score (0-100)
      2. List of specific issues found
      3. Actionable suggestions for improvement

      Focus on:
      - Code clarity and readability
      - Naming conventions
      - Function complexity
      - Documentation
    output_schema:
      type: object
      properties:
        score: { type: number, minimum: 0, maximum: 100 }
        issues:
          type: array
          items:
            type: object
            properties:
              file: { type: string }
              line: { type: number }
              severity: { type: string, enum: [low, medium, high] }
              description: { type: string }
        suggestions:
          type: array
          items:
            type: object
            properties:
              category: { type: string }
              description: { type: string }
      required: [score, issues, suggestions]
```

### library/notifications/slack-notify.yaml

```yaml
name: slack-notification
description: Sends a notification to Slack

inputs:
  - name: channel
    type: string
    required: true
  - name: message
    type: string
    required: true
  - name: priority
    type: string
    default: "normal"
    description: "Priority level: low, normal, high, urgent"

outputs:
  - name: sent
    type: boolean
    value: "true"
  - name: timestamp
    type: string
    value: "{{.steps.send.outputs.ts}}"

steps:
  - id: format_message
    type: llm
    model: fast
    prompt: |
      Format this message for Slack with appropriate emojis and structure:

      Priority: {{.inputs.priority}}
      Message: {{.inputs.message}}

  - id: send
    slack.post_message:
      channel: "{{.inputs.channel}}"
      text: "{{.steps.format_message.outputs.response}}"
```

### apps/pr-review-bot.yaml

```yaml
name: pr-review-bot
description: Automated PR review workflow

inputs:
  - name: pr_number
    type: number
    required: true
  - name: repo
    type: string
    required: true

steps:
  # Fetch PR details from library
  - id: fetch_pr
    type: workflow
    workflow: ../library/github/fetch-pr.yaml
    inputs:
      pr_number: "{{.inputs.pr_number}}"
      repo: "{{.inputs.repo}}"

  # Run code quality analysis from library
  - id: quality_check
    type: workflow
    workflow: ../library/analysis/code-quality.yaml
    inputs:
      files: "{{.steps.fetch_pr.outputs.files_changed}}"

  # Create review comment using library workflow
  - id: post_review
    type: workflow
    workflow: ../library/github/create-comment.yaml
    inputs:
      pr_number: "{{.inputs.pr_number}}"
      repo: "{{.inputs.repo}}"
      comment: |
        ## Code Quality Review

        Quality Score: {{.steps.quality_check.outputs.quality_score}}/100

        ### Issues Found
        {{.steps.quality_check.outputs.issues}}

        ### Suggestions
        {{.steps.quality_check.outputs.suggestions}}

  # Notify team via Slack using library workflow
  - id: notify_team
    type: workflow
    workflow: ../library/notifications/slack-notify.yaml
    inputs:
      channel: "#code-reviews"
      priority: >
        {{.steps.quality_check.outputs.quality_score < 70 ? "high" : "normal"}}
      message: >
        PR #{{.inputs.pr_number}} reviewed.
        Quality score: {{.steps.quality_check.outputs.quality_score}}/100
```

## Example 3: LLM-as-Router Agent Pattern

Use an LLM to route requests to specialized sub-workflows.

### Directory Structure

```
personal-assistant/
├── main.yaml
└── capabilities/
    ├── email-triage.yaml
    ├── calendar-management.yaml
    ├── task-prioritization.yaml
    └── web-search.yaml
```

### capabilities/email-triage.yaml

```yaml
name: email-triage
description: Triages and processes emails

inputs:
  - name: request
    type: string
    required: true
    description: "User's email-related request"

outputs:
  - name: result
    type: string
    value: "{{.steps.process.outputs.response}}"
  - name: action_taken
    type: string
    value: "{{.steps.process.outputs.action}}"

steps:
  - id: process
    type: llm
    model: balanced
    prompt: |
      Process this email request:
      {{.inputs.request}}

      Perform appropriate actions:
      - Categorize by priority and type
      - Draft responses if needed
      - Suggest follow-up actions

      Return action taken and result.
    output_schema:
      type: object
      properties:
        action: { type: string }
        response: { type: string }
      required: [action, response]
```

### capabilities/calendar-management.yaml

```yaml
name: calendar-management
description: Manages calendar events and scheduling

inputs:
  - name: request
    type: string
    required: true

outputs:
  - name: result
    type: string
    value: "{{.steps.manage.outputs.result}}"
  - name: events_modified
    type: array
    value: "{{.steps.manage.outputs.events}}"

steps:
  - id: manage
    type: llm
    model: balanced
    prompt: |
      Handle this calendar request:
      {{.inputs.request}}

      Actions you can take:
      - Create new events
      - Reschedule existing events
      - Check availability
      - Send meeting invites

      Return the result and list of events modified.
    output_schema:
      type: object
      properties:
        result: { type: string }
        events:
          type: array
          items:
            type: object
            properties:
              title: { type: string }
              start: { type: string }
              action: { type: string, enum: [created, updated, deleted] }
      required: [result, events]
```

### capabilities/task-prioritization.yaml

```yaml
name: task-prioritization
description: Analyzes and prioritizes tasks

inputs:
  - name: request
    type: string
    required: true

outputs:
  - name: result
    type: string
    value: "{{.steps.prioritize.outputs.summary}}"
  - name: prioritized_tasks
    type: array
    value: "{{.steps.prioritize.outputs.tasks}}"

steps:
  - id: prioritize
    type: llm
    model: balanced
    prompt: |
      Analyze and prioritize these tasks:
      {{.inputs.request}}

      For each task, determine:
      - Urgency (1-5)
      - Importance (1-5)
      - Estimated time
      - Priority rank

      Provide a prioritized list with reasoning.
    output_schema:
      type: object
      properties:
        summary: { type: string }
        tasks:
          type: array
          items:
            type: object
            properties:
              description: { type: string }
              urgency: { type: number }
              importance: { type: number }
              time_estimate: { type: string }
              rank: { type: number }
      required: [summary, tasks]
```

### capabilities/web-search.yaml

```yaml
name: web-search
description: Searches the web and summarizes results

inputs:
  - name: query
    type: string
    required: true

outputs:
  - name: result
    type: string
    value: "{{.steps.search.outputs.response}}"

steps:
  - id: search
    type: llm
    model: balanced
    prompt: |
      User wants to search for: {{.inputs.query}}

      Simulate a web search and provide:
      1. Top 3-5 relevant results
      2. Brief summary of each
      3. Key takeaways

      Format as a helpful summary.
```

### main.yaml

```yaml
name: personal-assistant
description: AI assistant that routes requests to specialized capabilities

inputs:
  - name: request
    type: string
    required: true
    description: "User's request in natural language"

steps:
  # Step 1: Use LLM to understand intent
  - id: understand_intent
    type: llm
    model: balanced
    prompt: |
      Analyze the user's request and determine their intent:

      "{{.inputs.request}}"

      Classify the intent and extract relevant details.
    output_schema:
      type: object
      properties:
        intent:
          type: string
          enum: [email, calendar, task, search, unknown]
        details: { type: string }
        confidence: { type: number, minimum: 0, maximum: 1 }
      required: [intent, details, confidence]

  # Step 2-5: Route to appropriate sub-workflow based on intent
  - id: handle_email
    condition:
      expression: 'steps.understand_intent.outputs.intent == "email"'
    type: workflow
    workflow: ./capabilities/email-triage.yaml
    inputs:
      request: "{{.steps.understand_intent.outputs.details}}"

  - id: handle_calendar
    condition:
      expression: 'steps.understand_intent.outputs.intent == "calendar"'
    type: workflow
    workflow: ./capabilities/calendar-management.yaml
    inputs:
      request: "{{.steps.understand_intent.outputs.details}}"

  - id: handle_task
    condition:
      expression: 'steps.understand_intent.outputs.intent == "task"'
    type: workflow
    workflow: ./capabilities/task-prioritization.yaml
    inputs:
      request: "{{.steps.understand_intent.outputs.details}}"

  - id: handle_search
    condition:
      expression: 'steps.understand_intent.outputs.intent == "search"'
    type: workflow
    workflow: ./capabilities/web-search.yaml
    inputs:
      query: "{{.steps.understand_intent.outputs.details}}"

  # Step 6: Synthesize results
  - id: respond
    type: llm
    model: balanced
    prompt: |
      Summarize what was accomplished for the user:

      Original request: {{.inputs.request}}
      Intent: {{.steps.understand_intent.outputs.intent}}

      Results:
      - Email: {{.steps.handle_email.outputs.result}}
      - Calendar: {{.steps.handle_calendar.outputs.result}}
      - Tasks: {{.steps.handle_task.outputs.result}}
      - Search: {{.steps.handle_search.outputs.result}}

      Provide a friendly, helpful response.

outputs:
  - name: response
    type: string
    value: "{{.steps.respond.outputs.response}}"
  - name: intent
    type: string
    value: "{{.steps.understand_intent.outputs.intent}}"
```

### Running the Example

```bash
# Email request
$ conductor run main.yaml -i request="Check my emails and draft responses for anything urgent"

Intent detected: email
✓ Email triage completed
Response: I've checked your emails. Found 3 urgent items and drafted responses...

# Calendar request
$ conductor run main.yaml -i request="Schedule a meeting with the team for next Tuesday"

Intent detected: calendar
✓ Calendar management completed
Response: I've scheduled a team meeting for next Tuesday at 2 PM...

# Task request
$ conductor run main.yaml -i request="Help me prioritize my tasks for today"

Intent detected: task
✓ Task prioritization completed
Response: I've analyzed your tasks. Here's your priority order: 1) Finish report (urgent)...
```

## Next Steps

- Try modifying these examples for your use case
- Explore [Sub-workflow Guide](../guides/sub-workflows.md) for more patterns
- Read [Architecture Documentation](../architecture/sub-workflows.md) for implementation details
