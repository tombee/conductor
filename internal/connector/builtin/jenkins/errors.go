package jenkins

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tombee/conductor/internal/connector/transport"
)

// JenkinsError represents a Jenkins API error response.
type JenkinsError struct {
	Message    string
	StatusCode int
	IsHTML     bool
	RawBody    string
}

// Error implements the error interface.
func (e *JenkinsError) Error() string {
	if e.IsHTML {
		return fmt.Sprintf("Jenkins API error: %s (status %d) - received HTML error page", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("Jenkins API error: %s (status %d)", e.Message, e.StatusCode)
}

// ParseError parses a Jenkins error response.
// Jenkins can return either JSON errors or HTML error pages.
func ParseError(resp *transport.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	jenkinsErr := &JenkinsError{
		StatusCode: resp.StatusCode,
		RawBody:    string(resp.Body),
	}

	// Check if response is HTML
	contentType := ""
	if ct, ok := resp.Headers["Content-Type"]; ok && len(ct) > 0 {
		contentType = strings.ToLower(ct[0])
	}

	isHTML := strings.Contains(contentType, "text/html") ||
		strings.HasPrefix(strings.TrimSpace(string(resp.Body)), "<!DOCTYPE") ||
		strings.HasPrefix(strings.TrimSpace(string(resp.Body)), "<html")

	if isHTML {
		jenkinsErr.IsHTML = true
		jenkinsErr.Message = extractHTMLTitle(string(resp.Body))
		if jenkinsErr.Message == "" {
			jenkinsErr.Message = getDefaultMessage(resp.StatusCode)
		}
		return jenkinsErr
	}

	// Try to parse JSON error response
	if len(resp.Body) > 0 {
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}

		if err := json.Unmarshal(resp.Body, &errResp); err == nil {
			if errResp.Message != "" {
				jenkinsErr.Message = errResp.Message
			} else if errResp.Error != "" {
				jenkinsErr.Message = errResp.Error
			}
		}
	}

	// If no message was parsed, use a generic message based on status code
	if jenkinsErr.Message == "" {
		jenkinsErr.Message = getDefaultMessage(resp.StatusCode)
	}

	return jenkinsErr
}

// extractHTMLTitle attempts to extract a meaningful error message from HTML.
// This is a simple extraction that looks for <title> tags.
func extractHTMLTitle(html string) string {
	// Look for <title> tag
	startTag := "<title>"
	endTag := "</title>"

	start := strings.Index(strings.ToLower(html), startTag)
	if start == -1 {
		return ""
	}
	start += len(startTag)

	end := strings.Index(strings.ToLower(html[start:]), endTag)
	if end == -1 {
		return ""
	}

	title := strings.TrimSpace(html[start : start+end])

	// Jenkins often includes "Error" or the status code in the title
	// Clean it up a bit
	title = strings.TrimPrefix(title, "Error ")
	title = strings.TrimPrefix(title, "Jenkins - ")

	return title
}

// getDefaultMessage returns a default error message for a status code.
func getDefaultMessage(statusCode int) string {
	switch statusCode {
		case 400:
			return "Bad request - check your input parameters"
		case 401:
			return "Unauthorized - check your authentication credentials"
		case 403:
			return "Forbidden - you don't have permission to perform this action"
		case 404:
			return "Not found - the requested resource does not exist"
		case 405:
			return "Method not allowed"
		case 409:
			return "Conflict - the request conflicts with the current state"
		case 500:
			return "Internal server error - Jenkins encountered an error"
		case 502:
			return "Bad gateway - Jenkins proxy error"
		case 503:
			return "Service unavailable - Jenkins may be starting up or overloaded"
		default:
			return fmt.Sprintf("Request failed with status %d", statusCode)
	}
}
