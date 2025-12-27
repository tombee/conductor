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

	"github.com/tombee/conductor/internal/daemon/runner"
)

// PublicRouter handles routing for the public-facing API.
// The public API serves webhooks and API-triggered workflows on a separate port
// from the control plane, with per-workflow authentication instead of global API keys.
type PublicRouter struct {
	mux *http.ServeMux
}

// PublicRouterConfig contains configuration for the public router.
type PublicRouterConfig struct {
	Runner       *runner.Runner
	WorkflowsDir string
}

// NewPublicRouter creates a new public API router.
// The public API only exposes endpoints that are safe for public access:
// - GET /health - Minimal health check (no internals exposed)
// - POST /webhooks/* - Webhook receivers (signature-verified)
// - POST /v1/start/* - API trigger endpoints (Bearer token auth)
func NewPublicRouter(cfg PublicRouterConfig) *PublicRouter {
	mux := http.NewServeMux()

	router := &PublicRouter{
		mux: mux,
	}

	// Register minimal health endpoint
	mux.HandleFunc("/health", router.handleHealth)

	// Register start handler for API-triggered workflows
	if cfg.Runner != nil && cfg.WorkflowsDir != "" {
		startHandler := NewStartHandler(cfg.Runner, cfg.WorkflowsDir)
		startHandler.RegisterRoutes(mux)

		// Register webhook handler
		webhookHandler := NewWebhookHandler(cfg.Runner, cfg.WorkflowsDir)
		webhookHandler.RegisterRoutes(mux)
	}

	return router
}

// Handler returns the http.Handler for the public API.
func (r *PublicRouter) Handler() http.Handler {
	return r.mux
}

// handleHealth handles GET /health for the public API.
// Returns a minimal status response without exposing internals.
// This endpoint requires no authentication and is intended for load balancer health checks.
func (r *PublicRouter) handleHealth(w http.ResponseWriter, req *http.Request) {
	// Only allow GET requests
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return minimal JSON response
	// No internals exposed (no uptime, schedules, MCP status, etc.)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
