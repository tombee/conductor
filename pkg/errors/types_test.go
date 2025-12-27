// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	conductorerrors "github.com/tombee/conductor/pkg/errors"
)

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *conductorerrors.ValidationError
		wantMsg string
	}{
		{
			name: "with field",
			err: &conductorerrors.ValidationError{
				Field:      "api_key",
				Message:    "required field is missing",
				Suggestion: "Set the API key in config",
			},
			wantMsg: "validation failed on api_key: required field is missing",
		},
		{
			name: "without field",
			err: &conductorerrors.ValidationError{
				Message:    "invalid format",
				Suggestion: "Check the input format",
			},
			wantMsg: "validation failed: invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("ValidationError.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestNotFoundError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *conductorerrors.NotFoundError
		wantMsg string
	}{
		{
			name: "workflow not found",
			err: &conductorerrors.NotFoundError{
				Resource: "workflow",
				ID:       "my-workflow",
			},
			wantMsg: "workflow not found: my-workflow",
		},
		{
			name: "tool not found",
			err: &conductorerrors.NotFoundError{
				Resource: "tool",
				ID:       "http_client",
			},
			wantMsg: "tool not found: http_client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("NotFoundError.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestProviderError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *conductorerrors.ProviderError
		want    []string // strings that should appear in error message
		notWant []string // strings that should not appear
	}{
		{
			name: "full error with all fields",
			err: &conductorerrors.ProviderError{
				Provider:   "anthropic",
				Code:       429,
				StatusCode: 429,
				Message:    "rate limit exceeded",
				RequestID:  "req_123",
			},
			want:    []string{"anthropic", "429", "HTTP 429", "rate limit exceeded", "req_123"},
			notWant: []string{},
		},
		{
			name: "minimal error",
			err: &conductorerrors.ProviderError{
				Provider: "openai",
				Message:  "connection failed",
			},
			want:    []string{"openai", "connection failed"},
			notWant: []string{"HTTP", "request-id"},
		},
		{
			name: "with status code only",
			err: &conductorerrors.ProviderError{
				Provider:   "ollama",
				StatusCode: 500,
				Message:    "internal server error",
			},
			want:    []string{"ollama", "HTTP 500", "internal server error"},
			notWant: []string{"request-id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("ProviderError.Error() = %q, want to contain %q", got, want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(got, notWant) {
					t.Errorf("ProviderError.Error() = %q, should not contain %q", got, notWant)
				}
			}
		})
	}
}

func TestProviderError_Unwrap(t *testing.T) {
	cause := errors.New("network error")
	err := &conductorerrors.ProviderError{
		Provider: "anthropic",
		Message:  "request failed",
		Cause:    cause,
	}

	if got := err.Unwrap(); got != cause {
		t.Errorf("ProviderError.Unwrap() = %v, want %v", got, cause)
	}
}

func TestConfigError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *conductorerrors.ConfigError
		wantMsg string
	}{
		{
			name: "with key",
			err: &conductorerrors.ConfigError{
				Key:    "database.host",
				Reason: "hostname is invalid",
			},
			wantMsg: "config error at database.host: hostname is invalid",
		},
		{
			name: "without key",
			err: &conductorerrors.ConfigError{
				Reason: "file not found",
			},
			wantMsg: "config error: file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("ConfigError.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestConfigError_Unwrap(t *testing.T) {
	cause := errors.New("file read error")
	err := &conductorerrors.ConfigError{
		Key:    "config",
		Reason: "failed to load",
		Cause:  cause,
	}

	if got := err.Unwrap(); got != cause {
		t.Errorf("ConfigError.Unwrap() = %v, want %v", got, cause)
	}
}

func TestTimeoutError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *conductorerrors.TimeoutError
		want    []string
		notWant []string
	}{
		{
			name: "llm timeout",
			err: &conductorerrors.TimeoutError{
				Operation: "LLM request",
				Duration:  30 * time.Second,
			},
			want:    []string{"LLM request", "30s"},
			notWant: []string{},
		},
		{
			name: "workflow step timeout",
			err: &conductorerrors.TimeoutError{
				Operation: "workflow step execution",
				Duration:  2 * time.Minute,
			},
			want:    []string{"workflow step execution", "2m0s"},
			notWant: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("TimeoutError.Error() = %q, want to contain %q", got, want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(got, notWant) {
					t.Errorf("TimeoutError.Error() = %q, should not contain %q", got, notWant)
				}
			}
		})
	}
}

func TestTimeoutError_Unwrap(t *testing.T) {
	cause := errors.New("context deadline exceeded")
	err := &conductorerrors.TimeoutError{
		Operation: "test",
		Duration:  5 * time.Second,
		Cause:     cause,
	}

	if got := err.Unwrap(); got != cause {
		t.Errorf("TimeoutError.Unwrap() = %v, want %v", got, cause)
	}
}

// Test error wrapping with fmt.Errorf
func TestErrorWrapping(t *testing.T) {
	t.Run("ValidationError can be wrapped", func(t *testing.T) {
		original := &conductorerrors.ValidationError{
			Field:   "email",
			Message: "invalid format",
		}
		wrapped := fmt.Errorf("user input validation: %w", original)

		var target *conductorerrors.ValidationError
		if !errors.As(wrapped, &target) {
			t.Error("errors.As should find ValidationError in wrapped error")
		}
		if target.Field != "email" {
			t.Errorf("unwrapped error Field = %q, want %q", target.Field, "email")
		}
	})

	t.Run("NotFoundError can be wrapped", func(t *testing.T) {
		original := &conductorerrors.NotFoundError{
			Resource: "workflow",
			ID:       "test",
		}
		wrapped := fmt.Errorf("loading workflow: %w", original)

		var target *conductorerrors.NotFoundError
		if !errors.As(wrapped, &target) {
			t.Error("errors.As should find NotFoundError in wrapped error")
		}
		if target.Resource != "workflow" {
			t.Errorf("unwrapped error Resource = %q, want %q", target.Resource, "workflow")
		}
	})

	t.Run("ProviderError preserves cause through wrapping", func(t *testing.T) {
		rootCause := errors.New("network timeout")
		providerErr := &conductorerrors.ProviderError{
			Provider: "anthropic",
			Message:  "request failed",
			Cause:    rootCause,
		}
		wrapped := fmt.Errorf("executing LLM call: %w", providerErr)

		// Should be able to extract provider error
		var target *conductorerrors.ProviderError
		if !errors.As(wrapped, &target) {
			t.Error("errors.As should find ProviderError in wrapped error")
		}

		// Should be able to unwrap to root cause
		if target.Unwrap() != rootCause {
			t.Error("ProviderError.Unwrap() should return root cause")
		}
	})

	t.Run("ConfigError preserves cause through wrapping", func(t *testing.T) {
		rootCause := errors.New("file not found")
		configErr := &conductorerrors.ConfigError{
			Key:    "api_key",
			Reason: "missing required field",
			Cause:  rootCause,
		}
		wrapped := fmt.Errorf("loading config: %w", configErr)

		var target *conductorerrors.ConfigError
		if !errors.As(wrapped, &target) {
			t.Error("errors.As should find ConfigError in wrapped error")
		}

		if target.Unwrap() != rootCause {
			t.Error("ConfigError.Unwrap() should return root cause")
		}
	})

	t.Run("TimeoutError preserves cause through wrapping", func(t *testing.T) {
		rootCause := errors.New("context deadline exceeded")
		timeoutErr := &conductorerrors.TimeoutError{
			Operation: "test",
			Duration:  5 * time.Second,
			Cause:     rootCause,
		}
		wrapped := fmt.Errorf("operation timeout: %w", timeoutErr)

		var target *conductorerrors.TimeoutError
		if !errors.As(wrapped, &target) {
			t.Error("errors.As should find TimeoutError in wrapped error")
		}

		if target.Unwrap() != rootCause {
			t.Error("TimeoutError.Unwrap() should return root cause")
		}
	})
}

// Test errors.Is behavior
func TestErrorsIs(t *testing.T) {
	t.Run("errors.Is works with wrapped ValidationError", func(t *testing.T) {
		original := &conductorerrors.ValidationError{Field: "test"}
		wrapped := fmt.Errorf("wrapper: %w", original)

		// errors.Is should find the original error
		if !errors.Is(wrapped, original) {
			t.Error("errors.Is should find original error in chain")
		}
	})

	t.Run("errors.Is works with wrapped NotFoundError", func(t *testing.T) {
		original := &conductorerrors.NotFoundError{Resource: "test", ID: "123"}
		wrapped := fmt.Errorf("wrapper: %w", original)

		if !errors.Is(wrapped, original) {
			t.Error("errors.Is should find original error in chain")
		}
	})
}
