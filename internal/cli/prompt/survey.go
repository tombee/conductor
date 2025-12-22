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
	"strconv"

	"github.com/AlecAivazis/survey/v2"
)

// SurveyPrompter implements Prompter using the survey library.
// It provides interactive terminal prompts with validation and retry logic.
type SurveyPrompter struct {
	interactive bool
}

// NewSurveyPrompter creates a new survey-based prompter.
func NewSurveyPrompter(interactive bool) *SurveyPrompter {
	return &SurveyPrompter{
		interactive: interactive,
	}
}

// PromptString collects a string input using survey.Input.
func (sp *SurveyPrompter) PromptString(ctx context.Context, name, desc string, def string) (string, error) {
	if !sp.interactive {
		return "", fmt.Errorf("cannot prompt in non-interactive mode")
	}

	var result string
	prompt := &survey.Input{
		Message: fmt.Sprintf("%s: %s", name, desc),
		Default: def,
	}

	err := survey.AskOne(prompt, &result, survey.WithValidator(func(ans interface{}) error {
		if str, ok := ans.(string); ok {
			return ValidateString(str)
		}
		return nil
	}))

	return result, err
}

// PromptNumber collects a numeric input using survey.Input with validation.
func (sp *SurveyPrompter) PromptNumber(ctx context.Context, name, desc string, def float64) (float64, error) {
	if !sp.interactive {
		return 0, fmt.Errorf("cannot prompt in non-interactive mode")
	}

	var input string
	defaultStr := ""
	if def != 0 {
		defaultStr = strconv.FormatFloat(def, 'f', -1, 64)
	}

	prompt := &survey.Input{
		Message: fmt.Sprintf("%s: %s", name, desc),
		Default: defaultStr,
	}

	err := survey.AskOne(prompt, &input, survey.WithValidator(func(ans interface{}) error {
		if str, ok := ans.(string); ok {
			_, err := ValidateNumber(str)
			return err
		}
		return nil
	}))

	if err != nil {
		return 0, err
	}

	return ValidateNumber(input)
}

// PromptBool collects a boolean input using survey.Confirm.
func (sp *SurveyPrompter) PromptBool(ctx context.Context, name, desc string, def bool) (bool, error) {
	if !sp.interactive {
		return false, fmt.Errorf("cannot prompt in non-interactive mode")
	}

	var result bool
	prompt := &survey.Confirm{
		Message: fmt.Sprintf("%s: %s", name, desc),
		Default: def,
	}

	err := survey.AskOne(prompt, &result)
	return result, err
}

// PromptEnum collects an enum selection using survey.Select.
func (sp *SurveyPrompter) PromptEnum(ctx context.Context, name, desc string, options []string, def string) (string, error) {
	if !sp.interactive {
		return "", fmt.Errorf("cannot prompt in non-interactive mode")
	}

	if len(options) == 0 {
		return "", fmt.Errorf("no options provided for enum input")
	}

	var result string
	prompt := &survey.Select{
		Message: fmt.Sprintf("%s: %s", name, desc),
		Options: options,
		Default: def,
	}

	err := survey.AskOne(prompt, &result)
	return result, err
}

// PromptArray collects an array input using survey.Input with parsing.
func (sp *SurveyPrompter) PromptArray(ctx context.Context, name, desc string) ([]interface{}, error) {
	if !sp.interactive {
		return nil, fmt.Errorf("cannot prompt in non-interactive mode")
	}

	var input string
	prompt := &survey.Input{
		Message: fmt.Sprintf("%s: %s (comma-separated or JSON array)", name, desc),
	}

	err := survey.AskOne(prompt, &input, survey.WithValidator(func(ans interface{}) error {
		if str, ok := ans.(string); ok {
			_, err := ValidateArray(str)
			return err
		}
		return nil
	}))

	if err != nil {
		return nil, err
	}

	return ValidateArray(input)
}

// PromptObject collects an object input using survey.Input with JSON validation.
func (sp *SurveyPrompter) PromptObject(ctx context.Context, name, desc string) (map[string]interface{}, error) {
	if !sp.interactive {
		return nil, fmt.Errorf("cannot prompt in non-interactive mode")
	}

	var input string
	prompt := &survey.Input{
		Message: fmt.Sprintf("%s: %s (JSON object)", name, desc),
	}

	err := survey.AskOne(prompt, &input, survey.WithValidator(func(ans interface{}) error {
		if str, ok := ans.(string); ok {
			_, err := ValidateObject(str)
			return err
		}
		return nil
	}))

	if err != nil {
		return nil, err
	}

	return ValidateObject(input)
}

// IsInteractive returns whether the prompter can display interactive prompts.
func (sp *SurveyPrompter) IsInteractive() bool {
	return sp.interactive
}
