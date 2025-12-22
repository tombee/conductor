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
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
)

// Helper function to create test messages
func testMessage(method string, params interface{}) *Message {
	paramsJSON, _ := json.Marshal(params)
	return &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-" + method,
		Method:        method,
		Params:        paramsJSON,
	}
}

func TestWorkflowHandlers_Create(t *testing.T) {
	store := workflow.NewMemoryStore()
	handlers := NewWorkflowHandlers(store, nil)

	tests := []struct {
		name      string
		params    interface{}
		wantError bool
	}{
		{
			name: "valid create",
			params: map[string]interface{}{
				"id":   "wf-1",
				"name": "Test Workflow",
				"metadata": map[string]interface{}{
					"key": "value",
				},
			},
			wantError: false,
		},
		{
			name: "missing id",
			params: map[string]interface{}{
				"name": "Test Workflow",
			},
			wantError: true,
		},
		{
			name: "missing name",
			params: map[string]interface{}{
				"id": "wf-2",
			},
			wantError: true,
		},
		{
			name: "duplicate id",
			params: map[string]interface{}{
				"id":   "wf-1",
				"name": "Duplicate Workflow",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := testMessage("workflow.create", tt.params)
			resp, err := handlers.handleCreate(context.Background(), req)
			if (err != nil) != tt.wantError {
				t.Errorf("handleCreate() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && resp == nil {
				t.Error("handleCreate() returned nil response")
			}
		})
	}
}

func TestWorkflowHandlers_Get(t *testing.T) {
	store := workflow.NewMemoryStore()
	handlers := NewWorkflowHandlers(store, nil)

	// Create a test workflow
	wf := &workflow.Workflow{
		ID:    "wf-test",
		Name:  "Test Workflow",
		State: workflow.StateCreated,
	}
	if err := store.Create(context.Background(), wf); err != nil {
		t.Fatalf("Failed to create test workflow: %v", err)
	}

	tests := []struct {
		name      string
		params    interface{}
		wantError bool
	}{
		{
			name: "valid get",
			params: map[string]interface{}{
				"id": "wf-test",
			},
			wantError: false,
		},
		{
			name:      "missing id",
			params:    map[string]interface{}{},
			wantError: true,
		},
		{
			name: "not found",
			params: map[string]interface{}{
				"id": "wf-nonexistent",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := testMessage("workflow.get", tt.params)
			resp, err := handlers.handleGet(context.Background(), req)
			if (err != nil) != tt.wantError {
				t.Errorf("handleGet() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && resp == nil {
				t.Error("handleGet() returned nil response")
			}
		})
	}
}

func TestWorkflowHandlers_StateTransitions(t *testing.T) {
	store := workflow.NewMemoryStore()
	handlers := NewWorkflowHandlers(store, nil)

	// Create a test workflow
	wf := &workflow.Workflow{
		ID:    "wf-test",
		Name:  "Test Workflow",
		State: workflow.StateCreated,
	}
	if err := store.Create(context.Background(), wf); err != nil {
		t.Fatalf("Failed to create test workflow: %v", err)
	}

	tests := []struct {
		name        string
		handler     Handler
		method      string
		params      interface{}
		wantError   bool
		expectState workflow.State
	}{
		{
			name:    "start workflow",
			handler: handlers.handleStart,
			method:  "workflow.start",
			params: map[string]interface{}{
				"id": "wf-test",
			},
			wantError:   false,
			expectState: workflow.StateRunning,
		},
		{
			name:    "pause workflow",
			handler: handlers.handlePause,
			method:  "workflow.pause",
			params: map[string]interface{}{
				"id": "wf-test",
			},
			wantError:   false,
			expectState: workflow.StatePaused,
		},
		{
			name:    "resume workflow",
			handler: handlers.handleResume,
			method:  "workflow.resume",
			params: map[string]interface{}{
				"id": "wf-test",
			},
			wantError:   false,
			expectState: workflow.StateRunning,
		},
		{
			name:    "complete workflow",
			handler: handlers.handleComplete,
			method:  "workflow.complete",
			params: map[string]interface{}{
				"id": "wf-test",
			},
			wantError:   false,
			expectState: workflow.StateCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := testMessage(tt.method, tt.params)
			resp, err := tt.handler(context.Background(), req)
			if (err != nil) != tt.wantError {
				t.Errorf("handler() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				if resp == nil {
					t.Error("handler() returned nil response")
					return
				}

				// Check the workflow state
				updated, err := store.Get(context.Background(), "wf-test")
				if err != nil {
					t.Errorf("Failed to get updated workflow: %v", err)
					return
				}
				if updated.State != tt.expectState {
					t.Errorf("Expected state %s, got %s", tt.expectState, updated.State)
				}
			}
		})
	}
}

func TestWorkflowHandlers_Fail(t *testing.T) {
	store := workflow.NewMemoryStore()
	handlers := NewWorkflowHandlers(store, nil)

	// Create and start a test workflow
	wf := &workflow.Workflow{
		ID:    "wf-test",
		Name:  "Test Workflow",
		State: workflow.StateCreated,
	}
	if err := store.Create(context.Background(), wf); err != nil {
		t.Fatalf("Failed to create test workflow: %v", err)
	}

	// Start it and pause it (fail only works from paused due to state machine design)
	startReq := testMessage("workflow.start", map[string]interface{}{
		"id": "wf-test",
	})
	if _, err := handlers.handleStart(context.Background(), startReq); err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	pauseReq := testMessage("workflow.pause", map[string]interface{}{
		"id": "wf-test",
	})
	if _, err := handlers.handlePause(context.Background(), pauseReq); err != nil {
		t.Fatalf("Failed to pause workflow: %v", err)
	}

	req := testMessage("workflow.fail", map[string]interface{}{
		"id":    "wf-test",
		"error": "Test error message",
	})

	resp, err := handlers.handleFail(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFail() error = %v", err)
	}
	if resp == nil {
		t.Fatal("handleFail() returned nil response")
	}

	// Verify state and error
	updated, err := store.Get(context.Background(), "wf-test")
	if err != nil {
		t.Fatalf("Failed to get updated workflow: %v", err)
	}
	if updated.State != workflow.StateFailed {
		t.Errorf("Expected state %s, got %s", workflow.StateFailed, updated.State)
	}
	if updated.Error != "Test error message" {
		t.Errorf("Expected error 'Test error message', got '%s'", updated.Error)
	}
}

func TestWorkflowHandlers_List(t *testing.T) {
	store := workflow.NewMemoryStore()
	handlers := NewWorkflowHandlers(store, nil)

	// Create test workflows
	workflows := []*workflow.Workflow{
		{
			ID:    "wf-1",
			Name:  "Workflow 1",
			State: workflow.StateCreated,
			Metadata: map[string]interface{}{
				"team": "backend",
			},
		},
		{
			ID:    "wf-2",
			Name:  "Workflow 2",
			State: workflow.StateRunning,
			Metadata: map[string]interface{}{
				"team": "frontend",
			},
		},
		{
			ID:    "wf-3",
			Name:  "Workflow 3",
			State: workflow.StateRunning,
			Metadata: map[string]interface{}{
				"team": "backend",
			},
		},
	}

	for _, wf := range workflows {
		if err := store.Create(context.Background(), wf); err != nil {
			t.Fatalf("Failed to create test workflow: %v", err)
		}
	}

	tests := []struct {
		name          string
		params        interface{}
		wantError     bool
		expectedCount int
	}{
		{
			name:          "list all",
			params:        map[string]interface{}{},
			wantError:     false,
			expectedCount: 3,
		},
		{
			name: "filter by state",
			params: map[string]interface{}{
				"state": "running",
			},
			wantError:     false,
			expectedCount: 2,
		},
		{
			name: "filter by metadata",
			params: map[string]interface{}{
				"metadata": map[string]interface{}{
					"team": "backend",
				},
			},
			wantError:     false,
			expectedCount: 2,
		},
		{
			name: "with limit",
			params: map[string]interface{}{
				"limit": 2,
			},
			wantError:     false,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := testMessage("workflow.list", tt.params)
			resp, err := handlers.handleList(context.Background(), req)
			if (err != nil) != tt.wantError {
				t.Errorf("handleList() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				if resp == nil {
					t.Error("handleList() returned nil response")
					return
				}

				var result map[string]interface{}
				if err := resp.UnmarshalResult(&result); err != nil {
					t.Errorf("Failed to unmarshal result: %v", err)
					return
				}

				count, ok := result["count"].(float64) // JSON numbers unmarshal as float64
				if !ok {
					t.Errorf("handleList() count is not a number, got %T", result["count"])
					return
				}

				if int(count) != tt.expectedCount {
					t.Errorf("Expected count %d, got %d", tt.expectedCount, int(count))
				}
			}
		})
	}
}

func TestWorkflowHandlers_Register(t *testing.T) {
	store := workflow.NewMemoryStore()
	handlers := NewWorkflowHandlers(store, nil)
	registry := NewRegistry()

	handlers.Register(registry)

	methods := []string{
		"workflow.create",
		"workflow.get",
		"workflow.start",
		"workflow.pause",
		"workflow.resume",
		"workflow.complete",
		"workflow.fail",
		"workflow.list",
	}

	for _, method := range methods {
		if !registry.HasMethod(method) {
			t.Errorf("Method %s not registered", method)
		}
	}
}
