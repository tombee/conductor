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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/cost"
)

// mockCostStore implements cost.CostStore for testing.
type mockCostStore struct {
	records    []llm.CostRecord
	aggregates map[string]llm.CostAggregate
}

func newMockCostStore() *mockCostStore {
	return &mockCostStore{
		records:    make([]llm.CostRecord, 0),
		aggregates: make(map[string]llm.CostAggregate),
	}
}

func (m *mockCostStore) Store(ctx context.Context, record llm.CostRecord) error {
	m.records = append(m.records, record)
	return nil
}

func (m *mockCostStore) GetByID(ctx context.Context, id string) (*llm.CostRecord, error) {
	for _, r := range m.records {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, nil
}

func (m *mockCostStore) GetByRequestID(ctx context.Context, requestID string) (*llm.CostRecord, error) {
	return nil, nil
}

func (m *mockCostStore) GetByRunID(ctx context.Context, runID string) ([]llm.CostRecord, error) {
	var results []llm.CostRecord
	for _, r := range m.records {
		if r.RunID == runID {
			results = append(results, r)
		}
	}
	return results, nil
}

func (m *mockCostStore) GetByWorkflowID(ctx context.Context, workflowID string) ([]llm.CostRecord, error) {
	return nil, nil
}

func (m *mockCostStore) GetByUserID(ctx context.Context, userID string) ([]llm.CostRecord, error) {
	return nil, nil
}

func (m *mockCostStore) GetByProvider(ctx context.Context, provider string) ([]llm.CostRecord, error) {
	return nil, nil
}

func (m *mockCostStore) GetByModel(ctx context.Context, model string) ([]llm.CostRecord, error) {
	return nil, nil
}

func (m *mockCostStore) GetByTimeRange(ctx context.Context, start, end time.Time) ([]llm.CostRecord, error) {
	return nil, nil
}

func (m *mockCostStore) Aggregate(ctx context.Context, opts cost.AggregateOptions) (*llm.CostAggregate, error) {
	agg := &llm.CostAggregate{
		TotalCost:     10.50,
		TotalRequests: 100,
		TotalTokens:   50000,
		Accuracy:      llm.CostMeasured,
	}
	return agg, nil
}

func (m *mockCostStore) AggregateByProvider(ctx context.Context, opts cost.AggregateOptions) (map[string]llm.CostAggregate, error) {
	return m.aggregates, nil
}

func (m *mockCostStore) AggregateByModel(ctx context.Context, opts cost.AggregateOptions) (map[string]llm.CostAggregate, error) {
	return m.aggregates, nil
}

func (m *mockCostStore) AggregateByWorkflow(ctx context.Context, opts cost.AggregateOptions) (map[string]llm.CostAggregate, error) {
	return m.aggregates, nil
}

func (m *mockCostStore) DeleteOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockCostStore) Close() error {
	return nil
}

// mockAuditStore implements cost.AuditStore for testing.
type mockAuditStore struct {
	entries []cost.AuditLogEntry
}

func newMockAuditStore() *mockAuditStore {
	return &mockAuditStore{
		entries: make([]cost.AuditLogEntry, 0),
	}
}

func (m *mockAuditStore) Log(ctx context.Context, entry cost.AuditLogEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockAuditStore) GetByUser(ctx context.Context, userID string, limit int) ([]cost.AuditLogEntry, error) {
	return nil, nil
}

func (m *mockAuditStore) GetByTimeRange(ctx context.Context, start, end time.Time, limit int) ([]cost.AuditLogEntry, error) {
	return nil, nil
}

func (m *mockAuditStore) GetRecent(ctx context.Context, limit int) ([]cost.AuditLogEntry, error) {
	return m.entries, nil
}

func (m *mockAuditStore) Close() error {
	return nil
}

func TestCostHandlers_HandleCosts(t *testing.T) {
	store := newMockCostStore()
	auditStore := newMockAuditStore()
	authz := NewCostAuthorizer()

	// Assign admin role to test user
	authz.AssignRole("test-user", "cost-admin")

	handlers := NewCostHandlers(store, auditStore, authz)

	mux := http.NewServeMux()
	handlers.RegisterHTTP(mux)

	// Create test request
	req := httptest.NewRequest("GET", "/v1/costs", nil)
	req.Header.Set("X-User-ID", "test-user")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response CostSummaryResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Summary.TotalCost != 10.50 {
		t.Errorf("expected total cost 10.50, got %f", response.Summary.TotalCost)
	}

	// Verify audit log
	if len(auditStore.entries) == 0 {
		t.Error("expected audit log entry, got none")
	}
}

func TestCostHandlers_Unauthorized(t *testing.T) {
	store := newMockCostStore()
	auditStore := newMockAuditStore()
	authz := NewCostAuthorizer()

	handlers := NewCostHandlers(store, auditStore, authz)

	mux := http.NewServeMux()
	handlers.RegisterHTTP(mux)

	// Create test request without auth
	req := httptest.NewRequest("GET", "/v1/costs", nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestCostHandlers_Forbidden(t *testing.T) {
	store := newMockCostStore()
	auditStore := newMockAuditStore()
	authz := NewCostAuthorizer()

	// Don't assign any roles - user has no permissions

	handlers := NewCostHandlers(store, auditStore, authz)

	mux := http.NewServeMux()
	handlers.RegisterHTTP(mux)

	// Create test request with user who has no permissions
	req := httptest.NewRequest("GET", "/v1/costs", nil)
	req.Header.Set("X-User-ID", "unauthorized-user")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestCostHandlers_HandleRunCosts(t *testing.T) {
	store := newMockCostStore()
	auditStore := newMockAuditStore()
	authz := NewCostAuthorizer()

	// Assign viewer role
	authz.AssignRole("test-user", "cost-viewer")

	// Add test data
	store.Store(context.Background(), llm.CostRecord{
		ID:       "rec1",
		RunID:    "run123",
		StepName: "step1",
		Usage: llm.TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
		},
		Cost: &llm.CostInfo{
			Amount:   0.05,
			Currency: "USD",
			Accuracy: llm.CostMeasured,
		},
	})

	handlers := NewCostHandlers(store, auditStore, authz)

	mux := http.NewServeMux()
	handlers.RegisterHTTP(mux)

	// Create test request
	req := httptest.NewRequest("GET", "/v1/runs/run123/costs", nil)
	req.Header.Set("X-User-ID", "test-user")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["run_id"] != "run123" {
		t.Errorf("expected run_id run123, got %v", response["run_id"])
	}

	if response["total_cost"] != 0.05 {
		t.Errorf("expected total_cost 0.05, got %v", response["total_cost"])
	}
}

func TestCostAuthorizer_Roles(t *testing.T) {
	authz := NewCostAuthorizer()

	// Test viewer role
	authz.AssignRole("viewer-user", "cost-viewer")

	if !authz.CanViewCosts("viewer-user") {
		t.Error("viewer should be able to view costs")
	}

	if authz.CanExportCosts("viewer-user") {
		t.Error("viewer should not be able to export costs")
	}

	if authz.IsAdmin("viewer-user") {
		t.Error("viewer should not be admin")
	}

	// Test admin role
	authz.AssignRole("admin-user", "cost-admin")

	if !authz.CanViewCosts("admin-user") {
		t.Error("admin should be able to view costs")
	}

	if !authz.CanExportCosts("admin-user") {
		t.Error("admin should be able to export costs")
	}

	if !authz.IsAdmin("admin-user") {
		t.Error("admin should be admin")
	}

	// Test role revocation
	authz.RevokeRole("admin-user", "cost-admin")

	if authz.IsAdmin("admin-user") {
		t.Error("admin role should be revoked")
	}
}

func TestExtractRunID(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/v1/runs/abc123/costs", "abc123"},
		{"/v1/runs/run-456", "run-456"},
		{"/v1/runs/", ""},
		{"/invalid", ""},
	}

	for _, tt := range tests {
		result := extractRunID(tt.path)
		if result != tt.expected {
			t.Errorf("extractRunID(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}
