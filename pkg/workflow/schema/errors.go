package schema

import "fmt"

// ValidationError represents a schema validation failure with detailed context.
type ValidationError struct {
	// Path is the JSON path to the failing field (e.g., "$.category", "$.items[0].name")
	Path string

	// Keyword is the schema keyword that failed (type, required, enum, etc.)
	Keyword string

	// Message is the human-readable error message
	Message string
}

// NewValidationError creates a new validation error.
func NewValidationError(path, keyword, message string) *ValidationError {
	return &ValidationError{
		Path:    path,
		Keyword: keyword,
		Message: message,
	}
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed at %s (%s): %s", e.Path, e.Keyword, e.Message)
}

// Is implements error equality checking for errors.Is().
func (e *ValidationError) Is(target error) bool {
	t, ok := target.(*ValidationError)
	if !ok {
		return false
	}
	return e.Path == t.Path && e.Keyword == t.Keyword
}
