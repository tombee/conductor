# Hello World

Verify Conductor installation.

## Workflow

Create `hello.yaml`:

```conductor
name: hello-world
steps:
  - id: greet
    type: llm
    prompt: Say hello in a creative way
```

## Execution

```bash
conductor run hello.yaml
```

Expected output:
```
Running: hello-world
[1/1] greet... OK

Greetings, fellow traveler of the digital realm...
```

## Troubleshooting

**No provider configured**

Install [Claude Code](https://claude.ai/download) or configure a provider:
```bash
conductor provider add
```

**Command not found**

Add Go binaries to PATH:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

## Next Steps

Continue with the [Tutorial](../tutorial/).
