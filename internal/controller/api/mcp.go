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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/mcp"
)

// MCPHandler handles MCP server API endpoints.
type MCPHandler struct {
	registry   *mcp.Registry
	logCapture *mcp.LogCapture
}

// NewMCPHandler creates a new MCP handler.
func NewMCPHandler(registry *mcp.Registry, logCapture *mcp.LogCapture) *MCPHandler {
	if logCapture == nil {
		logCapture = mcp.NewLogCapture()
	}
	return &MCPHandler{
		registry:   registry,
		logCapture: logCapture,
	}
}

// RegisterRoutes registers MCP API routes on the given mux.
func (h *MCPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/mcp/servers", h.handleListServers)
	mux.HandleFunc("GET /v1/mcp/servers/{name}", h.handleGetServer)
	mux.HandleFunc("GET /v1/mcp/servers/{name}/tools", h.handleGetServerTools)
	mux.HandleFunc("GET /v1/mcp/servers/{name}/health", h.handleGetServerHealth)
	mux.HandleFunc("GET /v1/mcp/servers/{name}/logs", h.handleGetServerLogs)
	mux.HandleFunc("POST /v1/mcp/servers/{name}/start", h.handleStartServer)
	mux.HandleFunc("POST /v1/mcp/servers/{name}/stop", h.handleStopServer)
	mux.HandleFunc("POST /v1/mcp/servers/{name}/restart", h.handleRestartServer)
	mux.HandleFunc("POST /v1/mcp/servers", h.handleRegisterServer)
	mux.HandleFunc("DELETE /v1/mcp/servers/{name}", h.handleRemoveServer)
}

// MCPServerResponse represents a server in API responses.
type MCPServerResponse struct {
	Name          string                   `json:"name"`
	Status        string                   `json:"status"`
	UptimeSeconds int64                    `json:"uptime_seconds"`
	ToolCount     *int                     `json:"tool_count"`
	FailureCount  int                      `json:"failure_count"`
	LastError     string                   `json:"last_error,omitempty"`
	Source        string                   `json:"source,omitempty"`
	Version       string                   `json:"version,omitempty"`
	Config        *MCPServerConfigResponse `json:"config,omitempty"`
	Capabilities  *MCPCapabilitiesResponse `json:"capabilities,omitempty"`
}

// MCPServerConfigResponse represents server configuration in API responses.
type MCPServerConfigResponse struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	Timeout int      `json:"timeout"`
}

// MCPCapabilitiesResponse represents server capabilities.
type MCPCapabilitiesResponse struct {
	Tools     bool `json:"tools"`
	Resources bool `json:"resources"`
	Prompts   bool `json:"prompts"`
}

// MCPToolResponse represents a tool in API responses.
type MCPToolResponse struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// MCPListResponse represents the list servers response.
type MCPListResponse struct {
	Servers []MCPServerResponse `json:"servers"`
}

// MCPHealthResponse represents a health check response.
type MCPHealthResponse struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
}

// MCPRegisterRequest represents a server registration request.
type MCPRegisterRequest struct {
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Args      []string `json:"args,omitempty"`
	Env       []string `json:"env,omitempty"`
	Timeout   int      `json:"timeout,omitempty"`
	AutoStart bool     `json:"auto_start,omitempty"`
}

// handleListServers handles GET /v1/mcp/servers
func (h *MCPHandler) handleListServers(w http.ResponseWriter, r *http.Request) {
	// Get optional status filter
	statusFilter := r.URL.Query().Get("status")

	statuses := h.registry.ListAllServers()

	servers := make([]MCPServerResponse, 0, len(statuses))
	for _, s := range statuses {
		// Apply status filter if provided
		if statusFilter != "" && string(s.State) != statusFilter {
			continue
		}

		resp := MCPServerResponse{
			Name:          s.Name,
			Status:        string(s.State),
			UptimeSeconds: int64(s.Uptime.Seconds()),
			ToolCount:     s.ToolCount,
			FailureCount:  s.FailureCount,
			LastError:     s.LastError,
			Source:        s.Source,
			Version:       s.Version,
		}

		servers = append(servers, resp)
	}

	writeJSON(w, http.StatusOK, MCPListResponse{Servers: servers})
}

// handleGetServer handles GET /v1/mcp/servers/{name}
func (h *MCPHandler) handleGetServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "server name is required")
		return
	}

	status, err := h.registry.GetServerStatus(name)
	if err != nil {
		if mcpErr := mcp.GetMCPError(err); mcpErr != nil && mcpErr.Code == mcp.ErrorCodeNotFound {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	resp := MCPServerResponse{
		Name:          status.Name,
		Status:        string(status.State),
		UptimeSeconds: int64(status.Uptime.Seconds()),
		ToolCount:     status.ToolCount,
		FailureCount:  status.FailureCount,
		LastError:     status.LastError,
		Source:        status.Source,
		Version:       status.Version,
	}

	// Add config with redacted env
	if status.Config != nil {
		resp.Config = &MCPServerConfigResponse{
			Command: status.Config.Command,
			Args:    status.Config.Args,
			Env:     mcp.RedactEnv(status.Config.Env),
			Timeout: int(status.Config.Timeout.Seconds()),
		}
	}

	// Get capabilities if server is running
	if status.Running {
		client, err := h.registry.GetClient(name)
		if err == nil {
			caps := client.Capabilities()
			if caps != nil {
				resp.Capabilities = &MCPCapabilitiesResponse{
					Tools:     caps.Tools != nil,
					Resources: caps.Resources != nil,
					Prompts:   caps.Prompts != nil,
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGetServerTools handles GET /v1/mcp/servers/{name}/tools
func (h *MCPHandler) handleGetServerTools(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "server name is required")
		return
	}

	client, err := h.registry.GetClient(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else if strings.Contains(err.Error(), "not ready") || strings.Contains(err.Error(), "not running") {
			writeError(w, http.StatusServiceUnavailable, "server is not running")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tools: "+err.Error())
		return
	}

	resp := make([]MCPToolResponse, len(tools))
	for i, t := range tools {
		resp[i] = MCPToolResponse{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"tools": resp})
}

// handleGetServerHealth handles GET /v1/mcp/servers/{name}/health
func (h *MCPHandler) handleGetServerHealth(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "server name is required")
		return
	}

	client, err := h.registry.GetClient(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeJSON(w, http.StatusOK, MCPHealthResponse{Status: "stopped", LatencyMs: 0})
		}
		return
	}

	// Ping the server and measure latency
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err = client.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		writeJSON(w, http.StatusOK, MCPHealthResponse{Status: "unhealthy", LatencyMs: latency.Milliseconds()})
		return
	}

	writeJSON(w, http.StatusOK, MCPHealthResponse{Status: "healthy", LatencyMs: latency.Milliseconds()})
}

// handleStartServer handles POST /v1/mcp/servers/{name}/start
func (h *MCPHandler) handleStartServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "server name is required")
		return
	}

	err := h.registry.StartServer(name)
	if err != nil {
		if mcpErr := mcp.GetMCPError(err); mcpErr != nil {
			switch mcpErr.Code {
			case mcp.ErrorCodeNotFound:
				writeError(w, http.StatusNotFound, mcpErr.Message)
			case mcp.ErrorCodeAlreadyRunning:
				writeError(w, http.StatusConflict, mcpErr.Message)
			default:
				writeError(w, http.StatusInternalServerError, mcpErr.Message)
			}
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// handleStopServer handles POST /v1/mcp/servers/{name}/stop
func (h *MCPHandler) handleStopServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "server name is required")
		return
	}

	err := h.registry.StopServer(name)
	if err != nil {
		if mcpErr := mcp.GetMCPError(err); mcpErr != nil {
			switch mcpErr.Code {
			case mcp.ErrorCodeNotFound:
				writeError(w, http.StatusNotFound, mcpErr.Message)
			case mcp.ErrorCodeNotRunning:
				writeError(w, http.StatusConflict, mcpErr.Message)
			default:
				writeError(w, http.StatusInternalServerError, mcpErr.Message)
			}
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// handleRestartServer handles POST /v1/mcp/servers/{name}/restart
func (h *MCPHandler) handleRestartServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "server name is required")
		return
	}

	err := h.registry.RestartServer(name)
	if err != nil {
		if mcpErr := mcp.GetMCPError(err); mcpErr != nil {
			switch mcpErr.Code {
			case mcp.ErrorCodeNotFound:
				writeError(w, http.StatusNotFound, mcpErr.Message)
			default:
				writeError(w, http.StatusInternalServerError, mcpErr.Message)
			}
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restarting"})
}

// handleRegisterServer handles POST /v1/mcp/servers
func (h *MCPHandler) handleRegisterServer(w http.ResponseWriter, r *http.Request) {
	var req MCPRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	// Set default timeout
	if req.Timeout == 0 {
		req.Timeout = 30
	}

	entry := &mcp.MCPServerEntry{
		Command:   req.Command,
		Args:      req.Args,
		Env:       req.Env,
		Timeout:   req.Timeout,
		AutoStart: req.AutoStart,
	}

	err := h.registry.RegisterGlobal(req.Name, entry, false)
	if err != nil {
		if mcpErr := mcp.GetMCPError(err); mcpErr != nil {
			switch mcpErr.Code {
			case mcp.ErrorCodeAlreadyExists:
				writeError(w, http.StatusConflict, mcpErr.Message)
			case mcp.ErrorCodeValidation:
				writeError(w, http.StatusBadRequest, mcpErr.Message)
			default:
				writeError(w, http.StatusInternalServerError, mcpErr.Message)
			}
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "registered", "name": req.Name})
}

// handleRemoveServer handles DELETE /v1/mcp/servers/{name}
func (h *MCPHandler) handleRemoveServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "server name is required")
		return
	}

	err := h.registry.UnregisterGlobal(name)
	if err != nil {
		if mcpErr := mcp.GetMCPError(err); mcpErr != nil {
			switch mcpErr.Code {
			case mcp.ErrorCodeNotFound:
				writeError(w, http.StatusNotFound, mcpErr.Message)
			default:
				writeError(w, http.StatusInternalServerError, mcpErr.Message)
			}
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// MCPLogEntry represents a log entry in API responses.
type MCPLogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Source    string `json:"source,omitempty"`
}

// MCPLogsResponse represents the logs response.
type MCPLogsResponse struct {
	ServerName      string        `json:"server_name"`
	Logs            []MCPLogEntry `json:"logs"`
	BufferSizeBytes int           `json:"buffer_size_bytes"`
}

// handleGetServerLogs handles GET /v1/mcp/servers/{name}/logs
func (h *MCPHandler) handleGetServerLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "server name is required")
		return
	}

	// Check if server exists
	_, err := h.registry.GetServerStatus(name)
	if err != nil {
		if mcpErr := mcp.GetMCPError(err); mcpErr != nil && mcpErr.Code == mcp.ErrorCodeNotFound {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Parse query parameters
	lines := 100 // default
	if linesStr := r.URL.Query().Get("lines"); linesStr != "" {
		if n, err := parseInt(linesStr); err == nil && n > 0 {
			lines = n
		}
	}

	var since time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		since = parseDuration(sinceStr)
	}

	// Get logs
	entries := h.logCapture.GetLogs(name, lines, since)

	// Calculate approximate buffer size in bytes
	bufferSizeBytes := 0
	for _, entry := range entries {
		// Rough estimate: timestamp (30) + level (10) + message length + source (10)
		bufferSizeBytes += 50 + len(entry.Message)
	}

	resp := MCPLogsResponse{
		ServerName:      name,
		Logs:            make([]MCPLogEntry, len(entries)),
		BufferSizeBytes: bufferSizeBytes,
	}

	for i, entry := range entries {
		resp.Logs[i] = MCPLogEntry{
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			Level:     string(entry.Level),
			Message:   entry.Message,
			Source:    entry.Source,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// parseInt parses an integer from a string.
func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// parseDuration parses a duration string like "5m" or "1h" and returns the time since then.
func parseDuration(s string) time.Time {
	// Try standard Go duration format first
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d)
	}
	return time.Time{}
}

// LogCapture returns the log capture instance.
func (h *MCPHandler) LogCapture() *mcp.LogCapture {
	return h.logCapture
}
