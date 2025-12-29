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

	"github.com/tombee/conductor/internal/controller/auth"
	"github.com/tombee/conductor/internal/controller/runner"
	"github.com/tombee/conductor/pkg/workflow"
)

const maxRequestBodySize = 1 * 1024 * 1024 // 1MB

// StartHandler handles POST /v1/start/{workflow} for the public API.
// This endpoint triggers workflows that have listen.api configured.
type StartHandler struct {
	runner       *runner.Runner
	workflowsDir string
}

// NewStartHandler creates a new start handler.
func NewStartHandler(r *runner.Runner, workflowsDir string) *StartHandler {
	return &StartHandler{
		runner:       r,
		workflowsDir: workflowsDir,
	}
}

// RegisterRoutes registers the start endpoint routes.
func (h *StartHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/start/{workflow}", h.handleStart)
}

// handleStart handles POST /v1/start/{workflow}.
// Requires Bearer token authentication matching the workflow's listen.api.secret.
func (h *StartHandler) handleStart(w http.ResponseWriter, r *http.Request) {
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

	// Find and load the workflow
	workflowPath, err := h.findWorkflow(workflowName)
	if err != nil {
		// Return 404 to prevent workflow enumeration
		// Don't reveal whether the workflow exists but lacks listen.api config
		writeError(w, http.StatusNotFound, "workflow not found or not available via API")
		return
	}

	// Read and parse workflow definition
	workflowYAML, err := os.ReadFile(workflowPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow not found or not available via API")
		return
	}

	def, err := workflow.ParseDefinition(workflowYAML)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse workflow")
		return
	}

	// Verify workflow has listen.api configured
	if def.Listen == nil || def.Listen.API == nil {
		// Return 404 to prevent enumeration of workflows without API access
		writeError(w, http.StatusNotFound, "workflow not found or not available via API")
		return
	}

	// Extract secret (expand environment variables if needed)
	secret := def.Listen.API.Secret
	if strings.HasPrefix(secret, "${") && strings.HasSuffix(secret, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(secret, "${"), "}")
		secret = os.Getenv(envVar)
	}

	if secret == "" {
		writeError(w, http.StatusInternalServerError, "workflow API secret not configured")
		return
	}

	// Extract Bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Conductor API"`)
		writeError(w, http.StatusUnauthorized, "missing Authorization header")
		return
	}

	// Parse Bearer token
	if !strings.HasPrefix(authHeader, "Bearer ") {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Conductor API"`)
		writeError(w, http.StatusUnauthorized, "invalid Authorization header format")
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	token = strings.TrimSpace(token)

	if token == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Conductor API"`)
		writeError(w, http.StatusUnauthorized, "empty Bearer token")
		return
	}

	// Verify the Bearer token against the workflow's secret
	authenticator := auth.NewBearerAuthenticator()
	if !authenticator.VerifyToken(token, secret) {
		writeError(w, http.StatusUnauthorized, "invalid Bearer token")
		return
	}

	// Parse inputs from request body (limit to 1MB)
	var inputs map[string]any
	if r.ContentLength > maxRequestBodySize {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large (max 1MB)")
		return
	}

	if r.ContentLength > 0 {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodySize))
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
		"message":  "Workflow started successfully",
	})
}

// findWorkflow looks for a workflow file by name.
// This is the same logic as trigger.go to maintain consistency.
func (h *StartHandler) findWorkflow(name string) (string, error) {
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
