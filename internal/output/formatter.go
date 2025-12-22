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

package output

import "io"

// Formatter defines the interface for output formatting.
// Implementations can provide different output formats (JSON, text, table, etc.)
type Formatter interface {
	// FormatSuccess formats a successful command result
	FormatSuccess(command string, data interface{}) error

	// FormatError formats an error response
	FormatError(command string, errors []JSONError) error

	// SetOutput sets the output writer
	SetOutput(w io.Writer)
}

// DefaultFormatter returns a formatter based on the JSON mode flag
func DefaultFormatter(jsonMode bool) Formatter {
	if jsonMode {
		return &JSONFormatter{}
	}
	return &TextFormatter{}
}

// JSONFormatter implements Formatter for JSON output
type JSONFormatter struct {
	out io.Writer
}

// FormatSuccess outputs JSON for successful results
func (f *JSONFormatter) FormatSuccess(command string, data interface{}) error {
	return EmitJSON(data)
}

// FormatError outputs JSON for errors
func (f *JSONFormatter) FormatError(command string, errors []JSONError) error {
	return EmitJSONError(command, errors)
}

// SetOutput sets the output writer
func (f *JSONFormatter) SetOutput(w io.Writer) {
	f.out = w
}

// TextFormatter implements Formatter for human-readable text output
type TextFormatter struct {
	out io.Writer
}

// FormatSuccess outputs text for successful results
func (f *TextFormatter) FormatSuccess(command string, data interface{}) error {
	// Text formatting is command-specific, so this is a placeholder
	// Each command should handle its own text output
	return nil
}

// FormatError outputs text for errors
func (f *TextFormatter) FormatError(command string, errors []JSONError) error {
	// Text error formatting is handled by the CLI error handling
	// This is a placeholder for future structured error output
	return nil
}

// SetOutput sets the output writer
func (f *TextFormatter) SetOutput(w io.Writer) {
	f.out = w
}
