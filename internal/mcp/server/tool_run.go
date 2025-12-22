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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tombee/conductor/pkg/workflow"
)

const (
	dryRunTimeout      = 5 * time.Minute  // NFR9: dry-run timeout
	executionTimeout   = 30 * time.Minute // NFR9: execution timeout
)

// RunResult represents the result of running a workflow
type RunResult struct {
	Success       bool          `json:"success"`
	Mode          string        `json:"mode"` // "dry_run" or "executed"
	ExecutionPlan []StepPlan    `json:"execution_plan"`
	Outputs       interface{}   `json:"outputs,omitempty"`
	Error         *RunError     `json:"error,omitempty"`
}

// StepPlan represents a step in the execution plan
type StepPlan struct {
	StepID string `json:"step_id"`
	Type   string `json:"type"`
	Status string `json:"status"` // "pending", "success", "failed", "skipped"
}

// RunError represents an error during execution
type RunError struct {
	StepID  string `json:"step_id,omitempty"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// handleRun implements the conductor_run tool
func (s *Server) handleRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check rate limit (always check call limit)
	if !s.rateLimiter.AllowCall() {
		return errorResponse("Rate limit exceeded. Please try again later."), nil
	}

	// Extract workflow_path argument
	workflowPath, err := request.RequireString("workflow_path")
	if err != nil {
		return errorResponse("Missing or invalid 'workflow_path' argument"), nil
	}

	// Validate path
	if err := ValidatePath(workflowPath); err != nil {
		return errorResponse(fmt.Sprintf("Invalid workflow path: %v", err)), nil
	}

	// Extract dry_run flag (defaults to true for safety)
	dryRun := request.GetBool("dry_run", true)

	// Check run rate limit for non-dry-run executions
	if !dryRun {
		if !s.rateLimiter.AllowRun() {
			return errorResponse("Rate limit exceeded for workflow execution. Please try again later or use dry_run=true."), nil
		}
	}

	// Extract inputs (optional) - use GetRawArguments and type assert
	var inputs map[string]interface{}
	if args := request.GetArguments(); args != nil {
		if inputsArg, ok := args["inputs"].(map[string]interface{}); ok {
			inputs = inputsArg
		}
	}

	// Read workflow file
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to read workflow file: %v", err)), nil
	}

	// Parse workflow
	def, err := workflow.ParseDefinition(data)
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to parse workflow: %v", err)), nil
	}

	// Execute or dry-run with timeout
	var result RunResult
	if dryRun {
		execCtx, cancel := context.WithTimeout(ctx, dryRunTimeout)
		defer cancel()
		result = executeDryRun(execCtx, def, inputs)
	} else {
		execCtx, cancel := context.WithTimeout(ctx, executionTimeout)
		defer cancel()
		result = executeWorkflow(execCtx, def, inputs, workflowPath)
	}

	// Marshal result to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to encode run result: %v", err)), nil
	}

	return textResponse(string(resultJSON)), nil
}

// executeDryRun creates an execution plan without running the workflow
func executeDryRun(ctx context.Context, def *workflow.Definition, inputs map[string]interface{}) RunResult {
	result := RunResult{
		Success:       true,
		Mode:          "dry_run",
		ExecutionPlan: []StepPlan{},
	}

	// Validate inputs match definition
	if err := validateInputs(def, inputs); err != nil {
		result.Success = false
		result.Error = &RunError{
			Message: fmt.Sprintf("Input validation failed: %v", err),
		}
		return result
	}

	// Build execution plan from steps
	for _, step := range def.Steps {
		plan := StepPlan{
			StepID: step.ID,
			Type:   string(step.Type),
			Status: "pending",
		}

		// Check if step would be skipped based on condition
		if step.Condition != nil {
			plan.Status = "conditional" // Would be evaluated at runtime
		}

		result.ExecutionPlan = append(result.ExecutionPlan, plan)
	}

	return result
}

// executeWorkflow actually executes the workflow (not implemented for v1)
// For v1, we return an error directing users to use the CLI
func executeWorkflow(ctx context.Context, def *workflow.Definition, inputs map[string]interface{}, workflowPath string) RunResult {
	// For v1, execution via MCP server is not implemented
	// Users should use the CLI or daemon for actual execution
	return RunResult{
		Success: false,
		Mode:    "executed",
		Error: &RunError{
			Message: "Workflow execution via MCP server is not yet implemented",
			Details: fmt.Sprintf("Please use 'conductor run %s' or 'conductor run --daemon %s' to execute this workflow", workflowPath, workflowPath),
		},
	}
}

// validateInputs checks that required inputs are provided
func validateInputs(def *workflow.Definition, inputs map[string]interface{}) error {
	for _, inputDef := range def.Inputs {
		if inputDef.Required {
			if _, ok := inputs[inputDef.Name]; !ok {
				// Check if there's a default value
				if inputDef.Default == nil {
					return fmt.Errorf("required input %q is missing", inputDef.Name)
				}
			}
		}
	}
	return nil
}
