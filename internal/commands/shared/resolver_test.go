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
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkflowPath(t *testing.T) {
	// Create temporary test directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Setup test files and directories
	// 1. A direct workflow file
	directFile := "direct.yaml"
	os.WriteFile(directFile, []byte("name: test"), 0644)

	// 2. A workflow with .yaml extension
	namedFile := "review.yaml"
	os.WriteFile(namedFile, []byte("name: review"), 0644)

	// 3. A directory with workflow.yaml
	dirWithWorkflow := "myworkflow"
	os.Mkdir(dirWithWorkflow, 0755)
	os.WriteFile(filepath.Join(dirWithWorkflow, "workflow.yaml"), []byte("name: myworkflow"), 0644)

	// 4. An absolute path workflow
	absFile := filepath.Join(tmpDir, "absolute.yaml")
	os.WriteFile(absFile, []byte("name: absolute"), 0644)

	tests := []struct {
		name        string
		arg         string
		expected    string
		shouldError bool
	}{
		{
			name:     "direct file path",
			arg:      "direct.yaml",
			expected: "direct.yaml",
		},
		{
			name:     "name without extension resolves to .yaml",
			arg:      "review",
			expected: "review.yaml",
		},
		{
			name:     "directory with workflow.yaml",
			arg:      "myworkflow",
			expected: filepath.Join("myworkflow", "workflow.yaml"),
		},
		{
			name:     "directory path with trailing slash",
			arg:      "myworkflow/",
			expected: filepath.Join("myworkflow", "workflow.yaml"),
		},
		{
			name:     "absolute path",
			arg:      absFile,
			expected: absFile,
		},
		{
			name:        "nonexistent file",
			arg:         "nonexistent",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveWorkflowPath(tt.arg)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Normalize paths for comparison
			expectedAbs, _ := filepath.Abs(tt.expected)
			resultAbs, _ := filepath.Abs(result)

			if resultAbs != expectedAbs {
				t.Errorf("expected %q, got %q", expectedAbs, resultAbs)
			}
		})
	}
}

func TestResolveWorkflowPath_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create empty directory
	emptyDir := "empty"
	os.Mkdir(emptyDir, 0755)

	_, err := ResolveWorkflowPath(emptyDir)
	if err == nil {
		t.Error("expected error for directory without workflow.yaml")
	}
}

func TestResolveWorkflowPath_PriorityOrder(t *testing.T) {
	// Test that exact file path takes precedence over name.yaml
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create both "test" directory and "test.yaml" file
	os.Mkdir("test", 0755)
	os.WriteFile(filepath.Join("test", "workflow.yaml"), []byte("name: dir"), 0644)
	os.WriteFile("test.yaml", []byte("name: file"), 0644)

	// When we pass "test.yaml", it should find the file directly
	result, err := ResolveWorkflowPath("test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test.yaml" {
		t.Errorf("expected test.yaml, got %s", result)
	}

	// When we pass "test", it should check as directory first (exists and has workflow.yaml)
	result, err = ResolveWorkflowPath("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("test", "workflow.yaml")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}
