package notion

import (
	"encoding/json"
	"fmt"

	"github.com/tombee/conductor/internal/operation/transport"
)

// NotionError represents a Notion API error.
type NotionError struct {
	ErrorCode  string
	Message    string
	StatusCode int
	Category   ErrorCategory
}

// ErrorCategory classifies errors for handling and metrics.
type ErrorCategory string

const (
	// ErrorCategoryAuth indicates authentication errors
	ErrorCategoryAuth ErrorCategory = "auth_invalid"

	// ErrorCategoryAccess indicates access/permission errors
	ErrorCategoryAccess ErrorCategory = "access_denied"

	// ErrorCategoryRateLimit indicates rate limiting errors
	ErrorCategoryRateLimit ErrorCategory = "rate_limited"

	// ErrorCategoryValidation indicates validation errors
	ErrorCategoryValidation ErrorCategory = "validation_error"

	// ErrorCategoryNetwork indicates network/transport errors
	ErrorCategoryNetwork ErrorCategory = "network_error"

	// ErrorCategoryTimeout indicates context deadline/timeout errors
	ErrorCategoryTimeout ErrorCategory = "timeout"
)

// Error implements the error interface.
func (e *NotionError) Error() string {
	msg := fmt.Sprintf("Notion API error: %s", e.ErrorCode)

	if suggestion := getErrorSuggestion(e.ErrorCode, e.StatusCode); suggestion != "" {
		msg += fmt.Sprintf(" - %s", suggestion)
	}

	if e.Message != "" && e.Message != e.ErrorCode {
		msg += fmt.Sprintf(" (%s)", e.Message)
	}

	return msg
}

// ParseError parses a Notion error response and categorizes it.
func ParseError(resp *transport.Response) error {
	// HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		category := categorizeHTTPError(resp.StatusCode)
		return &NotionError{
			ErrorCode:  fmt.Sprintf("http_%d", resp.StatusCode),
			Message:    getHTTPErrorMessage(resp.StatusCode),
			StatusCode: resp.StatusCode,
			Category:   category,
		}
	}

	// Parse Notion error response
	if len(resp.Body) > 0 {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body, &errResp); err != nil {
			return &NotionError{
				ErrorCode:  "parse_error",
				Message:    fmt.Sprintf("failed to parse response: %v", err),
				StatusCode: resp.StatusCode,
				Category:   ErrorCategoryValidation,
			}
		}

		if errResp.Code != "" {
			return &NotionError{
				ErrorCode:  errResp.Code,
				Message:    errResp.Message,
				StatusCode: errResp.Status,
				Category:   categorizeNotionError(errResp.Code),
			}
		}
	}

	return nil
}

// categorizeNotionError assigns an error category based on the Notion error code.
func categorizeNotionError(code string) ErrorCategory {
	switch code {
	case "unauthorized", "invalid_request":
		return ErrorCategoryAuth
	case "restricted_resource", "object_not_found":
		return ErrorCategoryAccess
	case "rate_limited":
		return ErrorCategoryRateLimit
	case "validation_error", "invalid_json":
		return ErrorCategoryValidation
	default:
		return ErrorCategoryNetwork
	}
}

// categorizeHTTPError assigns an error category based on HTTP status code.
func categorizeHTTPError(statusCode int) ErrorCategory {
	switch {
	case statusCode == 401:
		return ErrorCategoryAuth
	case statusCode == 403:
		return ErrorCategoryAccess
	case statusCode == 429:
		return ErrorCategoryRateLimit
	case statusCode >= 400 && statusCode < 500:
		return ErrorCategoryValidation
	default:
		return ErrorCategoryNetwork
	}
}

// getErrorSuggestion returns a helpful suggestion for common Notion errors.
func getErrorSuggestion(errorCode string, statusCode int) string {
	suggestions := map[string]string{
		"unauthorized":        "Token is invalid. Check your integration token at https://www.notion.so/my-integrations",
		"restricted_resource": "Cannot access this resource. Share the page or database with your integration at https://www.notion.so/my-integrations",
		"object_not_found":    "Page or database not found. Verify the ID and ensure it's shared with your integration",
		"rate_limited":        "Too many requests. Notion free tier allows 3-6 requests/second",
		"validation_error":    "Request validation failed. Check your input parameters",
		"invalid_json":        "Request body is not valid JSON",
		"invalid_request":     "Request is malformed. Check the API documentation",
	}

	if suggestion, ok := suggestions[errorCode]; ok {
		return suggestion
	}

	// HTTP-specific suggestions
	if statusCode == 429 {
		return "Too many requests. Retrying with exponential backoff"
	}

	return ""
}

// getHTTPErrorMessage returns a message for HTTP error codes.
func getHTTPErrorMessage(statusCode int) string {
	switch statusCode {
	case 400:
		return "Bad request - check your parameters"
	case 401:
		return "Unauthorized - check your integration token"
	case 403:
		return "Forbidden - page not shared with integration"
	case 404:
		return "Not found - page or database does not exist"
	case 429:
		return "Rate limited - too many requests"
	case 500:
		return "Internal server error"
	case 503:
		return "Service unavailable"
	default:
		return fmt.Sprintf("HTTP error %d", statusCode)
	}
}
