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

// Package workspace implements workspace-based configuration boundaries for managing integrations.
//
// Workspaces provide a configuration layer that separates integration credentials from
// workflow definitions, enabling portable workflows that declare what they need (requires:)
// rather than how to access it.
//
// Key concepts:
//   - Workspace: Configuration boundary containing named integrations
//   - Integration: Named connection to external service (type + config + credentials)
//   - Runtime Binding: Maps workflow requirements to workspace integrations at execution time
package workspace

import (
	"time"
)

// Workspace represents a configuration boundary containing integrations.
//
// The "default" workspace is automatically created and used when no workspace
// is explicitly specified. This provides a transparent experience for solo developers
// while supporting multi-workspace scenarios for teams.
type Workspace struct {
	// Name is the unique workspace identifier
	// "default" is reserved for the implicit workspace
	Name string `json:"name"`

	// Description provides human-readable context
	Description string `json:"description"`

	// CreatedAt is when the workspace was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the workspace was last modified
	UpdatedAt time.Time `json:"updated_at"`
}

// Integration represents a named connection to an external service.
//
// Integrations are scoped to a workspace and identified by name within that workspace.
// The same integration name can exist in different workspaces with different configurations.
type Integration struct {
	// ID is the unique identifier (UUID for PostgreSQL, generated string for SQLite)
	ID string `json:"id"`

	// WorkspaceName identifies the workspace this integration belongs to
	WorkspaceName string `json:"workspace_name"`

	// Name is the user-defined identifier within the workspace
	// Defaults to Type if not specified
	Name string `json:"name"`

	// Type is the integration type (github, slack, jira, etc.)
	// This determines which integration implementation to use
	Type string `json:"type"`

	// BaseURL is the API base URL
	// Has sensible defaults per type (e.g., https://api.github.com for github)
	BaseURL string `json:"base_url,omitempty"`

	// Auth contains authentication configuration
	Auth AuthConfig `json:"auth"`

	// Headers are additional HTTP headers to include in all requests
	Headers map[string]string `json:"headers,omitempty"`

	// TimeoutSeconds is the request timeout in seconds
	// Defaults to 30 seconds if not specified
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// CreatedAt is when the integration was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the integration was last modified
	UpdatedAt time.Time `json:"updated_at"`
}

// AuthConfig contains authentication configuration for an integration.
//
// Authentication is configured through one of several methods:
//   - Token: Bearer token authentication (most common for APIs)
//   - Basic: Username/password authentication
//   - APIKey: Custom header-based authentication
//   - None: No authentication
//
// Secret values (Token, Password, APIKeyValue) can be:
//   - Direct values (encrypted at rest in database)
//   - Secret references (${VAR}, env:VAR, file:/path, keychain:name)
//
// Secret references are stored as-is and resolved at runtime when workflows execute.
type AuthConfig struct {
	// Type specifies the authentication method
	Type AuthType `json:"type"`

	// Token is the bearer token (for Token auth type)
	// Can be a secret reference: ${GITHUB_TOKEN}, env:TOKEN, file:/path, keychain:name
	Token string `json:"token,omitempty"`

	// Username is the username (for Basic auth type)
	Username string `json:"username,omitempty"`

	// Password is the password (for Basic auth type)
	// Can be a secret reference
	Password string `json:"password,omitempty"`

	// APIKeyHeader is the header name (for APIKey auth type)
	// Example: "X-API-Key", "Authorization"
	APIKeyHeader string `json:"api_key_header,omitempty"`

	// APIKeyValue is the header value (for APIKey auth type)
	// Can be a secret reference
	APIKeyValue string `json:"api_key_value,omitempty"`
}

// AuthType specifies the authentication method for an integration.
type AuthType string

const (
	// AuthTypeNone indicates no authentication
	AuthTypeNone AuthType = "none"

	// AuthTypeToken indicates bearer token authentication
	// Token is sent as: Authorization: Bearer <token>
	AuthTypeToken AuthType = "token"

	// AuthTypeBasic indicates HTTP basic authentication
	// Username and password are sent as: Authorization: Basic <base64(username:password)>
	AuthTypeBasic AuthType = "basic"

	// AuthTypeAPIKey indicates custom header-based authentication
	// Custom header is sent as: <header>: <value>
	AuthTypeAPIKey AuthType = "api-key"
)

// IntegrationStatus represents the current status of an integration.
// This is determined by the last connectivity test.
type IntegrationStatus string

const (
	// StatusUnknown indicates the integration hasn't been tested
	StatusUnknown IntegrationStatus = "unknown"

	// StatusConnected indicates the integration successfully connected
	StatusConnected IntegrationStatus = "connected"

	// StatusAuthFailed indicates authentication failed
	StatusAuthFailed IntegrationStatus = "auth_failed"

	// StatusNetworkError indicates a network connectivity issue
	StatusNetworkError IntegrationStatus = "network_error"

	// StatusRateLimited indicates the integration hit rate limits
	StatusRateLimited IntegrationStatus = "rate_limited"

	// StatusInvalidConfig indicates the configuration is invalid
	StatusInvalidConfig IntegrationStatus = "invalid_config"
)

// DefaultBaseURL returns the default base URL for a given integration type.
// Returns empty string if no default is defined for the type.
func DefaultBaseURL(integrationType string) string {
	defaults := map[string]string{
		"github":    "https://api.github.com",
		"slack":     "https://slack.com/api",
		"jira":      "", // Requires custom instance URL
		"discord":   "https://discord.com/api/v10",
		"pagerduty": "https://api.pagerduty.com",
	}
	return defaults[integrationType]
}

// DefaultTimeout returns the default timeout in seconds for API requests.
const DefaultTimeout = 30
