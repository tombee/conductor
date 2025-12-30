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

package runner

import (
	"testing"

	"github.com/tombee/conductor/internal/controller/backend"
)

func TestValidateReplayConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *backend.ReplayConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "cannot be nil",
		},
		{
			name: "missing parent run ID",
			config: &backend.ReplayConfig{
				ParentRunID: "",
			},
			wantErr: true,
			errMsg:  "parent_run_id is required",
		},
		{
			name: "valid minimal config",
			config: &backend.ReplayConfig{
				ParentRunID: "run-123",
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: &backend.ReplayConfig{
				ParentRunID: "run-123",
				FromStepID:  "step_2",
				OverrideInputs: map[string]any{
					"user_id": "user-456",
					"count":   42,
				},
				MaxCost:        10.0,
				ValidateSchema: true,
			},
			wantErr: false,
		},
		{
			name: "negative max cost",
			config: &backend.ReplayConfig{
				ParentRunID: "run-123",
				MaxCost:     -5.0,
			},
			wantErr: true,
			errMsg:  "cannot be negative",
		},
		{
			name: "template injection in override input",
			config: &backend.ReplayConfig{
				ParentRunID: "run-123",
				OverrideInputs: map[string]any{
					"malicious": "{{ .secrets }}",
				},
			},
			wantErr: true,
			errMsg:  "template expressions",
		},
		{
			name: "invalid input key",
			config: &backend.ReplayConfig{
				ParentRunID: "run-123",
				OverrideInputs: map[string]any{
					"bad-key!": "value",
				},
			},
			wantErr: true,
			errMsg:  "invalid input key",
		},
		{
			name: "invalid step ID",
			config: &backend.ReplayConfig{
				ParentRunID: "run-123",
				OverrideSteps: map[string]any{
					"bad-step!": map[string]any{"output": "test"},
				},
			},
			wantErr: true,
			errMsg:  "invalid step ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReplayConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateReplayConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateReplayConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestSanitizeOverrideInputs(t *testing.T) {
	tests := []struct {
		name     string
		inputs   map[string]any
		expected map[string]any
	}{
		{
			name:     "nil inputs",
			inputs:   nil,
			expected: nil,
		},
		{
			name:     "empty inputs",
			inputs:   map[string]any{},
			expected: map[string]any{},
		},
		{
			name: "escape template delimiters",
			inputs: map[string]any{
				"data": "{{ .secret }}",
			},
			expected: map[string]any{
				"data": "&#123;&#123; .secret &#125;&#125;",
			},
		},
		{
			name: "escape shell syntax",
			inputs: map[string]any{
				"script": "${MALICIOUS}",
			},
			expected: map[string]any{
				"script": "&#36;&#123;MALICIOUS}",
			},
		},
		{
			name: "preserve numbers and booleans",
			inputs: map[string]any{
				"count":   42,
				"enabled": true,
			},
			expected: map[string]any{
				"count":   42,
				"enabled": true,
			},
		},
		{
			name: "sanitize nested maps",
			inputs: map[string]any{
				"nested": map[string]any{
					"value": "{{ .inject }}",
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"value": "&#123;&#123; .inject &#125;&#125;",
				},
			},
		},
		{
			name: "sanitize arrays",
			inputs: map[string]any{
				"items": []any{
					"{{ .item1 }}",
					"safe string",
					42,
				},
			},
			expected: map[string]any{
				"items": []any{
					"&#123;&#123; .item1 &#125;&#125;",
					"safe string",
					42,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeOverrideInputs(tt.inputs)
			if !mapsEqual(result, tt.expected) {
				t.Errorf("SanitizeOverrideInputs() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", false},
		{"alphanumeric", "user_id", true},
		{"numbers", "user123", true},
		{"underscore", "user_id_123", true},
		{"hyphen", "user-id", false},
		{"special chars", "user!@#", false},
		{"spaces", "user id", false},
		{"dot", "user.id", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidIdentifier(tt.input); got != tt.want {
				t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Helper function to compare maps deeply
func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !valuesEqual(v, bv) {
			return false
		}
	}
	return true
}

// Helper function to compare values deeply
func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	switch va := a.(type) {
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok {
			return false
		}
		return mapsEqual(va, vb)
	case []any:
		vb, ok := b.([]any)
		if !ok {
			return false
		}
		if len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !valuesEqual(va[i], vb[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
