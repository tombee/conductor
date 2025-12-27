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

package errors

import (
	"fmt"
	"time"
)

// ValidationError represents user input validation failures.
// Use this for invalid user input, malformed data, or constraint violations.
type ValidationError struct {
	// Field identifies which input field failed validation
	Field string

	// Message is the human-readable error description
	Message string

	// Suggestion provides actionable guidance for fixing the error
	Suggestion string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation failed: %s", e.Message)
}

// NotFoundError represents a resource not found error.
// Use this when a requested resource does not exist.
type NotFoundError struct {
	// Resource is the type of resource (e.g., "workflow", "tool", "connector")
	Resource string

	// ID is the identifier that was not found
	ID string
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// ProviderError represents LLM provider failures.
// Use this for errors originating from external LLM providers.
type ProviderError struct {
	// Provider is the name of the LLM provider (e.g., "anthropic", "openai")
	Provider string

	// Code is the provider-specific error code
	Code int

	// StatusCode is the HTTP status code (if applicable)
	StatusCode int

	// Message is the human-readable error message
	Message string

	// Suggestion provides actionable guidance for resolution
	Suggestion string

	// RequestID correlates this error with provider logs
	RequestID string

	// Cause is the underlying error
	Cause error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	msg := fmt.Sprintf("provider %s error", e.Provider)

	if e.Code > 0 {
		msg = fmt.Sprintf("%s (%d)", msg, e.Code)
	}

	if e.StatusCode > 0 {
		msg = fmt.Sprintf("%s [HTTP %d]", msg, e.StatusCode)
	}

	msg = fmt.Sprintf("%s: %s", msg, e.Message)

	if e.RequestID != "" {
		msg = fmt.Sprintf("%s (request-id: %s)", msg, e.RequestID)
	}

	return msg
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// ConfigError represents configuration problems.
// Use this for configuration file errors, missing settings, or invalid config values.
type ConfigError struct {
	// Key is the configuration key that has the problem (e.g., "api_key", "database.host")
	Key string

	// Reason explains what's wrong with the configuration
	Reason string

	// Cause is the underlying error (e.g., file read error, parse error)
	Cause error
}

// Error implements the error interface.
func (e *ConfigError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("config error at %s: %s", e.Key, e.Reason)
	}
	return fmt.Sprintf("config error: %s", e.Reason)
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *ConfigError) Unwrap() error {
	return e.Cause
}

// TimeoutError represents operation timeouts.
// Use this when an operation exceeds its configured timeout.
type TimeoutError struct {
	// Operation describes what timed out (e.g., "LLM request", "workflow step")
	Operation string

	// Duration is how long the operation ran before timing out
	Duration time.Duration

	// Cause is the underlying error (if any)
	Cause error
}

// Error implements the error interface.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s operation timed out after %v", e.Operation, e.Duration)
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *TimeoutError) Unwrap() error {
	return e.Cause
}
