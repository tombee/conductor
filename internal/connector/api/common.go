// Package api provides common types and utilities for API connectors.
package api

import (
	"github.com/tombee/conductor/internal/connector/transport"
)

// ConnectorConfig holds configuration for API connectors.
type ConnectorConfig struct {
	// Transport is the HTTP transport for making requests
	Transport transport.Transport

	// BaseURL is the API base URL (required for Jira and Jenkins)
	BaseURL string

	// Token is the authentication token (API key, OAuth token, etc.)
	Token string

	// AdditionalAuth holds connector-specific auth data (e.g., Jenkins username/password)
	AdditionalAuth map[string]string
}

// OperationInfo provides metadata about a connector operation.
type OperationInfo struct {
	// Name is the operation identifier (e.g., "create_issue")
	Name string

	// Description is a human-readable description
	Description string

	// Category groups related operations (e.g., "issues", "pulls", "repos")
	Category string

	// Tags classify operations (e.g., "write", "paginated", "destructive")
	Tags []string
}

// OperationSchema describes an operation's inputs and outputs.
type OperationSchema struct {
	// Description is a human-readable description
	Description string

	// Parameters describes the operation inputs
	Parameters []ParameterInfo

	// ResponseFields describes the response structure
	ResponseFields []ResponseFieldInfo
}

// ParameterInfo describes an operation parameter.
type ParameterInfo struct {
	// Name is the parameter identifier
	Name string

	// Type is the parameter type (string, integer, boolean, array, object)
	Type string

	// Description is a human-readable description
	Description string

	// Required indicates if the parameter is required
	Required bool

	// Default is the default value (nil if no default)
	Default interface{}
}

// ResponseFieldInfo describes a response field.
type ResponseFieldInfo struct {
	// Name is the field identifier
	Name string

	// Type is the field type (string, integer, boolean, array, object)
	Type string

	// Description is a human-readable description
	Description string
}

// TypedConnector extends the base Connector interface with type-safe operations.
// All API connectors (GitHub, Slack, Jira, Discord, Jenkins) implement this interface.
type TypedConnector interface {
	// Operations returns the list of available operations with metadata.
	Operations() []OperationInfo

	// OperationSchema returns the operation description and parameter information.
	// Returns nil if the operation doesn't exist.
	OperationSchema(operation string) *OperationSchema
}
