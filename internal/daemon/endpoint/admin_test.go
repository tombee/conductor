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

package endpoint

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminHandler_Create(t *testing.T) {
	registry := NewRegistry()
	state := NewState(t.TempDir())
	handler := NewAdminHandler(registry, state)

	ep := &Endpoint{
		Name:        "test-endpoint",
		Workflow:    "test.yaml",
		Description: "Test endpoint",
	}

	body, _ := json.Marshal(ep)
	req := httptest.NewRequest("POST", "/v1/admin/endpoints", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	// Verify endpoint was added to registry
	if got := registry.Get("test-endpoint"); got == nil {
		t.Error("endpoint not added to registry")
	}
}

func TestAdminHandler_CreateDuplicate(t *testing.T) {
	registry := NewRegistry()
	state := NewState(t.TempDir())
	handler := NewAdminHandler(registry, state)

	ep := &Endpoint{
		Name:     "test-endpoint",
		Workflow: "test.yaml",
	}
	registry.Add(ep)

	body, _ := json.Marshal(ep)
	req := httptest.NewRequest("POST", "/v1/admin/endpoints", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleCreate(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestAdminHandler_List(t *testing.T) {
	registry := NewRegistry()
	state := NewState(t.TempDir())
	handler := NewAdminHandler(registry, state)

	registry.Add(&Endpoint{Name: "ep1", Workflow: "w1.yaml"})
	registry.Add(&Endpoint{Name: "ep2", Workflow: "w2.yaml"})

	req := httptest.NewRequest("GET", "/v1/admin/endpoints", nil)
	w := httptest.NewRecorder()

	handler.handleList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	endpoints := resp["endpoints"].([]any)
	if len(endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(endpoints))
	}
}

func TestAdminHandler_Get(t *testing.T) {
	registry := NewRegistry()
	state := NewState(t.TempDir())
	handler := NewAdminHandler(registry, state)

	ep := &Endpoint{
		Name:     "test-endpoint",
		Workflow: "test.yaml",
	}
	registry.Add(ep)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/admin/endpoints/test-endpoint", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Endpoint
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "test-endpoint" {
		t.Errorf("expected name test-endpoint, got %s", resp.Name)
	}
}

func TestAdminHandler_GetNotFound(t *testing.T) {
	registry := NewRegistry()
	state := NewState(t.TempDir())
	handler := NewAdminHandler(registry, state)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/admin/endpoints/missing", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAdminHandler_Update(t *testing.T) {
	registry := NewRegistry()
	state := NewState(t.TempDir())
	handler := NewAdminHandler(registry, state)

	ep := &Endpoint{
		Name:     "test-endpoint",
		Workflow: "test.yaml",
	}
	registry.Add(ep)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	updated := &Endpoint{
		Name:        "test-endpoint",
		Workflow:    "updated.yaml",
		Description: "Updated",
	}

	body, _ := json.Marshal(updated)
	req := httptest.NewRequest("PUT", "/v1/admin/endpoints/test-endpoint", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify endpoint was updated
	got := registry.Get("test-endpoint")
	if got.Workflow != "updated.yaml" {
		t.Errorf("expected workflow updated.yaml, got %s", got.Workflow)
	}
}

func TestAdminHandler_Delete(t *testing.T) {
	registry := NewRegistry()
	state := NewState(t.TempDir())
	handler := NewAdminHandler(registry, state)

	ep := &Endpoint{
		Name:     "test-endpoint",
		Workflow: "test.yaml",
	}
	registry.Add(ep)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("DELETE", "/v1/admin/endpoints/test-endpoint", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify endpoint was removed
	if got := registry.Get("test-endpoint"); got != nil {
		t.Error("endpoint should have been removed")
	}
}
