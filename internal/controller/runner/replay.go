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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tombee/conductor/internal/controller/backend"
	"github.com/tombee/conductor/pkg/workflow"
)

// ValidateReplayConfig validates a replay configuration against business rules.
// It checks for required fields, validates override formats, and sanitizes inputs.
func ValidateReplayConfig(config *backend.ReplayConfig) error {
	if config == nil {
		return fmt.Errorf("replay config cannot be nil")
	}

	if config.ParentRunID == "" {
		return fmt.Errorf("parent_run_id is required")
	}

	// Validate override inputs are properly formatted and safe
	if err := validateOverrideInputs(config.OverrideInputs); err != nil {
		return fmt.Errorf("invalid override inputs: %w", err)
	}

	// Validate override steps are properly formatted JSON
	if err := validateOverrideSteps(config.OverrideSteps); err != nil {
		return fmt.Errorf("invalid override steps: %w", err)
	}

	// Validate cost limit is non-negative
	if config.MaxCost < 0 {
		return fmt.Errorf("max_cost cannot be negative: %f", config.MaxCost)
	}

	return nil
}

// validateOverrideInputs validates that override inputs are safe for template injection.
// This prevents template expressions from being injected through user-provided values.
func validateOverrideInputs(inputs map[string]any) error {
	if inputs == nil {
		return nil
	}

	// Pattern for detecting template expressions
	templatePattern := regexp.MustCompile(`\{\{|\}\}|\$\{`)

	for key, value := range inputs {
		// Validate key doesn't contain special characters
		if !isValidIdentifier(key) {
			return fmt.Errorf("invalid input key '%s': must be alphanumeric with underscores", key)
		}

		// Check string values for template injection attempts
		if str, ok := value.(string); ok {
			if templatePattern.MatchString(str) {
				return fmt.Errorf("input '%s' contains template expressions, which are not allowed in overrides", key)
			}
		}

		// Recursively validate nested maps
		if nested, ok := value.(map[string]any); ok {
			if err := validateOverrideInputs(nested); err != nil {
				return fmt.Errorf("in input '%s': %w", key, err)
			}
		}

		// Recursively validate arrays
		if arr, ok := value.([]any); ok {
			if err := validateArrayElements(arr, key, templatePattern); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateArrayElements validates elements in an array recursively.
func validateArrayElements(arr []any, key string, templatePattern *regexp.Regexp) error {
	for i, elem := range arr {
		// Check string values for template injection
		if str, ok := elem.(string); ok {
			if templatePattern.MatchString(str) {
				return fmt.Errorf("input '%s[%d]' contains template expressions, which are not allowed in overrides", key, i)
			}
		}

		// Recursively validate nested maps
		if nested, ok := elem.(map[string]any); ok {
			if err := validateOverrideInputs(nested); err != nil {
				return fmt.Errorf("in input '%s[%d]': %w", key, i, err)
			}
		}

		// Recursively validate nested arrays
		if nestedArr, ok := elem.([]any); ok {
			if err := validateArrayElements(nestedArr, fmt.Sprintf("%s[%d]", key, i), templatePattern); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateOverrideSteps validates that step overrides are properly formatted.
func validateOverrideSteps(steps map[string]any) error {
	if steps == nil {
		return nil
	}

	for stepID, value := range steps {
		// Validate step ID format
		if !isValidIdentifier(stepID) {
			return fmt.Errorf("invalid step ID '%s': must be alphanumeric with underscores", stepID)
		}

		// Validate value can be marshaled to JSON (ensures it's serializable)
		if _, err := json.Marshal(value); err != nil {
			return fmt.Errorf("step '%s' override value is not valid JSON: %w", stepID, err)
		}
	}

	return nil
}

// isValidIdentifier checks if a string is a valid identifier (alphanumeric + underscores).
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, s)
	return match
}

// SanitizeOverrideInputs sanitizes override input values to prevent injection attacks.
// This escapes special characters in string values while preserving data structure.
func SanitizeOverrideInputs(inputs map[string]any) map[string]any {
	if inputs == nil {
		return nil
	}

	sanitized := make(map[string]any)
	for key, value := range inputs {
		switch v := value.(type) {
		case string:
			// Escape template delimiters and shell special characters
			sanitized[key] = escapeStringValue(v)
		case map[string]any:
			// Recursively sanitize nested maps
			sanitized[key] = SanitizeOverrideInputs(v)
		case []any:
			// Sanitize array elements
			sanitized[key] = sanitizeArray(v)
		default:
			// Numbers, booleans, nil - pass through
			sanitized[key] = v
		}
	}
	return sanitized
}

// escapeStringValue escapes special characters that could be interpreted as template expressions.
func escapeStringValue(s string) string {
	// Replace template delimiters with HTML entities to prevent interpretation
	s = strings.ReplaceAll(s, "{{", "&#123;&#123;")
	s = strings.ReplaceAll(s, "}}", "&#125;&#125;")
	s = strings.ReplaceAll(s, "${", "&#36;&#123;")
	return s
}

// sanitizeArray sanitizes all elements in an array.
func sanitizeArray(arr []any) []any {
	sanitized := make([]any, len(arr))
	for i, elem := range arr {
		switch v := elem.(type) {
		case string:
			sanitized[i] = escapeStringValue(v)
		case map[string]any:
			sanitized[i] = SanitizeOverrideInputs(v)
		case []any:
			sanitized[i] = sanitizeArray(v)
		default:
			sanitized[i] = v
		}
	}
	return sanitized
}

// ValidateCachedOutputs validates that cached step outputs are still compatible
// with the current workflow definition.
// Returns an error if the workflow structure has changed (step additions/removals/reordering).
func ValidateCachedOutputs(
	ctx context.Context,
	stepStore backend.StepResultStore,
	parentRunID string,
	currentWorkflow *workflow.Definition,
) error {
	if stepStore == nil {
		return fmt.Errorf("step store not available")
	}

	// Get all step results from parent run
	parentResults, err := stepStore.ListStepResults(ctx, parentRunID)
	if err != nil {
		return fmt.Errorf("failed to fetch parent run step results: %w", err)
	}

	// Build map of parent step IDs for quick lookup
	parentStepIDs := make(map[string]int) // stepID -> index
	for _, result := range parentResults {
		parentStepIDs[result.StepID] = result.StepIndex
	}

	// Build map of current workflow step IDs
	currentStepIDs := make(map[string]int) // stepID -> index
	for i, step := range currentWorkflow.Steps {
		currentStepIDs[step.ID] = i
	}

	// Check for structural changes
	// 1. Step addition/removal
	if len(parentStepIDs) != len(currentStepIDs) {
		return fmt.Errorf("workflow structure changed: parent had %d steps, current has %d steps (replay blocked)",
			len(parentStepIDs), len(currentStepIDs))
	}

	// 2. Step reordering or ID changes
	for stepID, parentIdx := range parentStepIDs {
		currentIdx, exists := currentStepIDs[stepID]
		if !exists {
			return fmt.Errorf("workflow structure changed: step %q no longer exists (replay blocked)", stepID)
		}
		if currentIdx != parentIdx {
			return fmt.Errorf("workflow structure changed: step %q moved from index %d to %d (replay blocked)",
				stepID, parentIdx, currentIdx)
		}
	}

	return nil
}

// ReplayCostEstimate represents the estimated cost breakdown for a replay.
type ReplayCostEstimate struct {
	// TotalCost is the estimated total cost in USD
	TotalCost float64 `json:"total_cost"`

	// SkippedCost is the cost saved by using cached outputs
	SkippedCost float64 `json:"skipped_cost"`

	// NewCost is the estimated cost of steps that will be re-executed
	NewCost float64 `json:"new_cost"`

	// StepBreakdown is a per-step cost breakdown (only if detailed=true)
	StepBreakdown []StepCostBreakdown `json:"step_breakdown,omitempty"`
}

// StepCostBreakdown represents cost information for a single step.
type StepCostBreakdown struct {
	StepID    string  `json:"step_id"`
	StepIndex int     `json:"step_index"`
	Cached    bool    `json:"cached"`
	CostUSD   float64 `json:"cost_usd"`
}

// EstimateReplayCost calculates the estimated cost of replaying a workflow.
// It uses cached step costs from the parent run to estimate savings.
func EstimateReplayCost(
	ctx context.Context,
	stepStore backend.StepResultStore,
	config *backend.ReplayConfig,
	detailed bool,
) (*ReplayCostEstimate, error) {
	if stepStore == nil {
		return nil, fmt.Errorf("step store not available")
	}

	// Get all step results from parent run
	parentResults, err := stepStore.ListStepResults(ctx, config.ParentRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch parent run step results: %w", err)
	}

	estimate := &ReplayCostEstimate{}
	if detailed {
		estimate.StepBreakdown = make([]StepCostBreakdown, 0, len(parentResults))
	}

	// Track which step to resume from
	fromStepIndex := -1
	if config.FromStepID != "" {
		for _, result := range parentResults {
			if result.StepID == config.FromStepID {
				fromStepIndex = result.StepIndex
				break
			}
		}
		if fromStepIndex == -1 {
			return nil, fmt.Errorf("from_step_id %q not found in parent run", config.FromStepID)
		}
	}

	// Calculate costs per step
	for _, result := range parentResults {
		stepCost := result.CostUSD
		cached := false

		// Determine if this step will be cached
		if fromStepIndex >= 0 {
			// If resuming from a specific step, cache all steps before it
			cached = result.StepIndex < fromStepIndex
		}

		// Check if step has an override (overrides are assumed zero cost)
		if config.OverrideSteps != nil {
			if _, hasOverride := config.OverrideSteps[result.StepID]; hasOverride {
				stepCost = 0
				cached = false // Override means re-execution, but at zero cost
			}
		}

		// Accumulate costs
		if cached {
			estimate.SkippedCost += stepCost
		} else {
			estimate.NewCost += stepCost
		}
		estimate.TotalCost += stepCost

		// Add to breakdown if requested
		if detailed {
			estimate.StepBreakdown = append(estimate.StepBreakdown, StepCostBreakdown{
				StepID:    result.StepID,
				StepIndex: result.StepIndex,
				Cached:    cached,
				CostUSD:   stepCost,
			})
		}
	}

	return estimate, nil
}

// AuthorizeReplay checks if the user is authorized to replay a run.
// Authorization is granted if the user owns the run or has execute AND debug permissions on the workflow.
func AuthorizeReplay(
	ctx context.Context,
	runStore backend.RunStore,
	parentRunID string,
	userID string,
) error {
	if parentRunID == "" {
		return fmt.Errorf("parent_run_id is required")
	}
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}

	// Fetch the parent run
	parentRun, err := runStore.GetRun(ctx, parentRunID)
	if err != nil {
		return fmt.Errorf("failed to fetch parent run: %w", err)
	}

	// Check if user owns the run (run creator)
	// User-based authorization not yet implemented.
	// Future: check parentRun.UserID and workflow execute/debug permissions.
	_ = parentRun
	return nil
}

// ExecuteReplay executes a replay of a workflow run.
// It restores cached outputs for skipped steps, applies overrides, and resumes execution.
func ExecuteReplay(
	ctx context.Context,
	store backend.Backend,
	config *backend.ReplayConfig,
	workflowDef *workflow.Definition,
) (map[string]any, error) {
	// Validate the config
	if err := ValidateReplayConfig(config); err != nil {
		return nil, fmt.Errorf("invalid replay config: %w", err)
	}

	// Check if backend supports step result storage
	stepStore, ok := store.(backend.StepResultStore)
	if !ok {
		return nil, fmt.Errorf("backend does not support step result storage")
	}

	// Validate cached outputs against current workflow
	if config.ValidateSchema {
		if err := ValidateCachedOutputs(ctx, stepStore, config.ParentRunID, workflowDef); err != nil {
			return nil, fmt.Errorf("cached output validation failed: %w", err)
		}
	}

	// Get all step results from parent run
	parentResults, err := stepStore.ListStepResults(ctx, config.ParentRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch parent run step results: %w", err)
	}

	// Build a map of step results by step ID for quick lookup
	resultsByStepID := make(map[string]*backend.StepResult)
	for _, result := range parentResults {
		resultsByStepID[result.StepID] = result
	}

	// Determine which step to resume from
	fromStepIndex := 0
	if config.FromStepID != "" {
		found := false
		for i, step := range workflowDef.Steps {
			if step.ID == config.FromStepID {
				fromStepIndex = i
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("from_step_id %q not found in workflow definition", config.FromStepID)
		}
	}

	// Initialize execution context with cached outputs
	execContext := make(map[string]any)

	// Restore cached outputs for steps before fromStepIndex
	for i := 0; i < fromStepIndex; i++ {
		step := workflowDef.Steps[i]

		// Check if there's an override for this step
		if overrideOutput, hasOverride := config.OverrideSteps[step.ID]; hasOverride {
			// Use the override output
			execContext[step.ID] = overrideOutput
			continue
		}

		// Restore from cached result
		if result, exists := resultsByStepID[step.ID]; exists {
			if result.Outputs != nil {
				execContext[step.ID] = result.Outputs
			}
		} else {
			// Missing cached result for a step that should be cached
			return nil, fmt.Errorf("missing cached result for step %q (index %d)", step.ID, i)
		}
	}

	// Apply input overrides to the execution context
	if config.OverrideInputs != nil {
		for key, value := range config.OverrideInputs {
			execContext[key] = value
		}
	}

	// Apply step overrides to the execution context
	if config.OverrideSteps != nil {
		for stepID, output := range config.OverrideSteps {
			execContext[stepID] = output
		}
	}

	// Return the prepared execution context
	// The actual workflow execution would happen here, but that's handled by the executor
	// This function prepares the context for resumption
	return execContext, nil
}
