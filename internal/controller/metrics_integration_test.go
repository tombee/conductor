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

package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tombee/conductor/pkg/security"
)

func TestSecurityMetricsIntegration(t *testing.T) {
	// Create security metrics collector
	metricsCollector := security.NewMetricsCollector()

	// Simulate some access decisions
	metricsCollector.RecordAccessDecision(security.AccessDecision{
		Allowed: true,
		Reason:  "test grant",
	}, security.ResourceTypeFile)

	metricsCollector.RecordAccessDecision(security.AccessDecision{
		Allowed: false,
		Reason:  "test deny",
	}, security.ResourceTypeNetwork)

	metricsCollector.RecordPermissionPrompt()

	// Create a simple OTel handler that returns minimal metrics
	otelHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Write([]byte("# OTel metrics placeholder\n"))
	})

	// Create combined handler
	handler := NewCombinedMetricsHandler(otelHandler, metricsCollector)

	// Create test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Verify OTel metrics are present
	if !strings.Contains(body, "# OTel metrics placeholder") {
		t.Error("Response should contain OTel metrics")
	}

	// Verify security metrics are present
	expectedMetrics := []string{
		"conductor_security_access_granted_total",
		"conductor_security_access_denied_total",
		"conductor_security_permission_prompts_total",
		"conductor_security_file_access_requests_total",
		"conductor_security_network_access_requests_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Response should contain security metric: %s", metric)
		}
	}

	// Verify metric values
	if !strings.Contains(body, "conductor_security_access_granted_total 1") {
		t.Error("Expected access_granted_total to be 1")
	}
	if !strings.Contains(body, "conductor_security_access_denied_total 1") {
		t.Error("Expected access_denied_total to be 1")
	}
	if !strings.Contains(body, "conductor_security_permission_prompts_total 1") {
		t.Error("Expected permission_prompts_total to be 1")
	}
}

func TestCombinedMetricsHandlerWithoutSecurity(t *testing.T) {
	// Create OTel-only handler
	otelHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Write([]byte("# OTel metrics only\n"))
	})

	// Create combined handler without security metrics
	handler := NewCombinedMetricsHandler(otelHandler, nil)

	// Create test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Verify OTel metrics are present
	if !strings.Contains(body, "# OTel metrics only") {
		t.Error("Response should contain OTel metrics")
	}

	// Verify no security metrics are present
	if strings.Contains(body, "conductor_security_") {
		t.Error("Response should not contain security metrics when collector is nil")
	}
}
