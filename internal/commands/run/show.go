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
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// showCmd represents the run show command.
func newShowCmd() *cobra.Command {
	var (
		jsonOutput bool
		stepID     string
	)

	cmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show details of a workflow run",
		Long: `Display detailed information about a workflow run including:
  - Run status, timing, and progress
  - Input parameters and output results
  - Step execution details (if --step is specified)

For LLM steps, displays the prompt, response, token counts, and model configuration.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]

			// Fetch run details from controller API
			run, err := fetchRunDetails(runID)
			if err != nil {
				return err
			}

			// If --json flag is set, output raw JSON
			if jsonOutput {
				return outputJSON(run)
			}

			// Format and display run details
			return displayRunDetails(run, stepID)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVar(&stepID, "step", "", "Show details for a specific step")

	return cmd
}

// RunDetails represents the response from the runs API.
type RunDetails struct {
	ID          string         `json:"id"`
	WorkflowID  string         `json:"workflow_id"`
	Workflow    string         `json:"workflow"`
	Status      string         `json:"status"`
	Inputs      map[string]any `json:"inputs,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	CurrentStep string         `json:"current_step,omitempty"`
	Completed   int            `json:"completed"`
	Total       int            `json:"total"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// fetchRunDetails fetches run details from the controller API.
func fetchRunDetails(runID string) (*RunDetails, error) {
	url := shared.BuildAPIURL(fmt.Sprintf("/v1/runs/%s", runID), nil)

	respBody, err := shared.MakeAPIRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch run details: %w", err)
	}

	var run RunDetails
	if err := json.Unmarshal(respBody, &run); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &run, nil
}

// outputJSON outputs the run details as JSON.
func outputJSON(run *RunDetails) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(run)
}

// displayRunDetails displays formatted run details.
func displayRunDetails(run *RunDetails, stepID string) error {
	fmt.Fprintf(os.Stdout, "Run: %s\n", run.ID)
	fmt.Fprintf(os.Stdout, "Workflow: %s\n", run.Workflow)
	fmt.Fprintf(os.Stdout, "Status: %s\n", run.Status)

	if run.StartedAt != nil {
		fmt.Fprintf(os.Stdout, "Started: %s\n", run.StartedAt.Format(time.RFC3339))
	}

	if run.CompletedAt != nil {
		fmt.Fprintf(os.Stdout, "Completed: %s\n", run.CompletedAt.Format(time.RFC3339))

		if run.StartedAt != nil {
			duration := run.CompletedAt.Sub(*run.StartedAt)
			fmt.Fprintf(os.Stdout, "Duration: %s\n", duration.Round(time.Millisecond))
		}
	}

	fmt.Fprintf(os.Stdout, "Progress: %d/%d steps\n", run.Completed, run.Total)

	if run.CurrentStep != "" {
		fmt.Fprintf(os.Stdout, "Current Step: %s\n", run.CurrentStep)
	}

	// Display inputs if present
	if len(run.Inputs) > 0 {
		fmt.Fprintf(os.Stdout, "\nInputs:\n")
		for key, value := range run.Inputs {
			fmt.Fprintf(os.Stdout, "  %s: %v\n", key, value)
		}
	}

	// Display output if present
	if len(run.Output) > 0 {
		fmt.Fprintf(os.Stdout, "\nOutput:\n")
		outputJSON, err := json.MarshalIndent(run.Output, "  ", "  ")
		if err == nil {
			fmt.Fprintf(os.Stdout, "  %s\n", outputJSON)
		}
	}

	// Display error if present
	if run.Error != "" {
		fmt.Fprintf(os.Stdout, "\nError: %s\n", run.Error)
	}

	// If step ID is specified, fetch and display step details
	if stepID != "" {
		fmt.Fprintf(os.Stdout, "\nStep details for '%s' not yet implemented (T30-T32)\n", stepID)
	}

	return nil
}
