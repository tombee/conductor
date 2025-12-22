// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package audit

import (
	"net/http"
	"strings"
)

// Middleware creates an HTTP middleware that logs API access
func Middleware(logger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract user ID from request (e.g., from authentication)
			userID := extractUserID(r)

			// Extract IP address
			ipAddress := extractIPAddress(r)

			// Wrap response writer to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call next handler
			next.ServeHTTP(wrapped, r)

			// Determine action from request path
			action := determineAction(r.Method, r.URL.Path)
			if action == "" {
				// Not an auditable endpoint
				return
			}

			// Determine result from status code
			result := determineResult(wrapped.statusCode)

			// Log the access
			entry := Entry{
				UserID:    userID,
				Action:    action,
				Resource:  r.URL.Path,
				Result:    result,
				IPAddress: ipAddress,
				UserAgent: r.UserAgent(),
			}

			// Log error (ignore logging errors to avoid cascading failures)
			_ = logger.Log(entry)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// extractUserID attempts to extract user ID from the request
func extractUserID(r *http.Request) string {
	// Try Authorization header first
	auth := r.Header.Get("Authorization")
	if auth != "" {
		// For Bearer tokens, we'd decode the JWT here
		// For simplicity, return a placeholder
		if strings.HasPrefix(auth, "Bearer ") {
			return "authenticated_user" // In real implementation, decode JWT
		}
		if strings.HasPrefix(auth, "Basic ") {
			return "basic_auth_user" // In real implementation, decode basic auth
		}
	}

	// Try API key header
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "api_key_user" // In real implementation, look up API key
	}

	// No authentication found
	return "anonymous"
}

// extractIPAddress gets the client IP address from the request
func extractIPAddress(r *http.Request) string {
	// Check X-Forwarded-For header (common in proxied environments)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	// Strip port if present
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		addr = addr[:idx]
	}

	return addr
}

// determineAction maps HTTP method and path to an audit action
func determineAction(method, path string) Action {
	// Trace endpoints
	if strings.HasPrefix(path, "/v1/traces") {
		if method == "GET" {
			return ActionTracesRead
		}
		if method == "DELETE" {
			return ActionTracesDelete
		}
	}

	// Event endpoints
	if strings.HasPrefix(path, "/v1/events") {
		if strings.HasSuffix(path, "/stream") {
			return ActionEventsStream
		}
		if method == "GET" {
			return ActionEventsRead
		}
	}

	// Not an auditable endpoint
	return ""
}

// determineResult maps HTTP status code to audit result
func determineResult(statusCode int) Result {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return ResultSuccess
	case statusCode == http.StatusUnauthorized:
		return ResultUnauthorized
	case statusCode == http.StatusForbidden:
		return ResultForbidden
	case statusCode == http.StatusNotFound:
		return ResultNotFound
	default:
		return ResultError
	}
}
