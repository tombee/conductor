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

package test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "test file with _test.yaml suffix",
			filename: "workflow_test.yaml",
			want:     true,
		},
		{
			name:     "test file with test_ prefix",
			filename: "test_workflow.yaml",
			want:     true,
		},
		{
			name:     "test file with _test.yml suffix",
			filename: "workflow_test.yml",
			want:     true,
		},
		{
			name:     "test file with test_ prefix and yml",
			filename: "test_workflow.yml",
			want:     true,
		},
		{
			name:     "regular workflow file",
			filename: "workflow.yaml",
			want:     false,
		},
		{
			name:     "non-yaml file",
			filename: "test_file.txt",
			want:     false,
		},
		{
			name:     "yaml file with test in middle of name",
			filename: "my_test_file.yaml",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTestFile(tt.filename)
			if got != tt.want {
				t.Errorf("isTestFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestDiscoverTests(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create test files
	testFiles := []struct {
		path    string
		content string
	}{
		{
			path: "simple_test.yaml",
			content: `workflow: workflow.yaml
name: Simple Test
inputs:
  foo: bar
assert:
  step1: 'status == "ok"'`,
		},
		{
			path: "subdir/nested_test.yaml",
			content: `workflow: ../workflow.yaml
name: Nested Test`,
		},
		{
			path: "workflow.yaml", // Not a test file
			content: `name: test
steps: []`,
		},
	}

	for _, tf := range testFiles {
		fullPath := filepath.Join(tmpDir, tf.path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(tf.content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	tests := []struct {
		name      string
		paths     []string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "discover all tests in directory",
			paths:     []string{tmpDir},
			wantCount: 2, // simple_test.yaml and nested_test.yaml
			wantErr:   false,
		},
		{
			name:      "discover specific test file",
			paths:     []string{filepath.Join(tmpDir, "simple_test.yaml")},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "error on non-existent path",
			paths:     []string{"/nonexistent/path"},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:      "error when no tests found",
			paths:     []string{filepath.Join(tmpDir, "workflow.yaml")},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := discoverTests(tt.paths)
			if (err != nil) != tt.wantErr {
				t.Errorf("discoverTests() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantCount {
				t.Errorf("discoverTests() found %d tests, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestParseTestFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		wantErr bool
		check   func(*testing.T, TestFile)
	}{
		{
			name: "valid test file",
			content: `workflow: workflow.yaml
name: Test Name
inputs:
  foo: bar
assert:
  step1: 'status == "ok"'`,
			wantErr: false,
			check: func(t *testing.T, tf TestFile) {
				if tf.Name != "Test Name" {
					t.Errorf("Name = %q, want %q", tf.Name, "Test Name")
				}
				if tf.Workflow == "" {
					t.Error("Workflow path should not be empty")
				}
				if len(tf.Inputs) != 1 || tf.Inputs["foo"] != "bar" {
					t.Errorf("Inputs = %v, want map[foo:bar]", tf.Inputs)
				}
				if len(tf.Assert) != 1 {
					t.Errorf("Assert count = %d, want 1", len(tf.Assert))
				}
			},
		},
		{
			name: "test file without name uses filename",
			content: `workflow: workflow.yaml
inputs:
  foo: bar`,
			wantErr: false,
			check: func(t *testing.T, tf TestFile) {
				if tf.Name == "" {
					t.Error("Name should be derived from filename")
				}
			},
		},
		{
			name:    "missing workflow field",
			content: `name: Test\ninputs:\n  foo: bar`,
			wantErr: true,
			check:   nil,
		},
		{
			name:    "invalid YAML",
			content: `invalid: yaml: content: [`,
			wantErr: true,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test_"+tt.name+".yaml")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			got, err := parseTestFile(testFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTestFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
