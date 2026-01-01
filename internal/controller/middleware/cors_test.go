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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_Disabled(t *testing.T) {
	config := CORSConfig{
		Enabled: false,
	}

	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should not have CORS headers when disabled
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no CORS headers when disabled")
	}
}

func TestCORS_AllowedOrigin(t *testing.T) {
	config := CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com", "https://app.example.com"},
	}

	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name          string
		origin        string
		expectAllowed bool
	}{
		{
			name:          "exact match allowed",
			origin:        "https://example.com",
			expectAllowed: true,
		},
		{
			name:          "second origin allowed",
			origin:        "https://app.example.com",
			expectAllowed: true,
		},
		{
			name:          "disallowed origin",
			origin:        "https://evil.com",
			expectAllowed: false,
		},
		{
			name:          "no origin header",
			origin:        "",
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
			if tt.expectAllowed {
				if allowOrigin != tt.origin {
					t.Errorf("expected Allow-Origin %q, got %q", tt.origin, allowOrigin)
				}
			} else {
				if allowOrigin != "" {
					t.Errorf("expected no Allow-Origin, got %q", allowOrigin)
				}
			}
		})
	}
}

func TestCORS_WildcardOrigin(t *testing.T) {
	config := CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
	}

	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://any-origin.com" {
		t.Errorf("expected origin to be allowed with wildcard, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_WildcardSuffix(t *testing.T) {
	config := CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*.example.com"},
	}

	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name          string
		origin        string
		expectAllowed bool
	}{
		{
			name:          "subdomain matches wildcard",
			origin:        "https://app.example.com",
			expectAllowed: true,
		},
		{
			name:          "nested subdomain matches wildcard",
			origin:        "https://api.app.example.com",
			expectAllowed: true,
		},
		{
			name:          "different domain not allowed",
			origin:        "https://example.org",
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
			if tt.expectAllowed {
				if allowOrigin != tt.origin {
					t.Errorf("expected Allow-Origin %q, got %q", tt.origin, allowOrigin)
				}
			} else {
				if allowOrigin != "" {
					t.Errorf("expected no Allow-Origin, got %q", allowOrigin)
				}
			}
		})
	}
}

func TestCORS_PreflightRequest(t *testing.T) {
	config := CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST", "DELETE"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"X-Custom-Header"},
		MaxAge:           3600,
		AllowCredentials: true,
	}

	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check preflight response
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected Allow-Origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	if rec.Header().Get("Access-Control-Allow-Methods") != "GET, POST, DELETE" {
		t.Errorf("expected Allow-Methods, got %q", rec.Header().Get("Access-Control-Allow-Methods"))
	}

	if rec.Header().Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
		t.Errorf("expected Allow-Headers, got %q", rec.Header().Get("Access-Control-Allow-Headers"))
	}

	if rec.Header().Get("Access-Control-Expose-Headers") != "X-Custom-Header" {
		t.Errorf("expected Expose-Headers, got %q", rec.Header().Get("Access-Control-Expose-Headers"))
	}

	if rec.Header().Get("Access-Control-Max-Age") != "3600" {
		t.Errorf("expected Max-Age 3600, got %q", rec.Header().Get("Access-Control-Max-Age"))
	}

	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected Allow-Credentials true, got %q", rec.Header().Get("Access-Control-Allow-Credentials"))
	}
}

func TestCORS_ExcludePaths(t *testing.T) {
	config := CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		ExcludePaths:   []string{"/v1/admin/"},
	}

	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		path       string
		expectCORS bool
	}{
		{
			name:       "regular endpoint has CORS",
			path:       "/v1/endpoints/test",
			expectCORS: true,
		},
		{
			name:       "admin endpoint excluded from CORS",
			path:       "/v1/admin/endpoints",
			expectCORS: false,
		},
		{
			name:       "nested admin path excluded",
			path:       "/v1/admin/endpoints/test",
			expectCORS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Origin", "https://example.com")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
			if tt.expectCORS {
				if allowOrigin == "" {
					t.Error("expected CORS headers")
				}
			} else {
				if allowOrigin != "" {
					t.Errorf("expected no CORS headers for admin path, got %q", allowOrigin)
				}
			}
		})
	}
}

func TestCORS_DefaultConfig(t *testing.T) {
	config := DefaultCORSConfig()

	if config.Enabled {
		t.Error("expected CORS to be disabled by default")
	}

	if len(config.AllowedMethods) == 0 {
		t.Error("expected default allowed methods")
	}

	if len(config.AllowedHeaders) == 0 {
		t.Error("expected default allowed headers")
	}

	if config.MaxAge != 86400 {
		t.Errorf("expected default max age 86400, got %d", config.MaxAge)
	}

	if !config.AllowCredentials {
		t.Error("expected credentials to be allowed by default")
	}

	if len(config.ExcludePaths) == 0 {
		t.Error("expected default exclude paths")
	}
}
