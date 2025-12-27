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
	"strings"
	"testing"

	conductorerrors "github.com/tombee/conductor/pkg/errors"
)

func TestWrap(t *testing.T) {
	t.Run("wraps error with context", func(t *testing.T) {
		original := errors.New("original error")
		wrapped := conductorerrors.Wrap(original, "additional context")

		if wrapped == nil {
			t.Fatal("Wrap should not return nil for non-nil error")
		}

		msg := wrapped.Error()
		if !strings.Contains(msg, "additional context") {
			t.Errorf("wrapped error should contain context, got: %s", msg)
		}
		if !strings.Contains(msg, "original error") {
			t.Errorf("wrapped error should contain original message, got: %s", msg)
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		wrapped := conductorerrors.Wrap(nil, "context")
		if wrapped != nil {
			t.Errorf("Wrap(nil, _) should return nil, got: %v", wrapped)
		}
	})

	t.Run("preserves error chain", func(t *testing.T) {
		original := errors.New("root cause")
		wrapped := conductorerrors.Wrap(original, "context")

		if !errors.Is(wrapped, original) {
			t.Error("wrapped error should match original with errors.Is")
		}

		unwrapped := errors.Unwrap(wrapped)
		if unwrapped != original {
			t.Errorf("Unwrap should return original error, got: %v", unwrapped)
		}
	})
}

func TestWrapf(t *testing.T) {
	t.Run("wraps error with formatted context", func(t *testing.T) {
		original := errors.New("file not found")
		wrapped := conductorerrors.Wrapf(original, "loading file %s", "/path/to/file")

		if wrapped == nil {
			t.Fatal("Wrapf should not return nil for non-nil error")
		}

		msg := wrapped.Error()
		if !strings.Contains(msg, "loading file /path/to/file") {
			t.Errorf("wrapped error should contain formatted context, got: %s", msg)
		}
		if !strings.Contains(msg, "file not found") {
			t.Errorf("wrapped error should contain original message, got: %s", msg)
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		wrapped := conductorerrors.Wrapf(nil, "loading file %s", "/path/to/file")
		if wrapped != nil {
			t.Errorf("Wrapf(nil, _, _) should return nil, got: %v", wrapped)
		}
	})

	t.Run("handles multiple format arguments", func(t *testing.T) {
		original := errors.New("connection failed")
		wrapped := conductorerrors.Wrapf(original, "connecting to %s:%d", "localhost", 8080)

		msg := wrapped.Error()
		if !strings.Contains(msg, "connecting to localhost:8080") {
			t.Errorf("wrapped error should contain formatted context, got: %s", msg)
		}
	})

	t.Run("preserves error chain", func(t *testing.T) {
		original := errors.New("root cause")
		wrapped := conductorerrors.Wrapf(original, "context: %s", "details")

		if !errors.Is(wrapped, original) {
			t.Error("wrapped error should match original with errors.Is")
		}
	})
}

func TestIs(t *testing.T) {
	t.Run("finds error in chain", func(t *testing.T) {
		target := &conductorerrors.ValidationError{Field: "test"}
		wrapped := conductorerrors.Wrap(target, "wrapper")

		if !conductorerrors.Is(wrapped, target) {
			t.Error("Is should find target error in chain")
		}
	})

	t.Run("returns false for different error", func(t *testing.T) {
		err := &conductorerrors.ValidationError{Field: "test"}
		target := &conductorerrors.NotFoundError{Resource: "test"}

		if conductorerrors.Is(err, target) {
			t.Error("Is should return false for different error types")
		}
	})

	t.Run("returns false for nil error", func(t *testing.T) {
		target := &conductorerrors.ValidationError{Field: "test"}

		if conductorerrors.Is(nil, target) {
			t.Error("Is should return false for nil error")
		}
	})
}

func TestAs(t *testing.T) {
	t.Run("extracts typed error from chain", func(t *testing.T) {
		original := &conductorerrors.ValidationError{
			Field:   "email",
			Message: "invalid format",
		}
		wrapped := conductorerrors.Wrap(original, "validation failed")

		var target *conductorerrors.ValidationError
		if !conductorerrors.As(wrapped, &target) {
			t.Fatal("As should extract ValidationError from chain")
		}

		if target.Field != "email" {
			t.Errorf("extracted error Field = %q, want %q", target.Field, "email")
		}
		if target.Message != "invalid format" {
			t.Errorf("extracted error Message = %q, want %q", target.Message, "invalid format")
		}
	})

	t.Run("returns false for different error type", func(t *testing.T) {
		err := &conductorerrors.ValidationError{Field: "test"}

		var target *conductorerrors.NotFoundError
		if conductorerrors.As(err, &target) {
			t.Error("As should return false when error type doesn't match")
		}
	})

	t.Run("returns false for nil error", func(t *testing.T) {
		var target *conductorerrors.ValidationError
		if conductorerrors.As(nil, &target) {
			t.Error("As should return false for nil error")
		}
	})

	t.Run("extracts all error types", func(t *testing.T) {
		tests := []struct {
			name   string
			err    error
			target interface{}
		}{
			{
				name:   "NotFoundError",
				err:    &conductorerrors.NotFoundError{Resource: "test", ID: "123"},
				target: &conductorerrors.NotFoundError{},
			},
			{
				name:   "ProviderError",
				err:    &conductorerrors.ProviderError{Provider: "test"},
				target: &conductorerrors.ProviderError{},
			},
			{
				name:   "ConfigError",
				err:    &conductorerrors.ConfigError{Key: "test"},
				target: &conductorerrors.ConfigError{},
			},
			{
				name:   "TimeoutError",
				err:    &conductorerrors.TimeoutError{Operation: "test"},
				target: &conductorerrors.TimeoutError{},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				wrapped := conductorerrors.Wrap(tt.err, "wrapper")
				if !conductorerrors.As(wrapped, &tt.target) {
					t.Errorf("As should extract %s from chain", tt.name)
				}
			})
		}
	})
}

func TestUnwrap(t *testing.T) {
	t.Run("unwraps single level", func(t *testing.T) {
		original := errors.New("original")
		wrapped := conductorerrors.Wrap(original, "wrapper")

		unwrapped := conductorerrors.Unwrap(wrapped)
		if unwrapped != original {
			t.Errorf("Unwrap should return original error, got: %v", unwrapped)
		}
	})

	t.Run("returns nil for error without cause", func(t *testing.T) {
		err := errors.New("simple error")
		unwrapped := conductorerrors.Unwrap(err)
		if unwrapped != nil {
			t.Errorf("Unwrap should return nil for error without cause, got: %v", unwrapped)
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		unwrapped := conductorerrors.Unwrap(nil)
		if unwrapped != nil {
			t.Errorf("Unwrap(nil) should return nil, got: %v", unwrapped)
		}
	})
}

func TestNew(t *testing.T) {
	t.Run("creates new error", func(t *testing.T) {
		err := conductorerrors.New("test error")
		if err == nil {
			t.Fatal("New should create non-nil error")
		}

		if err.Error() != "test error" {
			t.Errorf("error message = %q, want %q", err.Error(), "test error")
		}
	})

	t.Run("creates unique error instances", func(t *testing.T) {
		err1 := conductorerrors.New("test")
		err2 := conductorerrors.New("test")

		if err1 == err2 {
			t.Error("New should create unique error instances")
		}
	})
}
