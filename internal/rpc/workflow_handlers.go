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
	"fmt"

	"github.com/tombee/conductor/pkg/workflow"
)

// WorkflowHandlers provides RPC handlers for workflow operations.
type WorkflowHandlers struct {
	store        workflow.Store
	stateMachine *workflow.StateMachine
}

// NewWorkflowHandlers creates a new set of workflow RPC handlers.
func NewWorkflowHandlers(store workflow.Store, stateMachine *workflow.StateMachine) *WorkflowHandlers {
	if stateMachine == nil {
		stateMachine = workflow.NewStateMachine(workflow.DefaultTransitions())
	}
	return &WorkflowHandlers{
		store:        store,
		stateMachine: stateMachine,
	}
}

// Register registers all workflow handlers with the registry.
func (h *WorkflowHandlers) Register(registry *Registry) {
	registry.Register("workflow.create", h.handleCreate)
	registry.Register("workflow.get", h.handleGet)
	registry.Register("workflow.start", h.handleStart)
	registry.Register("workflow.pause", h.handlePause)
	registry.Register("workflow.resume", h.handleResume)
	registry.Register("workflow.complete", h.handleComplete)
	registry.Register("workflow.fail", h.handleFail)
	registry.Register("workflow.list", h.handleList)
}

// CreateRequest is the request payload for workflow.create.
type CreateRequest struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// handleCreate creates a new workflow.
func (h *WorkflowHandlers) handleCreate(ctx context.Context, req *Message) (*Message, error) {
	var createReq CreateRequest
	if err := req.UnmarshalParams(&createReq); err != nil {
		return nil, fmt.Errorf("invalid create request: %w", err)
	}

	if createReq.ID == "" {
		return nil, fmt.Errorf("workflow ID is required")
	}
	if createReq.Name == "" {
		return nil, fmt.Errorf("workflow name is required")
	}

	// Create workflow instance
	wf := &workflow.Workflow{
		ID:       createReq.ID,
		Name:     createReq.Name,
		State:    workflow.StateCreated,
		Metadata: createReq.Metadata,
	}

	if err := h.store.Create(ctx, wf); err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	// Retrieve the created workflow (to get timestamps)
	created, err := h.store.Get(ctx, createReq.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve created workflow: %w", err)
	}

	return NewResponse(req.CorrelationID, created)
}

// GetRequest is the request payload for workflow.get.
type GetRequest struct {
	ID string `json:"id"`
}

// handleGet retrieves a workflow by ID.
func (h *WorkflowHandlers) handleGet(ctx context.Context, req *Message) (*Message, error) {
	var getReq GetRequest
	if err := req.UnmarshalParams(&getReq); err != nil {
		return nil, fmt.Errorf("invalid get request: %w", err)
	}

	if getReq.ID == "" {
		return nil, fmt.Errorf("workflow ID is required")
	}

	wf, err := h.store.Get(ctx, getReq.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return NewResponse(req.CorrelationID, wf)
}

// StateTransitionRequest is the request payload for state transition operations.
type StateTransitionRequest struct {
	ID    string                 `json:"id"`
	Error string                 `json:"error,omitempty"` // For fail operation
	Data  map[string]interface{} `json:"data,omitempty"`  // Optional metadata updates
}

// handleStart starts a workflow.
func (h *WorkflowHandlers) handleStart(ctx context.Context, req *Message) (*Message, error) {
	return h.handleTransition(ctx, req, "start")
}

// handlePause pauses a running workflow.
func (h *WorkflowHandlers) handlePause(ctx context.Context, req *Message) (*Message, error) {
	return h.handleTransition(ctx, req, "pause")
}

// handleResume resumes a paused workflow.
func (h *WorkflowHandlers) handleResume(ctx context.Context, req *Message) (*Message, error) {
	return h.handleTransition(ctx, req, "resume")
}

// handleComplete marks a workflow as completed.
func (h *WorkflowHandlers) handleComplete(ctx context.Context, req *Message) (*Message, error) {
	return h.handleTransition(ctx, req, "complete")
}

// handleFail marks a workflow as failed.
func (h *WorkflowHandlers) handleFail(ctx context.Context, req *Message) (*Message, error) {
	var transReq StateTransitionRequest
	if err := req.UnmarshalParams(&transReq); err != nil {
		return nil, fmt.Errorf("invalid fail request: %w", err)
	}

	if transReq.ID == "" {
		return nil, fmt.Errorf("workflow ID is required")
	}

	// Get workflow
	wf, err := h.store.Get(ctx, transReq.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Set error message if provided
	if transReq.Error != "" {
		wf.Error = transReq.Error
	}

	// Trigger fail event
	if err := h.stateMachine.Trigger(ctx, wf, "fail"); err != nil {
		return nil, fmt.Errorf("failed to fail workflow: %w", err)
	}

	// Update metadata if provided
	if transReq.Data != nil {
		for k, v := range transReq.Data {
			wf.Metadata[k] = v
		}
	}

	// Save updated workflow
	if err := h.store.Update(ctx, wf); err != nil {
		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}

	return NewResponse(req.CorrelationID, wf)
}

// handleTransition handles generic state transitions.
func (h *WorkflowHandlers) handleTransition(ctx context.Context, req *Message, event string) (*Message, error) {
	var transReq StateTransitionRequest
	if err := req.UnmarshalParams(&transReq); err != nil {
		return nil, fmt.Errorf("invalid transition request: %w", err)
	}

	if transReq.ID == "" {
		return nil, fmt.Errorf("workflow ID is required")
	}

	// Get workflow
	wf, err := h.store.Get(ctx, transReq.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Trigger event
	if err := h.stateMachine.Trigger(ctx, wf, event); err != nil {
		return nil, fmt.Errorf("failed to trigger event %s: %w", event, err)
	}

	// Update metadata if provided
	if transReq.Data != nil {
		for k, v := range transReq.Data {
			wf.Metadata[k] = v
		}
	}

	// Save updated workflow
	if err := h.store.Update(ctx, wf); err != nil {
		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}

	return NewResponse(req.CorrelationID, wf)
}

// ListRequest is the request payload for workflow.list.
type ListRequest struct {
	State    *workflow.State        `json:"state,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Limit    int                    `json:"limit,omitempty"`
	Offset   int                    `json:"offset,omitempty"`
}

// handleList lists workflows matching the query.
func (h *WorkflowHandlers) handleList(ctx context.Context, req *Message) (*Message, error) {
	var listReq ListRequest
	if err := req.UnmarshalParams(&listReq); err != nil {
		return nil, fmt.Errorf("invalid list request: %w", err)
	}

	query := &workflow.Query{
		State:    listReq.State,
		Metadata: listReq.Metadata,
		Limit:    listReq.Limit,
		Offset:   listReq.Offset,
	}

	workflows, err := h.store.List(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	return NewResponse(req.CorrelationID, map[string]interface{}{
		"workflows": workflows,
		"count":     len(workflows),
	})
}
