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
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/tombee/conductor/internal/daemon/runner"
)

// Handler handles endpoint-related HTTP requests.
type Handler struct {
	registry     *Registry
	runner       *runner.Runner
	workflowsDir string
	logger       *slog.Logger
}

// NewHandler creates a new endpoint handler.
func NewHandler(registry *Registry, r *runner.Runner, workflowsDir string) *Handler {
	return &Handler{
		registry:     registry,
		runner:       r,
		workflowsDir: workflowsDir,
		logger:       slog.Default().With(slog.String("component", "endpoint")),
	}
}

// RegisterRoutes registers endpoint API routes on the router.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/endpoints", h.handleList)
	mux.HandleFunc("GET /v1/endpoints/{name}", h.handleGet)
	mux.HandleFunc("POST /v1/endpoints/{name}/runs", h.handleCreateRun)
	mux.HandleFunc("GET /v1/endpoints/{name}/runs", h.handleListRuns)
}

// EndpointResponse represents an endpoint in API responses.
type EndpointResponse struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Inputs      map[string]any `json:"inputs,omitempty"`
}

// handleList handles GET /v1/endpoints.
// Returns a list of all available endpoints.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	endpoints := h.registry.List()

	// Convert to response format
	response := make([]EndpointResponse, 0, len(endpoints))
	for _, ep := range endpoints {
		response = append(response, EndpointResponse{
			Name:        ep.Name,
			Description: ep.Description,
			Inputs:      ep.Inputs,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"endpoints": response,
	})
}

// handleGet handles GET /v1/endpoints/{name}.
// Returns detailed metadata for a specific endpoint.
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "endpoint name is required")
		return
	}

	ep := h.registry.Get(name)
	if ep == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("endpoint %q not found", name))
		return
	}

	writeJSON(w, http.StatusOK, EndpointResponse{
		Name:        ep.Name,
		Description: ep.Description,
		Inputs:      ep.Inputs,
	})
}

// CreateRunRequest is the request body for creating an endpoint run.
type CreateRunRequest struct {
	Inputs    map[string]any `json:"inputs,omitempty"`
	Workspace string         `json:"workspace,omitempty"`
	Profile   string         `json:"profile,omitempty"`
}

// handleCreateRun handles POST /v1/endpoints/{name}/runs.
// Creates a new workflow run for the specified endpoint.
func (h *Handler) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	// Check if runner is draining (graceful shutdown in progress)
	if h.runner.IsDraining() {
		w.Header().Set("Retry-After", "10")
		writeError(w, http.StatusServiceUnavailable, "daemon is shutting down gracefully")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "endpoint name is required")
		return
	}

	// Get endpoint
	ep := h.registry.Get(name)
	if ep == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("endpoint %q not found", name))
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req CreateRunRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
			return
		}
	}

	// Merge endpoint default inputs with request inputs
	// Request inputs take precedence
	inputs := make(map[string]any)
	for k, v := range ep.Inputs {
		inputs[k] = v
	}
	for k, v := range req.Inputs {
		inputs[k] = v
	}

	// Find workflow file
	workflowPath, err := findWorkflow(ep.Workflow, h.workflowsDir)
	if err != nil {
		h.logger.Error("Workflow not found for endpoint",
			slog.String("endpoint", name),
			slog.String("workflow", ep.Workflow),
			slog.Any("error", err),
		)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("workflow %q not found", ep.Workflow))
		return
	}

	// Read workflow file
	workflowYAML, err := io.ReadAll(mustOpen(workflowPath))
	if err != nil {
		h.logger.Error("Failed to read workflow file",
			slog.String("endpoint", name),
			slog.String("path", workflowPath),
			slog.Any("error", err),
		)
		writeError(w, http.StatusInternalServerError, "failed to read workflow file")
		return
	}

	// Submit run
	run, err := h.runner.Submit(r.Context(), runner.SubmitRequest{
		WorkflowYAML: workflowYAML,
		Inputs:       inputs,
		Workspace:    req.Workspace,
		Profile:      req.Profile,
	})
	if err != nil {
		h.logger.Error("Failed to submit run",
			slog.String("endpoint", name),
			slog.Any("error", err),
		)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to submit run: %v", err))
		return
	}

	h.logger.Info("Endpoint run created",
		slog.String("endpoint", name),
		slog.String("run_id", run.ID),
		slog.String("workflow", ep.Workflow),
	)

	// Return 202 Accepted with run details
	w.Header().Set("Location", fmt.Sprintf("/v1/runs/%s", run.ID))
	writeJSON(w, http.StatusAccepted, run)
}

// handleListRuns handles GET /v1/endpoints/{name}/runs.
// Lists all runs for a specific endpoint.
func (h *Handler) handleListRuns(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "endpoint name is required")
		return
	}

	// Verify endpoint exists
	ep := h.registry.Get(name)
	if ep == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("endpoint %q not found", name))
		return
	}

	// Get all runs (no filter)
	runs := h.runner.List(runner.ListFilter{})

	// Filter runs for this endpoint's workflow
	// Note: This is a simple implementation that matches by workflow name.
	// In a production system, you might want to tag runs with endpoint name.
	endpointRuns := make([]*runner.RunSnapshot, 0)
	for _, run := range runs {
		// Match runs that use the same workflow as this endpoint
		if run.WorkflowID == ep.Name || containsPath(run.SourceURL, ep.Workflow) {
			endpointRuns = append(endpointRuns, run)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"runs": endpointRuns,
	})
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to write JSON response", slog.Any("error", err))
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func mustOpen(path string) io.ReadCloser {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	return f
}

func containsPath(sourceURL, workflow string) bool {
	// Simple check if workflow name appears in source URL
	// This is a heuristic for matching runs to endpoints
	if sourceURL == "" || workflow == "" {
		return false
	}
	return sourceURL == workflow || strings.Contains(sourceURL, workflow)
}
