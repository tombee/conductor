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
	"runtime"
	"time"
)

// HealthResponse is the response format for /v1/health.
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Uptime    string            `json:"uptime,omitempty"`
	Checks    map[string]string `json:"checks,omitempty"`
}

var startTime = time.Now()

// handleHealth handles GET /v1/health.
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	uptime := time.Since(startTime)

	checks := map[string]string{
		"api":     "ok",
		"runtime": runtime.Version(),
	}

	// Add schedule status if available
	if r.scheduleProvider != nil {
		total := r.scheduleProvider.GetScheduleCount()
		enabled := r.scheduleProvider.GetEnabledScheduleCount()
		checks["schedules"] = formatScheduleStatus(total, enabled)
	}

	// Add MCP server status if available
	if r.mcpProvider != nil {
		summary := r.mcpProvider.GetSummary()
		checks["mcp_servers"] = formatMCPStatus(summary)
	}

	// Add audit rotation status if available
	if r.auditProvider != nil {
		status := r.auditProvider.GetAuditRotationStatus()
		checks["audit_rotation"] = formatAuditStatus(status)
	}

	resp := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    uptime.Round(time.Second).String(),
		Checks:    checks,
	}

	writeJSON(w, http.StatusOK, resp)
}

// formatScheduleStatus formats schedule status for display.
func formatScheduleStatus(total, enabled int) string {
	if total == 0 {
		return "none"
	}
	return fmt.Sprintf("%d/%d active", enabled, total)
}

// formatMCPStatus formats MCP server status for display.
func formatMCPStatus(summary MCPServerSummary) string {
	if summary.Total == 0 {
		return "none"
	}
	if summary.Error > 0 {
		return fmt.Sprintf("%d/%d running (%d errors)", summary.Running, summary.Total, summary.Error)
	}
	return fmt.Sprintf("%d/%d running", summary.Running, summary.Total)
}

// formatAuditStatus formats audit rotation status for display.
func formatAuditStatus(status AuditRotationStatus) string {
	if !status.Enabled {
		return "disabled"
	}
	if status.Status != "" {
		return status.Status
	}
	if status.CurrentFiles > 0 {
		sizeKB := status.TotalSize / 1024
		return fmt.Sprintf("%d files (%.1f KB)", status.CurrentFiles, float64(sizeKB))
	}
	return "enabled"
}
