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
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// TestJSONResponseEnvelope verifies the base envelope structure
func TestJSONResponseEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		version string
		command string
		success bool
	}{
		{
			name:    "successful response",
			version: "1.0",
			command: "validate",
			success: true,
		},
		{
			name:    "failed response",
			version: "1.0",
			command: "run",
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := JSONResponse{
				Version: tt.version,
				Command: tt.command,
				Success: tt.success,
			}

			// Verify structure can be marshaled
			data, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("failed to marshal JSONResponse: %v", err)
			}

			// Verify structure can be unmarshaled
			var decoded JSONResponse
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("failed to unmarshal JSONResponse: %v", err)
			}

			// Verify fields
			if decoded.Version != tt.version {
				t.Errorf("version = %q, want %q", decoded.Version, tt.version)
			}
			if decoded.Command != tt.command {
				t.Errorf("command = %q, want %q", decoded.Command, tt.command)
			}
			if decoded.Success != tt.success {
				t.Errorf("success = %v, want %v", decoded.Success, tt.success)
			}

			// Verify @version field is present in JSON
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("failed to unmarshal to map: %v", err)
			}
			if _, ok := raw["@version"]; !ok {
				t.Error("@version field not present in JSON output")
			}
		})
	}
}

// TestJSONErrorStructure verifies error envelope structure
func TestJSONErrorStructure(t *testing.T) {
	tests := []struct {
		name    string
		command string
		errors  []JSONError
	}{
		{
			name:    "single error without location",
			command: "validate",
			errors: []JSONError{
				{
					Code:       "E001",
					Message:    "workflow file not found",
					Suggestion: "Check that the file path is correct",
				},
			},
		},
		{
			name:    "error with location",
			command: "validate",
			errors: []JSONError{
				{
					Code:    "E002",
					Message: "invalid YAML syntax",
					Location: &JSONLocation{
						Line:   10,
						Column: 5,
					},
					Suggestion: "Check for missing quotes or incorrect indentation",
				},
			},
		},
		{
			name:    "multiple errors",
			command: "run",
			errors: []JSONError{
				{
					Code:       "E100",
					Message:    "step failed",
					StepID:     "step-1",
					Suggestion: "Check the step configuration",
				},
				{
					Code:       "E101",
					Message:    "provider not configured",
					Suggestion: "Run 'conductor providers add' to configure a provider",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Emit error
			if err := EmitJSONError(tt.command, tt.errors); err != nil {
				t.Fatalf("EmitJSONError failed: %v", err)
			}

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			io.Copy(&buf, r)

			// Parse JSON
			var response struct {
				JSONResponse
				Errors []JSONError `json:"errors"`
			}
			if err := json.Unmarshal(buf.Bytes(), &response); err != nil {
				t.Fatalf("failed to unmarshal error response: %v", err)
			}

			// Verify envelope
			if response.Version != "1.0" {
				t.Errorf("version = %q, want %q", response.Version, "1.0")
			}
			if response.Command != tt.command {
				t.Errorf("command = %q, want %q", response.Command, tt.command)
			}
			if response.Success != false {
				t.Error("success should be false for error response")
			}

			// Verify errors array
			if len(response.Errors) != len(tt.errors) {
				t.Fatalf("errors count = %d, want %d", len(response.Errors), len(tt.errors))
			}

			for i, err := range response.Errors {
				if err.Code != tt.errors[i].Code {
					t.Errorf("error[%d].code = %q, want %q", i, err.Code, tt.errors[i].Code)
				}
				if err.Message != tt.errors[i].Message {
					t.Errorf("error[%d].message = %q, want %q", i, err.Message, tt.errors[i].Message)
				}
				if err.Suggestion != tt.errors[i].Suggestion {
					t.Errorf("error[%d].suggestion = %q, want %q", i, err.Suggestion, tt.errors[i].Suggestion)
				}
				if err.StepID != tt.errors[i].StepID {
					t.Errorf("error[%d].step_id = %q, want %q", i, err.StepID, tt.errors[i].StepID)
				}
			}
		})
	}
}

// TestBackwardCompatibility ensures the JSON structure doesn't break existing consumers
func TestBackwardCompatibility(t *testing.T) {
	// Test that old fields are still present
	resp := JSONResponse{
		Version: "1.0",
		Command: "test",
		Success: true,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// These fields must always be present for backward compatibility
	requiredFields := []string{"@version", "command", "success"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("required field %q missing from JSON output", field)
		}
	}
}

// TestEmitJSON verifies the EmitJSON function works correctly
func TestEmitJSON(t *testing.T) {
	type testData struct {
		JSONResponse
		Result string `json:"result"`
	}

	data := testData{
		JSONResponse: JSONResponse{
			Version: "1.0",
			Command: "test",
			Success: true,
		},
		Result: "test result",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Emit JSON
	if err := EmitJSON(data); err != nil {
		t.Fatalf("EmitJSON failed: %v", err)
	}

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)

	// Verify output is valid JSON
	var decoded testData
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal emitted JSON: %v", err)
	}

	// Verify data matches
	if decoded.Version != data.Version {
		t.Errorf("version = %q, want %q", decoded.Version, data.Version)
	}
	if decoded.Command != data.Command {
		t.Errorf("command = %q, want %q", decoded.Command, data.Command)
	}
	if decoded.Success != data.Success {
		t.Errorf("success = %v, want %v", decoded.Success, data.Success)
	}
	if decoded.Result != data.Result {
		t.Errorf("result = %q, want %q", decoded.Result, data.Result)
	}
}

// TestJSONLocationOptional verifies Location field is optional
func TestJSONLocationOptional(t *testing.T) {
	// Error without location
	err1 := JSONError{
		Code:    "E001",
		Message: "test error",
	}

	data, marshalErr := json.Marshal(err1)
	if marshalErr != nil {
		t.Fatalf("failed to marshal error without location: %v", marshalErr)
	}

	var decoded JSONError
	if unmarshalErr := json.Unmarshal(data, &decoded); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal error without location: %v", unmarshalErr)
	}

	if decoded.Location != nil {
		t.Error("location should be nil for error without location")
	}

	// Error with location
	err2 := JSONError{
		Code:    "E002",
		Message: "test error",
		Location: &JSONLocation{
			Line:   10,
			Column: 5,
		},
	}

	data2, marshalErr2 := json.Marshal(err2)
	if marshalErr2 != nil {
		t.Fatalf("failed to marshal error with location: %v", marshalErr2)
	}

	var decoded2 JSONError
	if unmarshalErr2 := json.Unmarshal(data2, &decoded2); unmarshalErr2 != nil {
		t.Fatalf("failed to unmarshal error with location: %v", unmarshalErr2)
	}

	if decoded2.Location == nil {
		t.Fatal("location should not be nil for error with location")
	}
	if decoded2.Location.Line != 10 {
		t.Errorf("location.line = %d, want 10", decoded2.Location.Line)
	}
	if decoded2.Location.Column != 5 {
		t.Errorf("location.column = %d, want 5", decoded2.Location.Column)
	}
}
