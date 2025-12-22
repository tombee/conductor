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

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/cost"
)

// CostHandlers provides HTTP handlers for cost data access.
type CostHandlers struct {
	store      cost.CostStore
	auditStore cost.AuditStore
	authz      *CostAuthorizer
}

// NewCostHandlers creates a new set of cost HTTP handlers.
func NewCostHandlers(store cost.CostStore, auditStore cost.AuditStore, authz *CostAuthorizer) *CostHandlers {
	if authz == nil {
		authz = NewCostAuthorizer()
	}
	return &CostHandlers{
		store:      store,
		auditStore: auditStore,
		authz:      authz,
	}
}

// RegisterHTTP registers all cost HTTP handlers with the provided mux.
func (h *CostHandlers) RegisterHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/v1/costs", h.handleCosts)
	mux.HandleFunc("/v1/costs/by-provider", h.handleCostsByProvider)
	mux.HandleFunc("/v1/costs/by-model", h.handleCostsByModel)
	mux.HandleFunc("/v1/costs/by-workflow", h.handleCostsByWorkflow)
	mux.HandleFunc("/v1/runs/", h.handleRunCosts)
}

// CostSummaryResponse is the response format for /v1/costs.
type CostSummaryResponse struct {
	Summary          CostSummary                   `json:"summary"`
	ByProvider       map[string]llm.CostAggregate  `json:"by_provider,omitempty"`
	AccuracyNote     string                        `json:"accuracy_note"`
}

// CostSummary contains aggregate cost statistics.
type CostSummary struct {
	TotalCost          float64                       `json:"total_cost"`
	TotalCostAccuracy  string                        `json:"total_cost_accuracy"`
	TotalTokens        int                           `json:"total_tokens"`
	TotalRequests      int                           `json:"total_requests"`
	Period             Period                        `json:"period"`
	AccuracyBreakdown  AccuracyBreakdown             `json:"accuracy_breakdown"`
}

// Period defines a time range for cost queries.
type Period struct {
	Start string `json:"start"` // RFC3339 timestamp
	End   string `json:"end"`   // RFC3339 timestamp
}

// AccuracyBreakdown shows how many requests fall into each accuracy category.
type AccuracyBreakdown struct {
	Measured    int `json:"measured"`
	Estimated   int `json:"estimated"`
	Unavailable int `json:"unavailable"`
}

// ProviderCostResponse is the response format for /v1/costs/by-provider.
type ProviderCostResponse struct {
	Providers    []ProviderCost `json:"providers"`
	AccuracyNote string         `json:"accuracy_note"`
}

// ProviderCost contains cost data for a single provider.
type ProviderCost struct {
	Provider string           `json:"provider"`
	TotalCost float64         `json:"total_cost"`
	Accuracy string           `json:"accuracy"`
	Requests int              `json:"requests"`
	Tokens   TokenBreakdown   `json:"tokens"`
}

// TokenBreakdown shows input/output token counts.
type TokenBreakdown struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// handleCosts handles GET /v1/costs - aggregate summary.
func (h *CostHandlers) handleCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Extract and validate user from context
	userID := h.getUserID(r)
	if userID == "" {
		h.logAuditFailure(ctx, "", "view_costs", "", "missing authentication")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	opts, err := h.parseAggregateOptions(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
		return
	}

	// Check authorization
	if !h.authz.CanViewCosts(userID) {
		h.logAuditFailure(ctx, userID, "view_costs", "aggregate", "insufficient permissions")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Filter by user if not admin
	if !h.authz.IsAdmin(userID) {
		opts.UserID = userID
	}

	// Get aggregate data
	aggregate, err := h.store.Aggregate(ctx, *opts)
	if err != nil {
		h.logAuditFailure(ctx, userID, "view_costs", "aggregate", err.Error())
		http.Error(w, fmt.Sprintf("Failed to get costs: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	response := h.buildCostSummaryResponse(aggregate, opts)

	// Log successful access
	h.logAuditSuccess(ctx, userID, "view_costs", "aggregate")

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCostsByProvider handles GET /v1/costs/by-provider.
func (h *CostHandlers) handleCostsByProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Extract and validate user from context
	userID := h.getUserID(r)
	if userID == "" {
		h.logAuditFailure(ctx, "", "view_costs_by_provider", "", "missing authentication")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	opts, err := h.parseAggregateOptions(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
		return
	}

	// Check authorization
	if !h.authz.CanViewCosts(userID) {
		h.logAuditFailure(ctx, userID, "view_costs_by_provider", "by_provider", "insufficient permissions")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Filter by user if not admin
	if !h.authz.IsAdmin(userID) {
		opts.UserID = userID
	}

	// Get aggregated data by provider
	aggregates, err := h.store.AggregateByProvider(ctx, *opts)
	if err != nil {
		h.logAuditFailure(ctx, userID, "view_costs_by_provider", "by_provider", err.Error())
		http.Error(w, fmt.Sprintf("Failed to get costs by provider: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	response := h.buildProviderCostResponse(aggregates)

	// Log successful access
	h.logAuditSuccess(ctx, userID, "view_costs_by_provider", "by_provider")

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCostsByModel handles GET /v1/costs/by-model.
func (h *CostHandlers) handleCostsByModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Extract and validate user from context
	userID := h.getUserID(r)
	if userID == "" {
		h.logAuditFailure(ctx, "", "view_costs_by_model", "", "missing authentication")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	opts, err := h.parseAggregateOptions(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
		return
	}

	// Check authorization
	if !h.authz.CanViewCosts(userID) {
		h.logAuditFailure(ctx, userID, "view_costs_by_model", "by_model", "insufficient permissions")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Filter by user if not admin
	if !h.authz.IsAdmin(userID) {
		opts.UserID = userID
	}

	// Get aggregated data by model
	aggregates, err := h.store.AggregateByModel(ctx, *opts)
	if err != nil {
		h.logAuditFailure(ctx, userID, "view_costs_by_model", "by_model", err.Error())
		http.Error(w, fmt.Sprintf("Failed to get costs by model: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response (same format as by-provider)
	var providers []ProviderCost
	for model, agg := range aggregates {
		providers = append(providers, ProviderCost{
			Provider: model,
			TotalCost: agg.TotalCost,
			Accuracy: string(agg.Accuracy),
			Requests: agg.TotalRequests,
			Tokens: TokenBreakdown{
				Input:  agg.TotalPromptTokens,
				Output: agg.TotalCompletionTokens,
			},
		})
	}

	response := ProviderCostResponse{
		Providers:    providers,
		AccuracyNote: "Costs marked as 'estimated' are approximations based on published pricing...",
	}

	// Log successful access
	h.logAuditSuccess(ctx, userID, "view_costs_by_model", "by_model")

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCostsByWorkflow handles GET /v1/costs/by-workflow.
func (h *CostHandlers) handleCostsByWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Extract and validate user from context
	userID := h.getUserID(r)
	if userID == "" {
		h.logAuditFailure(ctx, "", "view_costs_by_workflow", "", "missing authentication")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	opts, err := h.parseAggregateOptions(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
		return
	}

	// Check authorization
	if !h.authz.CanViewCosts(userID) {
		h.logAuditFailure(ctx, userID, "view_costs_by_workflow", "by_workflow", "insufficient permissions")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Filter by user if not admin
	if !h.authz.IsAdmin(userID) {
		opts.UserID = userID
	}

	// Get aggregated data by workflow
	aggregates, err := h.store.AggregateByWorkflow(ctx, *opts)
	if err != nil {
		h.logAuditFailure(ctx, userID, "view_costs_by_workflow", "by_workflow", err.Error())
		http.Error(w, fmt.Sprintf("Failed to get costs by workflow: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response (same format as by-provider)
	var workflows []ProviderCost
	for workflowID, agg := range aggregates {
		workflows = append(workflows, ProviderCost{
			Provider: workflowID,
			TotalCost: agg.TotalCost,
			Accuracy: string(agg.Accuracy),
			Requests: agg.TotalRequests,
			Tokens: TokenBreakdown{
				Input:  agg.TotalPromptTokens,
				Output: agg.TotalCompletionTokens,
			},
		})
	}

	response := ProviderCostResponse{
		Providers:    workflows,
		AccuracyNote: "Costs marked as 'estimated' are approximations based on published pricing...",
	}

	// Log successful access
	h.logAuditSuccess(ctx, userID, "view_costs_by_workflow", "by_workflow")

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRunCosts handles GET /v1/runs/{id}/costs.
func (h *CostHandlers) handleRunCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Extract run ID from path
	runID := extractRunID(r.URL.Path)
	if runID == "" {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	// Extract and validate user from context
	userID := h.getUserID(r)
	if userID == "" {
		h.logAuditFailure(ctx, "", "view_run_costs", runID, "missing authentication")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check authorization
	if !h.authz.CanViewRunCosts(userID, runID) {
		h.logAuditFailure(ctx, userID, "view_run_costs", runID, "insufficient permissions")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get cost records for the run
	records, err := h.store.GetByRunID(ctx, runID)
	if err != nil {
		h.logAuditFailure(ctx, userID, "view_run_costs", runID, err.Error())
		http.Error(w, fmt.Sprintf("Failed to get run costs: %v", err), http.StatusInternalServerError)
		return
	}

	// Aggregate costs by step
	stepCosts := make(map[string]*llm.CostAggregate)
	var totalAggregate llm.CostAggregate

	for _, record := range records {
		// Add to step aggregate
		if stepCosts[record.StepName] == nil {
			stepCosts[record.StepName] = &llm.CostAggregate{}
		}
		h.addRecordToAggregate(stepCosts[record.StepName], record)

		// Add to total aggregate
		h.addRecordToAggregate(&totalAggregate, record)
	}

	// Build response
	response := map[string]interface{}{
		"run_id":      runID,
		"total_cost":  totalAggregate.TotalCost,
		"accuracy":    string(totalAggregate.Accuracy),
		"total_tokens": totalAggregate.TotalTokens,
		"requests":    totalAggregate.TotalRequests,
		"step_costs":  stepCosts,
	}

	// Log successful access
	h.logAuditSuccess(ctx, userID, "view_run_costs", runID)

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// parseAggregateOptions parses query parameters into AggregateOptions.
func (h *CostHandlers) parseAggregateOptions(r *http.Request) (*cost.AggregateOptions, error) {
	opts := &cost.AggregateOptions{}

	// Parse time range
	if since := r.URL.Query().Get("since"); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			return nil, fmt.Errorf("invalid since timestamp: %w", err)
		}
		opts.StartTime = &t
	} else {
		// Default to last 30 days if no start time specified
		defaultStart := time.Now().AddDate(0, 0, -30)
		opts.StartTime = &defaultStart
	}

	if until := r.URL.Query().Get("until"); until != "" {
		t, err := time.Parse(time.RFC3339, until)
		if err != nil {
			return nil, fmt.Errorf("invalid until timestamp: %w", err)
		}
		opts.EndTime = &t
	}

	// Parse filters
	opts.Provider = r.URL.Query().Get("provider")
	opts.Model = r.URL.Query().Get("model")
	opts.WorkflowID = r.URL.Query().Get("workflow")
	opts.RunID = r.URL.Query().Get("run")

	return opts, nil
}

// buildCostSummaryResponse builds a CostSummaryResponse from an aggregate.
func (h *CostHandlers) buildCostSummaryResponse(agg *llm.CostAggregate, opts *cost.AggregateOptions) CostSummaryResponse {
	period := Period{}
	if opts.StartTime != nil {
		period.Start = opts.StartTime.Format(time.RFC3339)
	}
	if opts.EndTime != nil {
		period.End = opts.EndTime.Format(time.RFC3339)
	} else {
		period.End = time.Now().Format(time.RFC3339)
	}

	return CostSummaryResponse{
		Summary: CostSummary{
			TotalCost:         agg.TotalCost,
			TotalCostAccuracy: string(agg.Accuracy),
			TotalTokens:       agg.TotalTokens,
			TotalRequests:     agg.TotalRequests,
			Period:            period,
			AccuracyBreakdown: AccuracyBreakdown{
				Measured:    agg.AccuracyBreakdown.Measured,
				Estimated:   agg.AccuracyBreakdown.Estimated,
				Unavailable: agg.AccuracyBreakdown.Unavailable,
			},
		},
		AccuracyNote: "Costs marked as 'estimated' are approximations based on published pricing...",
	}
}

// buildProviderCostResponse builds a ProviderCostResponse from aggregates.
func (h *CostHandlers) buildProviderCostResponse(aggregates map[string]llm.CostAggregate) ProviderCostResponse {
	var providers []ProviderCost

	for name, agg := range aggregates {
		providers = append(providers, ProviderCost{
			Provider:  name,
			TotalCost: agg.TotalCost,
			Accuracy:  string(agg.Accuracy),
			Requests:  agg.TotalRequests,
			Tokens: TokenBreakdown{
				Input:  agg.TotalPromptTokens,
				Output: agg.TotalCompletionTokens,
			},
		})
	}

	return ProviderCostResponse{
		Providers:    providers,
		AccuracyNote: "Costs marked as 'estimated' are approximations based on published pricing...",
	}
}

// addRecordToAggregate adds a cost record to an aggregate.
func (h *CostHandlers) addRecordToAggregate(agg *llm.CostAggregate, record llm.CostRecord) {
	if record.Cost != nil {
		agg.TotalCost += record.Cost.Amount

		// Track accuracy breakdown
		switch record.Cost.Accuracy {
		case llm.CostMeasured:
			agg.AccuracyBreakdown.Measured++
		case llm.CostEstimated:
			agg.AccuracyBreakdown.Estimated++
		case llm.CostUnavailable:
			agg.AccuracyBreakdown.Unavailable++
		}
	}

	agg.TotalRequests++
	agg.TotalTokens += record.Usage.TotalTokens
	agg.TotalPromptTokens += record.Usage.PromptTokens
	agg.TotalCompletionTokens += record.Usage.CompletionTokens
	agg.TotalCacheCreationTokens += record.Usage.CacheCreationTokens
	agg.TotalCacheReadTokens += record.Usage.CacheReadTokens

	// Determine overall accuracy
	totalWithAccuracy := agg.AccuracyBreakdown.Measured + agg.AccuracyBreakdown.Estimated + agg.AccuracyBreakdown.Unavailable
	if totalWithAccuracy > 0 {
		if agg.AccuracyBreakdown.Measured == totalWithAccuracy {
			agg.Accuracy = llm.CostMeasured
		} else if agg.AccuracyBreakdown.Unavailable == totalWithAccuracy {
			agg.Accuracy = llm.CostUnavailable
		} else {
			agg.Accuracy = llm.CostEstimated
		}
	}
}

// getUserID extracts the user ID from the request context or headers.
// In production, this would come from validated auth tokens.
func (h *CostHandlers) getUserID(r *http.Request) string {
	// Try context first (set by auth middleware)
	if userID := r.Context().Value("user_id"); userID != nil {
		if str, ok := userID.(string); ok {
			return str
		}
	}

	// Fallback to header (for testing/development)
	return r.Header.Get("X-User-ID")
}

// logAuditSuccess logs successful cost data access.
func (h *CostHandlers) logAuditSuccess(ctx context.Context, userID, action, resource string) {
	if h.auditStore == nil {
		return
	}

	entry := cost.AuditLogEntry{
		Timestamp:    time.Now(),
		UserID:       userID,
		Action:       action,
		Resource:     resource,
		Success:      true,
	}

	h.auditStore.Log(ctx, entry)
}

// logAuditFailure logs failed cost data access attempts.
func (h *CostHandlers) logAuditFailure(ctx context.Context, userID, action, resource, errorMsg string) {
	if h.auditStore == nil {
		return
	}

	entry := cost.AuditLogEntry{
		Timestamp:    time.Now(),
		UserID:       userID,
		Action:       action,
		Resource:     resource,
		Success:      false,
		ErrorMessage: errorMsg,
	}

	h.auditStore.Log(ctx, entry)
}

// extractRunID extracts the run ID from a URL path like /v1/runs/{id}/costs.
func extractRunID(path string) string {
	// Simple extraction: /v1/runs/{id}/costs or /v1/runs/{id}
	// This is a minimal implementation; production would use a router
	if len(path) < 10 {
		return ""
	}

	// Remove /v1/runs/ prefix
	if path[:9] == "/v1/runs/" {
		path = path[9:]
	}

	// Find next slash or end of string
	for i, c := range path {
		if c == '/' {
			return path[:i]
		}
	}

	return path
}
