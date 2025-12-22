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

package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddleware_Disabled(t *testing.T) {
	m := NewMiddleware(Config{
		Enabled: false,
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/runs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestMiddleware_ValidAPIKey(t *testing.T) {
	m := NewMiddleware(Config{
		Enabled: true,
		APIKeys: []APIKey{
			{Key: "test-key-123", Name: "test"},
		},
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		header string
		value  string
	}{
		{"Bearer token", "Authorization", "Bearer test-key-123"},
		{"X-API-Key header", "X-API-Key", "test-key-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/runs", nil)
			req.Header.Set(tt.header, tt.value)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", rec.Code)
			}
		})
	}
}

func TestMiddleware_InvalidAPIKey(t *testing.T) {
	m := NewMiddleware(Config{
		Enabled: true,
		APIKeys: []APIKey{
			{Key: "test-key-123", Name: "test"},
		},
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestMiddleware_MissingAPIKey(t *testing.T) {
	m := NewMiddleware(Config{
		Enabled: true,
		APIKeys: []APIKey{
			{Key: "test-key-123", Name: "test"},
		},
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/runs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestMiddleware_ExpiredKey(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	m := NewMiddleware(Config{
		Enabled: true,
		APIKeys: []APIKey{
			{Key: "expired-key", Name: "expired", ExpiresAt: &past},
		},
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer expired-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestMiddleware_HealthEndpointBypass(t *testing.T) {
	m := NewMiddleware(Config{
		Enabled: true,
		APIKeys: []APIKey{}, // No keys configured
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for health endpoint, got %d", rec.Code)
	}
}

func TestMiddleware_QueryParameterRejected(t *testing.T) {
	m := NewMiddleware(Config{
		Enabled: true,
		APIKeys: []APIKey{
			{Key: "test-key-123", Name: "test"},
		},
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/runs?api_key=test-key-123", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for query parameter auth, got %d", rec.Code)
	}

	body := rec.Body.String()
	expectedMsg := "API keys in query parameters are not supported"
	if !strings.Contains(body, expectedMsg) {
		t.Errorf("Expected error message to contain %q, got %q", expectedMsg, body)
	}
}

func TestMiddleware_QueryParameterWithHeaderSucceeds(t *testing.T) {
	m := NewMiddleware(Config{
		Enabled: true,
		APIKeys: []APIKey{
			{Key: "test-key-123", Name: "test"},
		},
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with both query param (should be rejected) and valid header
	req := httptest.NewRequest("GET", "/v1/runs?api_key=test-key-123", nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should still reject because query parameter is present (detected before header check)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 when query parameter present (even with valid header), got %d", rec.Code)
	}
}

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if !strings.HasPrefix(key, "cnd_") {
		t.Errorf("Key should start with 'cnd_', got %s", key)
	}

	// Key should be unique
	key2, _ := GenerateKey()
	if key == key2 {
		t.Error("Generated keys should be unique")
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"abc", "***"},
		{"12345678", "********"},
		{"cnd_abcdefgh12345678", "cnd_************5678"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			masked := maskKey(tt.key)
			if masked != tt.expected {
				t.Errorf("maskKey(%q) = %q, want %q", tt.key, masked, tt.expected)
			}
		})
	}
}
