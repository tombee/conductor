// Package llm provides abstractions for Large Language Model providers.
package llm

import (
	"fmt"
	"strings"
)

// Credentials is the interface that all provider credential types must implement.
// This provides a unified way to handle authentication across different providers.
type Credentials interface {
	// Validate checks if the credentials are properly formatted and present.
	// Returns an error if credentials are missing or malformed.
	Validate() error

	// Redacted returns a safe-to-log version of the credentials.
	// Sensitive values (API keys, tokens) are replaced with masked versions.
	Redacted() string

	// ProviderType returns the type of provider these credentials are for.
	ProviderType() string
}

// APIKeyCredentials holds authentication for API-based providers (Anthropic, OpenAI).
type APIKeyCredentials struct {
	// APIKey is the authentication token for the provider's API.
	APIKey string

	// BaseURL is an optional override for the API endpoint.
	// If empty, the provider's default endpoint is used.
	BaseURL string
}

// Validate checks that the API key is present.
// Length and format validation is left to individual providers since key formats vary.
func (c APIKeyCredentials) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	return nil
}

// Redacted returns a safe-to-log version with the API key masked.
func (c APIKeyCredentials) Redacted() string {
	masked := maskSecret(c.APIKey)
	if c.BaseURL != "" {
		return fmt.Sprintf("APIKey: %s, BaseURL: %s", masked, c.BaseURL)
	}
	return fmt.Sprintf("APIKey: %s", masked)
}

// ProviderType returns "api" indicating this is for API-based providers.
func (c APIKeyCredentials) ProviderType() string {
	return "api"
}

// CLIAuthCredentials holds authentication for CLI-based providers (Claude Code).
type CLIAuthCredentials struct {
	// CLIPath is the path to the CLI binary.
	// If empty, the CLI is expected to be in PATH.
	CLIPath string
}

// Validate checks that CLI authentication is properly configured.
// For CLI providers, the actual authentication is managed by the CLI itself.
func (c CLIAuthCredentials) Validate() error {
	// CLI credentials don't require validation at this level.
	// The CLI binary handles its own authentication.
	return nil
}

// Redacted returns a safe-to-log version of the CLI credentials.
func (c CLIAuthCredentials) Redacted() string {
	if c.CLIPath != "" {
		return fmt.Sprintf("CLIPath: %s", c.CLIPath)
	}
	return "CLIPath: (default)"
}

// ProviderType returns "cli" indicating this is for CLI-based providers.
func (c CLIAuthCredentials) ProviderType() string {
	return "cli"
}

// OllamaCredentials holds configuration for Ollama providers.
type OllamaCredentials struct {
	// BaseURL is the Ollama API endpoint.
	// Defaults to http://localhost:11434 if empty.
	BaseURL string
}

// Validate checks that Ollama configuration is valid.
func (c OllamaCredentials) Validate() error {
	// Ollama doesn't require API keys, just a valid endpoint
	return nil
}

// Redacted returns a safe-to-log version of the Ollama credentials.
func (c OllamaCredentials) Redacted() string {
	if c.BaseURL != "" {
		return fmt.Sprintf("BaseURL: %s", c.BaseURL)
	}
	return "BaseURL: http://localhost:11434 (default)"
}

// ProviderType returns "ollama" indicating this is for Ollama providers.
func (c OllamaCredentials) ProviderType() string {
	return "ollama"
}

// maskSecret returns a masked version of a secret string.
// Shows first 4 and last 4 characters with asterisks in between.
func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return strings.Repeat("*", len(secret))
	}
	return secret[:4] + strings.Repeat("*", len(secret)-8) + secret[len(secret)-4:]
}

// Compile-time interface implementation checks
var (
	_ Credentials = APIKeyCredentials{}
	_ Credentials = CLIAuthCredentials{}
	_ Credentials = OllamaCredentials{}
)
