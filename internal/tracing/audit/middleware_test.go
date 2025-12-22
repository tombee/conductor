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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/internal/controller/auth"
)

func TestMiddleware_AuditableEndpoints(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		userID         string
		expectedAction Action
		shouldAudit    bool
	}{
		// POST endpoints
		{
			name:           "workflow trigger",
			method:         "POST",
			path:           "/v1/trigger",
			userID:         "test-user",
			expectedAction: "workflow:trigger",
			shouldAudit:    true,
		},
		{
			name:           "run creation",
			method:         "POST",
			path:           "/v1/runs",
			userID:         "test-user",
			expectedAction: "workflow:trigger",
			shouldAudit:    true,
		},
		{
			name:           "schedule create",
			method:         "POST",
			path:           "/v1/schedules",
			userID:         "test-user",
			expectedAction: "schedule:create",
			shouldAudit:    true,
		},
		{
			name:           "exporter config",
			method:         "POST",
			path:           "/v1/exporters",
			userID:         "test-user",
			expectedAction: ActionExportersCfg,
			shouldAudit:    true,
		},
		{
			name:           "mcp start",
			method:         "POST",
			path:           "/v1/mcp/server1",
			userID:         "test-user",
			expectedAction: "mcp:start",
			shouldAudit:    true,
		},

		// DELETE endpoints
		{
			name:           "traces delete",
			method:         "DELETE",
			path:           "/v1/traces/123",
			userID:         "test-user",
			expectedAction: ActionTracesDelete,
			shouldAudit:    true,
		},
		{
			name:           "workflow cancel",
			method:         "DELETE",
			path:           "/v1/runs/abc",
			userID:         "test-user",
			expectedAction: "workflow:cancel",
			shouldAudit:    true,
		},
		{
			name:           "schedule delete",
			method:         "DELETE",
			path:           "/v1/schedules/sched1",
			userID:         "test-user",
			expectedAction: "schedule:delete",
			shouldAudit:    true,
		},

		// PUT endpoints
		{
			name:           "schedule update",
			method:         "PUT",
			path:           "/v1/schedules/sched1",
			userID:         "test-user",
			expectedAction: "schedule:update",
			shouldAudit:    true,
		},
		{
			name:           "exporter update",
			method:         "PUT",
			path:           "/v1/exporters/exp1",
			userID:         "test-user",
			expectedAction: ActionExportersCfg,
			shouldAudit:    true,
		},

		// GET endpoints should NOT be audited
		{
			name:        "health check",
			method:      "GET",
			path:        "/v1/health",
			userID:      "test-user",
			shouldAudit: false,
		},
		{
			name:        "traces read",
			method:      "GET",
			path:        "/v1/traces",
			userID:      "test-user",
			shouldAudit: false,
		},
		{
			name:        "events read",
			method:      "GET",
			path:        "/v1/events",
			userID:      "test-user",
			shouldAudit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture audit logs
			var logBuf bytes.Buffer
			logger := NewLogger(&logBuf)

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Create middleware with no trusted proxies
			middleware := Middleware(logger, nil)
			wrappedHandler := middleware(handler)

			// Create request with user context
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.userID != "" {
				// Add user to context (simulating auth middleware)
				user := &auth.User{ID: tt.userID, Name: tt.userID}
				ctx := auth.ContextWithUser(req.Context(), user)
				req = req.WithContext(ctx)
			}

			// Execute request
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)

			// Check if audit log was written
			logContent := logBuf.String()
			if tt.shouldAudit {
				if logContent == "" {
					t.Errorf("expected audit log for %s %s, got none", tt.method, tt.path)
					return
				}

				// Parse the audit entry
				var entry Entry
				if err := json.Unmarshal([]byte(logContent), &entry); err != nil {
					t.Fatalf("failed to parse audit log: %v", err)
				}

				// Verify fields
				if entry.UserID != tt.userID {
					t.Errorf("expected userID %q, got %q", tt.userID, entry.UserID)
				}
				if entry.Action != tt.expectedAction {
					t.Errorf("expected action %q, got %q", tt.expectedAction, entry.Action)
				}
				if entry.Resource != tt.path {
					t.Errorf("expected resource %q, got %q", tt.path, entry.Resource)
				}
				if entry.Result != ResultSuccess {
					t.Errorf("expected result %q, got %q", ResultSuccess, entry.Result)
				}
			} else {
				if logContent != "" {
					t.Errorf("expected no audit log for %s %s, got: %s", tt.method, tt.path, logContent)
				}
			}
		})
	}
}

func TestMiddleware_TrustedProxies(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		xff            string
		trustedProxies []string
		expectedIP     string
	}{
		{
			name:           "direct connection",
			remoteAddr:     "192.168.1.100:12345",
			xff:            "",
			trustedProxies: nil,
			expectedIP:     "192.168.1.100",
		},
		{
			name:           "untrusted proxy with xff",
			remoteAddr:     "10.0.0.1:54321",
			xff:            "203.0.113.5",
			trustedProxies: []string{"10.0.0.2"}, // Different IP
			expectedIP:     "10.0.0.1",           // Should use direct IP
		},
		{
			name:           "trusted proxy with xff",
			remoteAddr:     "10.0.0.1:54321",
			xff:            "203.0.113.5, 10.0.0.2",
			trustedProxies: []string{"10.0.0.1"},
			expectedIP:     "203.0.113.5", // Should use first IP in XFF
		},
		{
			name:           "trusted proxy with multiple xff ips",
			remoteAddr:     "10.0.0.1:54321",
			xff:            "203.0.113.5, 198.51.100.10, 10.0.0.2",
			trustedProxies: []string{"10.0.0.1"},
			expectedIP:     "203.0.113.5", // Should use first IP
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture audit logs
			var logBuf bytes.Buffer
			logger := NewLogger(&logBuf)

			// Create test handler that triggers audit
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Create middleware with trusted proxies
			middleware := Middleware(logger, tt.trustedProxies)
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("POST", "/v1/trigger", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}

			// Add user to context
			user := &auth.User{ID: "test-user", Name: "test-user"}
			ctx := auth.ContextWithUser(req.Context(), user)
			req = req.WithContext(ctx)

			// Execute request
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)

			// Parse audit log
			var entry Entry
			if err := json.Unmarshal(logBuf.Bytes(), &entry); err != nil {
				t.Fatalf("failed to parse audit log: %v", err)
			}

			// Verify IP address
			if entry.IPAddress != tt.expectedIP {
				t.Errorf("expected IP %q, got %q", tt.expectedIP, entry.IPAddress)
			}
		})
	}
}

func TestMiddleware_StatusCodeMapping(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectedResult Result
	}{
		{
			name:           "success 200",
			statusCode:     http.StatusOK,
			expectedResult: ResultSuccess,
		},
		{
			name:           "success 201",
			statusCode:     http.StatusCreated,
			expectedResult: ResultSuccess,
		},
		{
			name:           "unauthorized",
			statusCode:     http.StatusUnauthorized,
			expectedResult: ResultUnauthorized,
		},
		{
			name:           "forbidden",
			statusCode:     http.StatusForbidden,
			expectedResult: ResultForbidden,
		},
		{
			name:           "not found",
			statusCode:     http.StatusNotFound,
			expectedResult: ResultNotFound,
		},
		{
			name:           "server error",
			statusCode:     http.StatusInternalServerError,
			expectedResult: ResultError,
		},
		{
			name:           "bad request",
			statusCode:     http.StatusBadRequest,
			expectedResult: ResultError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture audit logs
			var logBuf bytes.Buffer
			logger := NewLogger(&logBuf)

			// Create test handler that returns the specified status code
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			// Create middleware
			middleware := Middleware(logger, nil)
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("POST", "/v1/trigger", nil)

			// Add user to context
			user := &auth.User{ID: "test-user", Name: "test-user"}
			ctx := auth.ContextWithUser(req.Context(), user)
			req = req.WithContext(ctx)

			// Execute request
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)

			// Parse audit log
			var entry Entry
			if err := json.Unmarshal(logBuf.Bytes(), &entry); err != nil {
				t.Fatalf("failed to parse audit log: %v", err)
			}

			// Verify result
			if entry.Result != tt.expectedResult {
				t.Errorf("expected result %q, got %q", tt.expectedResult, entry.Result)
			}
		})
	}
}

func TestMiddleware_AnonymousUser(t *testing.T) {
	// Create a buffer to capture audit logs
	var logBuf bytes.Buffer
	logger := NewLogger(&logBuf)

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware
	middleware := Middleware(logger, nil)
	wrappedHandler := middleware(handler)

	// Create request without user context
	req := httptest.NewRequest("POST", "/v1/trigger", nil)

	// Execute request
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	// Parse audit log
	var entry Entry
	if err := json.Unmarshal(logBuf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse audit log: %v", err)
	}

	// Verify user is "anonymous"
	if entry.UserID != "anonymous" {
		t.Errorf("expected userID %q, got %q", "anonymous", entry.UserID)
	}
}
