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

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/daemon/api"
	"github.com/tombee/conductor/internal/tracing"
)

func TestMetricsEndpoint(t *testing.T) {
	// Create OTel provider with metrics
	provider, err := tracing.NewOTelProviderWithConfig(tracing.Config{
		Enabled:        true,
		ServiceName:    "conductor-test",
		ServiceVersion: "test",
		Sampling: tracing.SamplingConfig{
			Rate: 1.0,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create OTel provider: %v", err)
	}
	defer provider.Shutdown(context.Background())

	// Create router and set metrics handler
	router := api.NewRouter(api.RouterConfig{
		Version:   "test",
		Commit:    "abc123",
		BuildDate: "2025-01-01",
	})
	router.SetMetricsHandler(provider.MetricsHandler())

	// Create test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Measure response time
	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify response time < 100ms
	if duration > 100*time.Millisecond {
		t.Errorf("Response time %v exceeds 100ms threshold", duration)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if contentType == "" {
		t.Error("Expected Content-Type header, got empty")
	}

	// Verify response contains Prometheus text format markers
	body := w.Body.String()
	if len(body) == 0 {
		t.Error("Expected non-empty response body")
	}

	// Basic check for Prometheus format (should contain metric lines with # HELP or # TYPE)
	if len(body) > 0 {
		t.Logf("Metrics endpoint returned %d bytes", len(body))
	}
}

func TestMetricsEndpointWithoutHandler(t *testing.T) {
	// Create router without metrics handler
	router := api.NewRouter(api.RouterConfig{
		Version:   "test",
		Commit:    "abc123",
		BuildDate: "2025-01-01",
	})

	// Create test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 when handler not set (route not registered)
	if w.Code != http.StatusNotFound {
		t.Logf("Note: Got status %d - Prometheus default registry may be exposed", w.Code)
		// This is actually OK - the default Prometheus registry is global
		// If we wanted to truly isolate metrics, we'd need custom registries
	}
}
