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

// Checkpoint save and resume logic.
// Handles persisting workflow execution state to enable recovery from interruptions,
// and resuming interrupted runs from saved checkpoints.
package runner

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/daemon/checkpoint"
)

// ResumeInterrupted attempts to resume any interrupted runs from checkpoints.
func (r *Runner) ResumeInterrupted(ctx context.Context) error {
	if r.lifecycle.checkpoints == nil || !r.lifecycle.checkpoints.Enabled() {
		return nil
	}

	runIDs, err := r.lifecycle.checkpoints.ListInterrupted(ctx)
	if err != nil {
		return fmt.Errorf("failed to list interrupted runs: %w", err)
	}

	for _, runID := range runIDs {
		cp, err := r.lifecycle.checkpoints.Load(ctx, runID)
		if err != nil {
			// Log and continue
			continue
		}
		if cp == nil {
			continue
		}

		// TODO: Implement actual resume logic
		// For now, just log that we found interrupted runs
		// Real implementation would reload workflow definition and continue from checkpoint
		_ = cp // Placeholder
	}

	return nil
}

// saveCheckpoint saves a checkpoint for the current execution state.
func (r *Runner) saveCheckpoint(run *Run, stepIndex int, workflowCtx map[string]any) {
	if r.lifecycle.checkpoints == nil || !r.lifecycle.checkpoints.Enabled() {
		return
	}

	cp := &checkpoint.Checkpoint{
		RunID:       run.ID,
		WorkflowID:  run.WorkflowID,
		StepID:      run.Progress.CurrentStep,
		StepIndex:   stepIndex,
		Context:     workflowCtx,
		StepOutputs: make(map[string]any),
	}

	// Copy step outputs
	for k, v := range workflowCtx {
		if k != "inputs" {
			cp.StepOutputs[k] = v
		}
	}

	if err := r.lifecycle.checkpoints.Save(context.Background(), cp); err != nil {
		r.addLog(run, "warn", fmt.Sprintf("Failed to save checkpoint: %v", err), "")
	}
}
