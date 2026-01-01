package operation

import (
	"fmt"
	"strings"

	"github.com/tombee/conductor/pkg/workflow"
)

// PackageDefinition represents an integration package YAML file.
type PackageDefinition struct {
	Version     string                                  `yaml:"version"`
	Name        string                                  `yaml:"name"`
	Description string                                  `yaml:"description"`
	BaseURL     string                                  `yaml:"base_url"`
	Headers     map[string]string                       `yaml:"headers,omitempty"`
	RateLimit   *workflow.RateLimitConfig               `yaml:"rate_limit,omitempty"`
	Operations  map[string]workflow.OperationDefinition `yaml:"operations"`
}

// loadPackage loads an integration package from a "from" reference.
// Supports:
//   - "integrations/github" -> bundled integration
//   - "./path/to/integration.yaml" -> local file (future)
//   - "github.com/org/integration@v1.0" -> remote package (future)
func loadPackage(from string) (*PackageDefinition, error) {
	// Check if it's a bundled integration reference
	if strings.HasPrefix(from, "integrations/") {
		return loadBundledPackage(from)
	}

	// Future: support local files and remote packages
	return nil, &Error{
		Type:        ErrorTypeNotImplemented,
		Message:     fmt.Sprintf("integration package source %q not yet supported", from),
		SuggestText: "use bundled integrations (from: integrations/<name>) or inline definitions",
	}
}

// builtinIntegrationInfo contains metadata for Go-based builtin integrations.
var builtinIntegrationInfo = map[string]struct {
	baseURL     string
	description string
}{
	"github":     {baseURL: "https://api.github.com", description: "GitHub REST API v3"},
	"slack":      {baseURL: "https://slack.com/api", description: "Slack Web API"},
	"jira":       {baseURL: "https://your-domain.atlassian.net", description: "Jira Cloud REST API v3"},
	"discord":    {baseURL: "https://discord.com/api/v10", description: "Discord REST API v10"},
	"jenkins":    {baseURL: "https://jenkins.example.com", description: "Jenkins REST API"},
	"cloudwatch": {baseURL: "https://logs.us-east-1.amazonaws.com", description: "AWS CloudWatch Logs and Metrics"},
}

// loadBundledPackage loads a bundled integration.
// For Go-based builtin integrations (github, slack, jira, discord, jenkins),
// this returns metadata from the builtin integration info.
func loadBundledPackage(from string) (*PackageDefinition, error) {
	// Extract integration name from "integrations/<name>"
	parts := strings.Split(from, "/")
	if len(parts) != 2 {
		return nil, &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("invalid bundled integration reference %q: expected format 'integrations/<name>'", from),
		}
	}

	integrationName := parts[1]

	// Check if it's a registered Go-based builtin integration
	if isBuiltinAPI(integrationName) {
		info, ok := builtinIntegrationInfo[integrationName]
		if !ok {
			// Fallback for unknown builtins
			info = struct{ baseURL, description string }{
				baseURL:     "https://api.example.com",
				description: "Builtin integration",
			}
		}

		// Return metadata for the builtin integration
		// Note: Operations are handled internally by Go integrations,
		// so we return an empty operations map for package metadata
		return &PackageDefinition{
			Version:     "2.0",
			Name:        integrationName,
			Description: info.description,
			BaseURL:     info.baseURL,
			Operations:  map[string]workflow.OperationDefinition{},
		}, nil
	}

	// Not a builtin integration - report not found
	return nil, &Error{
		Type:        ErrorTypeNotFound,
		Message:     fmt.Sprintf("bundled integration %q not found", integrationName),
		SuggestText: "available bundled integrations: github, slack, jira, discord, jenkins",
	}
}

// mergePackageWithOverrides merges a package definition with user overrides.
// User can override: base_url, auth, headers, rate_limit
func mergePackageWithOverrides(pkg *PackageDefinition, def *workflow.IntegrationDefinition) *workflow.IntegrationDefinition {
	merged := &workflow.IntegrationDefinition{
		Name: def.Name,
		From: def.From,
	}

	// Base URL: user override takes precedence
	if def.BaseURL != "" {
		merged.BaseURL = def.BaseURL
	} else {
		merged.BaseURL = pkg.BaseURL
	}

	// Auth: always from user (packages don't include auth)
	merged.Auth = def.Auth

	// Headers: merge package headers with user overrides
	merged.Headers = make(map[string]string)
	for k, v := range pkg.Headers {
		merged.Headers[k] = v
	}
	for k, v := range def.Headers {
		merged.Headers[k] = v
	}

	// Rate limit: user override or package default
	if def.RateLimit != nil {
		merged.RateLimit = def.RateLimit
	} else {
		merged.RateLimit = pkg.RateLimit
	}

	// Operations: always from package
	merged.Operations = pkg.Operations

	return merged
}
