package transport

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *RetryConfig
		wantErr bool
	}{
		{
			name: "valid default config",
			config: &RetryConfig{
				MaxAttempts:     3,
				InitialBackoff:  1 * time.Second,
				MaxBackoff:      30 * time.Second,
				BackoffFactor:   2.0,
				RetryableErrors: []int{429, 500, 502, 503, 504},
			},
			wantErr: false,
		},
		{
			name: "max_attempts too low",
			config: &RetryConfig{
				MaxAttempts:    0,
				InitialBackoff: 1 * time.Second,
				MaxBackoff:     30 * time.Second,
				BackoffFactor:  2.0,
			},
			wantErr: true,
		},
		{
			name: "negative initial_backoff",
			config: &RetryConfig{
				MaxAttempts:    3,
				InitialBackoff: -1 * time.Second,
				MaxBackoff:     30 * time.Second,
				BackoffFactor:  2.0,
			},
			wantErr: true,
		},
		{
			name: "max_backoff less than initial_backoff",
			config: &RetryConfig{
				MaxAttempts:    3,
				InitialBackoff: 30 * time.Second,
				MaxBackoff:     1 * time.Second,
				BackoffFactor:  2.0,
			},
			wantErr: true,
		},
		{
			name: "backoff_factor less than 1.0",
			config: &RetryConfig{
				MaxAttempts:    3,
				InitialBackoff: 1 * time.Second,
				MaxBackoff:     30 * time.Second,
				BackoffFactor:  0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRetryConfig_IsRetryable(t *testing.T) {
	config := &RetryConfig{
		RetryableErrors: []int{408, 429, 500, 502, 503, 504},
	}

	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"408 Request Timeout", 408, true},
		{"429 Too Many Requests", 429, true},
		{"500 Internal Server Error", 500, true},
		{"502 Bad Gateway", 502, true},
		{"503 Service Unavailable", 503, true},
		{"504 Gateway Timeout", 504, true},
		{"400 Bad Request", 400, false},
		{"401 Unauthorized", 401, false},
		{"404 Not Found", 404, false},
		{"200 OK", 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := config.IsRetryable(tt.statusCode); got != tt.want {
				t.Errorf("IsRetryable(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := &RetryConfig{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	}

	tests := []struct {
		name       string
		attempt    int
		retryAfter time.Duration
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{
			name:       "first retry",
			attempt:    1,
			retryAfter: 0,
			wantMin:    1 * time.Second,              // base delay
			wantMax:    1*time.Second + 100*time.Millisecond, // base + max jitter
		},
		{
			name:       "second retry",
			attempt:    2,
			retryAfter: 0,
			wantMin:    2 * time.Second,              // 1s * 2^1
			wantMax:    2*time.Second + 100*time.Millisecond,
		},
		{
			name:       "third retry",
			attempt:    3,
			retryAfter: 0,
			wantMin:    4 * time.Second,              // 1s * 2^2
			wantMax:    4*time.Second + 100*time.Millisecond,
		},
		{
			name:       "capped at max_backoff",
			attempt:    10,
			retryAfter: 0,
			wantMin:    30 * time.Second,             // capped at MaxBackoff
			wantMax:    30*time.Second + 100*time.Millisecond,
		},
		{
			name:       "retry-after overrides if larger",
			attempt:    1,
			retryAfter: 5 * time.Second,
			wantMin:    5 * time.Second,              // retry-after wins
			wantMax:    5*time.Second + 100*time.Millisecond,
		},
		{
			name:       "calculated backoff wins if larger than retry-after",
			attempt:    3,
			retryAfter: 1 * time.Second,
			wantMin:    4 * time.Second,              // calculated (4s) > retry-after (1s)
			wantMax:    4*time.Second + 100*time.Millisecond,
		},
		{
			name:       "retry-after capped at max_backoff",
			attempt:    1,
			retryAfter: 60 * time.Second,
			wantMin:    30 * time.Second,             // capped at MaxBackoff
			wantMax:    30*time.Second + 100*time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := calculateBackoff(config, tt.attempt, tt.retryAfter)

			if delay < tt.wantMin || delay > tt.wantMax {
				t.Errorf("calculateBackoff() = %v, want between %v and %v", delay, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestExtractRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     time.Duration
	}{
		{
			name:     "no metadata",
			metadata: nil,
			want:     0,
		},
		{
			name:     "no retry-after header",
			metadata: map[string]interface{}{"foo": "bar"},
			want:     0,
		},
		{
			name:     "numeric retry-after (seconds)",
			metadata: map[string]interface{}{"retry_after": "120"},
			want:     120 * time.Second,
		},
		{
			name:     "numeric retry-after (zero)",
			metadata: map[string]interface{}{"retry_after": "0"},
			want:     0,
		},
		{
			name:     "malformed retry-after",
			metadata: map[string]interface{}{"retry_after": "invalid"},
			want:     0,
		},
		{
			name:     "wrong type",
			metadata: map[string]interface{}{"retry_after": 123},
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &TransportError{
				Metadata: tt.metadata,
			}

			got := extractRetryAfter(err)
			if got != tt.want {
				t.Errorf("extractRetryAfter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecute_Success(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()

	expectedResp := &Response{
		StatusCode: 200,
		Body:       []byte("success"),
	}

	callCount := 0
	fn := func(ctx context.Context) (*Response, error) {
		callCount++
		return expectedResp, nil
	}

	resp, err := Execute(ctx, config, fn)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if resp != expectedResp {
		t.Errorf("Execute() response mismatch")
	}
	if callCount != 1 {
		t.Errorf("Execute() called function %d times, want 1", callCount)
	}

	// Check retry count metadata
	if retryCount, ok := resp.Metadata[MetadataRetryCount].(int); !ok || retryCount != 0 {
		t.Errorf("Execute() retry_count = %v, want 0", resp.Metadata[MetadataRetryCount])
	}
}

func TestExecute_RetryableError(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      100 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{500, 503},
	}

	callCount := 0
	fn := func(ctx context.Context) (*Response, error) {
		callCount++
		if callCount < 3 {
			// Fail first 2 attempts
			return nil, &TransportError{
				Type:       ErrorTypeServer,
				StatusCode: 503,
				Message:    "service unavailable",
				Retryable:  true,
			}
		}
		// Succeed on 3rd attempt
		return &Response{StatusCode: 200, Body: []byte("success")}, nil
	}

	start := time.Now()
	resp, err := Execute(ctx, config, fn)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Execute() status code = %d, want 200", resp.StatusCode)
	}
	if callCount != 3 {
		t.Errorf("Execute() called function %d times, want 3", callCount)
	}

	// Should have waited for 2 retries (10ms + 20ms + jitter)
	// Expect at least 30ms (no jitter) and less than 200ms (with max jitter)
	if elapsed < 30*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Errorf("Execute() elapsed time = %v, want ~30-200ms", elapsed)
	}

	// Check retry count metadata
	if retryCount, ok := resp.Metadata[MetadataRetryCount].(int); !ok || retryCount != 2 {
		t.Errorf("Execute() retry_count = %v, want 2", resp.Metadata[MetadataRetryCount])
	}
}

func TestExecute_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()

	callCount := 0
	fn := func(ctx context.Context) (*Response, error) {
		callCount++
		return nil, &TransportError{
			Type:       ErrorTypeClient,
			StatusCode: 400,
			Message:    "bad request",
			Retryable:  false,
		}
	}

	_, err := Execute(ctx, config, fn)
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}

	transportErr, ok := err.(*TransportError)
	if !ok {
		t.Fatalf("Execute() error type = %T, want *TransportError", err)
	}
	if transportErr.StatusCode != 400 {
		t.Errorf("Execute() error status code = %d, want 400", transportErr.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("Execute() called function %d times, want 1 (no retries)", callCount)
	}
}

func TestExecute_MaxRetriesExhausted(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  1 * time.Millisecond,
		MaxBackoff:      10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{500},
	}

	callCount := 0
	fn := func(ctx context.Context) (*Response, error) {
		callCount++
		return nil, &TransportError{
			Type:       ErrorTypeServer,
			StatusCode: 500,
			Message:    "internal server error",
			Retryable:  true,
		}
	}

	_, err := Execute(ctx, config, fn)
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}

	if callCount != 3 {
		t.Errorf("Execute() called function %d times, want 3", callCount)
	}
}

func TestExecute_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := &RetryConfig{
		MaxAttempts:     5,
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      1 * time.Second,
		BackoffFactor:   2.0,
		RetryableErrors: []int{500},
	}

	callCount := 0
	fn := func(ctx context.Context) (*Response, error) {
		callCount++
		if callCount == 2 {
			// Cancel context after first retry
			cancel()
		}
		return nil, &TransportError{
			Type:       ErrorTypeServer,
			StatusCode: 500,
			Message:    "server error",
			Retryable:  true,
		}
	}

	_, err := Execute(ctx, config, fn)
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}

	transportErr, ok := err.(*TransportError)
	if !ok {
		t.Fatalf("Execute() error type = %T, want *TransportError", err)
	}
	if transportErr.Type != ErrorTypeCancelled {
		t.Errorf("Execute() error type = %v, want %v", transportErr.Type, ErrorTypeCancelled)
	}

	// Should have stopped after cancellation
	if callCount > 3 {
		t.Errorf("Execute() called function %d times, want <= 3 (should stop on cancel)", callCount)
	}
}

func TestExecute_UnknownError(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()

	unknownErr := errors.New("unknown error type")

	callCount := 0
	fn := func(ctx context.Context) (*Response, error) {
		callCount++
		return nil, unknownErr
	}

	_, err := Execute(ctx, config, fn)
	if err != unknownErr {
		t.Errorf("Execute() error = %v, want %v", err, unknownErr)
	}

	// Unknown errors should not be retried
	if callCount != 1 {
		t.Errorf("Execute() called function %d times, want 1 (no retries for unknown error)", callCount)
	}
}

func TestPow(t *testing.T) {
	tests := []struct {
		base float64
		exp  int
		want float64
	}{
		{2.0, 0, 1.0},
		{2.0, 1, 2.0},
		{2.0, 2, 4.0},
		{2.0, 3, 8.0},
		{2.0, 10, 1024.0},
		{1.5, 3, 3.375},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := pow(tt.base, tt.exp)
			if got != tt.want {
				t.Errorf("pow(%v, %d) = %v, want %v", tt.base, tt.exp, got, tt.want)
			}
		})
	}
}
