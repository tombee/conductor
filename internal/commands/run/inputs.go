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
	"io"
	"os"
	"strings"

	"github.com/tombee/conductor/internal/cli/prompt"
	"github.com/tombee/conductor/pkg/workflow"
	"golang.org/x/term"
)

// isInteractiveModeAllowed determines if interactive prompts are allowed.
// Interactive mode is disabled if:
// - --no-interactive flag is set
// - CONDUCTOR_NO_INTERACTIVE env var is set
// - Running in a CI environment
// - stdin is not a TTY
func isInteractiveModeAllowed(noInteractive bool) bool {
	// Explicit flag takes precedence
	if noInteractive {
		return false
	}

	// Check CONDUCTOR_NO_INTERACTIVE environment variable
	if envVal := os.Getenv("CONDUCTOR_NO_INTERACTIVE"); envVal != "" {
		switch strings.ToLower(envVal) {
		case "true", "1", "yes":
			return false
		}
	}

	// Check for CI environment variables
	ciEnvVars := []string{
		"CI",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"TRAVIS",
		"BUILDKITE",
		"DRONE",
		"JENKINS_HOME",
		"TEAMCITY_VERSION",
	}
	for _, envVar := range ciEnvVars {
		if os.Getenv(envVar) != "" {
			return false
		}
	}

	// Check if stdin is a TTY
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}

	return true
}

// formatMissingInputsError creates a structured error message for missing inputs.
func formatMissingInputsError(missing []prompt.MissingInput) string {
	var sb strings.Builder
	sb.WriteString("Missing required inputs:\n")
	for _, input := range missing {
		sb.WriteString(fmt.Sprintf("  - %s (%s): %s\n", input.Name, input.Type, input.Description))
		if len(input.Enum) > 0 {
			sb.WriteString(fmt.Sprintf("    Valid values: %s\n", strings.Join(input.Enum, ", ")))
		}
	}
	sb.WriteString("\nRun with --help-inputs to see all workflow inputs.")
	return sb.String()
}

// showWorkflowInputs displays all workflow inputs in a user-friendly format.
func showWorkflowInputs(def *workflow.Definition) {
	if len(def.Inputs) == 0 {
		fmt.Println("This workflow has no defined inputs.")
		return
	}

	fmt.Println("Workflow Inputs:")
	fmt.Println()
	for _, input := range def.Inputs {
		// Inputs without a default are required
		required := "required"
		if input.Default != nil {
			required = "optional"
		}

		fmt.Printf("  %s (%s, %s)\n", input.Name, input.Type, required)
		if input.Description != "" {
			fmt.Printf("    %s\n", input.Description)
		}
		if input.Default != nil {
			fmt.Printf("    Default: %v\n", input.Default)
		}
		if len(input.Enum) > 0 {
			fmt.Printf("    Valid values: %s\n", strings.Join(input.Enum, ", "))
		}
		fmt.Println()
	}
}

// loadInputFile loads inputs from a JSON file or stdin
func loadInputFile(path string) (map[string]interface{}, error) {
	var data []byte
	var err error

	if path == "-" {
		// Check if stdin has data (not a terminal)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return nil, fmt.Errorf("--input-file - requires input on stdin (pipe or redirect)")
		}
		// Read from stdin
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		// Read from file
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read input file: %w", err)
		}
	}

	var inputs map[string]interface{}
	if err := json.Unmarshal(data, &inputs); err != nil {
		return nil, fmt.Errorf("failed to parse JSON input: %w", err)
	}

	return inputs, nil
}

// parseInputs parses input arguments in key=value format and optionally merges with file inputs
func parseInputs(inputArgs []string, inputFile string) (map[string]interface{}, error) {
	// Start with inputs from file (if provided)
	var inputs map[string]interface{}
	if inputFile != "" {
		var err error
		inputs, err = loadInputFile(inputFile)
		if err != nil {
			return nil, err
		}
	} else {
		inputs = make(map[string]interface{})
	}

	// Override with command-line inputs
	for _, arg := range inputArgs {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid input format %q (expected key=value)", arg)
		}
		inputs[parts[0]] = parts[1]
	}

	return inputs, nil
}
