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

// InputType represents the type of workflow input.
type InputType string

const (
	// InputTypeString represents string inputs
	InputTypeString InputType = "string"

	// InputTypeNumber represents numeric inputs (integers and floats)
	InputTypeNumber InputType = "number"

	// InputTypeBoolean represents boolean inputs
	InputTypeBoolean InputType = "boolean"

	// InputTypeArray represents array inputs
	InputTypeArray InputType = "array"

	// InputTypeObject represents object inputs (JSON)
	InputTypeObject InputType = "object"

	// InputTypeEnum represents enumerated value inputs
	InputTypeEnum InputType = "enum"
)

// PromptConfig holds configuration for a single prompt.
// Only used for missing inputs (which are always required with no default).
type PromptConfig struct {
	Name        string
	Description string
	Type        InputType
	Options     []string // For enum types
}

// ValidationError represents an input validation failure.
type ValidationError struct {
	InputName string
	InputType string
	Reason    string
}

func (e *ValidationError) Error() string {
	return e.Reason
}

// MaxRetries is the maximum number of validation retry attempts per input.
const MaxRetries = 3

// MaxInputSize is the maximum allowed input size in bytes.
const MaxInputSize = 65536

// MaxNestedDepth is the maximum allowed nesting depth for objects.
const MaxNestedDepth = 10
