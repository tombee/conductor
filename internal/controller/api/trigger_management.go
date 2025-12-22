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
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/tombee/conductor/internal/triggers"
)

// TriggerManagementHandler handles trigger configuration management API endpoints.
type TriggerManagementHandler struct {
	manager *triggers.Manager
}

// NewTriggerManagementHandler creates a new trigger management handler.
func NewTriggerManagementHandler(manager *triggers.Manager) *TriggerManagementHandler {
	return &TriggerManagementHandler{
		manager: manager,
	}
}

// HandleCreateWebhook handles POST /v1/triggers/webhooks.
func (h *TriggerManagementHandler) HandleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req triggers.CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	if err := h.manager.AddWebhook(ctx, req); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"path": req.Path,
		"url":  getWebhookURLFromRequest(r, req.Path),
	})
}

// HandleListWebhooks handles GET /v1/triggers/webhooks.
func (h *TriggerManagementHandler) HandleListWebhooks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhooks, err := h.manager.ListWebhooks(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list webhooks")
		return
	}

	writeJSON(w, http.StatusOK, webhooks)
}

// HandleDeleteWebhook handles DELETE /v1/triggers/webhooks/{path...}.
func (h *TriggerManagementHandler) HandleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	encodedPath := r.PathValue("path")
	path, err := url.PathUnescape(encodedPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid path encoding")
		return
	}

	// Reconstruct the full path with slashes
	path = "/" + path

	ctx := r.Context()
	if err := h.manager.RemoveWebhook(ctx, path); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "Failed to remove webhook")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleCreateSchedule handles POST /v1/triggers/schedules.
func (h *TriggerManagementHandler) HandleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req triggers.CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	if err := h.manager.AddSchedule(ctx, req); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"name": req.Name,
	})
}

// HandleListSchedules handles GET /v1/triggers/schedules.
func (h *TriggerManagementHandler) HandleListSchedules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	schedules, err := h.manager.ListSchedules(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list schedules")
		return
	}

	writeJSON(w, http.StatusOK, schedules)
}

// HandleDeleteSchedule handles DELETE /v1/triggers/schedules/{name}.
func (h *TriggerManagementHandler) HandleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	ctx := r.Context()
	if err := h.manager.RemoveSchedule(ctx, name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "Failed to remove schedule")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleCreateEndpoint handles POST /v1/triggers/endpoints.
func (h *TriggerManagementHandler) HandleCreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var req triggers.CreateEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	if err := h.manager.AddEndpoint(ctx, req); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"name": req.Name,
		"url":  getEndpointURLFromRequest(r, req.Name),
	})
}

// HandleListEndpoints handles GET /v1/triggers/endpoints.
func (h *TriggerManagementHandler) HandleListEndpoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	endpoints, err := h.manager.ListEndpoints(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list endpoints")
		return
	}

	writeJSON(w, http.StatusOK, endpoints)
}

// HandleDeleteEndpoint handles DELETE /v1/triggers/endpoints/{name}.
func (h *TriggerManagementHandler) HandleDeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	ctx := r.Context()
	if err := h.manager.RemoveEndpoint(ctx, name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "Failed to remove endpoint")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleListAll handles GET /v1/triggers.
func (h *TriggerManagementHandler) HandleListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	webhooks, err := h.manager.ListWebhooks(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list webhooks")
		return
	}

	schedules, err := h.manager.ListSchedules(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list schedules")
		return
	}

	endpoints, err := h.manager.ListEndpoints(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list endpoints")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"webhooks":  webhooks,
		"schedules": schedules,
		"endpoints": endpoints,
	})
}

// Helper functions

func getWebhookURLFromRequest(r *http.Request, path string) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + r.Host + path
}

func getEndpointURLFromRequest(r *http.Request, name string) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + r.Host + "/v1/endpoints/" + name
}
