//go:build integration

package providers

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/testing/integration"
	"github.com/tombee/conductor/pkg/llm"
)

// checkOllamaAvailable checks if Ollama is running locally.
func checkOllamaAvailable(t *testing.T, url string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/tags", nil)
	if err != nil {
		t.Skipf("Skipping: cannot create request: %v", err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("Skipping: Ollama not available at %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping: Ollama returned status %d", resp.StatusCode)
	}
}

// TestOllamaComplete_RealAPI tests a real completion call to local Ollama.
// This test requires Ollama running locally with a model installed.
func TestOllamaComplete_RealAPI(t *testing.T) {
	cfg := integration.LoadConfig()
	checkOllamaAvailable(t, cfg.OllamaURL)

	// Create provider
	provider, err := NewOllamaProvider(cfg.OllamaURL)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create a simple completion request
	// Using a minimal model that's commonly installed
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Try with default model (provider should have a reasonable default)
	req := integration.SimpleCompletionRequest("", "Say 'hello' and nothing else")

	// Execute with retry for transient failures
	var resp *llm.CompletionResponse
	err = integration.Retry(ctx, func() error {
		var retryErr error
		resp, retryErr = provider.Complete(ctx, req)
		return retryErr
	}, integration.DefaultRetryConfig())

	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify response structure
	if resp == nil {
		t.Fatal("Response is nil")
	}
	if resp.Content == "" {
		t.Error("Response content is empty")
	}
	if resp.FinishReason == "" {
		t.Error("Finish reason is empty")
	}
	if resp.Model == "" {
		t.Error("Model is empty")
	}

	// Token usage may not be available from Ollama
	t.Logf("Response (model: %s): %s", resp.Model, resp.Content)
}

// TestOllamaStream_RealAPI tests real streaming completion from local Ollama.
func TestOllamaStream_RealAPI(t *testing.T) {
	cfg := integration.LoadConfig()
	checkOllamaAvailable(t, cfg.OllamaURL)

	// Create provider
	provider, err := NewOllamaProvider(cfg.OllamaURL)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create streaming request
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := integration.StreamingCompletionRequest("", "Count from 1 to 3")

	// Execute stream
	chunks, err := provider.Stream(ctx, req)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Collect chunks
	var content strings.Builder
	chunkCount := 0

	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("Stream error: %v", chunk.Error)
		}

		if chunk.Delta.Content != "" {
			content.WriteString(chunk.Delta.Content)
			chunkCount++
		}
	}

	// Verify streaming worked
	if chunkCount == 0 {
		t.Error("No content chunks received")
	}

	finalContent := content.String()
	if finalContent == "" {
		t.Error("Final content is empty")
	}

	t.Logf("Stream test (chunks: %d): %s", chunkCount, finalContent)
}

// TestOllamaErrorHandling_RealAPI tests error handling with Ollama.
func TestOllamaErrorHandling_RealAPI(t *testing.T) {
	t.Run("Invalid URL", func(t *testing.T) {
		// Create provider with invalid URL
		provider, err := NewOllamaProvider("http://localhost:99999")
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req := integration.SimpleCompletionRequest("", "test")

		// This should fail with connection error
		_, err = provider.Complete(ctx, req)
		if err == nil {
			t.Error("Expected connection error but got success")
		}
		t.Logf("Connection error (expected): %v", err)
	})

	t.Run("Context Timeout", func(t *testing.T) {
		cfg := integration.LoadConfig()
		checkOllamaAvailable(t, cfg.OllamaURL)

		provider, err := NewOllamaProvider(cfg.OllamaURL)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}

		// Create context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		time.Sleep(2 * time.Millisecond) // Ensure timeout

		req := integration.SimpleCompletionRequest("", "test")
		_, err = provider.Complete(ctx, req)

		if err == nil {
			t.Error("Expected timeout error but got success")
		}
		t.Logf("Timeout error (expected): %v", err)
	})
}
