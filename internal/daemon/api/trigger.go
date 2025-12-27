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
	"os"
	"path/filepath"
	"strings"

	"github.com/tombee/conductor/internal/daemon/runner"
)

// TriggerHandler handles workflow trigger endpoints.
type TriggerHandler struct {
	runner       *runner.Runner
	workflowsDir string
}

// NewTriggerHandler creates a new trigger handler.
func NewTriggerHandler(r *runner.Runner, workflowsDir string) *TriggerHandler {
	return &TriggerHandler{
		runner:       r,
		workflowsDir: workflowsDir,
	}
}

// RegisterRoutes registers trigger routes on the given mux.
func (h *TriggerHandler) RegisterRoutes(mux *http.ServeMux) {
	// Trigger workflow by name
	mux.HandleFunc("POST /run/{workflow}", h.handleTrigger)
	mux.HandleFunc("POST /run/{workflow}/{rest...}", h.handleTrigger)
}

// handleTrigger triggers a workflow by name.
func (h *TriggerHandler) handleTrigger(w http.ResponseWriter, r *http.Request) {
	// Check if runner is draining (graceful shutdown in progress)
	if h.runner.IsDraining() {
		w.Header().Set("Retry-After", "10")
		writeError(w, http.StatusServiceUnavailable, "daemon is shutting down gracefully")
		return
	}

	workflowName := r.PathValue("workflow")
	if workflowName == "" {
		writeError(w, http.StatusBadRequest, "workflow name required")
		return
	}

	// Clean the workflow name to prevent directory traversal
	workflowName = filepath.Clean(workflowName)
	if strings.Contains(workflowName, "..") {
		writeError(w, http.StatusBadRequest, "invalid workflow name")
		return
	}

	// Try to find the workflow file
	workflowPath, err := h.findWorkflow(workflowName)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("workflow not found: %s", workflowName))
		return
	}

	// Read workflow file
	workflowYAML, err := os.ReadFile(workflowPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read workflow: %v", err))
		return
	}

	// Parse inputs from request body
	var inputs map[string]any
	if r.ContentLength > 0 {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read request body")
			return
		}

		if len(body) > 0 {
			if err := json.Unmarshal(body, &inputs); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
				return
			}
		}
	}

	// Also accept inputs from query parameters
	for key, values := range r.URL.Query() {
		if inputs == nil {
			inputs = make(map[string]any)
		}
		if len(values) == 1 {
			inputs[key] = values[0]
		} else {
			inputs[key] = values
		}
	}

	// Submit the workflow
	run, err := h.runner.Submit(r.Context(), runner.SubmitRequest{
		WorkflowYAML: workflowYAML,
		Inputs:       inputs,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to submit workflow: %v", err))
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":       run.ID,
		"workflow": run.Workflow,
		"status":   run.Status,
		"message":  "Workflow triggered successfully",
	})
}

// findWorkflow looks for a workflow file by name.
func (h *TriggerHandler) findWorkflow(name string) (string, error) {
	// Try various extensions and locations
	extensions := []string{".yaml", ".yml", ""}
	baseDirs := []string{h.workflowsDir, "."}

	for _, baseDir := range baseDirs {
		if baseDir == "" {
			continue
		}
		for _, ext := range extensions {
			path := filepath.Join(baseDir, name+ext)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("workflow not found: %s", name)
}
