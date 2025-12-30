package pagerduty

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tombee/conductor/internal/operation/transport"
)

// ParseError checks if the response contains a PagerDuty API error.
func ParseError(resp *transport.Response) error {
	// Check HTTP status code first
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Try to parse PagerDuty error response
	var errResp struct {
		Error *APIError `json:"error"`
	}

	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Error != nil {
		return &PagerDutyError{
			StatusCode: resp.StatusCode,
			Code:       errResp.Error.Code,
			Message:    errResp.Error.Message,
			Errors:     errResp.Error.Errors,
		}
	}

	// Generic HTTP error
	return &PagerDutyError{
		StatusCode: resp.StatusCode,
		Message:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(resp.Body)),
	}
}

// PagerDutyError represents an error from the PagerDuty API.
type PagerDutyError struct {
	StatusCode int
	Code       int
	Message    string
	Errors     []string
}

// Error implements the error interface.
func (e *PagerDutyError) Error() string {
	if len(e.Errors) > 0 {
		return fmt.Sprintf("PagerDuty API error (HTTP %d, code %d): %s - %s",
			e.StatusCode, e.Code, e.Message, strings.Join(e.Errors, "; "))
	}
	if e.Code != 0 {
		return fmt.Sprintf("PagerDuty API error (HTTP %d, code %d): %s",
			e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("PagerDuty API error (HTTP %d): %s", e.StatusCode, e.Message)
}

// IsNotFound returns true if the error is a 404 not found error.
func (e *PagerDutyError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsRateLimited returns true if the error is a rate limit error.
func (e *PagerDutyError) IsRateLimited() bool {
	return e.StatusCode == 429
}

// IsAuthError returns true if the error is an authentication/authorization error.
func (e *PagerDutyError) IsAuthError() bool {
	return e.StatusCode == 401 || e.StatusCode == 403
}
