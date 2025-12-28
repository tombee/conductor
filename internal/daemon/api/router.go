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

// Package api provides the HTTP API for the daemon.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/tombee/conductor/internal/daemon/httputil"
	"github.com/tombee/conductor/internal/log"
	"github.com/tombee/conductor/internal/tracing"
)

// RouterConfig holds configuration for the API router.
type RouterConfig struct {
	Version   string
	Commit    string
	BuildDate string
}

// ScheduleStatusProvider provides schedule status for health checks.
type ScheduleStatusProvider interface {
	GetScheduleCount() int
	GetEnabledScheduleCount() int
}

// MCPStatusProvider provides MCP server status for health checks.
type MCPStatusProvider interface {
	GetSummary() MCPServerSummary
}

// MCPServerSummary represents a summary of MCP server status.
type MCPServerSummary struct {
	Total   int
	Running int
	Stopped int
	Error   int
}

// AuditStatusProvider provides audit rotation status for health checks.
type AuditStatusProvider interface {
	GetAuditRotationStatus() AuditRotationStatus
}

// AuditRotationStatus represents the status of audit log rotation.
type AuditRotationStatus struct {
	Enabled      bool   `json:"enabled"`
	CurrentFiles int    `json:"current_files,omitempty"`
	TotalSize    int64  `json:"total_size,omitempty"`
	Status       string `json:"status"`
}

// MetricsHandler provides a Prometheus metrics endpoint
type MetricsHandler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// ActivityRecorder tracks daemon activity for idle timeout monitoring.
type ActivityRecorder interface {
	RecordActivity()
}

// Router wraps an http.ServeMux with additional functionality.
type Router struct {
	mux              *http.ServeMux
	config           RouterConfig
	scheduleProvider ScheduleStatusProvider
	mcpProvider      MCPStatusProvider
	auditProvider    AuditStatusProvider
	metricsHandler   MetricsHandler
	activityRecorder ActivityRecorder
	logger           *slog.Logger
}

// SetScheduleProvider sets the schedule status provider.
func (r *Router) SetScheduleProvider(provider ScheduleStatusProvider) {
	r.scheduleProvider = provider
}

// SetMCPProvider sets the MCP status provider.
func (r *Router) SetMCPProvider(provider MCPStatusProvider) {
	r.mcpProvider = provider
}

// SetAuditProvider sets the audit status provider.
func (r *Router) SetAuditProvider(provider AuditStatusProvider) {
	r.auditProvider = provider
}

// SetMetricsHandler sets the Prometheus metrics handler.
func (r *Router) SetMetricsHandler(handler MetricsHandler) {
	r.metricsHandler = handler
	if handler != nil {
		r.mux.HandleFunc("GET /metrics", handler.ServeHTTP)
	}
}

// SetActivityRecorder sets the activity recorder for idle timeout tracking.
func (r *Router) SetActivityRecorder(recorder ActivityRecorder) {
	r.activityRecorder = recorder
}

// SetOverrideHandler sets the override management handler and registers override routes.
func (r *Router) SetOverrideHandler(handler *OverrideHandler) {
	if handler != nil {
		r.mux.HandleFunc("POST /v1/override", handler.HandleCreate)
		r.mux.HandleFunc("GET /v1/override", handler.HandleList)
		r.mux.HandleFunc("DELETE /v1/override/{type}", handler.HandleRevoke)
	}
}

// NewRouter creates a new HTTP router with all API endpoints.
func NewRouter(cfg RouterConfig) *Router {
	r := &Router{
		mux:    http.NewServeMux(),
		config: cfg,
		logger: log.New(log.FromEnv()),
	}

	// Register API v1 endpoints
	r.mux.HandleFunc("GET /v1/health", r.handleHealth)
	r.mux.HandleFunc("GET /v1/version", r.handleVersion)

	// Root endpoint for basic connectivity check
	r.mux.HandleFunc("GET /", r.handleRoot)

	return r
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Record activity for idle timeout tracking
	if r.activityRecorder != nil {
		r.activityRecorder.RecordActivity()
	}

	// Build middleware chain from innermost to outermost:
	// 1. HTTP trace context extraction (innermost - must run first)
	// 2. Tracing middleware (creates spans)
	// 3. Correlation middleware
	// 4. Request logging (outermost)

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mux.ServeHTTP(w, req)
	})

	// Apply request logging middleware
	// Capture the inner handler to avoid closure over reassigned variable
	innerHandler := handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Log request with correlation ID
		start := time.Now()
		correlationID := tracing.FromContextOrEmpty(req.Context())
		logger := log.WithCorrelationID(r.logger, string(correlationID))

		defer func() {
			logger.Info("request completed",
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			)
		}()

		innerHandler.ServeHTTP(w, req)
	})

	// Apply correlation middleware
	handler = tracing.CorrelationMiddleware(handler)

	// Apply tracing middleware to create spans for requests
	handler = tracing.TracingMiddleware(handler)

	// Apply HTTP middleware to extract trace context from headers (must be first)
	handler = tracing.HTTPMiddleware(handler)

	handler.ServeHTTP(w, req)
}

// Mux returns the underlying ServeMux for registering additional routes.
func (r *Router) Mux() *http.ServeMux {
	return r.mux
}

// handleRoot handles GET / for basic connectivity.
func (r *Router) handleRoot(w http.ResponseWriter, req *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"name":    "conductord",
		"version": r.config.Version,
	})
}
