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

package profile

import "fmt"

// ErrorCategory represents the category of profile-related errors.
// These categories are used for error reporting and remediation guidance.
type ErrorCategory string

const (
	// ErrorCategoryNotFound indicates a profile or binding was not found
	ErrorCategoryNotFound ErrorCategory = "NOT_FOUND"

	// ErrorCategoryAccessDenied indicates permission/access issues
	ErrorCategoryAccessDenied ErrorCategory = "ACCESS_DENIED"

	// ErrorCategoryTimeout indicates a timeout during secret resolution
	ErrorCategoryTimeout ErrorCategory = "TIMEOUT"

	// ErrorCategoryInvalidSyntax indicates malformed configuration
	ErrorCategoryInvalidSyntax ErrorCategory = "INVALID_SYNTAX"

	// ErrorCategoryCircularRef indicates circular secret references
	ErrorCategoryCircularRef ErrorCategory = "CIRCULAR_REF"

	// ErrorCategoryValidation indicates validation failures
	ErrorCategoryValidation ErrorCategory = "VALIDATION"
)

// ProfileError represents a profile-related error with context.
type ProfileError struct {
	Category    ErrorCategory
	Message     string
	Profile     string
	Workspace   string
	Reference   string // Truncated for security
	Remediation string
	Cause       error
}

func (e *ProfileError) Error() string {
	if e.Workspace != "" && e.Profile != "" {
		return fmt.Sprintf("profile error (%s) in %s/%s: %s", e.Category, e.Workspace, e.Profile, e.Message)
	}
	if e.Profile != "" {
		return fmt.Sprintf("profile error (%s) in profile %s: %s", e.Category, e.Profile, e.Message)
	}
	return fmt.Sprintf("profile error (%s): %s", e.Category, e.Message)
}

func (e *ProfileError) Unwrap() error {
	return e.Cause
}

// ValidationError represents a validation failure with field context.
type ValidationError struct {
	Field   string
	Message string
	Value   string
}

func (e *ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("%s: %s (value: %q)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// SecretResolutionError represents an error during secret resolution.
// Error messages are sanitized to prevent leaking secret values or paths.
type SecretResolutionError struct {
	Category      ErrorCategory
	Reference     string // Truncated reference (e.g., "ref:vault/***-token")
	Provider      string
	Message       string
	Remediation   string
	OriginalError error // Logged server-side only, not shown to user
}

func (e *SecretResolutionError) Error() string {
	if e.Remediation != "" {
		return fmt.Sprintf("secret resolution failed (%s): %s (provider: %s, ref: %s) - %s",
			e.Category, e.Message, e.Provider, e.Reference, e.Remediation)
	}
	return fmt.Sprintf("secret resolution failed (%s): %s (provider: %s, ref: %s)",
		e.Category, e.Message, e.Provider, e.Reference)
}

func (e *SecretResolutionError) Unwrap() error {
	return e.OriginalError
}

// BindingError represents an error during binding resolution.
type BindingError struct {
	Workflow    string
	Profile     string
	Workspace   string
	Requirement string // e.g., "connectors.github"
	Message     string
	Remediation string
	Cause       error
}

func (e *BindingError) Error() string {
	msg := fmt.Sprintf("binding error for workflow %q", e.Workflow)
	if e.Workspace != "" && e.Profile != "" {
		msg += fmt.Sprintf(" with profile %s/%s", e.Workspace, e.Profile)
	} else if e.Profile != "" {
		msg += fmt.Sprintf(" with profile %q", e.Profile)
	}
	msg += fmt.Sprintf(": requirement %q %s", e.Requirement, e.Message)
	return msg
}

func (e *BindingError) Unwrap() error {
	return e.Cause
}

// CircularReferenceError indicates a circular dependency in secret references.
type CircularReferenceError struct {
	Chain []string // Chain of references forming the cycle
}

func (e *CircularReferenceError) Error() string {
	return fmt.Sprintf("circular secret reference detected: %v", e.Chain)
}

// TruncateReference truncates a secret reference for safe error messages.
// Examples:
//   - "file:/etc/secrets/github-token" -> "file:***-token"
//   - "${GITHUB_TOKEN}" -> "$***TOKEN"
//   - "env:ANTHROPIC_API_KEY" -> "env:***_KEY"
func TruncateReference(ref string) string {
	if len(ref) <= 8 {
		return "***"
	}

	// For file paths, show only scheme and last component hint
	if len(ref) > 20 {
		// Show first 8 chars (likely scheme) and last 4
		return ref[:8] + "***" + ref[len(ref)-4:]
	}

	// For shorter refs, show first 4 and last 4
	return ref[:4] + "***" + ref[len(ref)-4:]
}

// NewProfileNotFoundError creates an error for missing profiles.
func NewProfileNotFoundError(workspace, profile string) *ProfileError {
	return &ProfileError{
		Category:    ErrorCategoryNotFound,
		Workspace:   workspace,
		Profile:     profile,
		Message:     "profile not found",
		Remediation: fmt.Sprintf("Create profile %q in workspace %q or use an existing profile", profile, workspace),
	}
}

// NewBindingNotFoundError creates an error for missing required bindings.
func NewBindingNotFoundError(workflow, profile, workspace, requirement string) *BindingError {
	return &BindingError{
		Workflow:    workflow,
		Profile:     profile,
		Workspace:   workspace,
		Requirement: requirement,
		Message:     "not bound in profile",
		Remediation: fmt.Sprintf("Add binding for %q to profile %s/%s", requirement, workspace, profile),
	}
}

// NewSecretResolutionError creates a sanitized error for secret resolution failures.
func NewSecretResolutionError(category ErrorCategory, reference, provider, message string, originalErr error) *SecretResolutionError {
	return &SecretResolutionError{
		Category:      category,
		Reference:     TruncateReference(reference),
		Provider:      provider,
		Message:       message,
		OriginalError: originalErr,
	}
}

// NewBindingError creates an error for binding resolution failures.
func NewBindingError(category ErrorCategory, requirement, profile, message string) *BindingError {
	return &BindingError{
		Profile:     profile,
		Requirement: requirement,
		Message:     message,
	}
}
