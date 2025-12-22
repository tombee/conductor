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

package prompt

import (
	"context"
	"fmt"
)

// MockPrompter implements Prompter with scripted responses for testing.
// It allows tests to simulate user input without requiring interactive terminals.
type MockPrompter struct {
	responses     []interface{}
	currentIndex  int
	interactive   bool
	callLog       []string
}

// NewMockPrompter creates a new mock prompter with pre-scripted responses.
func NewMockPrompter(interactive bool, responses ...interface{}) *MockPrompter {
	return &MockPrompter{
		responses:   responses,
		interactive: interactive,
		callLog:     make([]string, 0),
	}
}

// PromptString returns the next string response.
func (mp *MockPrompter) PromptString(ctx context.Context, name, desc string, def string) (string, error) {
	mp.callLog = append(mp.callLog, fmt.Sprintf("PromptString(%s)", name))

	if mp.currentIndex >= len(mp.responses) {
		return def, nil
	}

	resp := mp.responses[mp.currentIndex]
	mp.currentIndex++

	if str, ok := resp.(string); ok {
		return str, nil
	}

	return "", fmt.Errorf("mock response is not a string")
}

// PromptNumber returns the next numeric response.
func (mp *MockPrompter) PromptNumber(ctx context.Context, name, desc string, def float64) (float64, error) {
	mp.callLog = append(mp.callLog, fmt.Sprintf("PromptNumber(%s)", name))

	if mp.currentIndex >= len(mp.responses) {
		return def, nil
	}

	resp := mp.responses[mp.currentIndex]
	mp.currentIndex++

	if num, ok := resp.(float64); ok {
		return num, nil
	}
	if num, ok := resp.(int); ok {
		return float64(num), nil
	}

	return 0, fmt.Errorf("mock response is not a number")
}

// PromptBool returns the next boolean response.
func (mp *MockPrompter) PromptBool(ctx context.Context, name, desc string, def bool) (bool, error) {
	mp.callLog = append(mp.callLog, fmt.Sprintf("PromptBool(%s)", name))

	if mp.currentIndex >= len(mp.responses) {
		return def, nil
	}

	resp := mp.responses[mp.currentIndex]
	mp.currentIndex++

	if b, ok := resp.(bool); ok {
		return b, nil
	}

	return false, fmt.Errorf("mock response is not a boolean")
}

// PromptEnum returns the next enum response.
func (mp *MockPrompter) PromptEnum(ctx context.Context, name, desc string, options []string, def string) (string, error) {
	mp.callLog = append(mp.callLog, fmt.Sprintf("PromptEnum(%s)", name))

	if mp.currentIndex >= len(mp.responses) {
		return def, nil
	}

	resp := mp.responses[mp.currentIndex]
	mp.currentIndex++

	if str, ok := resp.(string); ok {
		return str, nil
	}

	return "", fmt.Errorf("mock response is not a string")
}

// PromptArray returns the next array response.
func (mp *MockPrompter) PromptArray(ctx context.Context, name, desc string) ([]interface{}, error) {
	mp.callLog = append(mp.callLog, fmt.Sprintf("PromptArray(%s)", name))

	if mp.currentIndex >= len(mp.responses) {
		return nil, fmt.Errorf("no mock response available")
	}

	resp := mp.responses[mp.currentIndex]
	mp.currentIndex++

	if arr, ok := resp.([]interface{}); ok {
		return arr, nil
	}

	return nil, fmt.Errorf("mock response is not an array")
}

// PromptObject returns the next object response.
func (mp *MockPrompter) PromptObject(ctx context.Context, name, desc string) (map[string]interface{}, error) {
	mp.callLog = append(mp.callLog, fmt.Sprintf("PromptObject(%s)", name))

	if mp.currentIndex >= len(mp.responses) {
		return nil, fmt.Errorf("no mock response available")
	}

	resp := mp.responses[mp.currentIndex]
	mp.currentIndex++

	if obj, ok := resp.(map[string]interface{}); ok {
		return obj, nil
	}

	return nil, fmt.Errorf("mock response is not an object")
}

// IsInteractive returns the configured interactive state.
func (mp *MockPrompter) IsInteractive() bool {
	return mp.interactive
}

// GetCallLog returns the log of all prompt calls made.
func (mp *MockPrompter) GetCallLog() []string {
	return mp.callLog
}

// Reset clears the call log and resets the response index.
func (mp *MockPrompter) Reset() {
	mp.currentIndex = 0
	mp.callLog = make([]string, 0)
}
