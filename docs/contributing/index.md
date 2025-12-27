# Contributing

Extend Conductor and contribute to the project.

---

## Overview

This section covers how to extend Conductor's capabilities and contribute to the project:

- **Development setup** for contributors
- **Building custom tools** and connectors
- **Embedding Conductor** in your applications
- **Architecture** and design decisions

---

## For Contributors

### [Development Setup](development-setup.md)
Set up your local development environment to contribute to Conductor.

Learn how to:
- Build from source
- Run tests
- Submit pull requests
- Follow code conventions

---

## Extending Conductor

### [Custom Tools](custom-tools.md)
Build custom tools and connectors to extend Conductor's capabilities.

Topics covered:
- Creating custom step types
- Implementing connectors for new services
- Tool API and interfaces
- Best practices for tool development

### [Embedding Conductor](embedding.md)
Integrate Conductor into your Go applications.

Use cases:
- Embed workflow execution in your app
- Build custom CLI tools
- Create workflow orchestrators
- Extend Conductor's core functionality

---

## Architecture

### [System Design](../architecture/)
Understand Conductor's architecture and design decisions.

Key topics:
- Component architecture
- Execution flow
- LLM provider abstraction
- Connector framework

---

## Getting Started

### Contributing Code

1. **Fork the repository:** [github.com/tombee/conductor](https://github.com/tombee/conductor)
2. **Set up development:** Follow the [development setup guide](development-setup.md)
3. **Find an issue:** Check [good first issues](https://github.com/tombee/conductor/labels/good%20first%20issue)
4. **Submit a PR:** Follow the contribution guidelines

### Building Extensions

1. **Review architecture:** Understand the [component model](../architecture/)
2. **Follow examples:** Study [existing connectors](../reference/connectors/)
3. **Implement your tool:** Use the [custom tools guide](custom-tools.md)
4. **Test thoroughly:** Write tests for your extension

### Embedding in Applications

1. **Review embedding guide:** See [embedding documentation](embedding.md)
2. **Check examples:** Study integration patterns
3. **Import the library:** Use Conductor as a Go module
4. **Build your integration:** Leverage Conductor's APIs

---

## Community

- **GitHub Discussions:** Ask questions and share ideas
- **Issue Tracker:** Report bugs and request features
- **Pull Requests:** Contribute code improvements
- **Documentation:** Help improve the docs

---

## Code of Conduct

We're committed to providing a welcoming and inclusive environment. Please read and follow our Code of Conduct when participating in the project.

---

## Additional Resources

- **[GitHub Repository](https://github.com/tombee/conductor)** — Source code
- **[API Reference](../reference/api.md)** — API documentation
- **[CLI Reference](../reference/cli.md)** — Command-line interface
