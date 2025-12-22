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

package shared

import (
	"os"

	"golang.org/x/term"
)

// IsNonInteractive detects if the current execution context is non-interactive.
// This function checks multiple indicators in priority order:
//
// 1. --non-interactive flag (checked by caller before calling this function)
// 2. CONDUCTOR_NON_INTERACTIVE=true environment variable
// 3. CI environment detection (CI, GITHUB_ACTIONS, GITLAB_CI, CIRCLECI, JENKINS_HOME)
// 4. stdin is not a TTY (lowest priority)
//
// Returns true if any non-interactive indicator is detected.
func IsNonInteractive() bool {
	// Priority 1: Explicit environment variable
	if os.Getenv("CONDUCTOR_NON_INTERACTIVE") == "true" {
		return true
	}

	// Priority 2: CI environment detection
	if isCIEnvironment() {
		return true
	}

	// Priority 3: stdin is not a TTY
	if !isTerminal() {
		return true
	}

	return false
}

// isCIEnvironment checks for common CI environment variables.
// Returns true if any CI indicator is detected.
func isCIEnvironment() bool {
	ciVars := []string{
		"CI",             // Generic CI indicator
		"GITHUB_ACTIONS", // GitHub Actions
		"GITLAB_CI",      // GitLab CI
		"CIRCLECI",       // CircleCI
		"JENKINS_HOME",   // Jenkins
	}

	for _, envVar := range ciVars {
		value := os.Getenv(envVar)
		if value == "true" || value == "1" {
			return true
		}
		// JENKINS_HOME is set to a path, just check if it exists
		if envVar == "JENKINS_HOME" && value != "" {
			return true
		}
	}

	return false
}

// isTerminal checks if stdin is connected to a terminal.
// Returns true if stdin is a TTY, false otherwise.
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
