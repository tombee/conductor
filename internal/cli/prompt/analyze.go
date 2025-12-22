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
	"github.com/tombee/conductor/pkg/workflow"
)

// InputAnalyzer analyzes workflow inputs and identifies gaps.
type InputAnalyzer struct {
	workflowInputs []workflow.InputDefinition
	providedInputs map[string]interface{}
}

// NewInputAnalyzer creates a new input analyzer.
func NewInputAnalyzer(workflowInputs []workflow.InputDefinition, providedInputs map[string]interface{}) *InputAnalyzer {
	return &InputAnalyzer{
		workflowInputs: workflowInputs,
		providedInputs: providedInputs,
	}
}

// MissingInput represents a workflow input that needs to be collected.
type MissingInput struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Default     interface{}
	Enum        []string
}

// FindMissingInputs identifies required inputs that haven't been provided.
// It applies defaults and skips optional inputs with defaults.
func (ia *InputAnalyzer) FindMissingInputs() []MissingInput {
	missing := make([]MissingInput, 0)

	for _, input := range ia.workflowInputs {
		// Check if input was provided
		if _, exists := ia.providedInputs[input.Name]; exists {
			continue
		}

		// If not required and has a default, skip it (will be applied later)
		if !input.Required && input.Default != nil {
			continue
		}

		// If required or no default, add to missing list
		if input.Required {
			missing = append(missing, MissingInput{
				Name:        input.Name,
				Type:        input.Type,
				Description: input.Description,
				Required:    input.Required,
				Default:     input.Default,
				Enum:        input.Enum,
			})
		}
	}

	return missing
}

// ApplyDefaults applies default values to the provided inputs map.
func (ia *InputAnalyzer) ApplyDefaults() map[string]interface{} {
	result := make(map[string]interface{})

	// Copy provided inputs
	for k, v := range ia.providedInputs {
		result[k] = v
	}

	// Apply defaults for missing inputs
	for _, input := range ia.workflowInputs {
		if _, exists := result[input.Name]; !exists && input.Default != nil {
			result[input.Name] = input.Default
		}
	}

	return result
}
