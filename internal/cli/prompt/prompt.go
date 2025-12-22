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

// Package prompt provides interactive input collection for workflows.
// It supports type-aware prompting with validation, retry logic, and
// non-interactive mode for CI/CD environments.
package prompt

import (
	"context"
	"fmt"
)

// Prompter defines the interface for interactive input collection.
// Implementations include SurveyPrompter (production) and MockPrompter (testing).
type Prompter interface {
	// PromptString collects a string input from the user
	PromptString(ctx context.Context, name, desc string, def string) (string, error)

	// PromptNumber collects a numeric input from the user
	PromptNumber(ctx context.Context, name, desc string, def float64) (float64, error)

	// PromptBool collects a boolean input from the user
	PromptBool(ctx context.Context, name, desc string, def bool) (bool, error)

	// PromptEnum presents a list of options and collects the user's selection
	PromptEnum(ctx context.Context, name, desc string, options []string, def string) (string, error)

	// PromptArray collects an array input from the user (comma-separated or JSON)
	PromptArray(ctx context.Context, name, desc string) ([]interface{}, error)

	// PromptObject collects an object input from the user (JSON)
	PromptObject(ctx context.Context, name, desc string) (map[string]interface{}, error)

	// IsInteractive returns true if prompts can be displayed
	IsInteractive() bool
}

// InputCollector manages a prompt session for collecting multiple inputs.
type InputCollector struct {
	prompter Prompter
	progress ProgressTracker
}

// ProgressTracker tracks progress through a multi-input prompt session.
type ProgressTracker struct {
	current int
	total   int
}

// NewInputCollector creates a new input collector with the given prompter.
func NewInputCollector(p Prompter) *InputCollector {
	return &InputCollector{
		prompter: p,
	}
}

// SetProgress configures the progress tracker for multi-input sessions.
func (ic *InputCollector) SetProgress(current, total int) {
	ic.progress = ProgressTracker{
		current: current,
		total:   total,
	}
}

// GetProgress returns the current progress information.
func (ic *InputCollector) GetProgress() (current, total int) {
	return ic.progress.current, ic.progress.total
}

// FormatProgressPrefix returns a formatted progress indicator string.
func (ic *InputCollector) FormatProgressPrefix() string {
	if ic.progress.total > 0 {
		return fmt.Sprintf("[Input %d of %d] ", ic.progress.current, ic.progress.total)
	}
	return ""
}

// CollectInput prompts for a single input with retry logic.
// Returns the collected value or an error after MaxRetries attempts.
func (ic *InputCollector) CollectInput(ctx context.Context, config PromptConfig) (interface{}, error) {
	var attempts int
	var lastErr error

	for attempts < MaxRetries {
		attempts++

		var value interface{}
		var err error

		switch config.Type {
		case InputTypeString:
			defStr := ""
			if config.Default != nil {
				defStr = fmt.Sprintf("%v", config.Default)
			}
			value, err = ic.prompter.PromptString(ctx, config.Name, config.Description, defStr)

		case InputTypeNumber:
			defNum := 0.0
			if config.Default != nil {
				if num, ok := config.Default.(float64); ok {
					defNum = num
				}
			}
			value, err = ic.prompter.PromptNumber(ctx, config.Name, config.Description, defNum)

		case InputTypeBoolean:
			defBool := false
			if config.Default != nil {
				if b, ok := config.Default.(bool); ok {
					defBool = b
				}
			}
			value, err = ic.prompter.PromptBool(ctx, config.Name, config.Description, defBool)

		case InputTypeEnum:
			defStr := ""
			if config.Default != nil {
				defStr = fmt.Sprintf("%v", config.Default)
			}
			value, err = ic.prompter.PromptEnum(ctx, config.Name, config.Description, config.Options, defStr)

		case InputTypeArray:
			value, err = ic.prompter.PromptArray(ctx, config.Name, config.Description)

		case InputTypeObject:
			value, err = ic.prompter.PromptObject(ctx, config.Name, config.Description)

		default:
			return nil, fmt.Errorf("unsupported input type: %s", config.Type)
		}

		if err == nil {
			return value, nil
		}

		lastErr = err

		// Show validation error without revealing user input
		if attempts < MaxRetries {
			fmt.Printf("Error: %s must be a %s (received invalid value)\n", config.Name, config.Type)
		}
	}

	return nil, fmt.Errorf("failed to collect input %s after %d attempts: %w", config.Name, MaxRetries, lastErr)
}

// CollectInputs prompts for multiple inputs in sequence.
func (ic *InputCollector) CollectInputs(ctx context.Context, configs []PromptConfig) (map[string]interface{}, error) {
	results := make(map[string]interface{})

	ic.progress.total = len(configs)

	for i, config := range configs {
		ic.progress.current = i + 1

		value, err := ic.CollectInput(ctx, config)
		if err != nil {
			return nil, err
		}

		results[config.Name] = value
	}

	return results, nil
}
