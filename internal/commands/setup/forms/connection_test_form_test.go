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

package forms

import (
	"os"
	"testing"
	"time"
)

func TestGetTestTimeout(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected time.Duration
	}{
		{
			name:     "default_timeout",
			envValue: "",
			expected: 10 * time.Second,
		},
		{
			name:     "custom_timeout",
			envValue: "15",
			expected: 15 * time.Second,
		},
		{
			name:     "invalid_timeout_uses_default",
			envValue: "invalid",
			expected: 10 * time.Second,
		},
		{
			name:     "zero_timeout_uses_default",
			envValue: "0",
			expected: 10 * time.Second,
		},
		{
			name:     "negative_timeout_uses_default",
			envValue: "-5",
			expected: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env value
			originalValue := os.Getenv("CONDUCTOR_SETUP_TIMEOUT")
			defer os.Setenv("CONDUCTOR_SETUP_TIMEOUT", originalValue)

			// Set test env value
			if tt.envValue != "" {
				os.Setenv("CONDUCTOR_SETUP_TIMEOUT", tt.envValue)
			} else {
				os.Unsetenv("CONDUCTOR_SETUP_TIMEOUT")
			}

			// Test
			result := getTestTimeout()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAddVerifiedIndicatorToCLIProvider(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		wantContains string
	}{
		{
			name:         "adds_verified_indicator",
			providerName: "ollama",
			wantContains: "ollama",
		},
		{
			name:         "adds_to_claude_code",
			providerName: "Claude Code",
			wantContains: "Claude Code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddVerifiedIndicatorToCLIProvider(tt.providerName)
			if result == "" {
				t.Error("expected non-empty result")
			}
			if result == tt.providerName {
				t.Error("expected result to be different from input (should have indicator added)")
			}
			// Result should contain the original name
			// We can't test for exact string due to styling, but we can verify it's not empty
		})
	}
}

func TestTestActionConstants(t *testing.T) {
	// Verify the test action constants are defined correctly
	actions := []TestAction{
		TestActionContinue,
		TestActionRetry,
		TestActionSkip,
		TestActionEdit,
		TestActionCancel,
	}

	for _, action := range actions {
		if string(action) == "" {
			t.Errorf("test action should not be empty")
		}
	}

	// Verify they're all unique
	seen := make(map[TestAction]bool)
	for _, action := range actions {
		if seen[action] {
			t.Errorf("duplicate test action: %s", action)
		}
		seen[action] = true
	}
}
