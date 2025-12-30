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

//go:build integration

package runner

import (
	"context"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/controller/backend"
	"github.com/tombee/conductor/internal/controller/backend/memory"
	"github.com/tombee/conductor/pkg/workflow"
)

// TestReplayFlow_FullReplayFromFailure tests a complete replay flow:
// 1. Run workflow that fails at step 3
// 2. Create replay from the failure point
// 3. Verify cached outputs are reused
// 4. Verify cost savings are calculated
func TestReplayFlow_FullReplayFromFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create backend
	b := memory.New()

	// Create a workflow that will fail at step 3
	def := &workflow.Definition{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []workflow.Step{
			{
				ID:   "step1",
				Name: "Step 1",
				Type: "shell",
				Config: map[string]any{
					"command": "echo 'step 1 output'",
				},
			},
			{
				ID:   "step2",
				Name: "Step 2",
				Type: "shell",
				Config: map[string]any{
					"command": "echo 'step 2 output'",
				},
			},
			{
				ID:   "step3",
				Name: "Step 3 (fails)",
				Type: "shell",
				Config: map[string]any{
					"command": "exit 1", // This will fail
				},
			},
			{
				ID:   "step4",
				Name: "Step 4",
				Type: "shell",
				Config: map[string]any{
					"command": "echo 'step 4 output'",
				},
			},
		},
	}

	// Create initial run (will fail at step3)
	initialRun := &backend.Run{
		ID:         "run-001",
		WorkflowID: "test-workflow",
		Status:     backend.StatusFailed,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Results: map[string]*backend.StepResult{
			"step1": {
				StepID: "step1",
				Status: backend.StatusSuccess,
				Output: "step 1 output",
				Cost:   0.001,
			},
			"step2": {
				StepID: "step2",
				Status: backend.StatusSuccess,
				Output: "step 2 output",
				Cost:   0.002,
			},
			"step3": {
				StepID: "step3",
				Status: backend.StatusFailed,
				Error:  "command failed with exit code 1",
			},
		},
	}

	// Store initial run
	if err := b.CreateRun(ctx, initialRun); err != nil {
		t.Fatalf("failed to create initial run: %v", err)
	}

	// Create replay configuration
	replayConfig := &backend.ReplayConfig{
		ParentRunID: "run-001",
		FromStep:    "step3",
		OverrideSteps: map[string]any{
			"step3": map[string]any{
				"command": "echo 'fixed step 3'", // Fix the failing step
			},
		},
	}

	// Validate replay config
	if err := ValidateReplayConfig(replayConfig); err != nil {
		t.Fatalf("replay config validation failed: %v", err)
	}

	// Verify workflow hasn't changed (structural validation)
	if err := ValidateWorkflowStructure(def, initialRun.Results); err != nil {
		t.Fatalf("workflow structure validation failed: %v", err)
	}

	// Calculate cost estimation
	estimation, err := EstimateReplayCost(ctx, b, replayConfig)
	if err != nil {
		t.Fatalf("cost estimation failed: %v", err)
	}

	// Verify cost savings
	expectedSavings := 0.003 // step1 (0.001) + step2 (0.002)
	if estimation.CostSavedUSD < expectedSavings-0.0001 || estimation.CostSavedUSD > expectedSavings+0.0001 {
		t.Errorf("expected cost savings ~%.4f, got %.4f", expectedSavings, estimation.CostSavedUSD)
	}

	// Verify skipped steps
	if len(estimation.SkippedSteps) != 2 {
		t.Errorf("expected 2 skipped steps, got %d", len(estimation.SkippedSteps))
	}

	// Create replay run
	replayRun := &backend.Run{
		ID:           "run-002",
		WorkflowID:   "test-workflow",
		ParentRunID:  "run-001",
		ReplayConfig: replayConfig,
		Status:       backend.StatusPending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Results:      make(map[string]*backend.StepResult),
	}

	if err := b.CreateRun(ctx, replayRun); err != nil {
		t.Fatalf("failed to create replay run: %v", err)
	}

	// Verify parent run linkage
	if replayRun.ParentRunID != "run-001" {
		t.Errorf("expected parent run ID 'run-001', got %q", replayRun.ParentRunID)
	}

	// Retrieve the run to verify storage
	stored, err := b.GetRun(ctx, "run-002")
	if err != nil {
		t.Fatalf("failed to retrieve replay run: %v", err)
	}

	if stored.ParentRunID != "run-001" {
		t.Errorf("stored run has incorrect parent ID: got %q, want 'run-001'", stored.ParentRunID)
	}
}

// TestReplayFlow_CostEstimationAccuracy tests the accuracy of cost estimation
func TestReplayFlow_CostEstimationAccuracy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	b := memory.New()

	// Create a run with known costs
	run := &backend.Run{
		ID:         "run-with-costs",
		WorkflowID: "cost-test",
		Status:     backend.StatusSuccess,
		Results: map[string]*backend.StepResult{
			"step1": {StepID: "step1", Status: backend.StatusSuccess, Cost: 0.010},
			"step2": {StepID: "step2", Status: backend.StatusSuccess, Cost: 0.025},
			"step3": {StepID: "step3", Status: backend.StatusSuccess, Cost: 0.040},
			"step4": {StepID: "step4", Status: backend.StatusSuccess, Cost: 0.015},
		},
	}

	if err := b.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	tests := []struct {
		name             string
		fromStep         string
		expectedSavings  float64
		expectedSkipped  int
	}{
		{
			name:            "replay from step 3",
			fromStep:        "step3",
			expectedSavings: 0.035, // step1 + step2
			expectedSkipped: 2,
		},
		{
			name:            "replay from step 2",
			fromStep:        "step2",
			expectedSavings: 0.010, // step1 only
			expectedSkipped: 1,
		},
		{
			name:            "replay from step 4",
			fromStep:        "step4",
			expectedSavings: 0.075, // step1 + step2 + step3
			expectedSkipped: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &backend.ReplayConfig{
				ParentRunID: "run-with-costs",
				FromStep:    tt.fromStep,
			}

			estimation, err := EstimateReplayCost(ctx, b, config)
			if err != nil {
				t.Fatalf("cost estimation failed: %v", err)
			}

			// Check cost savings (with small tolerance for floating point)
			tolerance := 0.0001
			if estimation.CostSavedUSD < tt.expectedSavings-tolerance ||
				estimation.CostSavedUSD > tt.expectedSavings+tolerance {
				t.Errorf("expected savings %.4f, got %.4f", tt.expectedSavings, estimation.CostSavedUSD)
			}

			// Check skipped steps count
			if len(estimation.SkippedSteps) != tt.expectedSkipped {
				t.Errorf("expected %d skipped steps, got %d", tt.expectedSkipped, len(estimation.SkippedSteps))
			}
		})
	}
}

// TestReplayFlow_CachedOutputValidation tests validation of cached outputs
func TestReplayFlow_CachedOutputValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	b := memory.New()

	def := &workflow.Definition{
		ID: "validation-test",
		Steps: []workflow.Step{
			{ID: "step1", Name: "Step 1", Type: "shell"},
			{ID: "step2", Name: "Step 2", Type: "shell"},
			{ID: "step3", Name: "Step 3", Type: "shell"},
		},
	}

	run := &backend.Run{
		ID:         "run-valid",
		WorkflowID: "validation-test",
		Results: map[string]*backend.StepResult{
			"step1": {StepID: "step1", Status: backend.StatusSuccess, Output: "valid"},
			"step2": {StepID: "step2", Status: backend.StatusSuccess, Output: "valid"},
		},
	}

	if err := b.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Test: Valid workflow structure
	if err := ValidateWorkflowStructure(def, run.Results); err != nil {
		t.Errorf("unexpected validation error for valid structure: %v", err)
	}

	// Test: Workflow with added step (structural change)
	defWithNewStep := &workflow.Definition{
		ID: "validation-test",
		Steps: []workflow.Step{
			{ID: "step1", Name: "Step 1", Type: "shell"},
			{ID: "step1.5", Name: "New Step", Type: "shell"}, // Added step
			{ID: "step2", Name: "Step 2", Type: "shell"},
			{ID: "step3", Name: "Step 3", Type: "shell"},
		},
	}

	if err := ValidateWorkflowStructure(defWithNewStep, run.Results); err == nil {
		t.Error("expected validation error for structural change (added step), got nil")
	}

	// Test: Workflow with removed step (structural change)
	defWithRemovedStep := &workflow.Definition{
		ID: "validation-test",
		Steps: []workflow.Step{
			{ID: "step1", Name: "Step 1", Type: "shell"},
			{ID: "step3", Name: "Step 3", Type: "shell"}, // step2 removed
		},
	}

	if err := ValidateWorkflowStructure(defWithRemovedStep, run.Results); err == nil {
		t.Error("expected validation error for structural change (removed step), got nil")
	}

	// Test: Workflow with reordered steps (structural change)
	defReordered := &workflow.Definition{
		ID: "validation-test",
		Steps: []workflow.Step{
			{ID: "step2", Name: "Step 2", Type: "shell"}, // Reordered
			{ID: "step1", Name: "Step 1", Type: "shell"},
			{ID: "step3", Name: "Step 3", Type: "shell"},
		},
	}

	if err := ValidateWorkflowStructure(defReordered, run.Results); err == nil {
		t.Error("expected validation error for structural change (reordered steps), got nil")
	}
}
