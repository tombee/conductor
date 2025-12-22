# Multi-Agent Workflows

Build sophisticated workflows where multiple LLM agents with distinct roles collaborate to solve complex problems.

**Difficulty:** Advanced
**Prerequisites:**
- Complete [Building a Code Review Bot](code-review-bot.md)
- Understanding of parallel execution
- Basic knowledge of workflow patterns

---

## What You'll Build

A content creation workflow that uses multiple AI agents:

1. **Researcher** — Gathers information and facts
2. **Writer** — Creates initial drafts
3. **Critic** — Reviews and provides feedback
4. **Editor** — Polishes and finalizes content

This tutorial demonstrates:
- Multi-agent design patterns
- Agent role definition and specialization
- Sequential vs parallel agent execution
- Context passing between agents
- Output aggregation and synthesis
- Real-world content production pipeline

---

## What Are Multi-Agent Workflows?

Multi-agent workflows use multiple LLM steps with distinct roles, personas, or objectives that work together to complete complex tasks.

### Single-Agent Approach

Traditional workflows use one LLM to handle everything:

```yaml
- id: create_article
  type: llm
  prompt: "Research and write a comprehensive article about {{.inputs.topic}}"
```

**Limitations:**
- Single perspective limits quality
- One prompt tries to do too much
- No iterative refinement
- Hard to optimize (can't tune different parts separately)

### Multi-Agent Approach

Break the task into specialized agents:

```yaml
- id: research
  type: llm
  system: "You are a research specialist..."
  prompt: "Research {{.inputs.topic}}"

- id: write
  type: llm
  system: "You are a professional writer..."
  prompt: "Write based on: {{.steps.research.response}}"

- id: review
  type: llm
  system: "You are an editor..."
  prompt: "Review: {{.steps.write.response}}"
```

**Benefits:**
- Specialized prompts for each role
- Better quality through iteration
- Independent optimization of each agent
- Mirrors human workflows (research → draft → review)

---

## Agent Execution Patterns

### Sequential Pattern

Agents execute one after another, each building on previous work:

```
Researcher → Writer → Editor → Publisher
```

**When to use:**
- Steps depend on each other
- Linear workflow
- Each agent needs previous outputs

**Example:** Content creation, data analysis pipelines

### Parallel Pattern

Multiple agents analyze the same input simultaneously:

```
        ┌─ Security Reviewer
Input ──┼─ Performance Reviewer ──→ Consolidator
        └─ Style Reviewer
```

**When to use:**
- Independent analyses
- Speed is important
- Multiple perspectives needed

**Example:** Code reviews, document analysis

### Hybrid Pattern

Combines sequential and parallel execution:

```
        ┌─ Technical Writer
Plan ──┼─ Marketing Writer  ──→ Editor ──→ Publisher
        └─ Legal Writer
```

**When to use:**
- Complex workflows with both dependencies and parallelism
- Some steps can be parallelized, others can't

**Example:** Multi-format content generation

---

## Step 1: Create the Content Workflow

Create `content-creation.yaml`:

```yaml
name: multi-agent-content-creation
description: Collaborative content creation using specialized AI agents
version: "1.0"
```

---

## Step 2: Define Inputs

Add inputs for the content to create:

```yaml
inputs:
  - name: topic
    type: string
    required: true
    description: Topic to research and write about

  - name: target_audience
    type: string
    default: "technical professionals"
    description: Target audience for the content

  - name: word_count
    type: integer
    default: 1000
    description: Target word count for the article

  - name: include_critique
    type: boolean
    default: true
    description: Include critical review step

  - name: tone
    type: string
    default: "professional"
    enum: ["professional", "casual", "academic", "enthusiastic"]
    description: Writing tone
```

---

## Step 3: Agent 1 - The Researcher

Create a specialized research agent:

```yaml
steps:
  - id: research
    name: Research Agent - Gather Information
    type: llm
    model: strategic
    system: |
      You are an expert research analyst.

      Your role:
      - Find accurate, up-to-date information
      - Identify key facts, statistics, and examples
      - Note authoritative sources
      - Organize findings logically
      - Flag knowledge gaps or uncertainties

      Output format:
      # Research Findings: [Topic]

      ## Key Points
      - [Main points as bullet list]

      ## Statistics & Data
      - [Relevant data with context]

      ## Examples
      - [Real-world examples]

      ## Sources
      - [Authoritative sources for verification]

      ## Gaps
      - [Areas needing more research]
    prompt: |
      Research this topic comprehensively: {{.inputs.topic}}

      Target audience: {{.inputs.target_audience}}

      Focus on information that would be valuable, accurate, and relevant
      to this audience. Prioritize recent developments and practical insights.
```

**Why strategic tier?** Research requires deep reasoning about what information is relevant and how to organize it effectively.

---

## Step 4: Agent 2 - The Writer

Add a writing agent that uses research findings:

```yaml
  - id: draft
    name: Writing Agent - Create Draft
    type: llm
    model: balanced
    system: |
      You are a professional content writer.

      Your role:
      - Create engaging, well-structured content
      - Write clearly for the target audience
      - Use provided research as foundation
      - Maintain specified tone and style
      - Meet word count targets
      - Create compelling introductions and conclusions

      Writing principles:
      - Start strong with a hook
      - Use clear structure with headings
      - Include concrete examples
      - Write in active voice
      - End with clear takeaways
    prompt: |
      Write a {{.inputs.word_count}}-word article about: {{.inputs.topic}}

      **Target audience:** {{.inputs.target_audience}}
      **Tone:** {{.inputs.tone}}

      **Research to incorporate:**
      {{.steps.research.response}}

      Create a complete, polished draft with:
      1. Compelling title
      2. Strong introduction
      3. Well-organized body with headings
      4. Concrete examples and data
      5. Clear conclusion with takeaways

      Output the complete article in Markdown format.
```

**Why balanced tier?** Writing requires creativity and structure but less complex reasoning than research.

---

## Step 5: Agent 3 - The Critic (Parallel Analysis)

Add parallel reviewers for comprehensive feedback:

```yaml
  - id: reviews
    name: Review Agents - Multi-Perspective Critique
    type: parallel
    max_concurrency: 3
    condition:
      expression: 'inputs.include_critique == true'
    steps:
      - id: content_review
        name: Content Quality Review
        type: llm
        model: balanced
        system: |
          You are a content quality expert.

          Evaluate:
          - Accuracy and factual correctness
          - Completeness and depth
          - Clarity and coherence
          - Example quality and relevance
          - Logical flow and structure

          Rate each dimension: EXCELLENT, GOOD, NEEDS_WORK
          Provide specific, actionable feedback.
        prompt: |
          Review this article for content quality:

          {{.steps.draft.response}}

          Provide structured feedback with:
          1. Overall assessment (2-3 sentences)
          2. Strengths (3-5 specific points)
          3. Issues (specific problems with line references if possible)
          4. Suggestions (prioritized improvements)

      - id: audience_review
        name: Audience Fit Review
        type: llm
        model: balanced
        system: |
          You are an audience engagement specialist.

          Evaluate:
          - Appropriate tone and language for audience
          - Accessibility and readability
          - Engagement and interest level
          - Practical value and applicability
          - Call-to-action effectiveness
        prompt: |
          Review this article for audience fit:

          **Target audience:** {{.inputs.target_audience}}
          **Intended tone:** {{.inputs.tone}}

          **Article:**
          {{.steps.draft.response}}

          Assess how well the content serves the target audience.
          Rate: EXCELLENT_FIT, GOOD_FIT, NEEDS_ADJUSTMENT
          Provide specific recommendations.

      - id: technical_review
        name: Technical Accuracy Review
        type: llm
        model: strategic
        system: |
          You are a technical accuracy reviewer.

          Evaluate:
          - Technical accuracy and precision
          - Appropriate complexity level
          - Correct terminology usage
          - Logical soundness of arguments
          - Potential misconceptions or errors
        prompt: |
          Review this article for technical accuracy:

          {{.steps.draft.response}}

          Flag any technical errors, questionable claims, or areas
          needing clarification. Suggest corrections.
```

**Why parallel?** Reviews are independent analyses that can run simultaneously, saving time.

**Model tier choices:**
- Content & Audience: `balanced` (pattern recognition, good judgment)
- Technical: `strategic` (deep technical reasoning)

---

## Step 6: Agent 4 - The Editor (Synthesis)

Create an editor agent that incorporates all feedback:

```yaml
  - id: consolidate_feedback
    name: Consolidate Review Feedback
    type: llm
    model: balanced
    condition:
      expression: 'inputs.include_critique == true'
    prompt: |
      Consolidate these three reviews into a prioritized action plan:

      **Content Quality Review:**
      {{.steps.reviews.content_review.response}}

      **Audience Fit Review:**
      {{.steps.reviews.audience_review.response}}

      **Technical Accuracy Review:**
      {{.steps.reviews.technical_review.response}}

      Create a unified revision plan with:
      1. Critical issues (must fix)
      2. Important improvements (should fix)
      3. Optional enhancements (nice to have)

      Output a clear, actionable list for the editor.

  - id: final_edit
    name: Editor Agent - Produce Final Version
    type: llm
    model: balanced
    system: |
      You are a professional editor.

      Your role:
      - Incorporate reviewer feedback
      - Improve clarity, flow, and impact
      - Fix errors and inconsistencies
      - Enhance readability
      - Ensure quality standards
      - Preserve author's voice

      Editing principles:
      - Make substantive improvements, not just polish
      - Fix technical errors
      - Improve weak sections
      - Tighten verbose passages
      - Strengthen conclusions
    prompt: |
      Revise this article based on consolidated feedback:

      **Original draft:**
      {{.steps.draft.response}}

      {{if .steps.consolidate_feedback}}
      **Feedback to address:**
      {{.steps.consolidate_feedback.response}}
      {{else}}
      Perform a general editorial review focusing on clarity,
      accuracy, and engagement for the target audience.
      {{end}}

      **Target audience:** {{.inputs.target_audience}}
      **Tone:** {{.inputs.tone}}
      **Word count target:** {{.inputs.word_count}}

      Produce the final, polished article in Markdown format.
```

**Conditional logic:**
- If critique enabled: incorporate consolidated feedback
- If critique skipped: perform general editorial review

---

## Step 7: Generate Metadata

Add an agent to create metadata and summary:

```yaml
  - id: generate_metadata
    name: Metadata Agent - Create Publishing Info
    type: llm
    model: fast
    system: |
      You are a content metadata specialist.

      Generate:
      - SEO-friendly title variations
      - Meta description (150-160 characters)
      - Keywords (10-15)
      - Social media snippets
      - Category tags
      - Reading time estimate
    prompt: |
      Create metadata for this article:

      {{.steps.final_edit.response}}

      **Topic:** {{.inputs.topic}}
      **Audience:** {{.inputs.target_audience}}

      Output structured metadata in this format:

      # Metadata

      **Primary Title:** [title]
      **Alternative Titles:** [3 variations]
      **Meta Description:** [150-160 chars]
      **Keywords:** [comma-separated]
      **Category:** [main category]
      **Tags:** [5-8 tags]
      **Reading Time:** [estimate]
      **Social Snippet:** [280 chars for social media]
```

---

## Step 8: Save Outputs

Write the final article and metadata to files:

```yaml
  - id: write_article
    name: Save Final Article
    file.write:
      path: "article-{{.inputs.topic}}.md"
      content: "{{.steps.final_edit.response}}"

  - id: write_metadata
    name: Save Metadata
    file.write:
      path: "metadata-{{.inputs.topic}}.md"
      content: |
        # Article Metadata

        {{.steps.generate_metadata.response}}

        ---

        # Production Notes

        **Topic:** {{.inputs.topic}}
        **Target Audience:** {{.inputs.target_audience}}
        **Word Count Target:** {{.inputs.word_count}}
        **Tone:** {{.inputs.tone}}
        **Critique Enabled:** {{.inputs.include_critique}}

        ---

        # Workflow Summary

        1. Research phase completed
        2. Initial draft created
        {{if .steps.consolidate_feedback}}
        3. Multi-perspective review performed
        4. Feedback consolidated
        5. Final edit incorporating feedback
        {{else}}
        3. Editorial review performed
        {{end}}
        6. Metadata generated
```

---

## Step 9: Define Outputs

Add workflow outputs for programmatic access:

```yaml
outputs:
  - name: article
    type: string
    value: "{{.steps.final_edit.response}}"
    description: Final polished article

  - name: article_file
    type: string
    value: "article-{{.inputs.topic}}.md"
    description: Path to saved article

  - name: metadata
    type: string
    value: "{{.steps.generate_metadata.response}}"
    description: Article metadata

  - name: research_summary
    type: string
    value: "{{.steps.research.response}}"
    description: Original research findings

  - name: reviews_performed
    type: boolean
    value: "{{.inputs.include_critique}}"
    description: Whether critical review was performed
```

---

## Step 10: Run the Workflow

Test with different configurations:

### Full Multi-Agent Pipeline

```bash
conductor run content-creation.yaml \
  -i topic="Microservices Architecture Patterns" \
  -i target_audience="software architects" \
  -i word_count=1500 \
  -i tone="professional" \
  -i include_critique=true
```

**Expected flow:**

```
[conductor] Starting workflow: multi-agent-content-creation

[conductor] Step 1/8: research (llm, strategic)
[conductor] ✓ Completed in 8.2s

[conductor] Step 2/8: draft (llm, balanced)
[conductor] ✓ Completed in 12.1s

[conductor] Step 3/8: reviews (parallel)
[conductor]   ├─ content_review (llm, balanced) ...
[conductor]   ├─ audience_review (llm, balanced) ...
[conductor]   └─ technical_review (llm, strategic) ...
[conductor] ✓ All parallel steps completed in 7.3s

[conductor] Step 4/8: consolidate_feedback (llm, balanced)
[conductor] ✓ Completed in 3.4s

[conductor] Step 5/8: final_edit (llm, balanced)
[conductor] ✓ Completed in 11.8s

[conductor] Step 6/8: generate_metadata (llm, fast)
[conductor] ✓ Completed in 2.1s

[conductor] Step 7/8: write_article (file.write)
[conductor] ✓ Completed in 0.02s

[conductor] Step 8/8: write_metadata (file.write)
[conductor] ✓ Completed in 0.01s

--- Output: article_file ---
article-Microservices Architecture Patterns.md

[workflow complete in 45.0s]
```

### Fast Mode (Skip Critique)

```bash
conductor run content-creation.yaml \
  -i topic="Getting Started with Docker" \
  -i target_audience="developers" \
  -i word_count=800 \
  -i include_critique=false
```

**Faster execution:** ~25 seconds (skips review steps)

### Different Tones

```bash
# Academic tone
conductor run content-creation.yaml \
  -i topic="Quantum Computing Fundamentals" \
  -i tone="academic" \
  -i target_audience="computer science students"

# Casual tone
conductor run content-creation.yaml \
  -i topic="5 Productivity Tips for Remote Work" \
  -i tone="casual" \
  -i target_audience="remote workers"
```

---

## Complete Workflow

Here's the full `content-creation.yaml`:

```yaml
name: multi-agent-content-creation
description: Collaborative content creation using specialized AI agents
version: "1.0"

inputs:
  - name: topic
    type: string
    required: true
    description: Topic to research and write about

  - name: target_audience
    type: string
    default: "technical professionals"
    description: Target audience for the content

  - name: word_count
    type: integer
    default: 1000
    description: Target word count for the article

  - name: include_critique
    type: boolean
    default: true
    description: Include critical review step

  - name: tone
    type: string
    default: "professional"
    enum: ["professional", "casual", "academic", "enthusiastic"]
    description: Writing tone

steps:
  - id: research
    name: Research Agent - Gather Information
    type: llm
    model: strategic
    system: |
      You are an expert research analyst.

      Your role:
      - Find accurate, up-to-date information
      - Identify key facts, statistics, and examples
      - Note authoritative sources
      - Organize findings logically
      - Flag knowledge gaps or uncertainties

      Output format:
      # Research Findings: [Topic]

      ## Key Points
      - [Main points as bullet list]

      ## Statistics & Data
      - [Relevant data with context]

      ## Examples
      - [Real-world examples]

      ## Sources
      - [Authoritative sources for verification]

      ## Gaps
      - [Areas needing more research]
    prompt: |
      Research this topic comprehensively: {{.inputs.topic}}

      Target audience: {{.inputs.target_audience}}

      Focus on information that would be valuable, accurate, and relevant
      to this audience. Prioritize recent developments and practical insights.

  - id: draft
    name: Writing Agent - Create Draft
    type: llm
    model: balanced
    system: |
      You are a professional content writer.

      Your role:
      - Create engaging, well-structured content
      - Write clearly for the target audience
      - Use provided research as foundation
      - Maintain specified tone and style
      - Meet word count targets
      - Create compelling introductions and conclusions

      Writing principles:
      - Start strong with a hook
      - Use clear structure with headings
      - Include concrete examples
      - Write in active voice
      - End with clear takeaways
    prompt: |
      Write a {{.inputs.word_count}}-word article about: {{.inputs.topic}}

      **Target audience:** {{.inputs.target_audience}}
      **Tone:** {{.inputs.tone}}

      **Research to incorporate:**
      {{.steps.research.response}}

      Create a complete, polished draft with:
      1. Compelling title
      2. Strong introduction
      3. Well-organized body with headings
      4. Concrete examples and data
      5. Clear conclusion with takeaways

      Output the complete article in Markdown format.

  - id: reviews
    name: Review Agents - Multi-Perspective Critique
    type: parallel
    max_concurrency: 3
    condition:
      expression: 'inputs.include_critique == true'
    steps:
      - id: content_review
        name: Content Quality Review
        type: llm
        model: balanced
        system: |
          You are a content quality expert.

          Evaluate:
          - Accuracy and factual correctness
          - Completeness and depth
          - Clarity and coherence
          - Example quality and relevance
          - Logical flow and structure

          Rate each dimension: EXCELLENT, GOOD, NEEDS_WORK
          Provide specific, actionable feedback.
        prompt: |
          Review this article for content quality:

          {{.steps.draft.response}}

          Provide structured feedback with:
          1. Overall assessment (2-3 sentences)
          2. Strengths (3-5 specific points)
          3. Issues (specific problems with line references if possible)
          4. Suggestions (prioritized improvements)

      - id: audience_review
        name: Audience Fit Review
        type: llm
        model: balanced
        system: |
          You are an audience engagement specialist.

          Evaluate:
          - Appropriate tone and language for audience
          - Accessibility and readability
          - Engagement and interest level
          - Practical value and applicability
          - Call-to-action effectiveness
        prompt: |
          Review this article for audience fit:

          **Target audience:** {{.inputs.target_audience}}
          **Intended tone:** {{.inputs.tone}}

          **Article:**
          {{.steps.draft.response}}

          Assess how well the content serves the target audience.
          Rate: EXCELLENT_FIT, GOOD_FIT, NEEDS_ADJUSTMENT
          Provide specific recommendations.

      - id: technical_review
        name: Technical Accuracy Review
        type: llm
        model: strategic
        system: |
          You are a technical accuracy reviewer.

          Evaluate:
          - Technical accuracy and precision
          - Appropriate complexity level
          - Correct terminology usage
          - Logical soundness of arguments
          - Potential misconceptions or errors
        prompt: |
          Review this article for technical accuracy:

          {{.steps.draft.response}}

          Flag any technical errors, questionable claims, or areas
          needing clarification. Suggest corrections.

  - id: consolidate_feedback
    name: Consolidate Review Feedback
    type: llm
    model: balanced
    condition:
      expression: 'inputs.include_critique == true'
    prompt: |
      Consolidate these three reviews into a prioritized action plan:

      **Content Quality Review:**
      {{.steps.reviews.content_review.response}}

      **Audience Fit Review:**
      {{.steps.reviews.audience_review.response}}

      **Technical Accuracy Review:**
      {{.steps.reviews.technical_review.response}}

      Create a unified revision plan with:
      1. Critical issues (must fix)
      2. Important improvements (should fix)
      3. Optional enhancements (nice to have)

      Output a clear, actionable list for the editor.

  - id: final_edit
    name: Editor Agent - Produce Final Version
    type: llm
    model: balanced
    system: |
      You are a professional editor.

      Your role:
      - Incorporate reviewer feedback
      - Improve clarity, flow, and impact
      - Fix errors and inconsistencies
      - Enhance readability
      - Ensure quality standards
      - Preserve author's voice

      Editing principles:
      - Make substantive improvements, not just polish
      - Fix technical errors
      - Improve weak sections
      - Tighten verbose passages
      - Strengthen conclusions
    prompt: |
      Revise this article based on consolidated feedback:

      **Original draft:**
      {{.steps.draft.response}}

      {{if .steps.consolidate_feedback}}
      **Feedback to address:**
      {{.steps.consolidate_feedback.response}}
      {{else}}
      Perform a general editorial review focusing on clarity,
      accuracy, and engagement for the target audience.
      {{end}}

      **Target audience:** {{.inputs.target_audience}}
      **Tone:** {{.inputs.tone}}
      **Word count target:** {{.inputs.word_count}}

      Produce the final, polished article in Markdown format.

  - id: generate_metadata
    name: Metadata Agent - Create Publishing Info
    type: llm
    model: fast
    system: |
      You are a content metadata specialist.

      Generate:
      - SEO-friendly title variations
      - Meta description (150-160 characters)
      - Keywords (10-15)
      - Social media snippets
      - Category tags
      - Reading time estimate
    prompt: |
      Create metadata for this article:

      {{.steps.final_edit.response}}

      **Topic:** {{.inputs.topic}}
      **Audience:** {{.inputs.target_audience}}

      Output structured metadata in this format:

      # Metadata

      **Primary Title:** [title]
      **Alternative Titles:** [3 variations]
      **Meta Description:** [150-160 chars]
      **Keywords:** [comma-separated]
      **Category:** [main category]
      **Tags:** [5-8 tags]
      **Reading Time:** [estimate]
      **Social Snippet:** [280 chars for social media]

  - id: write_article
    name: Save Final Article
    file.write:
      path: "article-{{.inputs.topic}}.md"
      content: "{{.steps.final_edit.response}}"

  - id: write_metadata
    name: Save Metadata
    file.write:
      path: "metadata-{{.inputs.topic}}.md"
      content: |
        # Article Metadata

        {{.steps.generate_metadata.response}}

        ---

        # Production Notes

        **Topic:** {{.inputs.topic}}
        **Target Audience:** {{.inputs.target_audience}}
        **Word Count Target:** {{.inputs.word_count}}
        **Tone:** {{.inputs.tone}}
        **Critique Enabled:** {{.inputs.include_critique}}

        ---

        # Workflow Summary

        1. Research phase completed
        2. Initial draft created
        {{if .steps.consolidate_feedback}}
        3. Multi-perspective review performed
        4. Feedback consolidated
        5. Final edit incorporating feedback
        {{else}}
        3. Editorial review performed
        {{end}}
        6. Metadata generated

outputs:
  - name: article
    type: string
    value: "{{.steps.final_edit.response}}"
    description: Final polished article

  - name: article_file
    type: string
    value: "article-{{.inputs.topic}}.md"
    description: Path to saved article

  - name: metadata
    type: string
    value: "{{.steps.generate_metadata.response}}"
    description: Article metadata

  - name: research_summary
    type: string
    value: "{{.steps.research.response}}"
    description: Original research findings

  - name: reviews_performed
    type: boolean
    value: "{{.inputs.include_critique}}"
    description: Whether critical review was performed
```

---

## Advanced Multi-Agent Patterns

### Debate and Consensus

Have agents debate and reach consensus:

```yaml
- id: initial_proposals
  type: parallel
  steps:
    - id: approach_a
      type: llm
      prompt: "Propose solution approach A..."
    - id: approach_b
      type: llm
      prompt: "Propose solution approach B..."

- id: debate
  type: llm
  prompt: |
    Compare these approaches:
    Approach A: {{.steps.initial_proposals.approach_a.response}}
    Approach B: {{.steps.initial_proposals.approach_b.response}}

    Identify strengths, weaknesses, and synthesize the best solution.
```

### Chain of Experts

Route work through specialists:

```yaml
- id: classify_query
  type: llm
  prompt: "Classify this question: {{.inputs.query}}"

- id: technical_expert
  condition:
    expression: 'steps.classify_query.response contains "technical"'
  type: llm
  system: "You are a technical expert..."
  prompt: "Answer: {{.inputs.query}}"

- id: business_expert
  condition:
    expression: 'steps.classify_query.response contains "business"'
  type: llm
  system: "You are a business expert..."
  prompt: "Answer: {{.inputs.query}}"
```

### Hierarchical Agents

Manager coordinates workers:

```yaml
- id: manager
  type: llm
  system: "You are a project manager..."
  prompt: |
    Break down this project: {{.inputs.project_spec}}
    Create 3-5 sub-tasks.

- id: workers
  type: parallel
  steps:
    # Dynamically create worker agents based on manager output
    # Each worker handles one sub-task
```

### Iterative Refinement

Agent improves its own work:

```yaml
- id: draft_v1
  type: llm
  prompt: "Create initial draft..."

- id: self_review
  type: llm
  prompt: |
    Review your draft and identify weaknesses:
    {{.steps.draft_v1.response}}

- id: draft_v2
  type: llm
  prompt: |
    Improve based on review:
    Original: {{.steps.draft_v1.response}}
    Review: {{.steps.self_review.response}}
```

---

## Context Management Between Agents

### Full Context Passing

Pass complete previous outputs:

```yaml
- id: agent2
  prompt: |
    Context from agent1:
    {{.steps.agent1.response}}

    Now do your task...
```

**Pros:** Complete information
**Cons:** Large context, expensive tokens

### Summarized Context

Summarize before passing:

```yaml
- id: summarize_research
  type: llm
  prompt: "Summarize key points: {{.steps.research.response}}"

- id: writer
  prompt: |
    Key points to incorporate:
    {{.steps.summarize_research.response}}

    Write article...
```

**Pros:** Reduced tokens, focused information
**Cons:** Might lose important details

### Selective Context

Extract specific information:

```yaml
- id: extract_metrics
  type: llm
  prompt: |
    Extract only statistics and numbers from:
    {{.steps.analysis.response}}

    Output as JSON: {"metrics": [...]}

- id: report
  prompt: |
    Create report using these metrics:
    {{.steps.extract_metrics.response}}
```

**Pros:** Efficient, structured
**Cons:** Requires extraction step

---

## Optimizing Multi-Agent Workflows

### Cost Optimization

**Use appropriate model tiers:**

```yaml
# Strategic for complex reasoning
- id: architecture_design
  model: strategic

# Balanced for most tasks
- id: write_documentation
  model: balanced

# Fast for simple tasks
- id: format_output
  model: fast
```

**Savings:** 10x cost reduction using fast vs strategic for simple tasks

### Speed Optimization

**Parallelize independent agents:**

```yaml
# Sequential: 15 seconds
- id: review1
  type: llm
- id: review2
  type: llm
- id: review3
  type: llm

# Parallel: 5 seconds
- id: reviews
  type: parallel
  steps:
    - id: review1
    - id: review2
    - id: review3
```

**Speedup:** 3x faster with parallel execution

### Quality Optimization

**Add validation agents:**

```yaml
- id: generate
  type: llm
  prompt: "Create solution..."

- id: validate
  type: llm
  prompt: "Validate solution: {{.steps.generate.response}}"

- id: regenerate
  condition:
    expression: 'steps.validate.response contains "INVALID"'
  type: llm
  prompt: |
    Fix issues:
    Original: {{.steps.generate.response}}
    Issues: {{.steps.validate.response}}
```

---

## Troubleshooting

### Agents produce inconsistent outputs

**Problem:** Different agents use different formats

**Solution:** Specify output format in system prompts:

```yaml
system: |
  You are a reviewer.
  ALWAYS output in this format:
  Rating: [EXCELLENT/GOOD/NEEDS_WORK]
  Feedback: [detailed feedback]
  Suggestions: [numbered list]
```

### Context too large for downstream agents

**Problem:** Token limit exceeded

**Solution:** Use summarization or extraction:

```yaml
- id: extract_key_points
  type: llm
  model: fast
  prompt: |
    Extract the 5 most important points from:
    {{.steps.long_analysis.response}}

    Output as bullet list only.
```

### Parallel agents taking too long

**Problem:** Too many concurrent LLM calls

**Solution:** Add `max_concurrency`:

```yaml
- id: reviews
  type: parallel
  max_concurrency: 3  # Limit to 3 concurrent calls
  steps:
    # 10 review agents...
```

### Feedback loop between agents

**Problem:** Agent A depends on B, B depends on A

**Solution:** Restructure workflow to be acyclic:

```yaml
# Bad: circular dependency
- id: agent_a
  prompt: "...{{.steps.agent_b.response}}"
- id: agent_b
  prompt: "...{{.steps.agent_a.response}}"

# Good: linear flow
- id: agent_a
  prompt: "..."
- id: agent_b
  prompt: "...{{.steps.agent_a.response}}"
- id: synthesize
  prompt: |
    Combine:
    A: {{.steps.agent_a.response}}
    B: {{.steps.agent_b.response}}
```

---

## Key Concepts Learned

!!! success "You now understand:"
    - **Multi-agent design** — Breaking complex tasks into specialized agents
    - **Agent roles** — Defining clear responsibilities and personas
    - **Execution patterns** — Sequential, parallel, and hybrid workflows
    - **Context passing** — Managing information flow between agents
    - **Output aggregation** — Consolidating results from multiple agents
    - **Optimization strategies** — Balancing cost, speed, and quality
    - **Real-world applications** — Content creation, analysis, decision-making

---

## What's Next?

### Related Guides

- **[Flow Control](../../guides/flow-control.md)** — Parallel execution and conditionals
- **[Performance](../../guides/performance.md)** — Speed and cost optimization

### Real-World Applications

Apply multi-agent patterns to:

1. **Code Review** — Security, performance, style agents
2. **Data Analysis** — Statistical, visualization, insight agents
3. **Decision Making** — Research, analysis, recommendation agents
4. **Content Moderation** — Safety, quality, compliance agents
5. **Testing** — Unit test, integration test, security test agents

### Production Enhancements

Make this production-ready:

1. **Add retry logic** — Handle transient LLM failures
2. **Implement caching** — Cache research and intermediate results
3. **Add metrics** — Track agent performance and costs
4. **Version control** — Track which agent versions produced content
5. **Quality gates** — Enforce minimum quality thresholds

---

## Additional Resources

- **[Workflow Schema Reference](../../reference/workflow-schema.md)**
- **[Template Variables](../concepts/template-variables.md)**
- **[Error Handling](../../guides/error-handling.md)**
- **[Testing](../../guides/testing.md)**
