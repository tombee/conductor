package github

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/tombee/conductor/internal/operation/transport"
)

// GitHubError represents a GitHub API error response.
type GitHubError struct {
	Message            string                  `json:"message"`
	DocumentationURL   string                  `json:"documentation_url,omitempty"`
	Errors             []GitHubValidationError `json:"errors,omitempty"`
	StatusCode         int
	RateLimitRemaining int
	RateLimitReset     time.Time
}

// GitHubValidationError represents a validation error in a GitHub API response.
type GitHubValidationError struct {
	Resource string `json:"resource"`
	Field    string `json:"field"`
	Code     string `json:"code"`
	Message  string `json:"message,omitempty"`
}

// Error implements the error interface.
func (e *GitHubError) Error() string {
	msg := fmt.Sprintf("GitHub API error: %s (status %d)", e.Message, e.StatusCode)

	if e.DocumentationURL != "" {
		msg += fmt.Sprintf(" - see %s", e.DocumentationURL)
	}

	if len(e.Errors) > 0 {
		msg += " - validation errors:"
		for _, ve := range e.Errors {
			msg += fmt.Sprintf(" [%s.%s: %s]", ve.Resource, ve.Field, ve.Code)
		}
	}

	if e.RateLimitRemaining == 0 {
		msg += fmt.Sprintf(" - rate limit exceeded, resets at %s", e.RateLimitReset.Format(time.RFC3339))
	}

	return msg
}

// ParseError parses a GitHub error response.
func ParseError(resp *transport.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	ghErr := &GitHubError{
		StatusCode: resp.StatusCode,
	}

	// Parse rate limit headers
	if remaining := resp.Headers["X-Ratelimit-Remaining"]; len(remaining) > 0 {
		if val, err := strconv.Atoi(remaining[0]); err == nil {
			ghErr.RateLimitRemaining = val
		}
	}
	if reset := resp.Headers["X-Ratelimit-Reset"]; len(reset) > 0 {
		if val, err := strconv.ParseInt(reset[0], 10, 64); err == nil {
			ghErr.RateLimitReset = time.Unix(val, 0)
		}
	}

	// Try to parse error body
	if len(resp.Body) > 0 {
		var errResp struct {
			Message          string                  `json:"message"`
			DocumentationURL string                  `json:"documentation_url"`
			Errors           []GitHubValidationError `json:"errors"`
		}

		if err := json.Unmarshal(resp.Body, &errResp); err == nil {
			ghErr.Message = errResp.Message
			ghErr.DocumentationURL = errResp.DocumentationURL
			ghErr.Errors = errResp.Errors
		} else {
			// Fallback to raw body as message
			ghErr.Message = string(resp.Body)
		}
	}

	// If no message was parsed, use a generic message based on status code
	if ghErr.Message == "" {
		ghErr.Message = getDefaultMessage(resp.StatusCode)
	}

	return ghErr
}

// getDefaultMessage returns a default error message for a status code.
func getDefaultMessage(statusCode int) string {
	switch statusCode {
	case 400:
		return "Bad request"
	case 401:
		return "Unauthorized - check your token"
	case 403:
		return "Forbidden - check your permissions"
	case 404:
		return "Not found"
	case 422:
		return "Unprocessable entity - validation failed"
	case 429:
		return "Rate limit exceeded"
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
