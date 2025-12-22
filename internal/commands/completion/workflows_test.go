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

package completion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteWorkflowFiles_GitHubPrefix(t *testing.T) {
	tests := []struct {
		name       string
		toComplete string
	}{
		{"starts with github:", "github:"},
		{"starts with g", "g"},
		{"starts with gi", "gi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, directive := CompleteWorkflowFiles(nil, nil, tt.toComplete)
			if len(results) != 1 || results[0] != "github:" {
				t.Errorf("Expected ['github:'], got %v", results)
			}
			if directive != cobra.ShellCompDirectiveNoSpace {
				t.Errorf("Expected NoSpace directive, got %v", directive)
			}
		})
	}
}

func TestIsWorkflowFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name: "valid workflow with name",
			content: `name: test-workflow
steps:
  - name: step1
    type: shell
`,
			expected: true,
		},
		{
			name: "YAML without name key",
			content: `steps:
  - name: step1
    type: shell
`,
			expected: false,
		},
		{
			name:     "invalid YAML",
			content:  `{{{invalid`,
			expected: false,
		},
		{
			name:     "empty file",
			content:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.yaml")
			if err := os.WriteFile(testFile, []byte(tt.content), 0600); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			result := isWorkflowFile(testFile)
			if result != tt.expected {
				t.Errorf("isWorkflowFile() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsWorkflowFile_NonexistentFile(t *testing.T) {
	result := isWorkflowFile("/nonexistent/file.yaml")
	if result {
		t.Error("isWorkflowFile() should return false for nonexistent file")
	}
}

func TestIsSafeFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular file
	regularFile := filepath.Join(tmpDir, "regular.yaml")
	if err := os.WriteFile(regularFile, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Test regular file is safe
	if !isSafeFile(regularFile) {
		t.Error("Regular file should be safe")
	}

	// Create a symlink
	symlinkFile := filepath.Join(tmpDir, "symlink.yaml")
	if err := os.Symlink(regularFile, symlinkFile); err != nil {
		t.Skipf("Cannot create symlink (may not be supported): %v", err)
	}

	// Test symlink is not safe
	if isSafeFile(symlinkFile) {
		t.Error("Symlink should not be safe")
	}
}

func TestDiscoverWorkflowFiles_DepthLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files at different depths
	// Depth 0
	file0 := filepath.Join(tmpDir, "workflow0.yaml")
	os.WriteFile(file0, []byte("name: workflow0\n"), 0600)

	// Depth 1
	dir1 := filepath.Join(tmpDir, "level1")
	os.Mkdir(dir1, 0755)
	file1 := filepath.Join(dir1, "workflow1.yaml")
	os.WriteFile(file1, []byte("name: workflow1\n"), 0600)

	// Depth 2
	dir2 := filepath.Join(dir1, "level2")
	os.Mkdir(dir2, 0755)
	file2 := filepath.Join(dir2, "workflow2.yaml")
	os.WriteFile(file2, []byte("name: workflow2\n"), 0600)

	// Depth 3 (should be excluded)
	dir3 := filepath.Join(dir2, "level3")
	os.Mkdir(dir3, 0755)
	file3 := filepath.Join(dir3, "workflow3.yaml")
	os.WriteFile(file3, []byte("name: workflow3\n"), 0600)

	// Change to tmpDir for relative paths
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	files, err := discoverWorkflowFiles(".", 2)
	if err != nil {
		t.Fatalf("discoverWorkflowFiles failed: %v", err)
	}

	// Should find files at depth 0, 1, 2 but not 3
	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}

	// Verify depth 3 file is not included
	for _, f := range files {
		if strings.Contains(f.path, "level3") {
			t.Error("Should not include files at depth 3")
		}
	}
}

func TestDiscoverWorkflowFiles_FileLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create more than maxWorkflowFiles
	for i := 0; i < 150; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("workflow%03d.yaml", i))
		content := fmt.Sprintf("name: workflow%03d\n", i)
		os.WriteFile(filename, []byte(content), 0600)
	}

	// Change to tmpDir
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	results, directive := CompleteWorkflowFiles(nil, nil, "")
	if directive != cobra.ShellCompDirectiveDefault {
		t.Errorf("Expected Default directive, got %v", directive)
	}

	// Should be limited to maxWorkflowFiles
	if len(results) > maxWorkflowFiles {
		t.Errorf("Expected max %d files, got %d", maxWorkflowFiles, len(results))
	}
}
