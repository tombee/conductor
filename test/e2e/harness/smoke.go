// +build smoke

package harness

import (
	"net/http"
	"os"
	"time"
)

// DetectAvailableProvider attempts to detect which LLM provider is available.
// Returns the provider name in priority order: ollama -> anthropic -> empty string if none available.
func DetectAvailableProvider() string {
	// Check Ollama (highest priority - free and local)
	if isOllamaAvailable() {
		return "ollama"
	}

	// Check Anthropic API key
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "anthropic"
	}

	// No provider available
	return ""
}

// isOllamaAvailable checks if Ollama is running and accessible.
func isOllamaAvailable() bool {
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	// Try to connect to Ollama with a short timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(ollamaURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Ollama should respond with 200 OK on the root endpoint
	return resp.StatusCode == http.StatusOK
}
