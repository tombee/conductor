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
	"net/http"
	"time"

	"github.com/tombee/conductor/internal/daemon/httputil"
	"github.com/tombee/conductor/pkg/security"
)

// OverrideHandler handles security override management
type OverrideHandler struct {
	manager *security.OverrideManager
}

// NewOverrideHandler creates a new override handler
func NewOverrideHandler(manager *security.OverrideManager) *OverrideHandler {
	return &OverrideHandler{
		manager: manager,
	}
}

// CreateOverrideRequest represents a request to create a security override
type CreateOverrideRequest struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
	TTL    string `json:"ttl,omitempty"` // Duration string like "1h", "30m"
}

// OverrideResponse represents an override in API responses
type OverrideResponse struct {
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ListOverridesResponse represents the response for listing overrides
type ListOverridesResponse struct {
	Overrides []OverrideResponse `json:"overrides"`
}

// HandleCreate handles POST /v1/override
func (h *OverrideHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "override management not enabled")
		return
	}

	var req CreateOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Validate type
	overrideType := security.OverrideType(req.Type)

	// Block disable-audit type at API level (defense in depth)
	if overrideType == security.OverrideDisableAudit {
		httputil.WriteError(w, http.StatusForbidden, "disable-audit override type is not allowed")
		return
	}

	if !isValidOverrideType(overrideType) {
		httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid override type: %s (valid types: disable-enforcement, disable-sandbox)", req.Type))
		return
	}

	// Validate reason
	if req.Reason == "" {
		httputil.WriteError(w, http.StatusBadRequest, "reason is required")
		return
	}

	// Parse TTL if provided
	var ttl time.Duration
	if req.TTL != "" {
		var err error
		ttl, err = time.ParseDuration(req.TTL)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid TTL format: "+err.Error())
			return
		}
	} else {
		// Default TTL: 1 hour
		ttl = time.Hour
	}

	// Apply override with TTL
	// Use "api" as appliedBy since we don't have user authentication yet
	override, err := h.manager.ApplyWithTTL(overrideType, req.Reason, "api", ttl)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to apply override: "+err.Error())
		return
	}

	response := OverrideResponse{
		Type:      string(override.Type),
		Reason:    override.Reason,
		ExpiresAt: override.ExpiresAt,
		CreatedAt: override.AppliedAt,
	}

	httputil.WriteJSON(w, http.StatusCreated, response)
}

// HandleList handles GET /v1/override
func (h *OverrideHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "override management not enabled")
		return
	}

	active := h.manager.GetActive()
	overrides := make([]OverrideResponse, 0, len(active))

	for _, override := range active {
		overrides = append(overrides, OverrideResponse{
			Type:      string(override.Type),
			Reason:    override.Reason,
			ExpiresAt: override.ExpiresAt,
			CreatedAt: override.AppliedAt,
		})
	}

	response := ListOverridesResponse{
		Overrides: overrides,
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// HandleRevoke handles DELETE /v1/override/{type}
func (h *OverrideHandler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "override management not enabled")
		return
	}

	// Extract type from path
	typeStr := r.PathValue("type")
	if typeStr == "" {
		httputil.WriteError(w, http.StatusBadRequest, "override type is required")
		return
	}

	overrideType := security.OverrideType(typeStr)
	if !isValidOverrideType(overrideType) {
		httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid override type: %s", typeStr))
		return
	}

	// Revoke override
	if err := h.manager.Revoke(overrideType); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to revoke override: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// isValidOverrideType checks if the override type is valid and allowed via API
func isValidOverrideType(t security.OverrideType) bool {
	switch t {
	case security.OverrideDisableEnforcement,
		security.OverrideDisableSandbox:
		return true
	case security.OverrideDisableAudit:
		// disable-audit is valid but not allowed via API
		return false
	default:
		return false
	}
}
