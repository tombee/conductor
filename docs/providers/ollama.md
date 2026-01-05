# Ollama

Ollama runs open-source models locally. This is ideal for privacy-sensitive workflows or when you want to avoid API costs.

## Prerequisites

1. Install Ollama from [ollama.com](https://ollama.com)
2. Start the Ollama service:
   ```bash
   ollama serve
   ```

## Setup

```bash
conductor provider add ollama
```

By default, Conductor connects to `http://localhost:11434`. To use a different address:

```bash
conductor provider add ollama --base-url http://your-server:11434
```

## Recommended Models

Choose models based on your available resources. Conductor uses three [model tiers](../features/model-tiers.md): **fast**, **balanced**, and **strategic**.

=== "Entry (8GB VRAM)"

    | Tier | Model |
    |------|-------|
    | fast | `qwen3:4b` |
    | balanced | `qwen3:8b` |
    | strategic | `qwen3:8b` |

    ```bash
    ollama pull qwen3:4b qwen3:8b
    ```

    At this level, balanced and strategic share a model. Good for simple tasks but limited complex reasoning.

=== "Mid-Range (16GB VRAM)"

    | Tier | Model |
    |------|-------|
    | fast | `qwen3:8b` |
    | balanced | `qwen3:32b` |
    | strategic | `deepseek-r1:32b` |

    ```bash
    ollama pull qwen3:8b qwen3:32b deepseek-r1:32b
    ```

    The 32B models provide strong reasoning. May run slower but significantly more capable.

=== "High-End (24GB+ VRAM)"

    | Tier | Model |
    |------|-------|
    | fast | `qwen3:8b` |
    | balanced | `qwen3:32b` |
    | strategic | `deepseek-r1:70b` |

    ```bash
    ollama pull qwen3:8b qwen3:32b deepseek-r1:70b
    ```

    The 70B model delivers reasoning comparable to top commercial models.

=== "Server (48GB+ VRAM)"

    | Tier | Model |
    |------|-------|
    | fast | `qwen3:8b` |
    | balanced | `qwen3:32b` |
    | strategic | `deepseek-r1:70b` |

    ```bash
    ollama pull qwen3:8b qwen3:32b deepseek-r1:70b
    ```

    With sufficient VRAM, the 70B model runs at interactive speeds. Consider `qwen3:235b` if you have multiple GPUs.

## Configure Model Tiers

After pulling models, assign them to tiers:

```bash
conductor model discover ollama
```

Follow the interactive prompts to map your models to fast, balanced, and strategic tiers.

## Verify

```bash
conductor provider test ollama
```

## Set as Default

To make Ollama your default provider:

```bash
conductor provider add ollama --default
```

Or set via environment variable:

```bash
export LLM_DEFAULT_PROVIDER=ollama
```

## Performance Tips

- **Keep models loaded**: Set `OLLAMA_KEEP_ALIVE=3600` to keep models in memory for an hour
- **GPU layers**: Ollama automatically optimizes GPU/CPU split based on available VRAM
- **Quantization**: Models ending in `-q4` use 4-bit quantization for lower memory usage with minimal quality loss

## Next Steps

- Learn about [model tiers](../features/model-tiers.md) and when to use each
- Continue to the [tutorial](../tutorial/index.md) to build your first workflow
