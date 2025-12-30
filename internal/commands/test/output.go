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
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

// outputResults writes test results in the specified format
func outputResults(summary TestSummary, opts RunOptions) error {
	var output io.Writer = os.Stdout
	if opts.OutputFile != "" {
		f, err := os.Create(opts.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		output = f
	}

	switch opts.OutputMode {
	case "junit":
		return outputJUnit(summary, output)
	case "human":
		return outputHuman(summary, output, opts.Verbose)
	default:
		return fmt.Errorf("unknown output mode: %s", opts.OutputMode)
	}
}

// outputHuman writes human-readable test results
func outputHuman(summary TestSummary, w io.Writer, verbose bool) error {
	// Print individual test results
	for _, result := range summary.Results {
		status := "PASS"
		if result.Status == TestStatusFail {
			status = "FAIL"
		} else if result.Status == TestStatusError {
			status = "ERROR"
		} else if result.Status == TestStatusSkipped {
			status = "SKIP"
		}

		fmt.Fprintf(w, "[%s] %s (%s)\n", status, result.Name, result.Duration)

		if verbose && result.Status == TestStatusPass {
			fmt.Fprintf(w, "  Steps: %d\n", result.StepCount)
		}

		if result.Status == TestStatusError {
			fmt.Fprintf(w, "  Error: %s\n", result.Error)
		}

		if len(result.Failures) > 0 {
			fmt.Fprintf(w, "  Failures:\n")
			for _, failure := range result.Failures {
				stepLabel := failure.StepID
				if failure.StepName != "" {
					stepLabel = fmt.Sprintf("%s (%s)", failure.StepName, failure.StepID)
				}

				fmt.Fprintf(w, "    Step %s:\n", stepLabel)
				fmt.Fprintf(w, "      Assertion: %s\n", failure.Message)
				fmt.Fprintf(w, "      Expected:  %v\n", formatValue(failure.Expected))
				fmt.Fprintf(w, "      Actual:    %v\n", formatValue(failure.Actual))
			}
		}

		fmt.Fprintln(w)
	}

	// Print summary
	fmt.Fprintf(w, "Test Summary:\n")
	fmt.Fprintf(w, "  Total:   %d\n", summary.Total)
	fmt.Fprintf(w, "  Passed:  %d\n", summary.Passed)
	fmt.Fprintf(w, "  Failed:  %d\n", summary.Failed)
	if summary.Errors > 0 {
		fmt.Fprintf(w, "  Errors:  %d\n", summary.Errors)
	}
	if summary.Skipped > 0 {
		fmt.Fprintf(w, "  Skipped: %d\n", summary.Skipped)
	}
	fmt.Fprintf(w, "  Duration: %s\n", summary.Duration)

	return nil
}

// outputJUnit writes test results in JUnit XML format
func outputJUnit(summary TestSummary, w io.Writer) error {
	// JUnit XML schema types
	type JUnitProperty struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	}

	type JUnitFailure struct {
		Message string `xml:"message,attr"`
		Type    string `xml:"type,attr"`
		Content string `xml:",chardata"`
	}

	type JUnitTestCase struct {
		Name      string          `xml:"name,attr"`
		Classname string          `xml:"classname,attr"`
		Time      string          `xml:"time,attr"`
		Failure   *JUnitFailure   `xml:"failure,omitempty"`
		Error     *JUnitFailure   `xml:"error,omitempty"`
		Skipped   *struct{}       `xml:"skipped,omitempty"`
	}

	type JUnitTestSuite struct {
		XMLName    xml.Name        `xml:"testsuite"`
		Name       string          `xml:"name,attr"`
		Tests      int             `xml:"tests,attr"`
		Failures   int             `xml:"failures,attr"`
		Errors     int             `xml:"errors,attr"`
		Skipped    int             `xml:"skipped,attr"`
		Time       string          `xml:"time,attr"`
		Properties []JUnitProperty `xml:"properties>property,omitempty"`
		TestCases  []JUnitTestCase `xml:"testcase"`
	}

	suite := JUnitTestSuite{
		Name:      "conductor-tests",
		Tests:     summary.Total,
		Failures:  summary.Failed,
		Errors:    summary.Errors,
		Skipped:   summary.Skipped,
		Time:      fmt.Sprintf("%.3f", summary.Duration.Seconds()),
		TestCases: make([]JUnitTestCase, len(summary.Results)),
	}

	for i, result := range summary.Results {
		testCase := JUnitTestCase{
			Name:      result.Name,
			Classname: result.File,
			Time:      fmt.Sprintf("%.3f", result.Duration.Seconds()),
		}

		if result.Status == TestStatusFail && len(result.Failures) > 0 {
			// Combine all failures into one message
			var failureMessages []string
			for _, f := range result.Failures {
				msg := fmt.Sprintf("Step %s: %s\nExpected: %v\nActual: %v",
					f.StepID, f.Message, formatValue(f.Expected), formatValue(f.Actual))
				failureMessages = append(failureMessages, msg)
			}

			testCase.Failure = &JUnitFailure{
				Message: fmt.Sprintf("%d assertion(s) failed", len(result.Failures)),
				Type:    "AssertionError",
				Content: strings.Join(failureMessages, "\n\n"),
			}
		} else if result.Status == TestStatusError {
			testCase.Error = &JUnitFailure{
				Message: "Test execution error",
				Type:    "ExecutionError",
				Content: result.Error,
			}
		} else if result.Status == TestStatusSkipped {
			testCase.Skipped = &struct{}{}
		}

		suite.TestCases[i] = testCase
	}

	// Write XML with header
	output, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JUnit XML: %w", err)
	}

	fmt.Fprintf(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	_, err = w.Write(output)
	if err != nil {
		return fmt.Errorf("failed to write JUnit XML: %w", err)
	}
	fmt.Fprintln(w) // Add trailing newline

	return nil
}

// formatValue formats a value for display
func formatValue(v interface{}) string {
	if v == nil {
		return "<nil>"
	}

	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// printJSON writes a value as JSON to stdout
func printJSON(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
