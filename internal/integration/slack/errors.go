package slack

import (
	"encoding/json"
	"fmt"

	"github.com/tombee/conductor/internal/operation/transport"
)

// SlackError represents a Slack API error.
type SlackError struct {
	ErrorCode  string
	Message    string
	StatusCode int
}

// Error implements the error interface.
func (e *SlackError) Error() string {
	msg := fmt.Sprintf("Slack API error: %s", e.ErrorCode)

	// Add helpful context for common errors
	if suggestion := getErrorSuggestion(e.ErrorCode); suggestion != "" {
		msg += fmt.Sprintf(" - %s", suggestion)
	}

	if e.Message != "" && e.Message != e.ErrorCode {
		msg += fmt.Sprintf(" (%s)", e.Message)
	}

	return msg
}

// ParseError parses a Slack error response.
// Slack APIs return ok:false with an error field when operations fail.
func ParseError(resp *transport.Response) error {
	// HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &SlackError{
			ErrorCode:  fmt.Sprintf("http_%d", resp.StatusCode),
			Message:    getHTTPErrorMessage(resp.StatusCode),
			StatusCode: resp.StatusCode,
		}
	}

	// Parse Slack response to check ok field
	if len(resp.Body) > 0 {
		var slackResp SlackResponse
		if err := json.Unmarshal(resp.Body, &slackResp); err != nil {
			// If we can't parse the response, return a generic error
			return &SlackError{
				ErrorCode:  "parse_error",
				Message:    fmt.Sprintf("failed to parse response: %v", err),
				StatusCode: resp.StatusCode,
			}
		}

		// Check if the response indicates an error
		if !slackResp.OK {
			return &SlackError{
				ErrorCode:  slackResp.Error,
				Message:    slackResp.Error,
				StatusCode: resp.StatusCode,
			}
		}
	}

	return nil
}

// getErrorSuggestion returns a helpful suggestion for common Slack errors.
func getErrorSuggestion(errorCode string) string {
	suggestions := map[string]string{
		"channel_not_found":   "Channel does not exist or bot is not a member",
		"not_in_channel":      "Bot is not in the specified channel. Invite the bot first",
		"user_not_found":      "User does not exist in the workspace",
		"invalid_auth":        "Token is invalid or has been revoked",
		"not_authed":          "No authentication token provided",
		"token_revoked":       "Token has been revoked. Generate a new token",
		"token_expired":       "Token has expired. Refresh or generate a new token",
		"account_inactive":    "Authentication token is for a deleted user or workspace",
		"missing_scope":       "Token does not have the required scope. Check bot permissions",
		"ratelimited":         "Too many requests. Slow down API calls",
		"cant_update_message": "Cannot update message. It may be too old or you lack permissions",
		"message_not_found":   "Message does not exist or has been deleted",
		"cant_delete_message": "Cannot delete message. Check permissions",
		"name_taken":          "Channel name is already in use",
		"no_channel":          "Channel parameter is required",
		"invalid_name":        "Channel name is invalid. Must be lowercase, no spaces",
		"user_is_bot":         "Cannot perform this action on a bot user",
		"already_in_channel":  "User is already a member of the channel",
		"cant_invite_self":    "Cannot invite yourself to a channel",
		"is_archived":         "Channel is archived. Unarchive it first",
	}

	if suggestion, ok := suggestions[errorCode]; ok {
		return suggestion
	}

	return ""
}

// getHTTPErrorMessage returns a message for HTTP error codes.
func getHTTPErrorMessage(statusCode int) string {
	switch statusCode {
	case 400:
		return "Bad request - check your parameters"
	case 401:
		return "Unauthorized - check your token"
	case 403:
		return "Forbidden - check your permissions"
	case 404:
		return "Not found"
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
