// Package file provides a builtin connector for filesystem operations.
package file

import "fmt"

// ErrorType represents the type of file connector error.
type ErrorType string

const (
	// ErrorTypeFileNotFound indicates the file does not exist.
	ErrorTypeFileNotFound ErrorType = "file_not_found"

	// ErrorTypePermissionDenied indicates insufficient permissions.
	ErrorTypePermissionDenied ErrorType = "permission_denied"

	// ErrorTypeDiskFull indicates no space available for write.
	ErrorTypeDiskFull ErrorType = "disk_full"

	// ErrorTypeEncodingError indicates invalid character encoding.
	ErrorTypeEncodingError ErrorType = "encoding_error"

	// ErrorTypeParseError indicates malformed JSON/YAML/CSV.
	ErrorTypeParseError ErrorType = "parse_error"

	// ErrorTypePathTraversal indicates attempted directory escape.
	ErrorTypePathTraversal ErrorType = "path_traversal"

	// ErrorTypeFileTooLarge indicates file exceeds size limit.
	ErrorTypeFileTooLarge ErrorType = "file_too_large"

	// ErrorTypeSymlinkDenied indicates symlink not allowed.
	ErrorTypeSymlinkDenied ErrorType = "symlink_denied"

	// ErrorTypeValidation indicates invalid input parameters.
	ErrorTypeValidation ErrorType = "validation"

	// ErrorTypeInternal indicates an internal error.
	ErrorTypeInternal ErrorType = "internal"

	// ErrorTypeConfiguration indicates a configuration error.
	ErrorTypeConfiguration ErrorType = "configuration_error"

	// ErrorTypeNotImplemented indicates operation is not implemented.
	ErrorTypeNotImplemented ErrorType = "not_implemented"
)

// OperationError represents an error from a file connector operation.
// This type is specific to file operations and includes file-specific fields
// (ErrorType for file errors, SuggestText). It implements the UserVisibleError
// interface differently than transform.OperationError due to different contexts.
type OperationError struct {
	Operation  string
	Message    string
	ErrorType  ErrorType
	Cause      error
	// SuggestText provides actionable guidance for resolving the error.
	// Renamed from Suggestion to avoid conflict with Suggestion() method.
	SuggestText string
}

// Error implements the error interface.
func (e *OperationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Operation, e.Message)
}

// Unwrap returns the underlying cause.
func (e *OperationError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error may succeed on retry.
func (e *OperationError) IsRetryable() bool {
	return e.ErrorType == ErrorTypeDiskFull
}

// IsUserVisible implements pkg/errors.UserVisibleError.
// File operation errors are always user-visible.
func (e *OperationError) IsUserVisible() bool {
	return true
}

// UserMessage implements pkg/errors.UserVisibleError.
// Returns a user-friendly message without technical details.
func (e *OperationError) UserMessage() string {
	return e.Message
}

// Suggestion implements pkg/errors.UserVisibleError.
// Returns actionable guidance for resolving the error.
func (e *OperationError) Suggestion() string {
	// Return existing suggestion if set, otherwise provide default based on error type
	if e.SuggestText != "" {
		return e.SuggestText
	}

	switch e.ErrorType {
	case ErrorTypeFileNotFound:
		return "Check that the file path is correct and the file exists"
	case ErrorTypePermissionDenied:
		return "Check file permissions or run with appropriate access rights"
	case ErrorTypeDiskFull:
		return "Free up disk space and retry the operation"
	case ErrorTypeEncodingError:
		return "Ensure the file uses UTF-8 encoding"
	case ErrorTypeParseError:
		return "Verify the file format is valid JSON, YAML, or CSV"
	case ErrorTypePathTraversal:
		return "Use paths within the allowed directories only"
	case ErrorTypeFileTooLarge:
		return "Use a smaller file or increase size limits in configuration"
	case ErrorTypeSymlinkDenied:
		return "Symlinks are not allowed - use direct file paths instead"
	case ErrorTypeValidation:
		return "Check input parameters against operation requirements"
	case ErrorTypeConfiguration:
		return "Review file connector configuration settings"
	case ErrorTypeNotImplemented:
		return "This operation is not yet supported"
	default:
		return ""
	}
}
