package connector

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/tombee/conductor/pkg/workflow"
)

// ApplyAuth adds authentication to an HTTP request based on auth configuration.
func ApplyAuth(req *http.Request, auth *workflow.AuthDefinition) error {
	if auth == nil {
		return nil
	}

	// Determine auth type (infer bearer if only token is present)
	authType := auth.Type
	if authType == "" && auth.Token != "" {
		authType = "bearer"
	}

	switch authType {
	case "bearer", "":
		return applyBearerAuth(req, auth)
	case "basic":
		return applyBasicAuth(req, auth)
	case "api_key":
		return applyAPIKeyAuth(req, auth)
	case "oauth2_client":
		return fmt.Errorf("oauth2_client auth type is not yet implemented")
	default:
		return fmt.Errorf("unsupported auth type: %s", authType)
	}
}

// applyBearerAuth adds Bearer token authentication.
func applyBearerAuth(req *http.Request, auth *workflow.AuthDefinition) error {
	token, err := expandEnvVar(auth.Token)
	if err != nil {
		return fmt.Errorf("bearer auth token expansion failed: %w", err)
	}
	if token == "" {
		return fmt.Errorf("bearer auth requires token")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return nil
}

// applyBasicAuth adds HTTP Basic authentication.
func applyBasicAuth(req *http.Request, auth *workflow.AuthDefinition) error {
	username, err := expandEnvVar(auth.Username)
	if err != nil {
		return fmt.Errorf("basic auth username expansion failed: %w", err)
	}
	password, err := expandEnvVar(auth.Password)
	if err != nil {
		return fmt.Errorf("basic auth password expansion failed: %w", err)
	}

	if username == "" {
		return fmt.Errorf("basic auth requires username")
	}
	if password == "" {
		return fmt.Errorf("basic auth requires password")
	}

	// Encode credentials
	credentials := fmt.Sprintf("%s:%s", username, password)
	encoded := base64.StdEncoding.EncodeToString([]byte(credentials))

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encoded))
	return nil
}

// applyAPIKeyAuth adds API key authentication via custom header.
func applyAPIKeyAuth(req *http.Request, auth *workflow.AuthDefinition) error {
	headerName := auth.Header
	if headerName == "" {
		return fmt.Errorf("api_key auth requires header name")
	}

	value, err := expandEnvVar(auth.Value)
	if err != nil {
		return fmt.Errorf("api_key auth value expansion failed: %w", err)
	}
	if value == "" {
		return fmt.Errorf("api_key auth requires value")
	}

	req.Header.Set(headerName, value)
	return nil
}

// validEnvVarName matches valid environment variable names (alphanumeric + underscore).
var validEnvVarName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// expandEnvVar expands environment variable references in the form ${VAR_NAME}.
// If the value doesn't contain ${...}, it's returned as-is.
// Returns error if variable name is invalid or variable is not found.
func expandEnvVar(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	// Check if value contains ${...} pattern
	if !strings.Contains(value, "${") {
		return value, nil
	}

	// Simple expansion - find ${VAR} and replace with os.Getenv(VAR)
	result := value
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}

		end := strings.Index(result[start:], "}")
		if end == -1 {
			return "", fmt.Errorf("malformed environment variable reference: unclosed ${")
		}
		end += start

		// Extract variable name
		varName := result[start+2 : end]

		// Validate variable name
		if !validEnvVarName.MatchString(varName) {
			return "", fmt.Errorf("invalid environment variable name: %q (must be alphanumeric with underscores)", varName)
		}

		// Get environment variable value
		varValue, exists := os.LookupEnv(varName)
		if !exists {
			return "", fmt.Errorf("environment variable %q not found", varName)
		}

		// Replace ${VAR} with value
		result = result[:start] + varValue + result[end+1:]
	}

	return result, nil
}
