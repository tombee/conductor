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

package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewCorrelationID(t *testing.T) {
	id := NewCorrelationID()

	if id == "" {
		t.Error("expected non-empty correlation ID")
	}

	if !id.IsValid() {
		t.Errorf("expected valid UUID format, got %q", id)
	}

	// Verify length is 36 (UUID format)
	if len(id) != 36 {
		t.Errorf("expected length 36, got %d", len(id))
	}
}

func TestCorrelationID_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		id    CorrelationID
		valid bool
	}{
		{"valid UUID", CorrelationID("550e8400-e29b-41d4-a716-446655440000"), true},
		{"valid UUID uppercase", CorrelationID("550E8400-E29B-41D4-A716-446655440000"), true},
		{"valid UUID mixed case", CorrelationID("550e8400-E29b-41d4-A716-446655440000"), true},
		{"empty", CorrelationID(""), false},
		{"too short", CorrelationID("550e8400-e29b-41d4"), false},
		{"too long", CorrelationID("550e8400-e29b-41d4-a716-446655440000-extra"), false},
		{"missing hyphens", CorrelationID("550e8400e29b41d4a716446655440000"), false},
		{"invalid characters", CorrelationID("550e8400-e29b-41d4-a716-44665544000g"), false},
		{"spaces", CorrelationID("550e8400 e29b-41d4-a716-446655440000"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestToContext_FromContext(t *testing.T) {
	ctx := context.Background()
	id := CorrelationID("550e8400-e29b-41d4-a716-446655440000")

	// Add to context
	ctx = ToContext(ctx, id)

	// Retrieve from context
	got := FromContext(ctx)
	if got != id {
		t.Errorf("FromContext() = %q, want %q", got, id)
	}
}

func TestFromContext_GeneratesNew(t *testing.T) {
	ctx := context.Background()

	// Should generate new ID when not in context
	got := FromContext(ctx)
	if got == "" {
		t.Error("FromContext() returned empty string, expected new ID")
	}

	if !got.IsValid() {
		t.Errorf("FromContext() returned invalid UUID: %q", got)
	}
}

func TestFromContextOrEmpty(t *testing.T) {
	t.Run("returns ID when present", func(t *testing.T) {
		ctx := context.Background()
		id := CorrelationID("550e8400-e29b-41d4-a716-446655440000")
		ctx = ToContext(ctx, id)

		got := FromContextOrEmpty(ctx)
		if got != id {
			t.Errorf("FromContextOrEmpty() = %q, want %q", got, id)
		}
	})

	t.Run("returns empty when not present", func(t *testing.T) {
		ctx := context.Background()

		got := FromContextOrEmpty(ctx)
		if got != "" {
			t.Errorf("FromContextOrEmpty() = %q, want empty string", got)
		}
	})
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid UUID", "550e8400-e29b-41d4-a716-446655440000", true},
		{"invalid format", "not-a-uuid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := ValidateUUID(tt.input)
			if ok != tt.valid {
				t.Errorf("ValidateUUID() ok = %v, want %v", ok, tt.valid)
			}
			if ok && string(id) != tt.input {
				t.Errorf("ValidateUUID() id = %q, want %q", id, tt.input)
			}
		})
	}
}

func TestExtractFromRequest(t *testing.T) {
	tests := []struct {
		name      string
		headers   map[string]string
		wantID    CorrelationID
		wantFound bool
	}{
		{
			name:      "X-Correlation-ID header",
			headers:   map[string]string{"X-Correlation-ID": "550e8400-e29b-41d4-a716-446655440000"},
			wantID:    "550e8400-e29b-41d4-a716-446655440000",
			wantFound: true,
		},
		{
			name:      "X-Request-ID fallback",
			headers:   map[string]string{"X-Request-ID": "660e8400-e29b-41d4-a716-446655440000"},
			wantID:    "660e8400-e29b-41d4-a716-446655440000",
			wantFound: true,
		},
		{
			name: "X-Correlation-ID takes precedence",
			headers: map[string]string{
				"X-Correlation-ID": "550e8400-e29b-41d4-a716-446655440000",
				"X-Request-ID":     "660e8400-e29b-41d4-a716-446655440000",
			},
			wantID:    "550e8400-e29b-41d4-a716-446655440000",
			wantFound: true,
		},
		{
			name:      "no header",
			headers:   map[string]string{},
			wantID:    "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			id, found := ExtractFromRequest(req)
			if found != tt.wantFound {
				t.Errorf("ExtractFromRequest() found = %v, want %v", found, tt.wantFound)
			}
			if id != tt.wantID {
				t.Errorf("ExtractFromRequest() id = %q, want %q", id, tt.wantID)
			}
		})
	}
}

func TestInjectIntoRequest(t *testing.T) {
	t.Run("injects ID from context", func(t *testing.T) {
		ctx := context.Background()
		id := CorrelationID("550e8400-e29b-41d4-a716-446655440000")
		ctx = ToContext(ctx, id)

		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(ctx)

		InjectIntoRequest(ctx, req)

		got := req.Header.Get(HeaderCorrelationID)
		if got != string(id) {
			t.Errorf("header = %q, want %q", got, id)
		}
	})

	t.Run("no header when no ID in context", func(t *testing.T) {
		ctx := context.Background()
		req := httptest.NewRequest("GET", "/test", nil)

		InjectIntoRequest(ctx, req)

		got := req.Header.Get(HeaderCorrelationID)
		if got != "" {
			t.Errorf("header = %q, want empty", got)
		}
	})
}

func TestInjectIntoResponse(t *testing.T) {
	t.Run("injects ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		id := CorrelationID("550e8400-e29b-41d4-a716-446655440000")

		InjectIntoResponse(w, id)

		got := w.Header().Get(HeaderCorrelationID)
		if got != string(id) {
			t.Errorf("header = %q, want %q", got, id)
		}
	})

	t.Run("no header for empty ID", func(t *testing.T) {
		w := httptest.NewRecorder()

		InjectIntoResponse(w, "")

		got := w.Header().Get(HeaderCorrelationID)
		if got != "" {
			t.Errorf("header = %q, want empty", got)
		}
	})
}

func TestCorrelationMiddleware(t *testing.T) {
	t.Run("uses provided valid ID", func(t *testing.T) {
		handler := CorrelationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := FromContext(r.Context())
			if id != "550e8400-e29b-41d4-a716-446655440000" {
				t.Errorf("context ID = %q, want %q", id, "550e8400-e29b-41d4-a716-446655440000")
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", "550e8400-e29b-41d4-a716-446655440000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}

		// Check response header
		respID := w.Header().Get("X-Correlation-ID")
		if respID != "550e8400-e29b-41d4-a716-446655440000" {
			t.Errorf("response header = %q, want %q", respID, "550e8400-e29b-41d4-a716-446655440000")
		}
	})

	t.Run("rejects invalid ID", func(t *testing.T) {
		handler := CorrelationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for invalid ID")
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", "not-a-valid-uuid")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("generates new ID when none provided", func(t *testing.T) {
		var capturedID CorrelationID
		handler := CorrelationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = FromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}

		if capturedID == "" {
			t.Error("expected generated correlation ID")
		}

		if !capturedID.IsValid() {
			t.Errorf("generated ID is not valid: %q", capturedID)
		}

		// Check response header contains the generated ID
		respID := w.Header().Get("X-Correlation-ID")
		if respID != string(capturedID) {
			t.Errorf("response header = %q, want %q", respID, capturedID)
		}
	})
}

func TestCorrelationRoundTripper(t *testing.T) {
	t.Run("injects correlation ID", func(t *testing.T) {
		var capturedHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedHeader = r.Header.Get(HeaderCorrelationID)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx := context.Background()
		id := CorrelationID("550e8400-e29b-41d4-a716-446655440000")
		ctx = ToContext(ctx, id)

		client := WrapHTTPClient(nil)
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)

		_, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		if capturedHeader != string(id) {
			t.Errorf("server received header = %q, want %q", capturedHeader, id)
		}
	})

	t.Run("no header when no ID in context", func(t *testing.T) {
		var capturedHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedHeader = r.Header.Get(HeaderCorrelationID)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := WrapHTTPClient(nil)
		req, _ := http.NewRequest("GET", server.URL, nil)

		_, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		if capturedHeader != "" {
			t.Errorf("server received header = %q, want empty", capturedHeader)
		}
	})
}

func TestWrapHTTPClient(t *testing.T) {
	t.Run("preserves client settings", func(t *testing.T) {
		original := &http.Client{
			Timeout: 30,
		}

		wrapped := WrapHTTPClient(original)

		if wrapped.Timeout != original.Timeout {
			t.Errorf("timeout = %v, want %v", wrapped.Timeout, original.Timeout)
		}
	})

	t.Run("handles nil client", func(t *testing.T) {
		wrapped := WrapHTTPClient(nil)
		if wrapped == nil {
			t.Error("expected non-nil client")
		}
	})
}

func BenchmarkNewCorrelationID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewCorrelationID()
	}
}

func BenchmarkFromContext(b *testing.B) {
	ctx := context.Background()
	id := CorrelationID("550e8400-e29b-41d4-a716-446655440000")
	ctx = ToContext(ctx, id)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FromContext(ctx)
	}
}

func BenchmarkIsValid(b *testing.B) {
	id := CorrelationID("550e8400-e29b-41d4-a716-446655440000")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.IsValid()
	}
}
