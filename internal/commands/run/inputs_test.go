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

package run

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseInputs_KeyValue(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		inputFile string
		wantKey   string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "single key-value",
			args:      []string{"name=Alice"},
			wantKey:   "name",
			wantValue: "Alice",
		},
		{
			name:      "multiple key-values",
			args:      []string{"name=Alice", "age=30"},
			wantKey:   "age",
			wantValue: "30",
		},
		{
			name:      "value with equals sign",
			args:      []string{"equation=a=b"},
			wantKey:   "equation",
			wantValue: "a=b",
		},
		{
			name:    "invalid format",
			args:    []string{"invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs, err := parseInputs(tt.args, tt.inputFile)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantKey != "" {
				val, ok := inputs[tt.wantKey]
				if !ok {
					t.Errorf("key %q not found in inputs", tt.wantKey)
				} else if val != tt.wantValue {
					t.Errorf("expected %q=%q, got %q=%q", tt.wantKey, tt.wantValue, tt.wantKey, val)
				}
			}
		})
	}
}

func TestLoadInputFile_ValidJSON(t *testing.T) {
	// Create temp file with valid JSON
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "inputs.json")
	content := `{"name": "Alice", "count": 42}`
	if err := os.WriteFile(jsonFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	inputs, err := loadInputFile(jsonFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inputs["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", inputs["name"])
	}
	if inputs["count"] != float64(42) { // JSON numbers are float64
		t.Errorf("expected count=42, got %v", inputs["count"])
	}
}

func TestLoadInputFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(jsonFile, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := loadInputFile(jsonFile)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadInputFile_FileNotFound(t *testing.T) {
	_, err := loadInputFile("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseInputs_MergeFileAndFlags(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "inputs.json")
	content := `{"name": "FileValue", "extra": "FromFile"}`
	if err := os.WriteFile(jsonFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Parse with both file and flag inputs
	inputs, err := parseInputs([]string{"name=FlagValue"}, jsonFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Flag should override file
	if inputs["name"] != "FlagValue" {
		t.Errorf("expected flag to override file: got %v", inputs["name"])
	}

	// File-only values should be preserved
	if inputs["extra"] != "FromFile" {
		t.Errorf("expected file value preserved: got %v", inputs["extra"])
	}
}

func TestDisplayStats(t *testing.T) {
	// Just verify it doesn't panic with various inputs
	tests := []struct {
		name  string
		stats *RunStats
	}{
		{
			name:  "nil stats",
			stats: nil,
		},
		{
			name: "full stats",
			stats: &RunStats{
				CostUSD:    0.003,
				TokensIn:   1234,
				TokensOut:  567,
				DurationMs: 2100,
			},
		},
		{
			name: "partial stats",
			stats: &RunStats{
				DurationMs: 1000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify no panic
			if tt.stats != nil {
				displayStats(tt.stats)
			}
		})
	}
}
