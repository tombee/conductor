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

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/security"
)

func TestOverrideAPI_CreateOverride(t *testing.T) {
	manager := security.NewOverrideManager(nil)
	handler := NewOverrideHandler(manager)

	tests := []struct {
		name           string
		requestBody    CreateOverrideRequest
		expectedStatus int
		expectError    bool
	}{
		{
			name: "valid override",
			requestBody: CreateOverrideRequest{
				Type:   "disable-enforcement",
				Reason: "Emergency production fix",
				TTL:    "30m",
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "default TTL",
			requestBody: CreateOverrideRequest{
				Type:   "disable-sandbox",
				Reason: "Testing",
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "missing reason",
			requestBody: CreateOverrideRequest{
				Type: "disable-enforcement",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "invalid type",
			requestBody: CreateOverrideRequest{
				Type:   "invalid-type",
				Reason: "Test",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "blocked disable-audit type",
			requestBody: CreateOverrideRequest{
				Type:   "disable-audit",
				Reason: "Trying to disable audit",
			},
			expectedStatus: http.StatusForbidden,
			expectError:    true,
		},
		{
			name: "invalid TTL format",
			requestBody: CreateOverrideRequest{
				Type:   "disable-enforcement",
				Reason: "Test",
				TTL:    "invalid",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/v1/override", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handler.HandleCreate(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if !tt.expectError {
				var response OverrideResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Type != tt.requestBody.Type {
					t.Errorf("Expected type %s, got %s", tt.requestBody.Type, response.Type)
				}

				if response.Reason != tt.requestBody.Reason {
					t.Errorf("Expected reason %q, got %q", tt.requestBody.Reason, response.Reason)
				}

				if response.CreatedAt.IsZero() {
					t.Error("CreatedAt should not be zero")
				}

				if response.ExpiresAt.IsZero() {
					t.Error("ExpiresAt should not be zero")
				}
			}
		})
	}
}

func TestOverrideAPI_ListOverrides(t *testing.T) {
	manager := security.NewOverrideManager(nil)
	handler := NewOverrideHandler(manager)

	// Create some overrides
	manager.ApplyWithTTL(security.OverrideDisableEnforcement, "Test 1", "api", time.Hour)
	manager.ApplyWithTTL(security.OverrideDisableSandbox, "Test 2", "api", time.Hour)

	req := httptest.NewRequest("GET", "/v1/override", nil)
	w := httptest.NewRecorder()

	handler.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response ListOverridesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Overrides) != 2 {
		t.Errorf("Expected 2 overrides, got %d", len(response.Overrides))
	}

	// Verify override fields
	for _, override := range response.Overrides {
		if override.Type == "" {
			t.Error("Override type should not be empty")
		}
		if override.Reason == "" {
			t.Error("Override reason should not be empty")
		}
		if override.CreatedAt.IsZero() {
			t.Error("Override CreatedAt should not be zero")
		}
		if override.ExpiresAt.IsZero() {
			t.Error("Override ExpiresAt should not be zero")
		}
	}
}

func TestOverrideAPI_RevokeOverride(t *testing.T) {
	manager := security.NewOverrideManager(nil)
	handler := NewOverrideHandler(manager)

	// Create an override
	manager.ApplyWithTTL(security.OverrideDisableEnforcement, "Test", "api", time.Hour)

	// Verify it exists
	if !manager.IsActive(security.OverrideDisableEnforcement) {
		t.Fatal("Override should be active")
	}

	// Revoke it
	req := httptest.NewRequest("DELETE", "/v1/override/disable-enforcement", nil)
	req.SetPathValue("type", "disable-enforcement")
	w := httptest.NewRecorder()

	handler.HandleRevoke(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify it's revoked
	if manager.IsActive(security.OverrideDisableEnforcement) {
		t.Error("Override should not be active after revocation")
	}
}

func TestOverrideAPI_RevokeNonExistent(t *testing.T) {
	manager := security.NewOverrideManager(nil)
	handler := NewOverrideHandler(manager)

	req := httptest.NewRequest("DELETE", "/v1/override/disable-enforcement", nil)
	req.SetPathValue("type", "disable-enforcement")
	w := httptest.NewRecorder()

	handler.HandleRevoke(w, req)

	// Should return error when revoking non-existent override
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestOverrideAPI_WithoutManager(t *testing.T) {
	handler := NewOverrideHandler(nil)

	tests := []struct {
		name    string
		method  string
		path    string
		body    interface{}
		handler func(http.ResponseWriter, *http.Request)
	}{
		{
			name:    "create without manager",
			method:  "POST",
			path:    "/v1/override",
			body:    CreateOverrideRequest{Type: "disable-enforcement", Reason: "Test"},
			handler: handler.HandleCreate,
		},
		{
			name:    "list without manager",
			method:  "GET",
			path:    "/v1/override",
			handler: handler.HandleList,
		},
		{
			name:    "revoke without manager",
			method:  "DELETE",
			path:    "/v1/override/disable-enforcement",
			handler: handler.HandleRevoke,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *bytes.Reader
			if tt.body != nil {
				bodyBytes, _ := json.Marshal(tt.body)
				body = bytes.NewReader(bodyBytes)
			} else {
				body = bytes.NewReader([]byte{})
			}

			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.method == "DELETE" {
				req.SetPathValue("type", "disable-enforcement")
			}
			w := httptest.NewRecorder()

			tt.handler(w, req)

			if w.Code != http.StatusServiceUnavailable {
				t.Errorf("Expected status 503, got %d", w.Code)
			}
		})
	}
}
