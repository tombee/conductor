package httpclient

import (
	"net/url"
	"strings"
)

// sensitiveParams contains query parameter names that should be redacted from logs.
// These are matched case-insensitively.
var sensitiveParams = []string{
	"api_key",
	"apikey",
	"token",
	"password",
	"auth",
	"secret",
	"key",
	"credential",
}

// sanitizeURL removes sensitive query parameters from URLs before logging.
// This prevents leaking API keys, tokens, and other secrets in logs.
func sanitizeURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	// Parse query parameters
	q := u.Query()

	// Check each query parameter against sensitive list (case-insensitive)
	for param := range q {
		if isSensitiveParam(param) {
			q.Set(param, "[REDACTED]")
		}
	}

	// Rebuild URL with sanitized query
	safe := *u
	safe.RawQuery = q.Encode()
	return safe.String()
}

// isSensitiveParam checks if a parameter name matches the sensitive list.
// Comparison is case-insensitive to catch variants like "API_KEY", "Api_Key", etc.
func isSensitiveParam(param string) bool {
	lower := strings.ToLower(param)
	for _, sensitive := range sensitiveParams {
		if strings.Contains(lower, sensitive) {
			return true
		}
	}
	return false
}
