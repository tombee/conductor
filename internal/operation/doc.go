// Package operation provides the shared framework for workflow operations.
//
// This package implements the core executor for all types of operations:
//   - Actions: Built-in local operations (file, shell, http, transform, utility)
//   - Integrations: External service API connections (GitHub, Slack, Jira, etc.)
//
// The operation framework handles:
//   - Execution lifecycle and error handling
//   - Registry for discovering and instantiating operations
//   - HTTP transport with auth, rate limiting, retries
//   - Security policies (SSRF protection, path injection prevention)
//   - Metrics and observability
//
// Architecture:
//
// All operations implement the Executor interface which provides Execute() method.
// The Registry manages registration and lookup of operation types. Each operation
// is instantiated with a config map and executed with input data.
//
// Operations are divided into two categories:
//   - Actions (internal/action): Local file system, shell, HTTP, data transformations
//   - Integrations (internal/integration): External API clients for third-party services
//
// Both share this common framework for consistency and code reuse.
package operation
