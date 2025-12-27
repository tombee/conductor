// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package connector provides runtime execution for declarative connectors.

Connectors are deterministic, schema-validated operations that execute without
LLM involvement. They provide HTTP-based integrations with external services,
including authentication, rate limiting, response transforms, and retry logic.

# Overview

Connectors differ from LLM tools:

  - Deterministic: Same inputs always produce same outputs
  - Schema-validated: Inputs and outputs are validated against schemas
  - No LLM involvement: Operations execute directly without model calls
  - Built-in security: SSRF protection, host allowlisting, rate limiting

# Built-in Connectors

The package includes connectors for common services:

  - GitHub: Issues, PRs, repos, actions, releases
  - Slack: Messages, channels, users, files
  - Jira: Issues, comments, search, projects
  - Discord: Messages, threads, embeds, webhooks
  - Jenkins: Builds, jobs, queue, nodes

# Usage

Execute a connector operation:

	registry := connector.NewBuiltinRegistry()
	conn, _ := registry.Get("github")

	result, err := conn.Execute(ctx, "create_issue", map[string]any{
	    "owner":  "myorg",
	    "repo":   "myrepo",
	    "title":  "Bug report",
	    "body":   "Description here",
	})

# Key Components

  - Connector: Interface for operation execution
  - Executor: Runs operations with rate limiting and retries
  - Registry: Stores and retrieves connectors
  - Config: Runtime configuration (timeouts, security, metrics)

# Security Features

SSRF protection is built-in:

  - Private IP range blocking (RFC1918, loopback, link-local)
  - Cloud metadata endpoint blocking (169.254.169.254, etc.)
  - Host allowlisting for production environments
  - URL validation and scheme enforcement

# Rate Limiting

Rate limits are enforced per-connector with state persistence:

  - Token bucket algorithm
  - Configurable limits per operation
  - State persisted to disk for daemon restarts

# Response Transforms

Operations can transform responses using jq expressions:

	transform: ".data.items | map({id, name})"

# Metrics

When enabled, Prometheus metrics are exported:

  - connector_operations_total{connector,operation,status}
  - connector_operation_duration_seconds{connector,operation}

# Subpackages

  - file: File system operations connector
  - shell: Shell command execution connector
  - transform: Data transformation utilities
  - transport: HTTP transport with security features
  - github, slack, jira, discord, jenkins: Service connectors
*/
package connector
