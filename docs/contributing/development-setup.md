# Contributing to Conductor

Thank you for your interest in contributing to Conductor! This document provides guidelines and setup instructions for contributors.

:::note[Prerequisites]
Before contributing, make sure you have:

- Go 1.22 or later installed
- Git configured with your name and email
- Familiarity with Go programming and testing
- Understanding of Conductor's [architecture](../architecture/overview.md)

New to Conductor? Start with the [Getting Started Guide](../getting-started/) to understand the user experience before diving into development.
:::


## Development Setup

### Getting Started

1. **Clone the repository:**
```bash
git clone https://github.com/tombee/conductor.git
cd conductor
```

2. **Install dependencies:**
```bash
go mod download
```

3. **Run tests:**
```bash
go test ./...
```

4. **Run linter:**
```bash
golangci-lint run
```

## Code Style

### Go Code Guidelines

- Follow standard Go conventions (see [Effective Go](https://golang.org/doc/effective_go.html))
- Use `gofmt` to format all code
- Write clear, concise GoDoc comments for all exported symbols
- Keep functions focused and testable
- Prefer interfaces over concrete types in public APIs

### Naming Conventions

- **Packages**: Short, lowercase, single-word names (e.g., `workflow`, `llm`, `agent`)
- **Interfaces**: Describe behavior (e.g., `Provider`, `Storage`, `Executor`)
- **Structs**: Nouns describing the entity (e.g., `WorkflowDefinition`, `ModelInfo`)
- **Functions**: Verbs describing the action (e.g., `Execute`, `Register`, `Stream`)

### Comments

Write evergreen comments that explain **what** and **why**, not **when** or **how it changed**:

- Avoid temporal references like "added for feature X" or "updated in v2.0"
- Git history tracks changes - comments should be timeless
- Focus on intent, constraints, and non-obvious decisions

**Example:**

```go
// Good: Provider interface abstracts LLM API clients for swappable implementations
type Provider interface { ... }

// Bad: Provider interface added in v0.1 to support multiple LLM providers
type Provider interface { ... }
```

## Agent-Friendly CLI Development

Conductor's CLI is designed to be fully usable by LLM coding agents like Claude Code, GitHub Copilot, and Cursor. When modifying or adding commands, follow these guidelines to maintain agent discoverability and usability.

For complete design principles and patterns, see [Agent-Friendly CLI Design](../design/agent-friendly-cli.md).

### Help Text Requirements

All commands must have:

1. **3+ usage examples** in the `Example` field showing:
   - Basic usage
   - JSON output for parsing (`--json`)
   - Pipeline integration (e.g., with `jq`)

2. **Short descriptions < 50 characters** for clean command listings

3. **Safe example data**:
   - Use `@example.com` for email addresses
   - Use `192.0.2.x` (TEST-NET-1) for IP addresses
   - Never include real API keys, tokens, or credentials

**Example:**

```go
cmd := &cobra.Command{
    Use:   "validate <workflow>",
    Short: "Validate workflow YAML syntax and schema",
    Example: `  # Example 1: Basic validation
  conductor validate workflow.yaml

  # Example 2: Validate with JSON output for parsing
  conductor validate workflow.yaml --json

  # Example 3: Validate and extract workflow metadata
  conductor validate workflow.yaml --json | jq '.workflow'`,
}
```

### JSON Output

All commands that output data must support `--json` flag:

- Use `shared.EmitJSON()` for consistent envelope structure
- Include `@version`, `command`, `success` fields in envelope
- Return errors as structured JSON with codes and suggestions

```go
if jsonOutput {
    return shared.EmitJSON(data, err)
}
```

### Dry-Run Support

Commands with side effects (create, modify, delete) must support `--dry-run`:

- Output planned actions with CREATE/MODIFY/DELETE prefixes
- Use placeholder paths (`<config-dir>`, `<workflow-dir>`)
- Mask sensitive values with `[REDACTED]`
- Return exit code 0 without executing

```go
if dryRun {
    fmt.Printf("CREATE: <config-dir>/workflow.yaml\n")
    return nil
}
```

### Non-Interactive Mode

Commands must gracefully handle non-interactive contexts (CI/CD, agents):

- Detect via `shared.IsNonInteractive()` (checks CI env vars and TTY)
- Return clear error messages when interactive input is required
- Support `--yes` flag for automatic confirmations
- Exit with code 2 for missing required inputs

### Lint Validation

Before submitting a PR:

```bash
# Run help text linter locally
./scripts/lint-help-text.sh
```

The CI will automatically run this check and block PRs with violations.

## Testing Requirements

All contributions must meet these testing standards:

### Unit Tests

- **Coverage**: 80%+ for `pkg/*` (embeddable packages), 70%+ for `internal/*`
- All exported functions must have tests
- Test both happy paths and error conditions
- Use table-driven tests for multiple scenarios
- Mock external dependencies (LLM APIs, filesystem, network)

**Example:**

```go
func TestProviderRegistry(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(*Registry)
        want    error
    }{
        {"register valid provider", setupValid, nil},
        {"register duplicate provider", setupDuplicate, ErrDuplicate},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### Integration Tests

- Demonstrate features work end-to-end
- Use real components where possible (e.g., SQLite)
- Mock only external services (LLM APIs)
- Place in `*_integration_test.go` files

### Test Organization

- **Unit tests**: Same package, `*_test.go` suffix
- **Integration tests**: Same package, `*_integration_test.go` suffix
- **Test helpers**: `testutil/` package
- **Mock implementations**: `mocks/` subdirectory per package

## Pull Request Process

### 1. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
```

### 2. Make Your Changes

- Write code following the style guidelines
- Add tests for new functionality
- Update documentation as needed
- Run tests and linter locally

### 3. Commit Your Changes

Use clear, descriptive commit messages following conventional commits format:

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `test:` - Test additions or updates
- `refactor:` - Code refactoring
- `chore:` - Maintenance tasks

**Example:**
```bash
git commit -m "feat(llm): add OpenAI provider implementation"
```

### 4. Push to Your Fork

```bash
git push origin feature/your-feature-name
```

### 5. Open a Pull Request

- Use the PR template
- Link to any related issues
- Provide a clear description of changes
- Ensure CI passes

### PR Checklist

Before submitting, verify:

- [ ] Tests cover new/changed code (coverage does not decrease)
- [ ] GoDoc comments on all new exported types/functions
- [ ] README updated if user-facing behavior changes
- [ ] CHANGELOG.md entry added for notable changes
- [ ] All tests pass locally (`go test ./...`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] No project-specific imports in `pkg/*` packages
- [ ] Help text linter passes (`./scripts/lint-help-text.sh`)
- [ ] New/modified commands have 3+ examples and `--json` support
- [ ] Commands with side effects support `--dry-run`

## Documentation Requirements

Every PR must include appropriate documentation:

### GoDoc Comments

All exported symbols (types, functions, constants) must have GoDoc comments:

```go
// Provider abstracts LLM API clients for swappable implementations.
// Implementations handle authentication, request formatting, and response parsing.
type Provider interface {
    // Name returns the unique identifier for this provider (e.g., "anthropic").
    Name() string

    // Complete sends a completion request and returns the full response.
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
}
```

### README Updates

Update READMEs when:
- User-facing behavior changes
- New features are added
- Configuration options change

### Architecture Documentation

Document significant design decisions in:
- `docs/advanced/architecture.md` - High-level architecture changes
- `docs/design/` - Detailed design documents for major features

### Runbooks

Create operational runbooks for new features:
- Installation procedures
- Configuration steps
- Troubleshooting guides

### CHANGELOG

Follow [Keep a Changelog](https://keepachangelog.com/) format:

```markdown
## [Unreleased]

### Added
- OpenAI provider implementation with streaming support

### Changed
- Provider interface now requires Capabilities() method

### Fixed
- Token counting accuracy for multi-message conversations
```

## Code Review

All contributions require code review before merging:

- Maintainers will review for code quality, test coverage, and documentation
- Address review feedback promptly
- Be open to suggestions and constructive criticism
- Once approved, a maintainer will merge your PR

## Package Design Principles

### Embeddable Packages (`pkg/*`)

Packages in `pkg/` are designed for embedding in external projects:

- **No project-specific dependencies**: Never import project-specific code
- **Interface-driven**: Expose interfaces, not concrete types
- **Stable APIs**: Breaking changes require major version bump
- **Well-documented**: Every exported symbol has GoDoc comments
- **Fully tested**: 80%+ coverage required

**Example:**

```go
// Good: Generic interface
type Storage interface {
    Save(ctx context.Context, data []byte) error
}

// Bad: Project-specific type
type ForemanStorage struct {
    client *ForemanClient
}
```

### Internal Packages (`internal/*`)

Packages in `internal/` are project-specific implementation details:

- May import project-specific code
- Not available to external consumers
- Can have breaking changes without version bump
- 70%+ coverage required

## Project Structure

Understanding the codebase structure:

```
conductor/
├── cmd/
│   ├── conductor/       # CLI client entry point
│   └── conductor/      # Daemon binary entry point
├── pkg/                 # Embeddable packages (stable API)
│   ├── llm/            # Provider abstraction
│   ├── workflow/       # Workflow orchestration
│   ├── agent/          # Agent execution loop
│   └── tools/          # Tool registry and execution
├── internal/           # Private implementation
│   └── [internal packages]
├── docs/               # Documentation
│   ├── learn/
│   ├── guides/
│   ├── connectors/
│   └── reference/
├── examples/           # Example workflows
└── testutil/          # Shared test utilities
```

## Common Development Tasks

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./pkg/llm/...

# Verbose output
go test -v ./...
```

### Running Linter

```bash
# Run all linters
golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

### Building Binaries

```bash
# Build CLI
go build -o bin/conductor ./cmd/conductor

# Build daemon
go build -o bin/conductor ./cmd/conductor
```

### Running Examples

```bash
# Run a workflow
./bin/conductor run examples/code-review/workflow.yaml --input pr_url=...
```

## Getting Help

- **Questions**: Open a [GitHub Discussion](https://github.com/tombee/conductor/discussions)
- **Bug Reports**: Open a [GitHub Issue](https://github.com/tombee/conductor/issues) with reproduction steps
- **Feature Requests**: Open a GitHub Issue describing the use case
- **Security Issues**: Email security@tombee.com (do not open public issues)

## Development Best Practices

### 1. Start Small

Begin with small, focused changes:
- Fix a typo in documentation
- Add a missing test
- Improve error messages

### 2. Read Existing Code

Before implementing new features:
- Read similar existing implementations
- Follow established patterns
- Ask questions if patterns aren't clear

### 3. Test-Driven Development

Write tests first when possible:
1. Write a failing test
2. Implement the minimum code to pass
3. Refactor while keeping tests green

### 4. Document as You Go

Don't leave documentation for later:
- Write GoDoc comments as you code
- Update README when behavior changes
- Create examples for new features

### 5. Keep PRs Focused

Each PR should:
- Address one concern
- Be reviewable in a single sitting
- Have a clear purpose

Break large features into multiple PRs.

## Release Process

(For maintainers)

1. Update CHANGELOG.md with version and date
2. Create version tag: `git tag v0.1.0`
3. Push tag: `git push origin v0.1.0`
4. GitHub Actions will build and publish release artifacts

## Code of Conduct

We are committed to providing a welcoming and inclusive environment:

- Be respectful and professional
- Welcome newcomers and help them learn
- Focus on constructive feedback
- Assume positive intent

## License

By contributing to Conductor, you agree that your contributions will be licensed under the Apache 2.0 License.

## Next Steps

Ready to contribute? Here's what to do next:

1. **Find an Issue**: Look for issues labeled `good-first-issue` or `help-wanted`
2. **Set Up Dev Environment**: Follow the setup steps above
3. **Make Your Changes**: Create a branch and implement your changes
4. **Submit a PR**: Open a pull request with your changes

Questions? Open a GitHub Discussion or reach out to the maintainers.

---

Thank you for contributing to Conductor!

*Last updated: 2025-12-23*
