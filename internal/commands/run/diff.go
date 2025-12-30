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
	"reflect"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// newDiffCmd creates the run diff command.
func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <run-id-1> <run-id-2>",
		Short: "Compare two workflow runs",
		Long: `Compare the inputs, outputs, and execution results of two workflow runs.

This command highlights differences between two runs, making it easy to identify
what changed between a working and failing run, or to compare different executions
of the same workflow.

Differences shown:
  - Input parameters
  - Output results
  - Final run status
  - Error messages (if any)

Examples:
  # Compare two runs
  conductor run diff abc123 def456

  # Compare a failed run with its replay
  conductor run diff original_run replay_run

  # Get diff as JSON for programmatic use
  conductor run diff abc123 def456 --json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID1 := args[0]
			runID2 := args[1]

			return compareRuns(runID1, runID2)
		},
	}

	return cmd
}

// RunData represents run information for comparison.
type RunData struct {
	ID       string         `json:"id"`
	Workflow string         `json:"workflow"`
	Status   string         `json:"status"`
	Inputs   map[string]any `json:"inputs,omitempty"`
	Output   map[string]any `json:"output,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// RunDiff represents the differences between two runs.
type RunDiff struct {
	Run1           RunData        `json:"run1"`
	Run2           RunData        `json:"run2"`
	StatusDiff     bool           `json:"status_diff"`
	InputDiffs     []FieldDiff    `json:"input_diffs,omitempty"`
	OutputDiffs    []FieldDiff    `json:"output_diffs,omitempty"`
	ErrorDiff      bool           `json:"error_diff"`
}

// FieldDiff represents a difference in a specific field.
type FieldDiff struct {
	Field  string `json:"field"`
	Value1 any    `json:"value1"`
	Value2 any    `json:"value2"`
}

// compareRuns fetches and compares two runs.
func compareRuns(runID1, runID2 string) error {
	// Fetch both runs
	run1, err := fetchRunData(runID1)
	if err != nil {
		return fmt.Errorf("failed to fetch run %s: %w", runID1, err)
	}

	run2, err := fetchRunData(runID2)
	if err != nil {
		return fmt.Errorf("failed to fetch run %s: %w", runID2, err)
	}

	// Compare the runs
	diff := diffRuns(run1, run2)

	// Display or output the diff
	if shared.GetJSON() {
		return outputJSONDiff(diff)
	}

	return displayDiff(diff)
}

// fetchRunData fetches run data from the API.
func fetchRunData(runID string) (*RunData, error) {
	url := shared.BuildAPIURL(fmt.Sprintf("/v1/runs/%s", runID), nil)

	respBody, err := shared.MakeAPIRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch run details: %w", err)
	}

	var run RunData
	if err := json.Unmarshal(respBody, &run); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &run, nil
}

// diffRuns compares two runs and returns the differences.
func diffRuns(run1, run2 *RunData) *RunDiff {
	diff := &RunDiff{
		Run1:       *run1,
		Run2:       *run2,
		StatusDiff: run1.Status != run2.Status,
		ErrorDiff:  run1.Error != run2.Error,
	}

	// Compare inputs
	diff.InputDiffs = compareFields(run1.Inputs, run2.Inputs)

	// Compare outputs
	diff.OutputDiffs = compareFields(run1.Output, run2.Output)

	return diff
}

// compareFields compares two maps and returns the differences.
func compareFields(map1, map2 map[string]any) []FieldDiff {
	var diffs []FieldDiff

	// Check all keys in map1
	for key, val1 := range map1 {
		if val2, exists := map2[key]; exists {
			// Key exists in both - check if values differ
			if !reflect.DeepEqual(val1, val2) {
				diffs = append(diffs, FieldDiff{
					Field:  key,
					Value1: val1,
					Value2: val2,
				})
			}
		} else {
			// Key only in map1
			diffs = append(diffs, FieldDiff{
				Field:  key,
				Value1: val1,
				Value2: nil,
			})
		}
	}

	// Check for keys only in map2
	for key, val2 := range map2 {
		if _, exists := map1[key]; !exists {
			diffs = append(diffs, FieldDiff{
				Field:  key,
				Value1: nil,
				Value2: val2,
			})
		}
	}

	return diffs
}

// outputJSONDiff outputs the diff as JSON.
func outputJSONDiff(diff *RunDiff) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(diff)
}

// displayDiff displays the diff in human-readable format.
func displayDiff(diff *RunDiff) error {
	fmt.Fprintf(os.Stdout, "Run Comparison\n")
	fmt.Fprintf(os.Stdout, "==============\n\n")

	fmt.Fprintf(os.Stdout, "Run 1: %s\n", diff.Run1.ID)
	fmt.Fprintf(os.Stdout, "  Workflow: %s\n", diff.Run1.Workflow)
	fmt.Fprintf(os.Stdout, "  Status:   %s\n", diff.Run1.Status)

	fmt.Fprintf(os.Stdout, "\nRun 2: %s\n", diff.Run2.ID)
	fmt.Fprintf(os.Stdout, "  Workflow: %s\n", diff.Run2.Workflow)
	fmt.Fprintf(os.Stdout, "  Status:   %s\n", diff.Run2.Status)

	// Show status difference
	if diff.StatusDiff {
		fmt.Fprintf(os.Stdout, "\n--- Status Difference ---\n")
		fmt.Fprintf(os.Stdout, "  Run 1: %s\n", diff.Run1.Status)
		fmt.Fprintf(os.Stdout, "  Run 2: %s\n", diff.Run2.Status)
	}

	// Show error difference
	if diff.ErrorDiff {
		fmt.Fprintf(os.Stdout, "\n--- Error Difference ---\n")
		if diff.Run1.Error != "" {
			fmt.Fprintf(os.Stdout, "  Run 1: %s\n", diff.Run1.Error)
		} else {
			fmt.Fprintf(os.Stdout, "  Run 1: (no error)\n")
		}
		if diff.Run2.Error != "" {
			fmt.Fprintf(os.Stdout, "  Run 2: %s\n", diff.Run2.Error)
		} else {
			fmt.Fprintf(os.Stdout, "  Run 2: (no error)\n")
		}
	}

	// Show input differences
	if len(diff.InputDiffs) > 0 {
		fmt.Fprintf(os.Stdout, "\n--- Input Differences ---\n")
		for _, fdiff := range diff.InputDiffs {
			fmt.Fprintf(os.Stdout, "  %s:\n", fdiff.Field)
			fmt.Fprintf(os.Stdout, "    Run 1: %v\n", formatValue(fdiff.Value1))
			fmt.Fprintf(os.Stdout, "    Run 2: %v\n", formatValue(fdiff.Value2))
		}
	}

	// Show output differences
	if len(diff.OutputDiffs) > 0 {
		fmt.Fprintf(os.Stdout, "\n--- Output Differences ---\n")
		for _, fdiff := range diff.OutputDiffs {
			fmt.Fprintf(os.Stdout, "  %s:\n", fdiff.Field)
			fmt.Fprintf(os.Stdout, "    Run 1: %v\n", formatValue(fdiff.Value1))
			fmt.Fprintf(os.Stdout, "    Run 2: %v\n", formatValue(fdiff.Value2))
		}
	}

	// Show summary
	if !diff.StatusDiff && !diff.ErrorDiff && len(diff.InputDiffs) == 0 && len(diff.OutputDiffs) == 0 {
		fmt.Fprintf(os.Stdout, "\nNo differences found.\n")
	}

	return nil
}

// formatValue formats a value for display.
func formatValue(v any) string {
	if v == nil {
		return "(not set)"
	}
	// For complex types, use JSON encoding
	if _, ok := v.(string); !ok {
		if jsonBytes, err := json.Marshal(v); err == nil {
			return string(jsonBytes)
		}
	}
	return fmt.Sprintf("%v", v)
}
