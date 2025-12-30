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
	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewCommand creates the test command
func NewCommand() *cobra.Command {
	var (
		fixtures   string
		outputMode string
		outputFile string
		verbose    bool
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "test [path...]",
		Short: "Run workflow tests",
		Annotations: map[string]string{
			"group": "testing",
		},
		Long: `Test discovers and executes workflow test files.

Test Discovery:
  Automatically finds test files matching these patterns:
    - *_test.yaml
    - test_*.yaml

  Searches recursively in the specified path(s) or current directory.

Test Execution:
  Tests run in mock mode by default, using fixtures for LLM and API responses.
  No real API calls are made unless explicitly configured.

Output Formats:
  --output human   Human-readable test results (default)
  --output junit   JUnit XML format for CI/CD integration

Exit Codes:
  0  All tests passed
  1  One or more tests failed
  2  Configuration or discovery error

Examples:
  conductor test                        Run all tests in current directory
  conductor test ./tests                Run tests in specific directory
  conductor test workflow_test.yaml     Run a single test file
  conductor test --output junit --output-file results.xml`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to current directory if no paths specified
			paths := args
			if len(paths) == 0 {
				paths = []string{"."}
			}

			opts := RunOptions{
				Paths:      paths,
				Fixtures:   fixtures,
				OutputMode: outputMode,
				OutputFile: outputFile,
				Verbose:    verbose,
				DryRun:     dryRun,
				JSON:       shared.GetJSON(),
			}

			return runTests(opts)
		},
	}

	cmd.Flags().StringVar(&fixtures, "fixtures", "", "Path to fixtures directory")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "human", "Output format: human, junit")
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Write output to file instead of stdout")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed test execution information")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Discover tests without running them")

	return cmd
}
