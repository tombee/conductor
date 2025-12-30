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

package run

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// newReplayCmd creates the run replay command.
func newReplayCmd() *cobra.Command {
	var (
		fromStep       string
		overrideInputs []string
		overrideSteps  []string
		maxCost        float64
		estimate       bool
		detailed       bool
	)

	cmd := &cobra.Command{
		Use:   "replay <run-id>",
		Short: "Replay a failed workflow run",
		Long: `Replay a workflow run from a specific step, using cached outputs for earlier steps.

This command allows you to resume a failed workflow without re-running expensive
steps that already completed successfully. The replay creates a new run linked
to the original via parent_run_id.

Examples:
  # Replay from the step that failed
  conductor run replay abc123 --from failed_step

  # Replay with modified input
  conductor run replay abc123 --from step2 --override-input key=newvalue

  # Inject a mock output for a step
  conductor run replay abc123 --override-step step1='{"result":"mocked"}'

  # Estimate cost before replaying
  conductor run replay abc123 --from step3 --estimate

  # Get detailed cost breakdown
  conductor run replay abc123 --from step3 --estimate --detailed

  # Set a cost limit
  conductor run replay abc123 --max-cost 5.00`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]

			// Parse override inputs
			overrideInputMap, err := parseKeyValuePairs(overrideInputs)
			if err != nil {
				return fmt.Errorf("failed to parse override inputs: %w", err)
			}

			// Parse override steps
			overrideStepMap, err := parseStepOverrides(overrideSteps)
			if err != nil {
				return fmt.Errorf("failed to parse override steps: %w", err)
			}

			// If --estimate flag is set, fetch and display cost estimate
			if estimate {
				return fetchAndDisplayCostEstimate(runID, fromStep, overrideInputMap, overrideStepMap, detailed)
			}

			// Submit replay request
			return submitReplay(runID, fromStep, overrideInputMap, overrideStepMap, maxCost)
		},
	}

	cmd.Flags().StringVar(&fromStep, "from", "", "Step ID to resume from (empty = start from beginning)")
	cmd.Flags().StringSliceVar(&overrideInputs, "override-input", nil, "Override workflow inputs (key=value format)")
	cmd.Flags().StringSliceVar(&overrideSteps, "override-step", nil, "Override step outputs (stepID=jsonValue format)")
	cmd.Flags().Float64Var(&maxCost, "max-cost", 0, "Maximum allowed cost in USD (0 = no limit)")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate cost without executing replay")
	cmd.Flags().BoolVar(&detailed, "detailed", false, "Show detailed per-step cost breakdown (requires --estimate)")

	return cmd
}

// parseKeyValuePairs parses key=value pairs into a map.
func parseKeyValuePairs(pairs []string) (map[string]any, error) {
	result := make(map[string]any)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key=value pair: %q", pair)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

// parseStepOverrides parses step override pairs in the format stepID=jsonValue.
func parseStepOverrides(pairs []string) (map[string]any, error) {
	result := make(map[string]any)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid step override pair: %q", pair)
		}

		stepID := parts[0]
		jsonValue := parts[1]

		// Parse JSON value
		var value any
		if err := json.Unmarshal([]byte(jsonValue), &value); err != nil {
			return nil, fmt.Errorf("invalid JSON for step %q: %w", stepID, err)
		}

		result[stepID] = value
	}
	return result, nil
}

// ReplayCostEstimate represents the cost estimate response.
type ReplayCostEstimate struct {
	TotalCost     float64            `json:"total_cost"`
	SkippedCost   float64            `json:"skipped_cost"`
	NewCost       float64            `json:"new_cost"`
	StepBreakdown []StepCostBreakdown `json:"step_breakdown,omitempty"`
}

// StepCostBreakdown represents cost information for a single step.
type StepCostBreakdown struct {
	StepID    string  `json:"step_id"`
	StepIndex int     `json:"step_index"`
	Cached    bool    `json:"cached"`
	CostUSD   float64 `json:"cost_usd"`
}

// fetchAndDisplayCostEstimate fetches and displays the cost estimate for a replay.
func fetchAndDisplayCostEstimate(runID, fromStep string, overrideInputs, overrideSteps map[string]any, detailed bool) error {
	// Build query parameters
	query := make(map[string]string)
	if fromStep != "" {
		query["from_step"] = fromStep
	}
	if detailed {
		query["detailed"] = "true"
	}

	// Build URL
	apiURL := shared.BuildAPIURL(fmt.Sprintf("/v1/runs/%s/replay/estimate", runID), query)

	// Build request body
	requestBody := map[string]any{
		"override_inputs": overrideInputs,
		"override_steps":  overrideSteps,
	}

	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Make API request
	respBody, err := shared.MakeAPIRequest("POST", apiURL, bodyJSON)
	if err != nil {
		return fmt.Errorf("failed to fetch cost estimate: %w", err)
	}

	// Parse response
	var estimate ReplayCostEstimate
	if err := json.Unmarshal(respBody, &estimate); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Display estimate
	fmt.Fprintf(os.Stdout, "Replay Cost Estimate\n")
	fmt.Fprintf(os.Stdout, "====================\n\n")
	fmt.Fprintf(os.Stdout, "Total Cost:   $%.4f\n", estimate.TotalCost)
	fmt.Fprintf(os.Stdout, "Skipped Cost: $%.4f (cached)\n", estimate.SkippedCost)
	fmt.Fprintf(os.Stdout, "New Cost:     $%.4f (to be executed)\n", estimate.NewCost)
	fmt.Fprintf(os.Stdout, "Savings:      $%.4f (%.1f%%)\n",
		estimate.SkippedCost,
		(estimate.SkippedCost/estimate.TotalCost)*100)

	// Display detailed breakdown if requested
	if detailed && len(estimate.StepBreakdown) > 0 {
		fmt.Fprintf(os.Stdout, "\nPer-Step Breakdown:\n")
		fmt.Fprintf(os.Stdout, "-------------------\n")
		for _, step := range estimate.StepBreakdown {
			status := "EXECUTE"
			if step.Cached {
				status = "CACHED "
			}
			fmt.Fprintf(os.Stdout, "[%s] Step %d (%s): $%.4f\n",
				status, step.StepIndex, step.StepID, step.CostUSD)
		}
	}

	return nil
}

// ReplayRequest represents a replay submission request.
type ReplayRequest struct {
	FromStepID     string         `json:"from_step_id,omitempty"`
	OverrideInputs map[string]any `json:"override_inputs,omitempty"`
	OverrideSteps  map[string]any `json:"override_steps,omitempty"`
	MaxCost        float64        `json:"max_cost,omitempty"`
}

// ReplayResponse represents the response from submitting a replay.
type ReplayResponse struct {
	RunID         string  `json:"run_id"`
	ParentRunID   string  `json:"parent_run_id"`
	SkippedSteps  int     `json:"skipped_steps"`
	CostSavings   float64 `json:"cost_savings"`
}

// submitReplay submits a replay request to the API.
func submitReplay(runID, fromStep string, overrideInputs, overrideSteps map[string]any, maxCost float64) error {
	// Build request
	request := ReplayRequest{
		FromStepID:     fromStep,
		OverrideInputs: overrideInputs,
		OverrideSteps:  overrideSteps,
		MaxCost:        maxCost,
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	apiURL := shared.BuildAPIURL(fmt.Sprintf("/v1/runs/%s/replay", runID), nil)

	// Make API request
	respBody, err := shared.MakeAPIRequest("POST", apiURL, requestJSON)
	if err != nil {
		return fmt.Errorf("failed to submit replay: %w", err)
	}

	// Parse response
	var response ReplayResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Display result
	fmt.Fprintf(os.Stdout, "Replay started successfully\n\n")
	fmt.Fprintf(os.Stdout, "New Run ID:     %s\n", response.RunID)
	fmt.Fprintf(os.Stdout, "Parent Run ID:  %s\n", response.ParentRunID)
	fmt.Fprintf(os.Stdout, "Skipped Steps:  %d\n", response.SkippedSteps)
	fmt.Fprintf(os.Stdout, "Cost Savings:   $%.4f\n", response.CostSavings)
	fmt.Fprintf(os.Stdout, "\nUse 'conductor run show %s' to track progress\n", response.RunID)

	return nil
}
