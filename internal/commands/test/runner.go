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

package test

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/tombee/conductor/internal/testing/assert"
	"github.com/tombee/conductor/pkg/workflow"
)

// ErrTestsFailed is returned when one or more tests fail.
var ErrTestsFailed = errors.New("one or more tests failed")

// runTests discovers and executes all tests
func runTests(opts RunOptions) error {
	// Discover test files
	tests, err := discoverTests(opts.Paths)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return displayDiscovery(tests, opts)
	}

	// Execute tests
	summary := TestSummary{
		Total:   len(tests),
		Results: make([]TestResult, 0, len(tests)),
	}

	startTime := time.Now()

	for _, test := range tests {
		result := executeTest(test, opts)
		summary.Results = append(summary.Results, result)

		switch result.Status {
		case TestStatusPass:
			summary.Passed++
		case TestStatusFail:
			summary.Failed++
		case TestStatusError:
			summary.Errors++
		case TestStatusSkipped:
			summary.Skipped++
		}
	}

	summary.Duration = time.Since(startTime)

	// Output results
	if err := outputResults(summary, opts); err != nil {
		return err
	}

	// Return error if any tests failed
	if summary.Failed > 0 || summary.Errors > 0 {
		return ErrTestsFailed
	}

	return nil
}

// executeTest runs a single test
func executeTest(test TestFile, opts RunOptions) TestResult {
	result := TestResult{
		Name:     test.Name,
		File:     test.Path,
		Status:   TestStatusPass,
		Failures: make([]AssertionFailure, 0),
	}

	startTime := time.Now()
	defer func() {
		result.Duration = time.Since(startTime)
	}()

	// Load workflow definition
	data, err := os.ReadFile(test.Workflow)
	if err != nil {
		result.Status = TestStatusError
		result.Error = fmt.Sprintf("Failed to read workflow file: %v", err)
		return result
	}

	workflowDef, err := workflow.ParseDefinition(data)
	if err != nil {
		result.Status = TestStatusError
		result.Error = fmt.Sprintf("Failed to parse workflow: %v", err)
		return result
	}

	// Determine fixtures directory
	fixturesDir := test.Fixtures
	if fixturesDir == "" && opts.Fixtures != "" {
		fixturesDir = opts.Fixtures
	}

	// For Phase 1, we'll mark as a placeholder implementation
	// Full workflow execution integration will be added in later phases
	// For now, we validate that the test file is correctly structured
	// and the workflow is loadable

	result.StepCount = len(workflowDef.Steps)

	// Validate that all assertion step IDs exist in the workflow
	stepIDs := make(map[string]bool)
	for _, step := range workflowDef.Steps {
		stepIDs[step.ID] = true
	}

	evaluator := assert.New()

	for stepID, expression := range test.Assert {
		if !stepIDs[stepID] {
			result.Failures = append(result.Failures, AssertionFailure{
				StepID:   stepID,
				Message:  expression,
				Expected: "step to exist in workflow",
				Actual:   "step not found",
			})
			continue
		}

		// Validate that the assertion expression compiles
		// Create a dummy context for validation
		dummyCtx := map[string]interface{}{
			"status": "ok",
		}
		assertResult := evaluator.Evaluate(expression, dummyCtx)
		if assertResult.Error != nil {
			result.Failures = append(result.Failures, AssertionFailure{
				StepID:   stepID,
				Message:  expression,
				Expected: "valid assertion expression",
				Actual:   fmt.Sprintf("compilation error: %v", assertResult.Error),
			})
		}
	}

	// Update status based on failures
	if len(result.Failures) > 0 {
		result.Status = TestStatusFail
	}

	// Mark as placeholder for now - full execution will be Phase 2
	if opts.Verbose && result.Status == TestStatusPass {
		result.Error = fmt.Sprintf("(placeholder) Workflow validated with %d steps and %d assertions",
			result.StepCount, len(test.Assert))
	}

	return result
}

// displayDiscovery shows discovered tests without running them
func displayDiscovery(tests []TestFile, opts RunOptions) error {
	if opts.JSON {
		// JSON output for --dry-run
		type discoveryOutput struct {
			Tests []struct {
				Name     string `json:"name"`
				File     string `json:"file"`
				Workflow string `json:"workflow"`
			} `json:"tests"`
			Total int `json:"total"`
		}

		output := discoveryOutput{
			Tests: make([]struct {
				Name     string `json:"name"`
				File     string `json:"file"`
				Workflow string `json:"workflow"`
			}, len(tests)),
			Total: len(tests),
		}

		for i, test := range tests {
			output.Tests[i].Name = test.Name
			output.Tests[i].File = test.Path
			output.Tests[i].Workflow = test.Workflow
		}

		return printJSON(output)
	}

	// Human-readable output
	fmt.Printf("Discovered %d test(s):\n\n", len(tests))
	for _, test := range tests {
		fmt.Printf("  %s\n", test.Name)
		fmt.Printf("    File:     %s\n", test.Path)
		fmt.Printf("    Workflow: %s\n", test.Workflow)
		if test.Fixtures != "" {
			fmt.Printf("    Fixtures: %s\n", test.Fixtures)
		}
		fmt.Println()
	}

	return nil
}
