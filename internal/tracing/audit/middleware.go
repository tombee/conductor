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

	"github.com/tombee/conductor/internal/controller/auth"
)

// Middleware creates an HTTP middleware that logs API access.
// The trustedProxies parameter specifies IP addresses from which X-Forwarded-For headers are trusted.
func Middleware(logger *Logger, trustedProxies []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract user ID from request context (set by auth middleware)
			userID := extractUserID(r)

			// Extract IP address
			ipAddress := extractIPAddress(r, trustedProxies)

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

// extractUserID extracts user ID from the authenticated request context.
// This should be called after auth middleware has validated the user.
func extractUserID(r *http.Request) string {
	// Use the auth package's UserFromContext function
	if user, ok := auth.UserFromContext(r.Context()); ok && user.ID != "" {
		return user.ID
	}

	// No authenticated user found
	return "anonymous"
}

// extractIPAddress gets the client IP address from the request.
// The trustedProxies parameter specifies IPs from which X-Forwarded-For is trusted.
func extractIPAddress(r *http.Request, trustedProxies []string) string {
	// Get the direct connection IP (strip port if present)
	remoteIP := r.RemoteAddr
	if idx := strings.LastIndex(remoteIP, ":"); idx != -1 {
		remoteIP = remoteIP[:idx]
	}

	// Check if this request comes from a trusted proxy
	isTrusted := false
	for _, proxy := range trustedProxies {
		if proxy == remoteIP {
			isTrusted = true
			break
		}
	}

	// Only trust X-Forwarded-For if the direct connection is from a trusted proxy
	if isTrusted {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the first IP in the list (the original client)
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}

		// Check X-Real-IP header as fallback
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
	}

	// Return the direct connection IP
	return remoteIP
}

// determineAction maps HTTP method and path to an audit action.
// Only POST, PUT, and DELETE methods are audited (mutations).
func determineAction(method, path string) Action {
	// Only audit mutation operations
	if method != "POST" && method != "PUT" && method != "DELETE" {
		return ""
	}

	// Trace endpoints
	if strings.HasPrefix(path, "/v1/traces") {
		if method == "DELETE" {
			return ActionTracesDelete
		}
	}

	// Event endpoints
	if strings.HasPrefix(path, "/v1/events") {
		// Event streaming is considered auditable even though it's GET
		// because it provides access to potentially sensitive data
		if strings.HasSuffix(path, "/stream") {
			return ActionEventsStream
		}
	}

	// Workflow trigger/execution endpoints
	if strings.HasPrefix(path, "/v1/trigger") || strings.HasPrefix(path, "/v1/runs") {
		if method == "POST" {
			return "workflow:trigger"
		}
		if method == "DELETE" {
			return "workflow:cancel"
		}
	}

	// Exporter configuration endpoints
	if strings.HasPrefix(path, "/v1/exporters") {
		if method == "POST" || method == "PUT" {
			return ActionExportersCfg
		}
		if method == "DELETE" {
			return "exporters:delete"
		}
	}

	// Webhook endpoints
	if strings.HasPrefix(path, "/v1/webhooks") || strings.HasPrefix(path, "/webhooks/") {
		return "webhook:invoke"
	}

	// Endpoint execution (named endpoints)
	if strings.HasPrefix(path, "/v1/endpoints/") {
		return "endpoint:execute"
	}

	// Schedule endpoints
	if strings.HasPrefix(path, "/v1/schedules") {
		if method == "POST" {
			return "schedule:create"
		}
		if method == "PUT" {
			return "schedule:update"
		}
		if method == "DELETE" {
			return "schedule:delete"
		}
	}

	// MCP server management
	if strings.HasPrefix(path, "/v1/mcp") {
		if method == "POST" {
			return "mcp:start"
		}
		if method == "DELETE" {
			return "mcp:stop"
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
