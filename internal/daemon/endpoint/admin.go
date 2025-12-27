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
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AdminHandler handles Admin API requests for endpoint management.
type AdminHandler struct {
	registry *Registry
	state    *State
}

// NewAdminHandler creates a new admin API handler.
func NewAdminHandler(registry *Registry, state *State) *AdminHandler {
	return &AdminHandler{
		registry: registry,
		state:    state,
	}
}

// RegisterRoutes registers admin API routes.
func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	// Admin endpoints require authentication and admin scope
	mux.HandleFunc("GET /v1/admin/endpoints", h.handleList)
	mux.HandleFunc("POST /v1/admin/endpoints", h.handleCreate)
	mux.HandleFunc("GET /v1/admin/endpoints/{name}", h.handleGet)
	mux.HandleFunc("PUT /v1/admin/endpoints/{name}", h.handleUpdate)
	mux.HandleFunc("DELETE /v1/admin/endpoints/{name}", h.handleDelete)
}

// handleList returns all endpoints.
func (h *AdminHandler) handleList(w http.ResponseWriter, r *http.Request) {
	endpoints := h.registry.List()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"endpoints": endpoints,
	})
}

// handleCreate creates a new endpoint.
func (h *AdminHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var ep Endpoint
	if err := json.NewDecoder(r.Body).Decode(&ep); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if ep.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if ep.Workflow == "" {
		http.Error(w, "workflow is required", http.StatusBadRequest)
		return
	}

	// Parse timeout if provided
	if timeoutStr, ok := r.URL.Query()["timeout"]; ok && len(timeoutStr) > 0 {
		timeout, err := time.ParseDuration(timeoutStr[0])
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid timeout: %v", err), http.StatusBadRequest)
			return
		}
		ep.Timeout = timeout
	}

	// Add to registry
	if err := h.registry.Add(&ep); err != nil {
		http.Error(w, fmt.Sprintf("failed to add endpoint: %v", err), http.StatusConflict)
		return
	}

	// Persist to state
	if err := h.state.Save(h.registry.List()); err != nil {
		// Rollback registry change
		h.registry.Remove(ep.Name)
		http.Error(w, fmt.Sprintf("failed to persist endpoint: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ep)
}

// handleGet returns a specific endpoint.
func (h *AdminHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	ep := h.registry.Get(name)
	if ep == nil {
		http.Error(w, "endpoint not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ep)
}

// handleUpdate updates an existing endpoint.
func (h *AdminHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Check if endpoint exists
	if h.registry.Get(name) == nil {
		http.Error(w, "endpoint not found", http.StatusNotFound)
		return
	}

	var ep Endpoint
	if err := json.NewDecoder(r.Body).Decode(&ep); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Ensure name matches URL parameter
	ep.Name = name

	// Validate required fields
	if ep.Workflow == "" {
		http.Error(w, "workflow is required", http.StatusBadRequest)
		return
	}

	// Update in registry
	if err := h.registry.Update(&ep); err != nil {
		http.Error(w, fmt.Sprintf("failed to update endpoint: %v", err), http.StatusInternalServerError)
		return
	}

	// Persist to state
	if err := h.state.Save(h.registry.List()); err != nil {
		http.Error(w, fmt.Sprintf("failed to persist endpoint: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ep)
}

// handleDelete removes an endpoint.
func (h *AdminHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Remove from registry
	if err := h.registry.Remove(name); err != nil {
		http.Error(w, "endpoint not found", http.StatusNotFound)
		return
	}

	// Persist to state
	if err := h.state.Save(h.registry.List()); err != nil {
		http.Error(w, fmt.Sprintf("failed to persist endpoint: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
