# Installation

Get Conductor running in under a minute.

## Install Conductor

Choose your preferred method:

**Homebrew (macOS/Linux)**
```bash
brew install conductor
```

**Go Install**
```bash
go install github.com/tombee/conductor/cmd/conductor@latest
```

**From Source**
```bash
git clone https://github.com/tombee/conductor
cd conductor
make install
```

**Verify installation:**
```bash
conductor --version
```

## Configure an LLM Provider

Conductor needs an LLM provider to run AI workflows.

### Option 1: Claude Code (Recommended)

If you have [Claude Code](https://claude.ai/download) installed, Conductor works automatically with no configuration:

```bash
# Verify Claude Code is available
claude --version
```

### Option 2: API Keys

Add a provider with your API key:

```bash
conductor provider add
```

This interactive command walks you through provider setup.

## Verify Your Setup

Run the [Hello World](/conductor/getting-started/hello-world) example to confirm everything works:

```bash
# Create hello.yaml
cat > hello.yaml << 'EOF'
name: hello-world
steps:
  - id: greet
    type: llm
    prompt: Say hello in a creative way
EOF

# Run it
conductor run hello.yaml
```

If you see AI-generated output, you're ready to go!

## Next Steps

- [Hello World](/conductor/getting-started/hello-world) - Verify your setup (30 seconds)
- [Tutorial](/conductor/tutorial) - Learn Conductor by building a real workflow (90 minutes)
- [Examples](/conductor/examples) - Copy-paste ready workflows
