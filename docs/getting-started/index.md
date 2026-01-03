# Installation

## Install Conductor

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

Verify installation:
```bash
conductor --version
```

## Configure LLM Provider

### Option 1: Claude Code

If [Claude Code](https://claude.ai/download) is installed, Conductor uses it automatically:

```bash
claude --version
```

### Option 2: API Key

```bash
conductor provider add
```

## Verify Setup

```bash
cat > hello.yaml << 'EOF'
name: hello-world
steps:
  - id: greet
    type: llm
    prompt: Say hello
EOF

conductor run hello.yaml
```

## Next Steps

- [Hello World](hello-world) - Verify installation
- [Tutorial](../tutorial/) - Build a complete workflow
- [Examples](../examples/) - Production workflows
