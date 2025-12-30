package sdk

import (
	"fmt"
	"strings"
)

// TokenLimitExceededError indicates workflow execution exceeded the token limit.
type TokenLimitExceededError struct {
	Limit  int
	Actual int
}

func (e *TokenLimitExceededError) Error() string {
	return fmt.Sprintf("token limit exceeded: %d > %d tokens", e.Actual, e.Limit)
}

// ValidationError indicates the workflow definition is invalid.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// StepExecutionError wraps errors from step execution.
// It provides both detailed error information for logging and
// safe error messages for user display.
type StepExecutionError struct {
	StepID   string
	Cause    error
	redacted string // Pre-computed safe message
}

func (e *StepExecutionError) Error() string {
	return fmt.Sprintf("step %s failed: %v", e.StepID, e.Cause)
}

// Unwrap returns the underlying error.
func (e *StepExecutionError) Unwrap() error {
	return e.Cause
}

// SafeMessage returns a message safe for end-user display.
// Internal details (file paths, prompts, tool outputs) are excluded.
func (e *StepExecutionError) SafeMessage() string {
	if e.redacted != "" {
		return e.redacted
	}

	// Generate safe message by removing sensitive details
	msg := fmt.Sprintf("Step %s failed", e.StepID)

	// Extract error type without details
	if e.Cause != nil {
		errType := strings.Split(e.Cause.Error(), ":")[0]
		msg = fmt.Sprintf("%s: %s", msg, errType)
	}

	return msg
}

// NewStepExecutionError creates a StepExecutionError with a safe message.
func NewStepExecutionError(stepID string, cause error) *StepExecutionError {
	return &StepExecutionError{
		StepID: stepID,
		Cause:  cause,
	}
}

// ProviderError wraps errors from LLM providers.
type ProviderError struct {
	Provider  string
	Cause     error
	Retryable bool
}

func (e *ProviderError) Error() string {
	retryable := ""
	if e.Retryable {
		retryable = " (retryable)"
	}
	return fmt.Sprintf("provider %s error%s: %v", e.Provider, retryable, e.Cause)
}

// Unwrap returns the underlying error.
func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// NewProviderError creates a ProviderError.
func NewProviderError(provider string, cause error, retryable bool) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Cause:     cause,
		Retryable: retryable,
	}
}
