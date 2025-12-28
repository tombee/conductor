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

package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

const (
	// RunStatusDryRun is the status for dry-run executions
	RunStatusDryRun RunStatus = "dry_run"
)

// DryRunPlan represents the execution plan for a workflow without running it.
type DryRunPlan struct {
	Steps            []DryRunStep `json:"steps"`
	EstimatedCost    string       `json:"estimated_cost,omitempty"`
	SecurityProfile  string       `json:"security_profile,omitempty"`
	TotalSteps       int          `json:"total_steps"`
	EstimatedDuration string       `json:"estimated_duration,omitempty"`
}

// DryRunStep represents a planned step execution.
type DryRunStep struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	EstimatedTokens int    `json:"estimated_tokens,omitempty"`
}

// DryRun analyzes a workflow and returns an execution plan without running it.
// This is used for the --dry-run flag to preview what would happen.
func (r *Runner) DryRun(ctx context.Context, req SubmitRequest) (*RunSnapshot, error) {
	// Get workflow YAML (either from request or remote)
	var workflowYAML []byte
	var err error

	if req.RemoteRef != "" {
		// Fetch remote workflow
		r.mu.RLock()
		fetcher := r.fetcher
		r.mu.RUnlock()

		if fetcher == nil {
			return nil, fmt.Errorf("remote workflows not supported (fetcher not configured)")
		}

		result, err := fetcher.Fetch(ctx, req.RemoteRef, req.NoCache)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch remote workflow: %w", err)
		}
		workflowYAML = result.Content
	} else {
		workflowYAML = req.WorkflowYAML
	}

	// Parse workflow definition
	definition, err := workflow.ParseDefinition(workflowYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Build dry-run plan
	plan := &DryRunPlan{
		Steps:           make([]DryRunStep, 0, len(definition.Steps)),
		TotalSteps:      len(definition.Steps),
		SecurityProfile: req.Security,
	}

	// Analyze each step
	for _, step := range definition.Steps {
		dryStep := DryRunStep{
			ID:   step.ID,
			Name: step.Name,
			Type: string(step.Type),
		}

		// For LLM steps, include model information
		if step.Type == workflow.StepTypeLLM {
			// Use override model if specified, otherwise step model
			if req.Model != "" {
				dryStep.Model = req.Model
			} else if step.Model != "" {
				dryStep.Model = step.Model
			}

			// Provider would need to be resolved from configuration
			// For now, use override if specified
			if req.Provider != "" {
				dryStep.Provider = req.Provider
			}

			// Rough token estimate (placeholder - real implementation would be more sophisticated)
			if step.Prompt != "" {
				// Simple heuristic: ~4 chars per token
				dryStep.EstimatedTokens = len(step.Prompt) / 4
			}
		}

		plan.Steps = append(plan.Steps, dryStep)
	}

	// Estimated duration (rough heuristic)
	if len(definition.Steps) > 0 {
		// Assume ~5 seconds per step on average
		estimatedSeconds := len(definition.Steps) * 5
		plan.EstimatedDuration = fmt.Sprintf("%ds", estimatedSeconds)
	}

	// Create a snapshot representing the dry-run result
	now := time.Now()
	snapshot := &RunSnapshot{
		ID:          fmt.Sprintf("dry-run-%d", now.Unix()),
		WorkflowID:  definition.Name,
		Workflow:    definition.Name,
		Status:      RunStatusDryRun,
		Inputs:      req.Inputs,
		CreatedAt:   now,
		Workspace:   req.Workspace,
		Profile:     req.Profile,
		Provider:    req.Provider,
		Model:       req.Model,
		Timeout:     req.Timeout,
		Security:    req.Security,
		AllowHosts:  req.AllowHosts,
		AllowPaths:  req.AllowPaths,
		MCPDev:      req.MCPDev,
	}

	// Attach the plan as output
	snapshot.Output = map[string]any{
		"plan": plan,
	}

	return snapshot, nil
}
