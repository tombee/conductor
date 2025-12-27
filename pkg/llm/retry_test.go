package llm

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	pkgerrors "github.com/tombee/conductor/pkg/errors"
)

// mockRetryProvider is a test provider that can simulate failures.
type mockRetryProvider struct {
	name           string
	failCount      int
	currentAttempt int
	failWith       error
	successResp    *CompletionResponse
}

func (m *mockRetryProvider) Name() string {
	return m.name
}

func (m *mockRetryProvider) Capabilities() Capabilities {
	return Capabilities{}
}

func (m *mockRetryProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	m.currentAttempt++

	if m.currentAttempt <= m.failCount {
		return nil, m.failWith
	}

	return m.successResp, nil
}

func (m *mockRetryProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	m.currentAttempt++

	if m.currentAttempt <= m.failCount {
		return nil, m.failWith
	}

	chunks := make(chan StreamChunk, 1)
	go func() {
		defer close(chunks)
		chunks <- StreamChunk{
			Delta: StreamDelta{Content: "test"},
		}
	}()

	return chunks, nil
}

func TestRetryableProvider_SuccessFirstAttempt(t *testing.T) {
	mock := &mockRetryProvider{
		name:      "test",
		failCount: 0,
		successResp: &CompletionResponse{
			Content: "success",
		},
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond // Speed up test

	retry := NewRetryableProvider(mock, config)

	ctx := context.Background()
	resp, err := retry.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Content != "success" {
		t.Errorf("expected content 'success', got '%s'", resp.Content)
	}

	if mock.currentAttempt != 1 {
		t.Errorf("expected 1 attempt, got %d", mock.currentAttempt)
	}
}

func TestRetryableProvider_SuccessAfterRetries(t *testing.T) {
	mock := &mockRetryProvider{
		name:      "test",
		failCount: 2,
		failWith:  &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"},
		successResp: &CompletionResponse{
			Content: "success",
		},
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond // Speed up test
	config.MaxRetries = 3

	retry := NewRetryableProvider(mock, config)

	ctx := context.Background()
	resp, err := retry.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err != nil {
		t.Fatalf("expected no error after retries, got %v", err)
	}

	if resp.Content != "success" {
		t.Errorf("expected content 'success', got '%s'", resp.Content)
	}

	if mock.currentAttempt != 3 {
		t.Errorf("expected 3 attempts, got %d", mock.currentAttempt)
	}
}

func TestRetryableProvider_MaxRetriesExceeded(t *testing.T) {
	mock := &mockRetryProvider{
		name:      "test",
		failCount: 10, // Always fail
		failWith:  &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"},
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond // Speed up test
	config.MaxRetries = 2

	retry := NewRetryableProvider(mock, config)

	ctx := context.Background()
	_, err := retry.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify we get a ProviderError wrapping the max retries exceeded message
	var provErr *pkgerrors.ProviderError
	if !errors.As(err, &provErr) {
		t.Errorf("expected ProviderError, got %T: %v", err, err)
	}

	if mock.currentAttempt != 3 {
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", mock.currentAttempt)
	}
}

func TestRetryableProvider_NonRetryableError(t *testing.T) {
	mock := &mockRetryProvider{
		name:      "test",
		failCount: 10, // Always fail
		failWith:  &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusUnauthorized, Message: "unauthorized"},
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond // Speed up test
	config.MaxRetries = 3

	retry := NewRetryableProvider(mock, config)

	ctx := context.Background()
	_, err := retry.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should fail immediately without retries
	if mock.currentAttempt != 1 {
		t.Errorf("expected 1 attempt (no retries for 401), got %d", mock.currentAttempt)
	}

	var httpErr *pkgerrors.ProviderError
	if !errors.As(err, &httpErr) {
		t.Errorf("expected HTTPError, got %T", err)
	}
}

func TestRetryableProvider_ContextCancelled(t *testing.T) {
	mock := &mockRetryProvider{
		name:      "test",
		failCount: 10, // Always fail
		failWith:  &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"},
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 100 * time.Millisecond
	config.MaxRetries = 5

	retry := NewRetryableProvider(mock, config)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := retry.Complete(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestRetryableProvider_StreamSuccess(t *testing.T) {
	mock := &mockRetryProvider{
		name:      "test",
		failCount: 0,
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond

	retry := NewRetryableProvider(mock, config)

	ctx := context.Background()
	chunks, err := retry.Stream(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var content string
	for chunk := range chunks {
		content += chunk.Delta.Content
	}

	if content != "test" {
		t.Errorf("expected content 'test', got '%s'", content)
	}
}

func TestRetryableProvider_StreamRetry(t *testing.T) {
	mock := &mockRetryProvider{
		name:      "test",
		failCount: 2,
		failWith:  &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"},
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond
	config.MaxRetries = 3

	retry := NewRetryableProvider(mock, config)

	ctx := context.Background()
	chunks, err := retry.Stream(ctx, CompletionRequest{
		Messages: []Message{{Content: "test"}},
	})

	if err != nil {
		t.Fatalf("expected no error after retries, got %v", err)
	}

	var content string
	for chunk := range chunks {
		content += chunk.Delta.Content
	}

	if content != "test" {
		t.Errorf("expected content 'test', got '%s'", content)
	}

	if mock.currentAttempt != 3 {
		t.Errorf("expected 3 attempts, got %d", mock.currentAttempt)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "HTTP 500",
			err:       &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusInternalServerError, Message: "internal error"},
			retryable: true,
		},
		{
			name:      "HTTP 502",
			err:       &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusBadGateway, Message: "bad gateway"},
			retryable: true,
		},
		{
			name:      "HTTP 503",
			err:       &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"},
			retryable: true,
		},
		{
			name:      "HTTP 429",
			err:       &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusTooManyRequests, Message: "rate limited"},
			retryable: true,
		},
		{
			name:      "HTTP 400",
			err:       &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusBadRequest, Message: "bad request"},
			retryable: false,
		},
		{
			name:      "HTTP 401",
			err:       &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusUnauthorized, Message: "unauthorized"},
			retryable: false,
		},
		{
			name:      "HTTP 403",
			err:       &pkgerrors.ProviderError{Provider: "test", StatusCode: http.StatusForbidden, Message: "forbidden"},
			retryable: false,
		},
		{
			name:      "context cancelled",
			err:       context.Canceled,
			retryable: false,
		},
		{
			name:      "context deadline exceeded",
			err:       context.DeadlineExceeded,
			retryable: false,
		},
		{
			name:      "generic error",
			err:       errors.New("generic error"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, result, tt.retryable)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0,
	}

	wrapper := NewRetryableProvider(nil, config)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{attempt: 1, expected: 100 * time.Millisecond},
		{attempt: 2, expected: 200 * time.Millisecond},
		{attempt: 3, expected: 400 * time.Millisecond},
		{attempt: 4, expected: 800 * time.Millisecond},
		{attempt: 5, expected: 1600 * time.Millisecond},
		{attempt: 6, expected: 3200 * time.Millisecond},
		{attempt: 7, expected: 5000 * time.Millisecond}, // Capped at MaxDelay
		{attempt: 8, expected: 5000 * time.Millisecond}, // Still capped
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := wrapper.calculateBackoff(tt.attempt)
			if delay != tt.expected {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tt.attempt, delay, tt.expected)
			}
		})
	}
}

func TestCalculateBackoffWithJitter(t *testing.T) {
	config := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.2, // 20% jitter
	}

	wrapper := NewRetryableProvider(nil, config)

	// With jitter, delay should be within ±20% of expected
	attempt := 3
	expectedBase := 400 * time.Millisecond
	minDelay := float64(expectedBase) * 0.8
	maxDelay := float64(expectedBase) * 1.2

	// Run multiple times to test randomness
	for i := 0; i < 100; i++ {
		delay := wrapper.calculateBackoff(attempt)
		if float64(delay) < minDelay || float64(delay) > maxDelay {
			t.Errorf("calculateBackoff(%d) = %v, want between %v and %v", attempt, delay, time.Duration(minDelay), time.Duration(maxDelay))
		}
	}
}
