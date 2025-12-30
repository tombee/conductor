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

package fixture

// Fixture represents a loaded fixture response.
type Fixture struct {
	// Response is the fixture response data
	Response interface{}

	// Source indicates where the fixture was loaded from
	Source string
}

// LLMFixture represents fixture data for LLM steps.
type LLMFixture struct {
	// Responses contains conditional and default responses
	Responses []LLMResponse `yaml:"responses" json:"responses"`

	// Response is used for simple step-specific fixtures
	Response string `yaml:"response,omitempty" json:"response,omitempty"`
}

// LLMResponse represents a single LLM response with optional conditions.
type LLMResponse struct {
	// When specifies the conditions for this response
	When *LLMCondition `yaml:"when,omitempty" json:"when,omitempty"`

	// Return is the response text when conditions match
	Return string `yaml:"return" json:"return"`

	// Default indicates this is the fallback response
	Default bool `yaml:"default,omitempty" json:"default,omitempty"`
}

// LLMCondition specifies when an LLM response should be used.
type LLMCondition struct {
	// PromptContains checks if the prompt contains this string
	PromptContains string `yaml:"prompt_contains,omitempty" json:"prompt_contains,omitempty"`

	// StepID matches against the step ID
	StepID string `yaml:"step_id,omitempty" json:"step_id,omitempty"`
}

// HTTPFixture represents fixture data for HTTP actions.
type HTTPFixture struct {
	// Responses contains conditional and default responses
	Responses []HTTPResponse `yaml:"responses" json:"responses"`

	// Response is used for simple step-specific fixtures
	Response *HTTPResponseData `yaml:"response,omitempty" json:"response,omitempty"`
}

// HTTPResponse represents a single HTTP response with optional conditions.
type HTTPResponse struct {
	// When specifies the conditions for this response
	When *HTTPCondition `yaml:"when,omitempty" json:"when,omitempty"`

	// Return is the response data when conditions match
	Return *HTTPResponseData `yaml:"return" json:"return"`

	// Default indicates this is the fallback response
	Default bool `yaml:"default,omitempty" json:"default,omitempty"`
}

// HTTPCondition specifies when an HTTP response should be used.
type HTTPCondition struct {
	// URL pattern to match (supports wildcards)
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// Method to match (GET, POST, etc.)
	Method string `yaml:"method,omitempty" json:"method,omitempty"`
}

// HTTPResponseData represents HTTP response data.
type HTTPResponseData struct {
	// Status code
	Status int `yaml:"status" json:"status"`

	// Body content
	Body interface{} `yaml:"body" json:"body"`

	// Headers
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// IntegrationFixture represents fixture data for integration steps.
type IntegrationFixture struct {
	// Responses contains conditional and default responses
	Responses []IntegrationResponse `yaml:"responses" json:"responses"`

	// Response is used for simple step-specific fixtures
	Response interface{} `yaml:"response,omitempty" json:"response,omitempty"`
}

// IntegrationResponse represents a single integration response with optional conditions.
type IntegrationResponse struct {
	// When specifies the conditions for this response
	When *IntegrationCondition `yaml:"when,omitempty" json:"when,omitempty"`

	// Return is the response data when conditions match
	Return interface{} `yaml:"return" json:"return"`

	// Default indicates this is the fallback response
	Default bool `yaml:"default,omitempty" json:"default,omitempty"`
}

// IntegrationCondition specifies when an integration response should be used.
type IntegrationCondition struct {
	// Operation name to match
	Operation string `yaml:"operation,omitempty" json:"operation,omitempty"`

	// Repo pattern for GitHub operations (supports wildcards)
	Repo string `yaml:"repo,omitempty" json:"repo,omitempty"`
}
