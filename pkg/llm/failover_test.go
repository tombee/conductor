package llm

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	pkgerrors "github.com/tombee/conductor/pkg/errors"
)

// mockFailoverProvider is a test provider that can simulate various failures.
type mockFailoverProvider struct {
	name        string
	shouldFail  bool
	failWith    error
	successResp *CompletionResponse
	callCount   int
}

func (m *mockFailoverProvider) Name() string {
	return m.name
}

func (m *mockFailoverProvider) Capabilities() Capabilities {
	return Capabilities{}
}

func (m *mockFailoverProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	m.callCount++

	if m.shouldFail {
		return nil, m.failWith
	}

	return m.successResp, nil
}

func (m *mockFailoverProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	m.callCount++

	if m.shouldFail {
		return nil, m.failWith
	}

	chunks := make(chan StreamChunk, 1)
	go func() {
		defer close(chunks)
		chunks <- StreamChunk{
			Delta: StreamDelta{Content: "success"},
		}
	}()

	return chunks, nil
}

func TestFailoverProvider_PrimarySuccess(t *testing.T) {
	registry := NewRegistry()

	primary := &mockFailoverProvider{
		name:       "primary",
		shouldFail: false,
		successResp: &CompletionResponse{
			Content: "primary success",
		},
	}

	secondary := &mockFailoverProvider{
		name: "secondary",
	}

	registry.Register(primary)
	registry.Register(secondary)

	config := FailoverConfig{
		ProviderOrder:           []string{"primary", "secondary"},
		CircuitBreakerThreshold: 0, // Disable circuit breaker for this test
	}

	failover, err := NewFailoverProvider(registry, config)
	if err != nil {
		t.Fatalf("failed to create failover provider: %v", err)
	}

	ctx := context.Background()
	resp, err := failover.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Content != "primary success" {
		t.Errorf("expected 'primary success', got '%s'", resp.Content)
	}

	if primary.callCount != 1 {
		t.Errorf("expected primary to be called once, got %d", primary.callCount)
	}

	if secondary.callCount != 0 {
		t.Errorf("expected secondary not to be called, got %d calls", secondary.callCount)
	}
}

func TestFailoverProvider_FailoverToSecondary(t *testing.T) {
	registry := NewRegistry()

	primary := &mockFailoverProvider{
		name:       "primary",
		shouldFail: true,
		failWith:   &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"},
	}

	secondary := &mockFailoverProvider{
		name:       "secondary",
		shouldFail: false,
		successResp: &CompletionResponse{
			Content: "secondary success",
		},
	}

	registry.Register(primary)
	registry.Register(secondary)

	var failoverFrom, failoverTo string
	config := FailoverConfig{
		ProviderOrder:           []string{"primary", "secondary"},
		CircuitBreakerThreshold: 0,
		OnFailover: func(from, to string, err error) {
			failoverFrom = from
			failoverTo = to
		},
	}

	failover, err := NewFailoverProvider(registry, config)
	if err != nil {
		t.Fatalf("failed to create failover provider: %v", err)
	}

	ctx := context.Background()
	resp, err := failover.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err != nil {
		t.Fatalf("expected no error after failover, got %v", err)
	}

	if resp.Content != "secondary success" {
		t.Errorf("expected 'secondary success', got '%s'", resp.Content)
	}

	if primary.callCount != 1 {
		t.Errorf("expected primary to be called once, got %d", primary.callCount)
	}

	if secondary.callCount != 1 {
		t.Errorf("expected secondary to be called once, got %d", secondary.callCount)
	}

	if failoverFrom != "primary" || failoverTo != "secondary" {
		t.Errorf("expected failover from primary to secondary, got %s to %s", failoverFrom, failoverTo)
	}
}

func TestFailoverProvider_AllProvidersFail(t *testing.T) {
	registry := NewRegistry()

	primary := &mockFailoverProvider{
		name:       "primary",
		shouldFail: true,
		failWith:   &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"},
	}

	secondary := &mockFailoverProvider{
		name:       "secondary",
		shouldFail: true,
		failWith:   &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusBadGateway, Message: "bad gateway"},
	}

	registry.Register(primary)
	registry.Register(secondary)

	config := FailoverConfig{
		ProviderOrder:           []string{"primary", "secondary"},
		CircuitBreakerThreshold: 0,
	}

	failover, err := NewFailoverProvider(registry, config)
	if err != nil {
		t.Fatalf("failed to create failover provider: %v", err)
	}

	ctx := context.Background()
	_, err = failover.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err == nil {
		t.Fatal("expected error when all providers fail, got nil")
	}

	// Verify we get a ProviderError wrapping the original errors
	var provErr *pkgerrors.ProviderError
	if !errors.As(err, &provErr) {
		t.Errorf("expected ProviderError, got %T: %v", err, err)
	}

	if primary.callCount != 1 {
		t.Errorf("expected primary to be called once, got %d", primary.callCount)
	}

	if secondary.callCount != 1 {
		t.Errorf("expected secondary to be called once, got %d", secondary.callCount)
	}
}

func TestFailoverProvider_NonRetryableError(t *testing.T) {
	registry := NewRegistry()

	primary := &mockFailoverProvider{
		name:       "primary",
		shouldFail: true,
		failWith: &pkgerrors.ProviderError{
			Provider:   "primary",
			StatusCode: http.StatusBadRequest,
			Message:    "bad request",
		},
	}

	secondary := &mockFailoverProvider{
		name: "secondary",
	}

	registry.Register(primary)
	registry.Register(secondary)

	config := FailoverConfig{
		ProviderOrder:           []string{"primary", "secondary"},
		CircuitBreakerThreshold: 0,
	}

	failover, err := NewFailoverProvider(registry, config)
	if err != nil {
		t.Fatalf("failed to create failover provider: %v", err)
	}

	ctx := context.Background()
	_, err = failover.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should not failover for non-retryable errors
	if primary.callCount != 1 {
		t.Errorf("expected primary to be called once, got %d", primary.callCount)
	}

	if secondary.callCount != 0 {
		t.Errorf("expected secondary not to be called, got %d calls", secondary.callCount)
	}
}

func TestFailoverProvider_CircuitBreaker(t *testing.T) {
	registry := NewRegistry()

	primary := &mockFailoverProvider{
		name:       "primary",
		shouldFail: true,
		failWith:   &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"},
	}

	secondary := &mockFailoverProvider{
		name:       "secondary",
		shouldFail: false,
		successResp: &CompletionResponse{
			Content: "secondary success",
		},
	}

	registry.Register(primary)
	registry.Register(secondary)

	config := FailoverConfig{
		ProviderOrder:           []string{"primary", "secondary"},
		CircuitBreakerThreshold: 3,
		CircuitBreakerTimeout:   100 * time.Millisecond,
	}

	failover, err := NewFailoverProvider(registry, config)
	if err != nil {
		t.Fatalf("failed to create failover provider: %v", err)
	}

	ctx := context.Background()

	// Make 3 requests to trip the circuit breaker
	for i := 0; i < 3; i++ {
		resp, err := failover.Complete(ctx, CompletionRequest{
			Messages: []Message{{Content: "test"}},
		})

		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}

		if resp.Content != "secondary success" {
			t.Errorf("request %d: expected 'secondary success', got '%s'", i+1, resp.Content)
		}
	}

	if primary.callCount != 3 {
		t.Errorf("expected primary to be called 3 times, got %d", primary.callCount)
	}

	// Circuit should now be open for primary
	resp, err := failover.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err != nil {
		t.Fatalf("request with open circuit failed: %v", err)
	}

	// Should skip primary and go directly to secondary
	if primary.callCount != 3 {
		t.Errorf("expected primary to still be called 3 times (circuit open), got %d", primary.callCount)
	}

	if secondary.callCount != 4 {
		t.Errorf("expected secondary to be called 4 times, got %d", secondary.callCount)
	}

	// Wait for circuit to recover
	time.Sleep(150 * time.Millisecond)

	// Circuit should now try primary again (half-open)
	primary.shouldFail = false
	primary.successResp = &CompletionResponse{
		Content: "primary recovered",
	}

	resp, err = failover.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err != nil {
		t.Fatalf("request after recovery failed: %v", err)
	}

	if resp.Content != "primary recovered" {
		t.Errorf("expected 'primary recovered', got '%s'", resp.Content)
	}

	if primary.callCount != 4 {
		t.Errorf("expected primary to be called 4 times (recovered), got %d", primary.callCount)
	}
}

func TestFailoverProvider_StreamFailover(t *testing.T) {
	registry := NewRegistry()

	primary := &mockFailoverProvider{
		name:       "primary",
		shouldFail: true,
		failWith:   &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"},
	}

	secondary := &mockFailoverProvider{
		name:       "secondary",
		shouldFail: false,
	}

	registry.Register(primary)
	registry.Register(secondary)

	config := FailoverConfig{
		ProviderOrder:           []string{"primary", "secondary"},
		CircuitBreakerThreshold: 0,
	}

	failover, err := NewFailoverProvider(registry, config)
	if err != nil {
		t.Fatalf("failed to create failover provider: %v", err)
	}

	ctx := context.Background()
	chunks, err := failover.Stream(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err != nil {
		t.Fatalf("expected no error after failover, got %v", err)
	}

	var content string
	for chunk := range chunks {
		content += chunk.Delta.Content
	}

	if content != "success" {
		t.Errorf("expected 'success', got '%s'", content)
	}

	if primary.callCount != 1 {
		t.Errorf("expected primary to be called once, got %d", primary.callCount)
	}

	if secondary.callCount != 1 {
		t.Errorf("expected secondary to be called once, got %d", secondary.callCount)
	}
}

func TestShouldFailover(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name: "HTTP 500",
			err: &pkgerrors.ProviderError{
				Provider:   "test",
				StatusCode: http.StatusInternalServerError,
				Message:    "internal error",
			},
			expected: true,
		},
		{
			name: "HTTP 502",
			err: &pkgerrors.ProviderError{
				Provider:   "test",
				StatusCode: http.StatusBadGateway,
				Message:    "bad gateway",
			},
			expected: true,
		},
		{
			name: "HTTP 503",
			err: &pkgerrors.ProviderError{
				Provider:   "test",
				StatusCode: http.StatusServiceUnavailable,
				Message:    "service unavailable",
			},
			expected: true,
		},
		{
			name: "HTTP 429",
			err: &pkgerrors.ProviderError{
				Provider:   "test",
				StatusCode: http.StatusTooManyRequests,
				Message:    "rate limited",
			},
			expected: true,
		},
		{
			name: "HTTP 408",
			err: &pkgerrors.ProviderError{
				Provider:   "test",
				StatusCode: http.StatusRequestTimeout,
				Message:    "timeout",
			},
			expected: true,
		},
		{
			name: "HTTP 400",
			err: &pkgerrors.ProviderError{
				Provider:   "test",
				StatusCode: http.StatusBadRequest,
				Message:    "bad request",
			},
			expected: false,
		},
		{
			name: "HTTP 401",
			err: &pkgerrors.ProviderError{
				Provider:   "test",
				StatusCode: http.StatusUnauthorized,
				Message:    "unauthorized",
			},
			expected: false,
		},
		{
			name: "HTTP 403",
			err: &pkgerrors.ProviderError{
				Provider:   "test",
				StatusCode: http.StatusForbidden,
				Message:    "forbidden",
			},
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "circuit breaker open",
			err:      ErrCircuitOpen,
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldFailover(tt.err)
			if result != tt.expected {
				t.Errorf("shouldFailover(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestCircuitBreaker_Basic(t *testing.T) {
	cb := newCircuitBreaker(3, 100*time.Millisecond)

	// Initially should allow requests
	if !cb.allowRequest("test-provider") {
		t.Error("expected circuit to allow initial request")
	}

	// Record failures up to threshold
	cb.recordFailure("test-provider")
	cb.recordFailure("test-provider")

	// Should still allow requests (below threshold)
	if !cb.allowRequest("test-provider") {
		t.Error("expected circuit to allow requests below threshold")
	}

	// One more failure should open the circuit
	cb.recordFailure("test-provider")

	// Should now block requests
	if cb.allowRequest("test-provider") {
		t.Error("expected circuit to block requests when open")
	}

	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)

	// Should now allow requests again (half-open)
	if !cb.allowRequest("test-provider") {
		t.Error("expected circuit to allow requests after timeout")
	}

	// Record success should reset state
	cb.recordSuccess("test-provider")

	// Should allow requests normally
	if !cb.allowRequest("test-provider") {
		t.Error("expected circuit to allow requests after success")
	}

	// Check status
	status := cb.getStatus()
	if providerStatus, ok := status["test-provider"]; !ok {
		t.Error("expected status for test-provider")
	} else {
		if providerStatus.Open {
			t.Error("expected circuit to be closed")
		}
		if providerStatus.ConsecutiveFailures != 0 {
			t.Errorf("expected 0 consecutive failures, got %d", providerStatus.ConsecutiveFailures)
		}
	}
}

func TestNewFailoverProvider_Validation(t *testing.T) {
	registry := NewRegistry()

	// Test empty provider order
	_, err := NewFailoverProvider(registry, FailoverConfig{
		ProviderOrder: []string{},
	})

	if err == nil {
		t.Error("expected error for empty provider order")
	}

	// Test non-existent provider
	_, err = NewFailoverProvider(registry, FailoverConfig{
		ProviderOrder: []string{"non-existent"},
	})

	if err == nil {
		t.Error("expected error for non-existent provider")
	}
}
