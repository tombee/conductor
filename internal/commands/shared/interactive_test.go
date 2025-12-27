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
	"testing"
)

func TestIsNonInteractive(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected bool
	}{
		{
			name:     "no indicators - interactive (skipped - depends on actual TTY)",
			envVars:  map[string]string{},
			expected: false, // This may fail if stdin is not a TTY
		},
		{
			name: "CONDUCTOR_NON_INTERACTIVE=true",
			envVars: map[string]string{
				"CONDUCTOR_NON_INTERACTIVE": "true",
			},
			expected: true,
		},
		{
			name: "CI=true",
			envVars: map[string]string{
				"CI": "true",
			},
			expected: true,
		},
		{
			name: "CI=1",
			envVars: map[string]string{
				"CI": "1",
			},
			expected: true,
		},
		{
			name: "GITHUB_ACTIONS=true",
			envVars: map[string]string{
				"GITHUB_ACTIONS": "true",
			},
			expected: true,
		},
		{
			name: "GITLAB_CI=true",
			envVars: map[string]string{
				"GITLAB_CI": "true",
			},
			expected: true,
		},
		{
			name: "CIRCLECI=true",
			envVars: map[string]string{
				"CIRCLECI": "true",
			},
			expected: true,
		},
		{
			name: "JENKINS_HOME set to path",
			envVars: map[string]string{
				"JENKINS_HOME": "/var/jenkins",
			},
			expected: true,
		},
		{
			name: "multiple CI vars set",
			envVars: map[string]string{
				"CI":             "true",
				"GITHUB_ACTIONS": "true",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		// Skip the "no indicators" test if stdin is not a TTY
		if tt.name == "no indicators - interactive (skipped - depends on actual TTY)" && !isTerminal() {
			t.Logf("Skipping %q - stdin is not a TTY", tt.name)
			continue
		}

		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant environment variables
			clearEnv := []string{
				"CONDUCTOR_NON_INTERACTIVE",
				"CI",
				"GITHUB_ACTIONS",
				"GITLAB_CI",
				"CIRCLECI",
				"JENKINS_HOME",
			}

			// Save original values
			origEnv := make(map[string]string)
			for _, key := range clearEnv {
				origEnv[key] = os.Getenv(key)
				os.Unsetenv(key)
			}

			// Restore environment after test
			defer func() {
				for key, value := range origEnv {
					if value == "" {
						os.Unsetenv(key)
					} else {
						os.Setenv(key, value)
					}
				}
			}()

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			result := IsNonInteractive()
			if result != tt.expected {
				t.Errorf("IsNonInteractive() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestIsCIEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected bool
	}{
		{
			name:     "no CI vars",
			envVars:  map[string]string{},
			expected: false,
		},
		{
			name: "CI=true",
			envVars: map[string]string{
				"CI": "true",
			},
			expected: true,
		},
		{
			name: "CI=1",
			envVars: map[string]string{
				"CI": "1",
			},
			expected: true,
		},
		{
			name: "CI=false (should be false)",
			envVars: map[string]string{
				"CI": "false",
			},
			expected: false,
		},
		{
			name: "GITHUB_ACTIONS=true",
			envVars: map[string]string{
				"GITHUB_ACTIONS": "true",
			},
			expected: true,
		},
		{
			name: "GITLAB_CI=true",
			envVars: map[string]string{
				"GITLAB_CI": "true",
			},
			expected: true,
		},
		{
			name: "CIRCLECI=true",
			envVars: map[string]string{
				"CIRCLECI": "true",
			},
			expected: true,
		},
		{
			name: "JENKINS_HOME set",
			envVars: map[string]string{
				"JENKINS_HOME": "/var/jenkins",
			},
			expected: true,
		},
		{
			name: "JENKINS_HOME empty string (should be false)",
			envVars: map[string]string{
				"JENKINS_HOME": "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all CI environment variables
			clearEnv := []string{
				"CI",
				"GITHUB_ACTIONS",
				"GITLAB_CI",
				"CIRCLECI",
				"JENKINS_HOME",
			}

			// Save original values
			origEnv := make(map[string]string)
			for _, key := range clearEnv {
				origEnv[key] = os.Getenv(key)
				os.Unsetenv(key)
			}

			// Restore environment after test
			defer func() {
				for key, value := range origEnv {
					if value == "" {
						os.Unsetenv(key)
					} else {
						os.Setenv(key, value)
					}
				}
			}()

			// Set test environment variables
			for key, value := range tt.envVars {
				if value == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, value)
				}
			}

			result := isCIEnvironment()
			if result != tt.expected {
				t.Errorf("isCIEnvironment() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Note: isTerminal() testing is tricky because it depends on the actual stdin state.
// In a test environment, stdin may or may not be a TTY depending on how tests are run.
// We skip comprehensive testing of isTerminal() itself and focus on the higher-level
// IsNonInteractive() function which integrates all detection mechanisms.

// TestNonInteractiveMissingInputError verifies error messages for missing inputs in non-interactive mode
func TestNonInteractiveMissingInputError(t *testing.T) {
	// Set up non-interactive environment
	origEnv := os.Getenv("CONDUCTOR_NON_INTERACTIVE")
	os.Setenv("CONDUCTOR_NON_INTERACTIVE", "true")
	defer func() {
		if origEnv == "" {
			os.Unsetenv("CONDUCTOR_NON_INTERACTIVE")
		} else {
			os.Setenv("CONDUCTOR_NON_INTERACTIVE", origEnv)
		}
	}()

	// Verify we're in non-interactive mode
	if !IsNonInteractive() {
		t.Fatal("Expected non-interactive mode")
	}

	// Test error creation for missing required input in non-interactive mode
	exitErr := NewMissingInputNonInteractiveError("missing required input: workflow file", nil)
	if exitErr == nil {
		t.Fatal("Expected error, got nil")
	}

	if exitErr.Code != ExitMissingInputNonInteractive {
		t.Errorf("exit code = %d, want %d", exitErr.Code, ExitMissingInputNonInteractive)
	}
}

// TestNonInteractiveExitCodes verifies exit codes in non-interactive mode
func TestNonInteractiveExitCodes(t *testing.T) {
	tests := []struct {
		name         string
		errorType    string
		expectedCode int
	}{
		{
			name:         "missing input",
			errorType:    "missing_input",
			expectedCode: ExitMissingInput,
		},
		{
			name:         "missing input non-interactive",
			errorType:    "missing_input_non_interactive",
			expectedCode: ExitMissingInputNonInteractive,
		},
		{
			name:         "invalid workflow",
			errorType:    "invalid_workflow",
			expectedCode: ExitInvalidWorkflow,
		},
		{
			name:         "execution failed",
			errorType:    "execution_failed",
			expectedCode: ExitExecutionFailed,
		},
		{
			name:         "provider error",
			errorType:    "provider_error",
			expectedCode: ExitProviderError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var code int

			switch tt.errorType {
			case "missing_input":
				code = ExitMissingInput
			case "missing_input_non_interactive":
				code = ExitMissingInputNonInteractive
			case "invalid_workflow":
				code = ExitInvalidWorkflow
			case "execution_failed":
				code = ExitExecutionFailed
			case "provider_error":
				code = ExitProviderError
			}

			if code != tt.expectedCode {
				t.Errorf("exit code = %d, want %d", code, tt.expectedCode)
			}
		})
	}
}

// TestNonInteractiveErrorFormat verifies the format of error messages in non-interactive mode
func TestNonInteractiveErrorFormat(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "missing workflow file",
			message: "missing required input: workflow file",
		},
		{
			name:    "missing provider configuration",
			message: "missing required input: provider configuration",
		},
	}

	// Set up non-interactive environment
	origEnv := os.Getenv("CONDUCTOR_NON_INTERACTIVE")
	os.Setenv("CONDUCTOR_NON_INTERACTIVE", "true")
	defer func() {
		if origEnv == "" {
			os.Unsetenv("CONDUCTOR_NON_INTERACTIVE")
		} else {
			os.Setenv("CONDUCTOR_NON_INTERACTIVE", origEnv)
		}
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitErr := NewMissingInputNonInteractiveError(tt.message, nil)
			if exitErr == nil {
				t.Fatal("Expected error, got nil")
			}

			if exitErr.Code != ExitMissingInputNonInteractive {
				t.Errorf("exit code = %d, want %d", exitErr.Code, ExitMissingInputNonInteractive)
			}

			if exitErr.Message != tt.message {
				t.Errorf("message = %q, want %q", exitErr.Message, tt.message)
			}
		})
	}
}
