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
	"fmt"
	"net/http"

	"github.com/tombee/conductor/internal/controller/backend"
)

// handleGetStep handles GET /v1/runs/{id}/steps/{step-id}.
func (h *RunsHandler) handleGetStep(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	stepID := r.PathValue("step_id")

	if runID == "" {
		writeError(w, http.StatusBadRequest, "run ID required")
		return
	}

	if stepID == "" {
		writeError(w, http.StatusBadRequest, "step ID required")
		return
	}

	// Get the step result from backend storage
	store, ok := h.runner.Backend().(backend.StepResultStore)
	if !ok {
		writeError(w, http.StatusNotImplemented, "step result storage not supported by backend")
		return
	}

	result, err := store.GetStepResult(r.Context(), runID, stepID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("step result not found: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleListSteps handles GET /v1/runs/{id}/steps.
func (h *RunsHandler) handleListSteps(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")

	if runID == "" {
		writeError(w, http.StatusBadRequest, "run ID required")
		return
	}

	// Get all step results for the run from backend storage
	store, ok := h.runner.Backend().(backend.StepResultStore)
	if !ok {
		writeError(w, http.StatusNotImplemented, "step result storage not supported by backend")
		return
	}

	results, err := store.ListStepResults(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list step results: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"steps": results,
		"count": len(results),
	})
}
