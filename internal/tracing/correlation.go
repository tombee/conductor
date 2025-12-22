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

// Package tracing provides distributed tracing and correlation ID support.
package tracing

import (
	"context"
	"net/http"
	"regexp"

	"github.com/google/uuid"
)

// CorrelationID represents a unique identifier for tracing requests across systems.
// It uses RFC 4122 UUID format (36 characters).
type CorrelationID string

// correlationKey is the context key for storing correlation IDs.
type correlationKeyType struct{}

var correlationKey = correlationKeyType{}

// HTTP header names for correlation ID propagation.
const (
	// HeaderCorrelationID is the primary header for correlation ID.
	HeaderCorrelationID = "X-Correlation-ID"
	// HeaderRequestID is an alternative header accepted for compatibility.
	HeaderRequestID = "X-Request-ID"
)

// uuidRegex validates RFC 4122 UUID format.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// NewCorrelationID generates a new unique correlation ID.
func NewCorrelationID() CorrelationID {
	return CorrelationID(uuid.New().String())
}

// String returns the string representation of the correlation ID.
func (c CorrelationID) String() string {
	return string(c)
}

// IsValid checks if the correlation ID is a valid UUID format.
func (c CorrelationID) IsValid() bool {
	return uuidRegex.MatchString(string(c))
}

// ToContext adds the correlation ID to the context.
func ToContext(ctx context.Context, id CorrelationID) context.Context {
	return context.WithValue(ctx, correlationKey, id)
}

// FromContext retrieves the correlation ID from the context.
// If no correlation ID is found, it generates a new one.
func FromContext(ctx context.Context) CorrelationID {
	if id, ok := ctx.Value(correlationKey).(CorrelationID); ok {
		return id
	}
	return NewCorrelationID()
}

// FromContextOrEmpty retrieves the correlation ID from the context.
// Returns empty string if no correlation ID is found.
func FromContextOrEmpty(ctx context.Context) CorrelationID {
	if id, ok := ctx.Value(correlationKey).(CorrelationID); ok {
		return id
	}
	return ""
}

// ValidateUUID checks if a string is a valid UUID format.
// Returns the correlation ID if valid, or an error if invalid.
func ValidateUUID(s string) (CorrelationID, bool) {
	if uuidRegex.MatchString(s) {
		return CorrelationID(s), true
	}
	return "", false
}

// ExtractFromRequest extracts the correlation ID from HTTP request headers.
// It checks X-Correlation-ID first, then X-Request-ID as fallback.
// Returns the extracted ID and whether one was found.
func ExtractFromRequest(r *http.Request) (CorrelationID, bool) {
	// Check X-Correlation-ID first
	if id := r.Header.Get(HeaderCorrelationID); id != "" {
		return CorrelationID(id), true
	}

	// Fall back to X-Request-ID
	if id := r.Header.Get(HeaderRequestID); id != "" {
		return CorrelationID(id), true
	}

	return "", false
}

// InjectIntoRequest adds the correlation ID to HTTP request headers.
func InjectIntoRequest(ctx context.Context, req *http.Request) {
	id := FromContextOrEmpty(ctx)
	if id != "" {
		req.Header.Set(HeaderCorrelationID, id.String())
	}
}

// InjectIntoResponse adds the correlation ID to HTTP response headers.
func InjectIntoResponse(w http.ResponseWriter, id CorrelationID) {
	if id != "" {
		w.Header().Set(HeaderCorrelationID, id.String())
	}
}

// CorrelationMiddleware returns an HTTP middleware that handles correlation ID
// extraction, validation, and propagation.
//
// For incoming requests:
//   - Extracts X-Correlation-ID or X-Request-ID header
//   - Validates UUID format (returns 400 if invalid)
//   - Generates new ID if no header provided
//   - Stores ID in request context
//
// For outgoing responses:
//   - Adds X-Correlation-ID header with the correlation ID
func CorrelationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var correlationID CorrelationID

		// Try to extract from headers
		if id, found := ExtractFromRequest(r); found {
			// Validate UUID format
			if !id.IsValid() {
				http.Error(w, "Invalid X-Correlation-ID format: must be UUID", http.StatusBadRequest)
				return
			}
			correlationID = id
		} else {
			// Generate new ID if not provided
			correlationID = NewCorrelationID()
		}

		// Add to context
		ctx := ToContext(r.Context(), correlationID)
		r = r.WithContext(ctx)

		// Add to response header
		InjectIntoResponse(w, correlationID)

		// Call next handler
		next.ServeHTTP(w, r)
	})
}

// CorrelationRoundTripper wraps an http.RoundTripper to inject correlation IDs
// into outbound HTTP requests.
type CorrelationRoundTripper struct {
	Transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *CorrelationRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Inject correlation ID from context
	InjectIntoRequest(req.Context(), req)

	// Use underlying transport or default
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return transport.RoundTrip(req)
}

// WrapHTTPClient wraps an HTTP client to inject correlation IDs into requests.
func WrapHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &http.Client{
		Transport:     &CorrelationRoundTripper{Transport: transport},
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
		Timeout:       client.Timeout,
	}
}
