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

// UserVisibleError defines errors that should be displayed to end users
// with user-friendly messages and actionable suggestions.
//
// Domain-specific errors (like connector.Error and MCPError) should
// implement this interface to integrate with CLI error formatting.
type UserVisibleError interface {
	error

	// IsUserVisible returns true if this error should be shown to users.
	// Internal errors or debugging details should return false.
	IsUserVisible() bool

	// UserMessage returns a user-friendly error message.
	// This should avoid technical jargon and implementation details.
	UserMessage() string

	// Suggestion returns actionable guidance for resolving the error.
	// Returns empty string if no suggestion is available.
	Suggestion() string
}

// ErrorClassifier defines methods for programmatic error handling.
// Errors that implement this interface can be classified by type
// for retry logic, error reporting, or specific handling paths.
type ErrorClassifier interface {
	error

	// ErrorType returns a string identifying the error category.
	// Examples: "validation", "not_found", "timeout", "provider"
	ErrorType() string

	// IsRetryable returns true if the operation should be retried.
	IsRetryable() bool
}
