# Model Tiers

Model tiers let you write provider-agnostic workflows. Instead of hardcoding model names like `claude-3-opus` or `gpt-4-turbo`, use semantic tiers that describe what you need.

## Why Use Tiers?

1. **Portability** — Switch providers without changing workflows
2. **Cost control** — Match model capability to task complexity
3. **Future-proof** — As providers update models, your workflows keep working

```conductor
# Provider-agnostic workflow
- id: analyze
  type: llm
  model: balanced  # Works with Anthropic, OpenAI, Google, Ollama
  prompt: "Analyze this code..."
```

## The Three Tiers

### `fast`

Low-latency, cost-effective models for simple tasks.

| Use Cases | Characteristics |
|-----------|-----------------|
| Text classification | Latency: <500ms typical |
| Simple extraction | Cost: Lowest per token |
| Formatting/summarization | Context: Limited (varies by provider) |
| High-volume automation | Capability: Basic reasoning |

```conductor
- id: classify
  type: llm
  model: fast
  prompt: "Classify this support ticket as bug, feature, or question: {{.inputs.ticket}}"
```

### `balanced`

General-purpose models for most workflows. **This is the default if you omit `model`.**

| Use Cases | Characteristics |
|-----------|-----------------|
| Code review | Latency: <2s typical |
| Content generation | Cost: Moderate |
| Analysis and synthesis | Context: Large (varies by provider) |
| Multi-step reasoning | Capability: Strong reasoning |

```conductor
- id: review
  type: llm
  model: balanced
  prompt: |
    Review this code for bugs, security issues, and style:
    {{.steps.get_diff.output}}
```

### `strategic`

Most capable models for complex reasoning tasks.

| Use Cases | Characteristics |
|-----------|-----------------|
| Complex analysis | Latency: <10s typical |
| Research synthesis | Cost: Highest per token |
| Creative problem-solving | Context: Largest available |
| Multi-domain expertise | Capability: Best available reasoning |

```conductor
- id: architect
  type: llm
  model: strategic
  prompt: |
    Design a microservices architecture for this system:
    {{.inputs.requirements}}

    Consider: scalability, security, cost, team structure
```

## Provider Mappings

Each provider maps tiers to their best available models:

| Tier | Anthropic | OpenAI | Google | Ollama |
|------|-----------|--------|--------|--------|
| `fast` | Claude 3 Haiku | GPT-3.5 Turbo | Gemini Flash | Llama 3 8B |
| `balanced` | Claude 3.5 Sonnet | GPT-4 Turbo | Gemini Pro | Llama 3 70B |
| `strategic` | Claude Opus 4.5 | GPT-4 | Gemini Ultra | Mixtral 8x22B |

*Last updated: January 2025*

!!! note "Mappings may become outdated"
    Provider models change frequently. If mappings seem incorrect, please [open an issue](https://github.com/tombee/conductor/issues) to help keep them current.

## Choosing the Right Tier

### Decision Framework

```
Is the task simple pattern matching?
├── Yes → fast
└── No → Does it require complex reasoning?
    ├── Yes → Does it involve novel problem-solving or synthesis?
    │   ├── Yes → strategic
    │   └── No → balanced
    └── No → balanced
```

### Task Examples

| Task | Recommended Tier | Reasoning |
|------|------------------|-----------|
| Classify support tickets | `fast` | Pattern recognition, short output |
| Generate commit messages | `fast` | Formulaic output, limited context |
| Code review | `balanced` | Multi-step reasoning, domain knowledge |
| Summarize long documents | `balanced` | Large context, synthesis |
| Explain complex code | `balanced` | Technical understanding |
| Architecture design | `strategic` | Novel problem-solving, trade-offs |
| Research synthesis | `strategic` | Multi-domain reasoning |
| Creative writing | `strategic` | Creativity, nuance |

### Cost vs. Capability

Higher tiers cost more per token but often use fewer tokens to solve problems:

- A `fast` model might need multiple attempts or longer prompts
- A `strategic` model might solve in one pass with better results

**Tip:** Start with `balanced` for new workflows. Downgrade to `fast` if results are good, upgrade to `strategic` if results are insufficient.

## Examples

### Multi-Tier Workflow

Use different tiers for different steps based on task complexity:

```conductor
name: code-review-pipeline
steps:
  # Fast: Simple extraction
  - id: get_files
    shell.run:
      command: ["git", "diff", "--name-only"]

  # Balanced: Code understanding
  - id: review
    type: llm
    model: balanced
    prompt: "Review this code for issues: {{.steps.get_diff.output}}"

  # Fast: Formatting
  - id: format
    type: llm
    model: fast
    prompt: "Format this review as markdown: {{.steps.review.response}}"
```

### Parallel Analysis with Mixed Tiers

```conductor
name: comprehensive-analysis
steps:
  - id: analyses
    type: parallel
    steps:
      # Fast for simple checks
      - id: lint_check
        type: llm
        model: fast
        prompt: "List any obvious syntax issues..."

      # Balanced for security analysis
      - id: security
        type: llm
        model: balanced
        prompt: "Identify security vulnerabilities..."

      # Strategic for architecture review
      - id: architecture
        type: llm
        model: strategic
        prompt: "Evaluate the architectural design..."
```

## Overriding Tiers

You can specify exact models when needed:

```conductor
# Use specific model instead of tier
- id: custom
  type: llm
  model: claude-3-5-sonnet-20241022
  prompt: "..."
```

**When to override:**

- Testing specific model behavior
- Requirements mandate a particular model
- Fine-tuned models (OpenAI)

## See Also

- [LLM Providers](../architecture/llm-providers.md) — Provider configuration
- [Cost Tracking](../production/cost-tracking.md) — Monitor token usage and costs
- [Configuration](../reference/configuration.md) — Provider setup
