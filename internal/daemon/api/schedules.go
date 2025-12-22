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
	"net/http"

	"github.com/tombee/conductor/internal/daemon/scheduler"
)

// SchedulesHandler handles schedule-related API requests.
type SchedulesHandler struct {
	scheduler *scheduler.Scheduler
}

// NewSchedulesHandler creates a new schedules handler.
func NewSchedulesHandler(s *scheduler.Scheduler) *SchedulesHandler {
	return &SchedulesHandler{scheduler: s}
}

// RegisterRoutes registers schedule API routes on the router.
func (h *SchedulesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/schedules", h.handleList)
	mux.HandleFunc("GET /v1/schedules/{name}", h.handleGet)
	mux.HandleFunc("POST /v1/schedules/{name}/enable", h.handleEnable)
	mux.HandleFunc("POST /v1/schedules/{name}/disable", h.handleDisable)
}

// handleList returns all schedules.
func (h *SchedulesHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"schedules": []any{},
		})
		return
	}

	statuses := h.scheduler.GetStatus()
	writeJSON(w, http.StatusOK, map[string]any{
		"schedules": statuses,
	})
}

// handleGet returns a specific schedule.
func (h *SchedulesHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "schedule name required")
		return
	}

	if h.scheduler == nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	sched, ok := h.scheduler.GetSchedule(name)
	if !ok {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	statuses := h.scheduler.GetStatus()
	for _, status := range statuses {
		if status.Name == name {
			writeJSON(w, http.StatusOK, status)
			return
		}
	}

	// Fallback to basic info
	writeJSON(w, http.StatusOK, map[string]any{
		"name":     sched.Name,
		"cron":     sched.Cron,
		"workflow": sched.Workflow,
		"enabled":  sched.Enabled,
	})
}

// handleEnable enables a schedule.
func (h *SchedulesHandler) handleEnable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "schedule name required")
		return
	}

	if h.scheduler == nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	if err := h.scheduler.SetEnabled(name, true); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "enabled",
		"message": "Schedule enabled",
	})
}

// handleDisable disables a schedule.
func (h *SchedulesHandler) handleDisable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "schedule name required")
		return
	}

	if h.scheduler == nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	if err := h.scheduler.SetEnabled(name, false); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "disabled",
		"message": "Schedule disabled",
	})
}

