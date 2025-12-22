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
	"strings"
	"testing"
)

func TestValidateString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid string",
			input:   "hello world",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "string with newlines",
			input:   "line1\nline2\n",
			wantErr: false,
		},
		{
			name:    "string with tabs",
			input:   "col1\tcol2\t",
			wantErr: false,
		},
		{
			name:    "string with carriage returns",
			input:   "line1\r\nline2\r\n",
			wantErr: false,
		},
		{
			name:    "null byte",
			input:   "hello\x00world",
			wantErr: true,
			errMsg:  "null byte",
		},
		{
			name:    "control character",
			input:   "hello\x01world",
			wantErr: true,
			errMsg:  "invalid control character",
		},
		{
			name:    "oversized input",
			input:   strings.Repeat("a", MaxInputSize+1),
			wantErr: true,
			errMsg:  "exceeds maximum size",
		},
		{
			name:    "max size input",
			input:   strings.Repeat("a", MaxInputSize),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateString() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
		errMsg  string
	}{
		{
			name:  "integer",
			input: "42",
			want:  42.0,
		},
		{
			name:  "float",
			input: "3.14159",
			want:  3.14159,
		},
		{
			name:  "negative number",
			input: "-100",
			want:  -100.0,
		},
		{
			name:  "zero",
			input: "0",
			want:  0.0,
		},
		{
			name:  "scientific notation",
			input: "1.5e10",
			want:  1.5e10,
		},
		{
			name:  "with whitespace",
			input: "  42  ",
			want:  42.0,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "empty",
		},
		{
			name:    "not a number",
			input:   "abc",
			wantErr: true,
			errMsg:  "must be a number",
		},
		{
			name:    "partial number",
			input:   "12abc",
			wantErr: true,
			errMsg:  "must be a number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ValidateNumber() = %v, want %v", got, tt.want)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateNumber() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateBool(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		// True values
		{name: "y", input: "y", want: true},
		{name: "Y", input: "Y", want: true},
		{name: "yes", input: "yes", want: true},
		{name: "YES", input: "YES", want: true},
		{name: "Yes", input: "Yes", want: true},
		{name: "true", input: "true", want: true},
		{name: "TRUE", input: "TRUE", want: true},
		{name: "1", input: "1", want: true},
		{name: "yes with spaces", input: "  yes  ", want: true},

		// False values
		{name: "n", input: "n", want: false},
		{name: "N", input: "N", want: false},
		{name: "no", input: "no", want: false},
		{name: "NO", input: "NO", want: false},
		{name: "false", input: "false", want: false},
		{name: "FALSE", input: "FALSE", want: false},
		{name: "0", input: "0", want: false},

		// Invalid values
		{name: "invalid", input: "maybe", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "number", input: "2", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateBool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ValidateBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateEnum(t *testing.T) {
	options := []string{"apple", "banana", "cherry"}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
		errMsg  string
	}{
		{
			name:  "exact match",
			input: "apple",
			want:  "apple",
		},
		{
			name:  "case insensitive match",
			input: "APPLE",
			want:  "apple",
		},
		{
			name:  "numeric selection 1",
			input: "1",
			want:  "apple",
		},
		{
			name:  "numeric selection 2",
			input: "2",
			want:  "banana",
		},
		{
			name:  "numeric selection 3",
			input: "3",
			want:  "cherry",
		},
		{
			name:  "with spaces",
			input: "  banana  ",
			want:  "banana",
		},
		{
			name:    "out of range",
			input:   "0",
			wantErr: true,
			errMsg:  "between 1 and 3",
		},
		{
			name:    "too high",
			input:   "4",
			wantErr: true,
			errMsg:  "between 1 and 3",
		},
		{
			name:    "invalid option",
			input:   "orange",
			wantErr: true,
			errMsg:  "valid option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateEnum(tt.input, options)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEnum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ValidateEnum() = %v, want %v", got, tt.want)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateEnum() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateEnum_EmptyOptions(t *testing.T) {
	_, err := ValidateEnum("test", []string{})
	if err == nil {
		t.Error("ValidateEnum() with empty options should return error")
	}
	if !strings.Contains(err.Error(), "no options") {
		t.Errorf("ValidateEnum() error = %v, want error containing 'no options'", err)
	}
}

func TestValidateArray(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  []interface{}{},
		},
		{
			name:  "single value",
			input: "apple",
			want:  []interface{}{"apple"},
		},
		{
			name:  "comma separated",
			input: "apple,banana,cherry",
			want:  []interface{}{"apple", "banana", "cherry"},
		},
		{
			name:  "with spaces",
			input: "apple , banana , cherry",
			want:  []interface{}{"apple", "banana", "cherry"},
		},
		{
			name:  "escaped comma",
			input: `apple\,pie,banana`,
			want:  []interface{}{"apple,pie", "banana"},
		},
		{
			name:  "JSON array",
			input: `["apple", "banana", "cherry"]`,
			want:  []interface{}{"apple", "banana", "cherry"},
		},
		{
			name:  "JSON array with numbers",
			input: `[1, 2, 3]`,
			want:  []interface{}{float64(1), float64(2), float64(3)},
		},
		{
			name:  "JSON mixed array",
			input: `["text", 42, true]`,
			want:  []interface{}{"text", float64(42), true},
		},
		{
			name:    "invalid JSON",
			input:   `["unclosed`,
			wantErr: true,
			errMsg:  "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateArray(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArray() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("ValidateArray() length = %d, want %d", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("ValidateArray()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateArray() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateObject(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
		check   func(map[string]interface{}) bool
	}{
		{
			name:  "simple object",
			input: `{"name": "test", "value": 42}`,
			check: func(obj map[string]interface{}) bool {
				return obj["name"] == "test" && obj["value"] == float64(42)
			},
		},
		{
			name:  "nested object",
			input: `{"outer": {"inner": "value"}}`,
			check: func(obj map[string]interface{}) bool {
				outer, ok := obj["outer"].(map[string]interface{})
				return ok && outer["inner"] == "value"
			},
		},
		{
			name:  "with array",
			input: `{"items": [1, 2, 3]}`,
			check: func(obj map[string]interface{}) bool {
				items, ok := obj["items"].([]interface{})
				return ok && len(items) == 3
			},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "empty",
		},
		{
			name:    "invalid JSON",
			input:   `{"unclosed"`,
			wantErr: true,
			errMsg:  "invalid JSON",
		},
		{
			name:    "not an object",
			input:   `["array"]`,
			wantErr: true,
			errMsg:  "invalid JSON",
		},
		{
			name:  "deeply nested object at limit",
			input: `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":"value"}}}}}}}}}}`,
			check: func(obj map[string]interface{}) bool {
				return true // Just need to not error
			},
		},
		{
			name:    "too deeply nested object",
			input:   `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":"value"}}}}}}}}}}}`,
			wantErr: true,
			errMsg:  "exceeds maximum depth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateObject(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				if !tt.check(got) {
					t.Errorf("ValidateObject() result failed validation check")
				}
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateObject() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestCheckDepth(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		depth   int
		wantErr bool
	}{
		{
			name:  "simple value at depth 0",
			value: "string",
			depth: 0,
		},
		{
			name: "map at depth 0",
			value: map[string]interface{}{
				"key": "value",
			},
			depth: 0,
		},
		{
			name: "nested map below max depth",
			value: map[string]interface{}{
				"key": "value",
			},
			depth: MaxNestedDepth - 1,
		},
		{
			name: "nested map exceeds depth",
			value: map[string]interface{}{
				"key": "value",
			},
			depth:   MaxNestedDepth + 1,
			wantErr: true,
		},
		{
			name: "array at depth",
			value: []interface{}{
				"a", "b", "c",
			},
			depth: 0,
		},
		{
			name: "nested array in map below limit",
			value: map[string]interface{}{
				"arr": []interface{}{
					map[string]interface{}{
						"nested": "value",
					},
				},
			},
			depth: MaxNestedDepth - 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDepth(tt.value, tt.depth)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkDepth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
