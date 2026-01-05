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

package workflow

import (
	"fmt"
	"regexp"
	"strings"
)

// IntegrationDefinition defines a declarative integration for external services.
// Integrations provide schema-validated, deterministic operations that execute without LLM involvement.
type IntegrationDefinition struct {
	// Name is inferred from the map key in workflow.integrations
	Name string `yaml:"-" json:"name,omitempty"`

	// From imports an integration package (e.g., "integrations/github", "github.com/org/integration@v1.0")
	// Mutually exclusive with inline definition (BaseURL + Operations)
	From string `yaml:"from,omitempty" json:"from,omitempty"`

	// BaseURL is the base URL for all operations (required for inline integrations)
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`

	// Transport specifies which transport to use ("http", "aws_sigv4", "oauth2")
	// Defaults to "http" if not specified
	Transport string `yaml:"transport,omitempty" json:"transport,omitempty"`

	// Auth defines authentication configuration (for http transport)
	Auth *AuthDefinition `yaml:"auth,omitempty" json:"auth,omitempty"`

	// AWS defines AWS SigV4 transport configuration (for aws_sigv4 transport)
	AWS *AWSConfig `yaml:"aws,omitempty" json:"aws,omitempty"`

	// OAuth2 defines OAuth2 transport configuration (for oauth2 transport)
	OAuth2 *OAuth2Config `yaml:"oauth2,omitempty" json:"oauth2,omitempty"`

	// Headers are default headers applied to all operations
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// RateLimit defines rate limiting configuration
	RateLimit *RateLimitConfig `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`

	// Operations define named operations for inline integrations
	// Not used when From is specified (operations come from package)
	Operations map[string]OperationDefinition `yaml:"operations,omitempty" json:"operations,omitempty"`
}

// OperationDefinition defines a single operation within an integration.
type OperationDefinition struct {
	// Method is the HTTP method (GET, POST, PUT, PATCH, DELETE)
	Method string `yaml:"method" json:"method"`

	// Path is the URL path template with {param} placeholders
	Path string `yaml:"path" json:"path"`

	// RequestSchema is the JSON Schema for operation inputs
	RequestSchema map[string]interface{} `yaml:"request_schema,omitempty" json:"request_schema,omitempty"`

	// ResponseTransform is a jq expression to transform the response
	ResponseTransform string `yaml:"response_transform,omitempty" json:"response_transform,omitempty"`

	// Headers are operation-specific headers (merged with integration headers)
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// Timeout is the operation-specific timeout in seconds
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Retry defines retry configuration for this operation
	Retry *RetryDefinition `yaml:"retry,omitempty" json:"retry,omitempty"`
}

// AuthDefinition defines authentication configuration for an integration.
type AuthDefinition struct {
	// Type is the authentication type (bearer, basic, api_key, oauth2_client)
	// Optional - inferred as "bearer" if only Token is present
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Token is the bearer token (for type: bearer or shorthand)
	// Can reference environment variables: ${GITHUB_TOKEN}
	Token string `yaml:"token,omitempty" json:"token,omitempty"`

	// Username for basic auth (type: basic)
	Username string `yaml:"username,omitempty" json:"username,omitempty"`

	// Password for basic auth (type: basic)
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// Header is the header name for API key auth (type: api_key)
	Header string `yaml:"header,omitempty" json:"header,omitempty"`

	// Value is the API key value (type: api_key)
	Value string `yaml:"value,omitempty" json:"value,omitempty"`

	// ClientID for OAuth2 client credentials flow (type: oauth2_client) - future
	ClientID string `yaml:"client_id,omitempty" json:"client_id,omitempty"`

	// ClientSecret for OAuth2 client credentials flow (type: oauth2_client) - future
	ClientSecret string `yaml:"client_secret,omitempty" json:"client_secret,omitempty"`

	// TokenURL for OAuth2 token endpoint (type: oauth2_client) - future
	TokenURL string `yaml:"token_url,omitempty" json:"token_url,omitempty"`

	// Scopes for OAuth2 (type: oauth2_client) - future
	Scopes []string `yaml:"scopes,omitempty" json:"scopes,omitempty"`
}

// RateLimitConfig defines rate limiting configuration for an integration.
type RateLimitConfig struct {
	// RequestsPerSecond limits the number of requests per second
	RequestsPerSecond float64 `yaml:"requests_per_second,omitempty" json:"requests_per_second,omitempty"`

	// RequestsPerMinute limits the number of requests per minute
	RequestsPerMinute int `yaml:"requests_per_minute,omitempty" json:"requests_per_minute,omitempty"`

	// Timeout is the maximum time to wait for rate limit (in seconds, default 30)
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// AWSConfig defines AWS SigV4 transport configuration.
type AWSConfig struct {
	// Service is the AWS service name (e.g., "s3", "dynamodb", "sqs")
	Service string `yaml:"service" json:"service"`

	// Region is the AWS region (e.g., "us-east-1", "eu-west-1")
	Region string `yaml:"region" json:"region"`
}

// OAuth2Config defines OAuth2 transport configuration.
type OAuth2Config struct {
	// Flow is the OAuth2 flow ("client_credentials" or "authorization_code")
	Flow string `yaml:"flow" json:"flow"`

	// ClientID is the OAuth2 client ID (must use ${ENV_VAR} syntax)
	ClientID string `yaml:"client_id" json:"client_id"`

	// ClientSecret is the OAuth2 client secret (must use ${ENV_VAR} syntax)
	ClientSecret string `yaml:"client_secret" json:"client_secret"`

	// TokenURL is the OAuth2 token endpoint URL
	TokenURL string `yaml:"token_url" json:"token_url"`

	// Scopes are the OAuth2 scopes to request
	Scopes []string `yaml:"scopes,omitempty" json:"scopes,omitempty"`

	// RefreshToken is the refresh token for authorization_code flow (must use ${ENV_VAR} syntax)
	RefreshToken string `yaml:"refresh_token,omitempty" json:"refresh_token,omitempty"`
}

// ParsedIntegrationRequirement represents a parsed integration requirement.
// It is derived from the string format in requires.integrations.
type ParsedIntegrationRequirement struct {
	// Type is the integration type (e.g., "github", "slack")
	Type string

	// Alias is the optional alias for this requirement (e.g., "source", "target")
	// Empty string means no alias (simple requirement)
	Alias string
}

// ParseIntegrationRequirement parses an integration requirement string.
// Supports two formats:
//   - Simple: "github" -> type="github", alias=""
//   - Aliased: "github as source" -> type="github", alias="source"
func ParseIntegrationRequirement(req string) ParsedIntegrationRequirement {
	// Check for "as" keyword (case-insensitive wouldn't make sense here, keep it exact)
	parts := regexp.MustCompile(`\s+as\s+`).Split(req, 2)

	if len(parts) == 2 {
		// Aliased format: "github as source"
		return ParsedIntegrationRequirement{
			Type:  strings.TrimSpace(parts[0]),
			Alias: strings.TrimSpace(parts[1]),
		}
	}

	// Simple format: "github"
	return ParsedIntegrationRequirement{
		Type:  strings.TrimSpace(req),
		Alias: "",
	}
}

// Validate checks if the requirements definition is valid.
func (r *RequirementsDefinition) Validate() error {
	// Validate integration requirements
	seenTypes := make(map[string]bool)
	seenAliases := make(map[string]bool)

	for i, reqStr := range r.Integrations {
		if reqStr == "" {
			return fmt.Errorf("integration requirement %d: cannot be empty", i)
		}

		// Parse the requirement
		parsed := ParseIntegrationRequirement(reqStr)

		// Check for duplicate types without aliases
		if parsed.Alias == "" {
			if seenTypes[parsed.Type] {
				return fmt.Errorf("duplicate integration requirement: %s", parsed.Type)
			}
			seenTypes[parsed.Type] = true
		}

		// Check for duplicate aliases
		if parsed.Alias != "" {
			if seenAliases[parsed.Alias] {
				return fmt.Errorf("duplicate integration alias: %s", parsed.Alias)
			}
			seenAliases[parsed.Alias] = true
		}
	}

	// Validate MCP server requirements
	mcpNames := make(map[string]bool)
	for i, req := range r.MCPServers {
		if req.Name == "" {
			return fmt.Errorf("mcp_server requirement %d: name is required", i)
		}
		if mcpNames[req.Name] {
			return fmt.Errorf("duplicate mcp_server requirement: %s", req.Name)
		}
		mcpNames[req.Name] = true
	}

	return nil
}

// Validate checks if the integration definition is valid.
func (c *IntegrationDefinition) Validate() error {
	// Validate name (should be set from map key)
	if c.Name == "" {
		return fmt.Errorf("integration name is required")
	}

	// Check for mutually exclusive fields: from vs inline definition
	hasFrom := c.From != ""
	hasInline := c.BaseURL != "" || len(c.Operations) > 0

	if !hasFrom && !hasInline {
		return fmt.Errorf("integration must specify either 'from' (package import) or inline definition (base_url + operations)")
	}

	if hasFrom && hasInline {
		return fmt.Errorf("integration cannot specify both 'from' and inline definition (base_url/operations)")
	}

	// For inline integrations, base_url is required
	if !hasFrom && c.BaseURL == "" {
		return fmt.Errorf("base_url is required for inline integration definition")
	}

	// For inline integrations, must have at least one operation
	if !hasFrom && len(c.Operations) == 0 {
		return fmt.Errorf("inline integration must define at least one operation")
	}

	// Validate auth if specified
	if c.Auth != nil {
		if err := c.Auth.Validate(); err != nil {
			return fmt.Errorf("invalid auth: %w", err)
		}
	}

	// Validate transport field if specified
	if c.Transport != "" {
		validTransports := map[string]bool{
			"http":      true,
			"aws_sigv4": true,
			"oauth2":    true,
		}
		if !validTransports[c.Transport] {
			return fmt.Errorf("invalid transport %q: must be http, aws_sigv4, or oauth2", c.Transport)
		}

		// Validate AWS config when transport is aws_sigv4
		if c.Transport == "aws_sigv4" {
			if c.AWS == nil {
				return fmt.Errorf("aws configuration is required when transport is aws_sigv4")
			}
			if c.AWS.Service == "" {
				return fmt.Errorf("aws.service is required when transport is aws_sigv4")
			}
			if c.AWS.Region == "" {
				return fmt.Errorf("aws.region is required when transport is aws_sigv4")
			}
		}

		// Validate OAuth2 config when transport is oauth2
		if c.Transport == "oauth2" {
			if c.OAuth2 == nil {
				return fmt.Errorf("oauth2 configuration is required when transport is oauth2")
			}
			if c.OAuth2.ClientID == "" {
				return fmt.Errorf("oauth2.client_id is required when transport is oauth2")
			}
			if c.OAuth2.ClientSecret == "" {
				return fmt.Errorf("oauth2.client_secret is required when transport is oauth2")
			}
			if c.OAuth2.TokenURL == "" {
				return fmt.Errorf("oauth2.token_url is required when transport is oauth2")
			}
			if c.OAuth2.Flow != "" && c.OAuth2.Flow != "client_credentials" && c.OAuth2.Flow != "authorization_code" {
				return fmt.Errorf("oauth2.flow must be client_credentials or authorization_code, got %q", c.OAuth2.Flow)
			}
		}
	}

	// Validate rate limit if specified
	if c.RateLimit != nil {
		if err := c.RateLimit.Validate(); err != nil {
			return fmt.Errorf("invalid rate_limit: %w", err)
		}
	}

	// Validate operations
	for name, op := range c.Operations {
		if err := op.Validate(); err != nil {
			return fmt.Errorf("invalid operation %s: %w", name, err)
		}
	}

	return nil
}

// Validate checks if the operation definition is valid.
func (o *OperationDefinition) Validate() error {
	// Method is required
	if o.Method == "" {
		return fmt.Errorf("method is required")
	}

	// Validate HTTP method
	validMethods := map[string]bool{
		"GET":    true,
		"POST":   true,
		"PUT":    true,
		"PATCH":  true,
		"DELETE": true,
		"HEAD":   true,
	}
	if !validMethods[o.Method] {
		return fmt.Errorf("invalid method: %s (must be GET, POST, PUT, PATCH, DELETE, or HEAD)", o.Method)
	}

	// Path is required
	if o.Path == "" {
		return fmt.Errorf("path is required")
	}

	// Validate timeout if specified
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}

	// Validate retry if specified
	if o.Retry != nil {
		if err := o.Retry.Validate(); err != nil {
			return fmt.Errorf("invalid retry: %w", err)
		}
	}

	return nil
}

// Validate checks if the auth definition is valid.
func (a *AuthDefinition) Validate() error {
	// Infer type if not specified
	authType := a.Type
	if authType == "" {
		// If only token is present, assume bearer
		if a.Token != "" {
			authType = "bearer"
		}
	}

	switch authType {
	case "bearer", "":
		if a.Token == "" {
			return fmt.Errorf("token is required for bearer auth")
		}

	case "basic":
		if a.Username == "" {
			return fmt.Errorf("username is required for basic auth")
		}
		if a.Password == "" {
			return fmt.Errorf("password is required for basic auth")
		}

	case "api_key":
		if a.Header == "" {
			return fmt.Errorf("header is required for api_key auth")
		}
		if a.Value == "" {
			return fmt.Errorf("value is required for api_key auth")
		}

	case "oauth2_client":
		// OAuth2 is future functionality
		return fmt.Errorf("oauth2_client auth type is not yet implemented")

	default:
		return fmt.Errorf("invalid auth type: %s (must be bearer, basic, api_key, or oauth2_client)", authType)
	}

	return nil
}

// Validate checks if the rate limit config is valid.
func (r *RateLimitConfig) Validate() error {
	// At least one limit must be specified
	if r.RequestsPerSecond == 0 && r.RequestsPerMinute == 0 {
		return fmt.Errorf("at least one of requests_per_second or requests_per_minute must be specified")
	}

	// Values must be positive
	if r.RequestsPerSecond < 0 {
		return fmt.Errorf("requests_per_second must be non-negative")
	}
	if r.RequestsPerMinute < 0 {
		return fmt.Errorf("requests_per_minute must be non-negative")
	}

	// Validate timeout
	if r.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}

	return nil
}
