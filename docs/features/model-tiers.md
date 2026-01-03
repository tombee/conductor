# Model Tiers

Model tiers provide semantic model selection for portable workflows.

## Overview

Instead of hardcoding model names, use tiers that describe capability needs:

```yaml
steps:
  - id: classify
    type: llm
    model: fast
    prompt: Classify this as bug, feature, or question

  - id: review
    type: llm
    prompt: Review this code for issues

  - id: architect
    type: llm
    model: strategic
    prompt: Design a scalable architecture
```

## The Three Tiers

### Fast

Low-latency, cost-effective models for simple tasks.

**Use for:**
- Text classification
- Simple extraction
- Formatting and summarization
- High-volume automation

**Characteristics:**
- Latency: <500ms typical
- Cost: Lowest per token
- Capability: Basic reasoning

### Balanced

General-purpose models for most workflows. This is the default if you omit `model`.

**Use for:**
- Code review
- Content generation
- Analysis and synthesis
- Multi-step reasoning

**Characteristics:**
- Latency: <2s typical
- Cost: Moderate
- Capability: Strong reasoning

### Strategic

Most capable models for complex reasoning.

**Use for:**
- Complex analysis
- Research synthesis
- Creative problem-solving
- Multi-domain expertise

**Characteristics:**
- Latency: <10s typical
- Cost: Highest per token
- Capability: Best available reasoning

## Provider Mappings

| Tier | Anthropic | OpenAI | Google |
|------|-----------|--------|--------|
| fast | Claude 3 Haiku | GPT-3.5 Turbo | Gemini Flash |
| balanced | Claude 3.5 Sonnet | GPT-4 Turbo | Gemini Pro |
| strategic | Claude Opus 4.5 | GPT-4 | Gemini Ultra |

## Choosing a Tier

```
Is the task simple pattern matching?
├── Yes → fast
└── No → Does it require complex reasoning?
    ├── Yes → Does it involve novel problem-solving?
    │   ├── Yes → strategic
    │   └── No → balanced
    └── No → balanced
```

**Tip:** Start with `balanced` (the default). Downgrade to `fast` if results are good, upgrade to `strategic` if needed.

## Multi-Tier Workflows

Use different tiers for different steps:

```yaml
steps:
  - id: review
    type: llm
    prompt: Review this code for issues

  - id: format
    type: llm
    model: fast
    prompt: "Format this review as markdown: {{.steps.review.response}}"
```
