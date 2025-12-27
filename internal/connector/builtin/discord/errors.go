package discord

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/tombee/conductor/internal/connector/transport"
)

// DiscordError represents a Discord API error response.
type DiscordError struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	StatusCode int

	// Rate limit information
	RateLimitRemaining int
	RateLimitReset     time.Time
	RateLimitBucket    string
	RetryAfter         float64
}

// Error implements the error interface.
func (e *DiscordError) Error() string {
	msg := fmt.Sprintf("Discord API error: %s (code %d, status %d)", e.Message, e.Code, e.StatusCode)

	if e.RateLimitRemaining == 0 && !e.RateLimitReset.IsZero() {
		msg += fmt.Sprintf(" - rate limit exceeded, resets at %s", e.RateLimitReset.Format(time.RFC3339))
	}

	if e.RetryAfter > 0 {
		msg += fmt.Sprintf(" - retry after %.2f seconds", e.RetryAfter)
	}

	return msg
}

// ParseError parses a Discord error response.
func ParseError(resp *transport.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	discordErr := &DiscordError{
		StatusCode: resp.StatusCode,
	}

	// Parse rate limit headers
	if remaining := resp.Headers["X-Ratelimit-Remaining"]; len(remaining) > 0 {
		if val, err := strconv.Atoi(remaining[0]); err == nil {
			discordErr.RateLimitRemaining = val
		}
	}
	if reset := resp.Headers["X-Ratelimit-Reset"]; len(reset) > 0 {
		if val, err := strconv.ParseFloat(reset[0], 64); err == nil {
			discordErr.RateLimitReset = time.Unix(int64(val), 0)
		}
	}
	if bucket := resp.Headers["X-Ratelimit-Bucket"]; len(bucket) > 0 {
		discordErr.RateLimitBucket = bucket[0]
	}
	if retryAfter := resp.Headers["Retry-After"]; len(retryAfter) > 0 {
		if val, err := strconv.ParseFloat(retryAfter[0], 64); err == nil {
			discordErr.RetryAfter = val
		}
	}

	// Try to parse error body
	if len(resp.Body) > 0 {
		var errResp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}

		if err := json.Unmarshal(resp.Body, &errResp); err == nil {
			discordErr.Code = errResp.Code
			discordErr.Message = errResp.Message
		} else {
			// Fallback to raw body as message
			discordErr.Message = string(resp.Body)
		}
	}

	// If no message was parsed, use a generic message based on status code
	if discordErr.Message == "" {
		discordErr.Message = getDefaultMessage(resp.StatusCode)
	}

	return discordErr
}

// getDefaultMessage returns a default error message for a status code.
func getDefaultMessage(statusCode int) string {
	switch statusCode {
	case 400:
		return "Bad request - the request was improperly formatted or missing required parameters"
	case 401:
		return "Unauthorized - invalid authentication token"
	case 403:
		return "Forbidden - missing permissions or access denied"
	case 404:
		return "Not found - resource does not exist"
	case 405:
		return "Method not allowed"
	case 429:
		return "Rate limit exceeded - too many requests"
	case 500:
		return "Internal server error"
	case 502:
		return "Bad gateway - Discord is having issues"
	case 503:
		return "Service unavailable - Discord is temporarily down"
	default:
		return fmt.Sprintf("Request failed with status %d", statusCode)
	}
}
