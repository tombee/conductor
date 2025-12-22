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
	"encoding/json"
	"os"
)

// JSONResponse is the base envelope for all JSON output
type JSONResponse struct {
	Version string `json:"@version"`
	Command string `json:"command"`
	Success bool   `json:"success"`
}

// JSONError represents a structured error with code, message, location, and suggestion
type JSONError struct {
	Code       string        `json:"code"`
	Message    string        `json:"message"`
	Location   *JSONLocation `json:"location,omitempty"`
	Suggestion string        `json:"suggestion,omitempty"`
	StepID     string        `json:"step_id,omitempty"`
}

// JSONLocation represents a position in a file
type JSONLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// emitJSON marshals a response to JSON and outputs it to stdout
// This ensures consistent formatting and error handling across all commands
func emitJSON(response interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}

// EmitJSON is the exported version of emitJSON for use by command packages
func EmitJSON(response interface{}) error {
	return emitJSON(response)
}

// emitJSONError creates and emits a JSON error response
func emitJSONError(command string, errors []JSONError) error {
	type errorResponse struct {
		JSONResponse
		Errors []JSONError `json:"errors"`
	}

	resp := errorResponse{
		JSONResponse: JSONResponse{
			Version: "1.0",
			Command: command,
			Success: false,
		},
		Errors: errors,
	}

	return emitJSON(resp)
}

// EmitJSONError is the exported version of emitJSONError for use by command packages
func EmitJSONError(command string, errors []JSONError) error {
	return emitJSONError(command, errors)
}
