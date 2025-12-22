package jira

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tombee/conductor/internal/operation/transport"
)

// JiraError represents a Jira API error response.
type JiraError struct {
	ErrorMessages []string          `json:"errorMessages,omitempty"`
	Errors        map[string]string `json:"errors,omitempty"`
	StatusCode    int
}

// Error implements the error interface.
func (e *JiraError) Error() string {
	var parts []string

	if len(e.ErrorMessages) > 0 {
		parts = append(parts, strings.Join(e.ErrorMessages, "; "))
	}

	if len(e.Errors) > 0 {
		var fieldErrors []string
		for field, msg := range e.Errors {
			fieldErrors = append(fieldErrors, fmt.Sprintf("%s: %s", field, msg))
		}
		parts = append(parts, strings.Join(fieldErrors, "; "))
	}

	msg := "Jira API error"
	if len(parts) > 0 {
		msg = strings.Join(parts, " - ")
	}

	return fmt.Sprintf("%s (status %d)", msg, e.StatusCode)
}

// ParseError parses a Jira error response.
func ParseError(resp *transport.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	jiraErr := &JiraError{
		StatusCode: resp.StatusCode,
	}

	// Try to parse error body
	if len(resp.Body) > 0 {
		var errResp struct {
			ErrorMessages []string          `json:"errorMessages"`
			Errors        map[string]string `json:"errors"`
		}

		if err := json.Unmarshal(resp.Body, &errResp); err == nil {
			jiraErr.ErrorMessages = errResp.ErrorMessages
			jiraErr.Errors = errResp.Errors
		} else {
			// If parsing fails, include raw body as error message
			jiraErr.ErrorMessages = []string{string(resp.Body)}
		}
	}

	// If no error details were parsed, use a generic message based on status code
	if len(jiraErr.ErrorMessages) == 0 && len(jiraErr.Errors) == 0 {
		jiraErr.ErrorMessages = []string{getDefaultMessage(resp.StatusCode)}
	}

	return jiraErr
}

// getDefaultMessage returns a default error message for a status code.
func getDefaultMessage(statusCode int) string {
	switch statusCode {
	case 400:
		return "Bad request - check your input parameters"
	case 401:
		return "Unauthorized - check your authentication credentials"
	case 403:
		return "Forbidden - you don't have permission to access this resource"
	case 404:
		return "Not found - the requested resource does not exist"
	case 405:
		return "Method not allowed"
	case 409:
		return "Conflict - the request conflicts with the current state"
	case 429:
		return "Rate limit exceeded - too many requests"
	case 500:
		return "Internal server error"
	case 502:
		return "Bad gateway"
	case 503:
		return "Service unavailable"
	default:
		return fmt.Sprintf("Request failed with status %d", statusCode)
	}
}
