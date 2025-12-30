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

import "time"

// RunOptions contains options for running tests
type RunOptions struct {
	Paths      []string // Paths to search for test files
	Fixtures   string   // Path to fixtures directory
	OutputMode string   // Output format: human, junit
	OutputFile string   // File to write output to (empty = stdout)
	Verbose    bool     // Show detailed execution information
	DryRun     bool     // Discover tests without running them
	JSON       bool     // Output in JSON format
}

// TestFile represents a parsed test file
type TestFile struct {
	Path     string                 // Path to the test file
	Workflow string                 // Path to the workflow being tested
	Name     string                 // Test name (optional, defaults to filename)
	Fixtures string                 // Fixtures directory override
	Inputs   map[string]interface{} // Input values for the workflow
	Assert   map[string]string      // Step-level assertions (stepID -> expression)
}

// TestResult represents the outcome of a single test
type TestResult struct {
	Name      string              // Test name
	File      string              // Test file path
	Status    TestStatus          // Pass, Fail, Error
	Duration  time.Duration       // Execution time
	Failures  []AssertionFailure  // Failed assertions
	Error     string              // Error message if status is Error
	StepCount int                 // Number of steps executed
}

// TestStatus represents the outcome of a test
type TestStatus string

const (
	// TestStatusPass indicates all assertions passed
	TestStatusPass TestStatus = "pass"
	// TestStatusFail indicates one or more assertions failed
	TestStatusFail TestStatus = "fail"
	// TestStatusError indicates an error occurred during execution
	TestStatusError TestStatus = "error"
	// TestStatusSkipped indicates the test was skipped
	TestStatusSkipped TestStatus = "skipped"
)

// AssertionFailure represents a failed assertion
type AssertionFailure struct {
	StepID   string      // Step where assertion failed
	StepName string      // Human-readable step name
	Message  string      // Assertion expression that failed
	Expected interface{} // Expected value
	Actual   interface{} // Actual value
}

// TestSummary aggregates results across all tests
type TestSummary struct {
	Total    int           // Total tests discovered
	Passed   int           // Tests that passed
	Failed   int           // Tests that failed
	Errors   int           // Tests with errors
	Skipped  int           // Tests skipped
	Duration time.Duration // Total execution time
	Results  []TestResult  // Individual test results
}
