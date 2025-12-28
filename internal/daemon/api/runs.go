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
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tombee/conductor/internal/daemon/runner"
)

// RunsHandler handles run-related API requests.
type RunsHandler struct {
	runner *runner.Runner
}

// NewRunsHandler creates a new runs handler.
func NewRunsHandler(r *runner.Runner) *RunsHandler {
	return &RunsHandler{runner: r}
}

// RegisterRoutes registers run API routes on the router.
func (h *RunsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/runs", h.handleCreate)
	mux.HandleFunc("GET /v1/runs", h.handleList)
	mux.HandleFunc("GET /v1/runs/{id}", h.handleGet)
	mux.HandleFunc("GET /v1/runs/{id}/output", h.handleGetOutput)
	mux.HandleFunc("GET /v1/runs/{id}/logs", h.handleGetLogs)
	mux.HandleFunc("DELETE /v1/runs/{id}", h.handleCancel)
}

// CreateRunRequest is the request body for creating a run.
type CreateRunRequest struct {
	Workflow  string         `json:"workflow"`
	Inputs    map[string]any `json:"inputs,omitempty"`
	Workspace string         `json:"workspace,omitempty"` // Workspace for profile resolution
	Profile   string         `json:"profile,omitempty"`   // Profile for binding resolution
}

// handleCreate handles POST /v1/runs.
func (h *RunsHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	// Check if runner is draining (graceful shutdown in progress)
	if h.runner.IsDraining() {
		w.Header().Set("Retry-After", "10")
		writeError(w, http.StatusServiceUnavailable, "daemon is shutting down gracefully")
		return
	}

	// Check for remote reference in query params
	remoteRef := r.URL.Query().Get("remote_ref")
	noCache := r.URL.Query().Get("no_cache") == "true"
	workspace := r.URL.Query().Get("workspace")
	profile := r.URL.Query().Get("profile")

	// If remote ref is provided, workflow YAML in body should be empty
	if remoteRef != "" {
		// Remote workflow - ignore body and use ref
		// Parse inputs from query params (excluding reserved params)
		inputs := make(map[string]any)
		for key, values := range r.URL.Query() {
			if key != "remote_ref" && key != "no_cache" && key != "workspace" && key != "profile" && len(values) > 0 {
				inputs[key] = values[0]
			}
		}

		// Submit remote workflow run
		run, err := h.runner.Submit(r.Context(), runner.SubmitRequest{
			RemoteRef: remoteRef,
			NoCache:   noCache,
			Inputs:    inputs,
			Workspace: workspace,
			Profile:   profile,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to submit run: %v", err))
			return
		}

		writeJSON(w, http.StatusAccepted, run)
		return
	}

	// Not a remote ref - handle local workflow submission
	// Check content type
	contentType := r.Header.Get("Content-Type")

	var req CreateRunRequest
	var workflowYAML []byte

	if strings.HasPrefix(contentType, "application/json") {
		// JSON request with workflow name reference
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		// For now, workflow must be provided inline in a workflow_yaml field
		// In the future, we'll support referencing workflows by name
		writeError(w, http.StatusBadRequest, "workflow_yaml field required (workflow reference not yet supported)")
		return
	} else if strings.HasPrefix(contentType, "application/x-yaml") || strings.HasPrefix(contentType, "text/yaml") {
		// YAML workflow directly in body
		var err error
		workflowYAML, err = io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to read workflow: %v", err))
			return
		}

		// Parse inputs from query params
		req.Inputs = make(map[string]any)
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				req.Inputs[key] = values[0]
			}
		}
	} else if strings.HasPrefix(contentType, "multipart/form-data") {
		// Multipart form with workflow file
		if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB limit
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse form: %v", err))
			return
		}

		file, _, err := r.FormFile("workflow")
		if err != nil {
			writeError(w, http.StatusBadRequest, "workflow file required")
			return
		}
		defer file.Close()

		workflowYAML, err = io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to read workflow file: %v", err))
			return
		}

		// Parse inputs from form
		req.Inputs = make(map[string]any)
		for key, values := range r.Form {
			if key != "workflow" && len(values) > 0 {
				req.Inputs[key] = values[0]
			}
		}
	} else {
		writeError(w, http.StatusUnsupportedMediaType, "content-type must be application/json, application/x-yaml, or multipart/form-data")
		return
	}

	// Submit run
	// Prefer workspace/profile from query params, fall back to request body
	submitWorkspace := workspace
	submitProfile := profile
	if submitWorkspace == "" {
		submitWorkspace = req.Workspace
	}
	if submitProfile == "" {
		submitProfile = req.Profile
	}

	run, err := h.runner.Submit(r.Context(), runner.SubmitRequest{
		WorkflowYAML: workflowYAML,
		Inputs:       req.Inputs,
		Workspace:    submitWorkspace,
		Profile:      submitProfile,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to submit run: %v", err))
		return
	}

	writeJSON(w, http.StatusAccepted, run)
}

// handleList handles GET /v1/runs.
func (h *RunsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	filter := runner.ListFilter{}

	// Parse query parameters
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = runner.RunStatus(status)
	}
	if workflow := r.URL.Query().Get("workflow"); workflow != "" {
		filter.Workflow = workflow
	}

	runs := h.runner.List(filter)
	writeJSON(w, http.StatusOK, map[string]any{
		"runs":  runs,
		"count": len(runs),
	})
}

// handleGet handles GET /v1/runs/{id}.
func (h *RunsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "run ID required")
		return
	}

	run, err := h.runner.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// handleGetOutput handles GET /v1/runs/{id}/output.
func (h *RunsHandler) handleGetOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "run ID required")
		return
	}

	run, err := h.runner.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if run.Status != runner.RunStatusCompleted {
		writeError(w, http.StatusConflict, fmt.Sprintf("run not completed (status: %s)", run.Status))
		return
	}

	writeJSON(w, http.StatusOK, run.Output)
}

// handleGetLogs handles GET /v1/runs/{id}/logs.
// Supports SSE streaming with Accept: text/event-stream.
func (h *RunsHandler) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "run ID required")
		return
	}

	run, err := h.runner.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Check if client wants SSE streaming
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/event-stream") {
		h.streamLogs(w, r, run)
		return
	}

	// Return existing logs as JSON
	writeJSON(w, http.StatusOK, map[string]any{
		"logs":  run.Logs,
		"count": len(run.Logs),
	})
}

// streamLogs streams logs via SSE.
func (h *RunsHandler) streamLogs(w http.ResponseWriter, r *http.Request, run *runner.RunSnapshot) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Send existing logs first
	for _, entry := range run.Logs {
		data, _ := json.Marshal(entry)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	// If run is complete, close the stream
	if run.Status == runner.RunStatusCompleted ||
		run.Status == runner.RunStatusFailed ||
		run.Status == runner.RunStatusCancelled {
		fmt.Fprintf(w, "event: done\ndata: {\"status\":\"%s\"}\n\n", run.Status)
		flusher.Flush()
		return
	}

	// Subscribe to new logs
	logCh, unsub := h.runner.Subscribe(run.ID)
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case entry, ok := <-logCh:
			if !ok {
				return
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Check if this is a completion log
			if entry.Level == "info" && (strings.Contains(entry.Message, "completed") ||
				strings.Contains(entry.Message, "failed") ||
				strings.Contains(entry.Message, "cancelled")) {
				// Get updated run status
				updatedRun, _ := h.runner.Get(run.ID)
				if updatedRun != nil && (updatedRun.Status == runner.RunStatusCompleted ||
					updatedRun.Status == runner.RunStatusFailed ||
					updatedRun.Status == runner.RunStatusCancelled) {
					fmt.Fprintf(w, "event: done\ndata: {\"status\":\"%s\"}\n\n", updatedRun.Status)
					flusher.Flush()
					return
				}
			}
		}
	}
}

// handleCancel handles DELETE /v1/runs/{id}.
func (h *RunsHandler) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "run ID required")
		return
	}

	if err := h.runner.Cancel(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusConflict, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "cancelled",
		"message": "run cancelled successfully",
	})
}
