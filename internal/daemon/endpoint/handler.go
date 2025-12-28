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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/daemon/auth"
	"github.com/tombee/conductor/internal/daemon/httputil"
	"github.com/tombee/conductor/internal/daemon/runner"
	"github.com/tombee/conductor/pkg/workflow"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gopkg.in/yaml.v3"
)

// Handler handles endpoint-related HTTP requests.
type Handler struct {
	registry     *Registry
	runner       *runner.Runner
	workflowsDir string
	rateLimiter  *auth.NamedRateLimiter
	logger       *slog.Logger

	// Metrics
	requestsTotal        metric.Int64Counter
	requestDuration      metric.Float64Histogram
	rateLimitExceeded    metric.Int64Counter
}

// NewHandler creates a new endpoint handler.
func NewHandler(registry *Registry, r *runner.Runner, workflowsDir string) *Handler {
	return &Handler{
		registry:     registry,
		runner:       r,
		workflowsDir: workflowsDir,
		rateLimiter:  auth.NewNamedRateLimiter(),
		logger:       slog.Default().With(slog.String("component", "endpoint")),
	}
}

// SetRateLimiter sets the rate limiter for this handler.
// This allows external configuration of rate limits.
func (h *Handler) SetRateLimiter(rl *auth.NamedRateLimiter) {
	h.rateLimiter = rl
}

// SetMetrics initializes metrics for the handler using the given meter provider.
func (h *Handler) SetMetrics(meterProvider metric.MeterProvider) error {
	meter := meterProvider.Meter("conductor")

	var err error

	h.requestsTotal, err = meter.Int64Counter(
		"conductor_endpoint_requests_total",
		metric.WithDescription("Total number of endpoint requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create requests counter: %w", err)
	}

	h.requestDuration, err = meter.Float64Histogram(
		"conductor_endpoint_request_duration_seconds",
		metric.WithDescription("Endpoint request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create duration histogram: %w", err)
	}

	h.rateLimitExceeded, err = meter.Int64Counter(
		"conductor_endpoint_rate_limit_exceeded_total",
		metric.WithDescription("Total number of rate limit exceeded events"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create rate limit counter: %w", err)
	}

	return nil
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
// Filters endpoints based on the caller's scopes.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	endpoints := h.registry.List()

	// Get user from context for scope filtering
	user, _ := auth.UserFromContext(r.Context())
	var userScopes []string
	if user != nil {
		userScopes = user.Scopes
	}

	// Convert to response format and filter by scopes
	response := make([]EndpointResponse, 0, len(endpoints))
	for _, ep := range endpoints {
		// Only include endpoints the user has access to
		if auth.MatchesScope(userScopes, ep.Name) {
			response = append(response, EndpointResponse{
				Name:        ep.Name,
				Description: ep.Description,
				Inputs:      ep.Inputs,
			})
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"endpoints": response,
	})
}

// handleGet handles GET /v1/endpoints/{name}.
// Returns detailed metadata for a specific endpoint.
// Returns 404 if the endpoint doesn't exist or caller lacks access (security by obscurity).
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "endpoint name is required")
		return
	}

	ep := h.registry.Get(name)
	if ep == nil {
		httputil.WriteError(w, http.StatusNotFound, fmt.Sprintf("endpoint %q not found", name))
		return
	}

	// Check scope access (skip for public endpoints)
	user, _ := auth.UserFromContext(r.Context())
	var userScopes []string
	if user != nil {
		userScopes = user.Scopes
	}

	if !ep.Public && !auth.MatchesScope(userScopes, ep.Name) {
		// Return 404 instead of 403 to avoid information disclosure
		h.logger.Warn("Endpoint access denied due to scopes",
			slog.String("endpoint", name),
			slog.String("user", getUserID(user)),
			slog.Any("user_scopes", userScopes),
		)
		httputil.WriteError(w, http.StatusNotFound, fmt.Sprintf("endpoint %q not found", name))
		return
	}

	// Log public endpoint access for audit
	if ep.Public {
		h.logger.Info("Public endpoint accessed",
			slog.String("endpoint", name),
			slog.String("user", getUserID(user)),
		)
	}

	httputil.WriteJSON(w, http.StatusOK, EndpointResponse{
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
	startTime := time.Now()
	name := r.PathValue("name")
	var statusCode int

	// Track metrics at the end of the request
	defer func() {
		duration := time.Since(startTime).Seconds()
		if h.requestDuration != nil {
			h.requestDuration.Record(r.Context(), duration,
				metric.WithAttributes(
					attribute.String("endpoint", name),
					attribute.String("method", "POST"),
				),
			)
		}
		if h.requestsTotal != nil && statusCode > 0 {
			h.requestsTotal.Add(r.Context(), 1,
				metric.WithAttributes(
					attribute.String("endpoint", name),
					attribute.String("status", fmt.Sprintf("%d", statusCode)),
				),
			)
		}
	}()

	// Check if runner is draining (graceful shutdown in progress)
	if h.runner.IsDraining() {
		statusCode = http.StatusServiceUnavailable
		w.Header().Set("Retry-After", "10")
		httputil.WriteError(w, statusCode, "daemon is shutting down gracefully")
		return
	}
	if name == "" {
		statusCode = http.StatusBadRequest
		httputil.WriteError(w, statusCode, "endpoint name is required")
		return
	}

	// Get endpoint
	ep := h.registry.Get(name)
	if ep == nil {
		statusCode = http.StatusNotFound
		httputil.WriteError(w, statusCode, fmt.Sprintf("endpoint %q not found", name))
		return
	}

	// Check scope access (skip for public endpoints)
	user, _ := auth.UserFromContext(r.Context())
	var userScopes []string
	if user != nil {
		userScopes = user.Scopes
	}

	if !ep.Public && !auth.MatchesScope(userScopes, ep.Name) {
		// Return 404 instead of 403 to avoid information disclosure
		h.logger.Warn("Endpoint access denied due to scopes",
			slog.String("endpoint", name),
			slog.String("user", getUserID(user)),
			slog.Any("user_scopes", userScopes),
		)
		statusCode = http.StatusNotFound
		httputil.WriteError(w, statusCode, fmt.Sprintf("endpoint %q not found", name))
		return
	}

	// Log public endpoint access for audit
	if ep.Public {
		h.logger.Info("Public endpoint accessed",
			slog.String("endpoint", name),
			slog.String("user", getUserID(user)),
		)
	}

	// Check rate limit for this endpoint
	if !h.checkRateLimit(w, r.Context(), ep.Name, &statusCode) {
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		statusCode = http.StatusBadRequest
		httputil.WriteError(w, statusCode, "failed to read request body")
		return
	}

	var req CreateRunRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			statusCode = http.StatusBadRequest
			httputil.WriteError(w, statusCode, fmt.Sprintf("invalid JSON: %v", err))
			return
		}
	}

	// Find workflow file first (needed for validation)
	workflowPath, err := findWorkflow(ep.Workflow, h.workflowsDir)
	if err != nil {
		h.logger.Error("Workflow not found for endpoint",
			slog.String("endpoint", name),
			slog.String("workflow", ep.Workflow),
			slog.Any("error", err),
		)
		statusCode = http.StatusInternalServerError
		httputil.WriteError(w, statusCode, fmt.Sprintf("workflow %q not found", ep.Workflow))
		return
	}

	// Read workflow file
	workflowFile, err := os.Open(workflowPath)
	if err != nil {
		h.logger.Error("Failed to open workflow file",
			slog.String("endpoint", name),
			slog.String("path", workflowPath),
			slog.Any("error", err),
		)
		statusCode = http.StatusInternalServerError
		httputil.WriteError(w, statusCode, "failed to read workflow file")
		return
	}
	defer workflowFile.Close()

	workflowYAML, err := io.ReadAll(workflowFile)
	if err != nil {
		h.logger.Error("Failed to read workflow file",
			slog.String("endpoint", name),
			slog.String("path", workflowPath),
			slog.Any("error", err),
		)
		statusCode = http.StatusInternalServerError
		httputil.WriteError(w, statusCode, "failed to read workflow file")
		return
	}

	// Parse workflow definition for input validation
	var def workflow.Definition
	if err := yaml.Unmarshal(workflowYAML, &def); err != nil {
		h.logger.Error("Failed to parse workflow definition",
			slog.String("endpoint", name),
			slog.Any("error", err),
		)
		statusCode = http.StatusInternalServerError
		httputil.WriteError(w, statusCode, "failed to parse workflow definition")
		return
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

	// Validate inputs against workflow schema
	if err := validateInputs(inputs, def.Inputs); err != nil {
		h.logger.Warn("Input validation failed",
			slog.String("endpoint", name),
			slog.Any("error", err),
		)
		statusCode = http.StatusBadRequest
		httputil.WriteError(w, statusCode, fmt.Sprintf("input validation failed: %v", err))
		return
	}

	// Determine timeout: use endpoint timeout if configured, otherwise runner will use its default
	timeout := ep.Timeout

	// Submit run
	run, err := h.runner.Submit(r.Context(), runner.SubmitRequest{
		WorkflowYAML: workflowYAML,
		Inputs:       inputs,
		Workspace:    req.Workspace,
		Profile:      req.Profile,
		Timeout:      timeout,
	})
	if err != nil {
		h.logger.Error("Failed to submit run",
			slog.String("endpoint", name),
			slog.Any("error", err),
		)
		statusCode = http.StatusBadRequest
		httputil.WriteError(w, statusCode, fmt.Sprintf("failed to submit run: %v", err))
		return
	}

	h.logger.Info("Endpoint run created",
		slog.String("endpoint", name),
		slog.String("run_id", run.ID),
		slog.String("workflow", ep.Workflow),
	)

	// Check for synchronous execution mode
	waitParam := r.URL.Query().Get("wait")
	streamParam := r.URL.Query().Get("stream")

	if waitParam == "true" {
		// Synchronous mode - wait for completion
		// Note: sync execution sets its own status codes
		h.handleSyncExecution(w, r, run, ep.Name, streamParam == "true")
		statusCode = http.StatusOK // Approximate - actual status set by handleSyncExecution
		return
	}

	// Async mode - return 202 Accepted with run details
	statusCode = http.StatusAccepted
	w.Header().Set("Location", fmt.Sprintf("/v1/runs/%s", run.ID))
	httputil.WriteJSON(w, statusCode, run)
}

// handleListRuns handles GET /v1/endpoints/{name}/runs.
// Lists all runs for a specific endpoint.
func (h *Handler) handleListRuns(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "endpoint name is required")
		return
	}

	// Verify endpoint exists
	ep := h.registry.Get(name)
	if ep == nil {
		httputil.WriteError(w, http.StatusNotFound, fmt.Sprintf("endpoint %q not found", name))
		return
	}

	// Check scope access (skip for public endpoints)
	user, _ := auth.UserFromContext(r.Context())
	var userScopes []string
	if user != nil {
		userScopes = user.Scopes
	}

	if !ep.Public && !auth.MatchesScope(userScopes, ep.Name) {
		// Return 404 instead of 403 to avoid information disclosure
		h.logger.Warn("Endpoint access denied due to scopes",
			slog.String("endpoint", name),
			slog.String("user", getUserID(user)),
			slog.Any("user_scopes", userScopes),
		)
		httputil.WriteError(w, http.StatusNotFound, fmt.Sprintf("endpoint %q not found", name))
		return
	}

	// Log public endpoint access for audit
	if ep.Public {
		h.logger.Info("Public endpoint accessed",
			slog.String("endpoint", name),
			slog.String("user", getUserID(user)),
		)
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

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"runs": endpointRuns,
	})
}

// handleSyncExecution handles synchronous execution of a workflow run.
// It waits for the run to complete or timeout, then returns the output directly.
// If streaming is enabled, it streams logs via SSE.
func (h *Handler) handleSyncExecution(w http.ResponseWriter, r *http.Request, run *runner.RunSnapshot, endpointName string, streaming bool) {
	// Parse timeout parameter
	timeoutParam := r.URL.Query().Get("timeout")
	timeout := 30 * time.Second // default 30s

	if timeoutParam != "" {
		// Parse duration string (e.g., "30s", "2m")
		parsedTimeout, err := time.ParseDuration(timeoutParam)
		if err != nil {
			// Try parsing as seconds
			if seconds, err := strconv.Atoi(timeoutParam); err == nil {
				parsedTimeout = time.Duration(seconds) * time.Second
			} else {
				httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid timeout: %v", err))
				return
			}
		}

		// Enforce max timeout of 5 minutes
		maxTimeout := 5 * time.Minute
		if parsedTimeout > maxTimeout {
			httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("timeout exceeds maximum of %v", maxTimeout))
			return
		}

		timeout = parsedTimeout
	}

	// If streaming mode, handle via SSE
	if streaming {
		h.streamRunExecution(w, r, run, timeout)
		return
	}

	// Non-streaming mode: wait for completion
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Subscribe to logs to detect completion
	logCh, unsub := h.runner.Subscribe(run.ID)
	defer unsub()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			// Check if it was a timeout or client disconnect
			if ctx.Err() == context.DeadlineExceeded {
				// Timeout - return 408 with run ID so client can poll
				h.logger.Info("Synchronous execution timed out, run continues in background",
					slog.String("endpoint", endpointName),
					slog.String("run_id", run.ID),
					slog.Duration("timeout", timeout),
				)

				w.Header().Set("X-Run-ID", run.ID)
				w.Header().Set("X-Run-Duration-Ms", fmt.Sprintf("%d", time.Since(startTime).Milliseconds()))
				httputil.WriteError(w, http.StatusRequestTimeout, fmt.Sprintf("execution timed out after %v, run continues as %s", timeout, run.ID))
				return
			}

			// Client disconnected
			h.logger.Info("Client disconnected during synchronous execution",
				slog.String("endpoint", endpointName),
				slog.String("run_id", run.ID),
			)
			return

		case <-logCh:
			// Got a log entry, check if run is complete
			currentRun, err := h.runner.Get(run.ID)
			if err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "failed to get run status")
				return
			}

			// Check if run finished
			if currentRun.Status == runner.RunStatusCompleted ||
				currentRun.Status == runner.RunStatusFailed ||
				currentRun.Status == runner.RunStatusCancelled {

				duration := time.Since(startTime)

				// Set response headers
				w.Header().Set("X-Run-ID", currentRun.ID)
				w.Header().Set("X-Run-Duration-Ms", fmt.Sprintf("%d", duration.Milliseconds()))

				// Return output or error
				if currentRun.Status == runner.RunStatusCompleted {
					// Success - return output
					httputil.WriteJSON(w, http.StatusOK, map[string]any{
						"status": currentRun.Status,
						"output": currentRun.Output,
					})
				} else {
					// Failed or cancelled - return error details
					statusCode := http.StatusInternalServerError
					if currentRun.Status == runner.RunStatusCancelled {
						statusCode = http.StatusConflict
					}

					httputil.WriteJSON(w, statusCode, map[string]any{
						"status": currentRun.Status,
						"error":  currentRun.Error,
					})
				}
				return
			}
		}
	}
}

// streamRunExecution streams the execution via SSE.
func (h *Handler) streamRunExecution(w http.ResponseWriter, r *http.Request, run *runner.RunSnapshot, timeout time.Duration) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Run-ID", run.ID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Subscribe to logs
	logCh, unsub := h.runner.Subscribe(run.ID)
	defer unsub()

	startTime := time.Now()

	// Send initial connection event
	fmt.Fprintf(w, "event: start\ndata: %s\n\n", toJSON(map[string]any{
		"run_id":    run.ID,
		"status":    run.Status,
		"timestamp": time.Now().Format(time.RFC3339),
	}))
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			// Timeout or client disconnect
			if ctx.Err() == context.DeadlineExceeded {
				// Send timeout event
				duration := time.Since(startTime)
				fmt.Fprintf(w, "event: timeout\ndata: %s\n\n", toJSON(map[string]any{
					"run_id":      run.ID,
					"timeout":     timeout.String(),
					"duration_ms": duration.Milliseconds(),
					"message":     "execution timed out, run continues in background",
				}))
				flusher.Flush()
			}
			return

		case entry, ok := <-logCh:
			if !ok {
				// Channel closed
				return
			}

			// Send log entry as event
			fmt.Fprintf(w, "event: log\ndata: %s\n\n", toJSON(entry))
			flusher.Flush()

			// Check if run is complete
			currentRun, err := h.runner.Get(run.ID)
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", toJSON(map[string]any{
					"error": "failed to get run status",
				}))
				flusher.Flush()
				return
			}

			// If run finished, send completion event
			if currentRun.Status == runner.RunStatusCompleted ||
				currentRun.Status == runner.RunStatusFailed ||
				currentRun.Status == runner.RunStatusCancelled {

				duration := time.Since(startTime)
				fmt.Fprintf(w, "event: done\ndata: %s\n\n", toJSON(map[string]any{
					"run_id":      currentRun.ID,
					"status":      currentRun.Status,
					"output":      currentRun.Output,
					"error":       currentRun.Error,
					"duration_ms": duration.Milliseconds(),
				}))
				flusher.Flush()
				return
			}
		}
	}
}

// Helper functions

func containsPath(sourceURL, workflow string) bool {
	// Simple check if workflow name appears in source URL
	// This is a heuristic for matching runs to endpoints
	if sourceURL == "" || workflow == "" {
		return false
	}
	return sourceURL == workflow || strings.Contains(sourceURL, workflow)
}

func getUserID(user *auth.User) string {
	if user == nil {
		return "anonymous"
	}
	return user.ID
}

func toJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// checkRateLimit checks the rate limit for an endpoint and writes error response if exceeded.
// Returns true if request is allowed, false if rate limit exceeded.
// statusCode will be set to 429 if rate limit is exceeded.
func (h *Handler) checkRateLimit(w http.ResponseWriter, ctx context.Context, endpointName string, statusCode *int) bool {
	// Check if rate limit is exceeded
	if !h.rateLimiter.Allow(endpointName) {
		// Get status for headers
		remaining, limit, resetAt, _ := h.rateLimiter.GetStatus(endpointName)

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", limit))
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))

		// Calculate retry-after in seconds
		retryAfter := int(time.Until(resetAt).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))

		// Log rate limit exceeded
		h.logger.Warn("Rate limit exceeded",
			slog.String("endpoint", endpointName),
			slog.Float64("limit", limit),
			slog.Float64("remaining", remaining),
			slog.Time("reset_at", resetAt),
		)

		// Record rate limit metric
		if h.rateLimitExceeded != nil {
			h.rateLimitExceeded.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("endpoint", endpointName),
				),
			)
		}

		// Return 429 Too Many Requests
		*statusCode = http.StatusTooManyRequests
		httputil.WriteJSON(w, *statusCode, map[string]any{
			"error":       "rate limit exceeded",
			"limit":       int(limit),
			"remaining":   0,
			"reset_at":    resetAt.Unix(),
			"retry_after": retryAfter,
		})
		return false
	}

	// Get status for headers (after successful request)
	remaining, limit, resetAt, exists := h.rateLimiter.GetStatus(endpointName)
	if exists {
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%.0f", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))
	}

	return true
}

// validateInputs validates request inputs against the workflow's input schema.
// Returns an error with detailed validation messages if validation fails.
func validateInputs(inputs map[string]any, inputDefs []workflow.InputDefinition) error {
	// Build a map of input definitions for quick lookup
	defMap := make(map[string]workflow.InputDefinition)
	for _, def := range inputDefs {
		defMap[def.Name] = def
	}

	// Check for required inputs
	for _, def := range inputDefs {
		// Skip if has default value
		if def.Default != nil {
			continue
		}

		// Check if required input is missing
		if def.Required {
			if _, ok := inputs[def.Name]; !ok {
				return fmt.Errorf("required input %q is missing", def.Name)
			}
		}
	}

	// Validate type and constraints for provided inputs
	for name, value := range inputs {
		def, ok := defMap[name]
		if !ok {
			// Unknown input - this is acceptable (workflow executor will handle)
			continue
		}

		// Skip nil values (will use default if available)
		if value == nil {
			continue
		}

		// Validate type
		if err := validateInputType(name, value, def.Type); err != nil {
			return err
		}

		// Validate enum constraint
		if len(def.Enum) > 0 {
			if err := validateEnum(name, value, def.Enum); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateInputType validates that an input value matches the expected type.
func validateInputType(name string, value any, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("input %q must be a string, got %T", name, value)
		}
	case "number":
		// Accept both int and float types as numbers
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			// Integer types are valid numbers
		case float32, float64:
			// Float types are valid numbers
		default:
			return fmt.Errorf("input %q must be a number, got %T", name, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("input %q must be a boolean, got %T", name, value)
		}
	case "object":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("input %q must be an object, got %T", name, value)
		}
	case "array":
		// Accept []any and []interface{} as array types (they're the same in Go)
		switch value.(type) {
		case []any:
			// Valid array type
		default:
			return fmt.Errorf("input %q must be an array, got %T", name, value)
		}
	case "enum":
		// Enum type - just check it's a string (actual enum validation happens separately)
		if _, ok := value.(string); !ok {
			return fmt.Errorf("input %q must be a string (enum), got %T", name, value)
		}
	case "":
		// No type specified - accept any value
		return nil
	default:
		// Unknown type - be lenient and accept it (workflow executor will validate)
		return nil
	}

	return nil
}

// validateEnum validates that a value is in the allowed enum values.
func validateEnum(name string, value any, allowedValues []string) error {
	strValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("input %q enum value must be a string, got %T", name, value)
	}

	for _, allowed := range allowedValues {
		if strValue == allowed {
			return nil
		}
	}

	return fmt.Errorf("input %q must be one of %v, got %q", name, allowedValues, strValue)
}
