# Contributing to Conduct

Thank you for your interest in contributing to Conduct! This document provides guidelines and setup instructions for contributors.

## Development Setup

### Prerequisites

- Go 1.22 or later
- Git

### Getting Started

1. Clone the repository:
```bash
git clone https://github.com/tombee/conductor.git
cd conduct
```

2. Install dependencies:
```bash
go mod download
```

3. Run tests:
```bash
go test ./...
```

4. Run linter:
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

- Write evergreen comments that explain **what** and **why**, not **when** or **how it changed**
- Avoid temporal references like "added for feature X" or "updated in v2.0"
- Git history tracks changes - comments should be timeless
- Example:
  ```go
  // Good: Provider interface abstracts LLM API clients for swappable implementations
  type Provider interface { ... }

  // Bad: Provider interface added in v0.1 to support multiple LLM providers
  type Provider interface { ... }
  ```

## Testing Requirements

All contributions must meet these testing standards:

### Unit Tests

- **Coverage**: 80%+ for `pkg/*` (embeddable packages), 70%+ for `internal/*`
- All exported functions must have tests
- Test both happy paths and error conditions
- Use table-driven tests for multiple scenarios
- Mock external dependencies (LLM APIs, filesystem, network)

Example:
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

- Unit tests: Same package, `*_test.go` suffix
- Integration tests: Same package, `*_integration_test.go` suffix
- Test helpers: `testutil/` package
- Mock implementations: `mocks/` subdirectory per package

## Pull Request Process

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**:
   - Write code following the style guidelines
   - Add tests for new functionality
   - Update documentation as needed
   - Run tests and linter locally

3. **Commit your changes**:
   - Use clear, descriptive commit messages
   - Follow conventional commits format: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`
   - Example: `feat(llm): add OpenAI provider implementation`

4. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```

5. **Open a Pull Request**:
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
- [ ] No foreman-specific imports in `pkg/*` packages

## Documentation Requirements

Every PR must include appropriate documentation:

- **GoDoc comments**: All exported symbols (types, functions, constants)
- **README updates**: Changes to user-facing behavior or APIs
- **Architecture docs**: Significant design decisions
- **Runbooks**: Operational procedures for new features
- **CHANGELOG**: User-facing changes following [Keep a Changelog](https://keepachangelog.com/)

## Code Review

All contributions require code review before merging:

- Maintainers will review for code quality, test coverage, and documentation
- Address review feedback promptly
- Be open to suggestions and constructive criticism
- Once approved, a maintainer will merge your PR

## Package Design Principles

### Embeddable Packages (`pkg/*`)

Packages in `pkg/` are designed for embedding in external projects:

- **No foreman dependencies**: Never import foreman-specific code
- **Interface-driven**: Expose interfaces, not concrete types
- **Stable APIs**: Breaking changes require major version bump
- **Well-documented**: Every exported symbol has GoDoc comments
- **Fully tested**: 80%+ coverage required

### Internal Packages (`internal/*`)

Packages in `internal/` are foreman-specific implementation details:

- May import foreman-specific code
- Not available to external consumers
- Can have breaking changes without version bump
- 70%+ coverage required

## Getting Help

- **Questions**: Open a GitHub Discussion
- **Bug Reports**: Open a GitHub Issue with reproduction steps
- **Feature Requests**: Open a GitHub Issue describing the use case
- **Security Issues**: Email security@tombee.com (do not open public issues)

## License

By contributing to Conduct, you agree that your contributions will be licensed under the Apache 2.0 License.
