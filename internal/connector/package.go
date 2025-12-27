package connector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tombee/conductor/pkg/workflow"
)

// PackageDefinition represents a connector package YAML file.
type PackageDefinition struct {
	Version     string                                 `yaml:"version"`
	Name        string                                 `yaml:"name"`
	Description string                                 `yaml:"description"`
	BaseURL     string                                 `yaml:"base_url"`
	Headers     map[string]string                      `yaml:"headers,omitempty"`
	RateLimit   *workflow.RateLimitConfig              `yaml:"rate_limit,omitempty"`
	Operations  map[string]workflow.OperationDefinition `yaml:"operations"`
}

// loadPackage loads a connector package from a "from" reference.
// Supports:
//   - "connectors/github" -> bundled connector
//   - "./path/to/connector.yaml" -> local file (future)
//   - "github.com/org/connector@v1.0" -> remote package (future)
func loadPackage(from string) (*PackageDefinition, error) {
	// Check if it's a bundled connector reference
	if strings.HasPrefix(from, "connectors/") {
		return loadBundledPackage(from)
	}

	// Future: support local files and remote packages
	return nil, &Error{
		Type:       ErrorTypeNotImplemented,
		Message:    fmt.Sprintf("connector package source %q not yet supported", from),
		SuggestText: "use bundled connectors (from: connectors/<name>) or inline definitions",
	}
}

// builtinConnectorInfo contains metadata for Go-based builtin connectors.
var builtinConnectorInfo = map[string]struct {
	baseURL     string
	description string
}{
	"github":  {baseURL: "https://api.github.com", description: "GitHub REST API v3"},
	"slack":   {baseURL: "https://slack.com/api", description: "Slack Web API"},
	"jira":    {baseURL: "https://your-domain.atlassian.net", description: "Jira Cloud REST API v3"},
	"discord": {baseURL: "https://discord.com/api/v10", description: "Discord REST API v10"},
	"jenkins": {baseURL: "https://jenkins.example.com", description: "Jenkins REST API"},
}

// loadBundledPackage loads a bundled connector.
// For Go-based builtin connectors (github, slack, jira, discord, jenkins),
// this returns metadata from the builtin connector info.
func loadBundledPackage(from string) (*PackageDefinition, error) {
	// Extract connector name from "connectors/<name>"
	parts := strings.Split(from, "/")
	if len(parts) != 2 {
		return nil, &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("invalid bundled connector reference %q: expected format 'connectors/<name>'", from),
		}
	}

	connectorName := parts[1]

	// Check if it's a registered Go-based builtin connector
	if isBuiltinAPI(connectorName) {
		info, ok := builtinConnectorInfo[connectorName]
		if !ok {
			// Fallback for unknown builtins
			info = struct{ baseURL, description string }{
				baseURL:     "https://api.example.com",
				description: "Builtin connector",
			}
		}

		// Return metadata for the builtin connector
		// Note: Operations are handled internally by Go connectors,
		// so we return an empty operations map for package metadata
		return &PackageDefinition{
			Version:     "2.0",
			Name:        connectorName,
			Description: info.description,
			BaseURL:     info.baseURL,
			Operations:  map[string]workflow.OperationDefinition{},
		}, nil
	}

	// Not a builtin connector - report not found
	return nil, &Error{
		Type:        ErrorTypeNotFound,
		Message:     fmt.Sprintf("bundled connector %q not found", connectorName),
		SuggestText: "available bundled connectors: github, slack, jira, discord, jenkins",
	}
}

// validatePackage validates a connector package definition.
func validatePackage(pkg *PackageDefinition) error {
	if pkg.Version == "" {
		return &Error{
			Type:    ErrorTypeValidation,
			Message: "connector package missing 'version' field",
		}
	}

	if pkg.Name == "" {
		return &Error{
			Type:    ErrorTypeValidation,
			Message: "connector package missing 'name' field",
		}
	}

	if pkg.BaseURL == "" {
		return &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("connector package %q missing 'base_url' field", pkg.Name),
		}
	}

	if len(pkg.Operations) == 0 {
		return &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("connector package %q has no operations defined", pkg.Name),
		}
	}

	// Validate each operation has required fields
	for opName, op := range pkg.Operations {
		if op.Method == "" {
			return &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("operation %q in package %q missing 'method' field", opName, pkg.Name),
			}
		}
		if op.Path == "" {
			return &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("operation %q in package %q missing 'path' field", opName, pkg.Name),
			}
		}
	}

	return nil
}

// mergePackageWithOverrides merges a package definition with user overrides.
// User can override: base_url, auth, headers, rate_limit
func mergePackageWithOverrides(pkg *PackageDefinition, def *workflow.ConnectorDefinition) *workflow.ConnectorDefinition {
	merged := &workflow.ConnectorDefinition{
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

// loadLocalPackage loads a connector from a local file path.
// This is a future feature for loading custom connectors from disk.
func loadLocalPackage(path string) (*PackageDefinition, error) {
	// Resolve relative path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("invalid connector path %q: %v", path, err),
		}
	}

	// Read file
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &Error{
				Type:       ErrorTypeNotFound,
				Message:    fmt.Sprintf("connector file not found: %s", absPath),
				SuggestText: "check that the file exists and the path is correct",
			}
		}
		return nil, &Error{
			Type:    ErrorTypeServer,
			Message: fmt.Sprintf("failed to read connector file %q: %v", absPath, err),
		}
	}

	// Parse YAML
	var pkg PackageDefinition
	if err := yaml.Unmarshal(data, &pkg); err != nil {
		return nil, &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("failed to parse connector file %q: %v", absPath, err),
		}
	}

	// Validate
	if err := validatePackage(&pkg); err != nil {
		return nil, err
	}

	return &pkg, nil
}
