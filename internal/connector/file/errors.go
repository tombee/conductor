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
type OperationError struct {
	Operation  string
	Message    string
	ErrorType  ErrorType
	Cause      error
	Suggestion string
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
