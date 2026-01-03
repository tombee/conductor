# Hello World

Verify your Conductor installation works in under 30 seconds.

## Create the Workflow

Create a file called `hello.yaml`:

```conductor
name: hello-world
steps:
  - id: greet
    type: llm
    prompt: Say hello in a creative way
```

## Run It

```bash
conductor run hello.yaml
```

You should see output like:

```
Running: hello-world
[1/1] greet... OK

Greetings, fellow traveler of the digital realm! May your code compile
on the first try and your coffee always be the perfect temperature.
```

## Success!

If you see AI-generated output, your setup is complete:
- Conductor is installed correctly
- Your LLM provider is configured and working
- You're ready to build real workflows

## Troubleshooting

**"No provider configured"**

Install [Claude Code](https://claude.ai/download) for zero-config setup, or configure a provider:
```bash
conductor provider add
```

**"Command not found"**

Add Go binaries to your PATH:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

## Next Steps

Ready to build something real? Start the [Tutorial](../tutorial/) to create a complete workflow from scratch.
