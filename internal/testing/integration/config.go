package integration

import (
	"os"
	"testing"
)

// TestConfig holds configuration for integration tests loaded from environment.
type TestConfig struct {
	// AnthropicAPIKey is the API key for Anthropic provider tests.
	AnthropicAPIKey string

	// OpenAIAPIKey is the API key for OpenAI provider tests.
	OpenAIAPIKey string

	// PostgresURL is the connection string for Postgres integration tests.
	PostgresURL string

	// OllamaURL is the base URL for Ollama tests (defaults to http://localhost:11434).
	OllamaURL string
}

// LoadConfig loads test configuration from environment variables.
// Does not fail if keys are missing - individual tests should use SkipWithoutEnv.
func LoadConfig() *TestConfig {
	cfg := &TestConfig{
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		PostgresURL:     os.Getenv("POSTGRES_URL"),
		OllamaURL:       os.Getenv("OLLAMA_URL"),
	}

	// Set default for Ollama
	if cfg.OllamaURL == "" {
		cfg.OllamaURL = "http://localhost:11434"
	}

	return cfg
}

// SkipWithoutEnv skips the test if the specified environment variable is not set.
// This allows tests to run conditionally based on available configuration.
func SkipWithoutEnv(t *testing.T, envVar string) {
	t.Helper()

	value := os.Getenv(envVar)
	if value == "" {
		t.Skipf("Skipping test: %s not set", envVar)
	}
}

// RequireEnv fails the test if the specified environment variable is not set.
// Use this for tests that should always run in CI but may skip locally.
func RequireEnv(t *testing.T, envVar string) string {
	t.Helper()

	value := os.Getenv(envVar)
	if value == "" {
		t.Fatalf("Required environment variable %s not set", envVar)
	}
	return value
}
