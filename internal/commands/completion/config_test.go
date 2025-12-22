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
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCheckFilePermissions(t *testing.T) {
	tests := []struct {
		name     string
		mode     os.FileMode
		expected bool
	}{
		{
			name:     "0600 permissions are acceptable",
			mode:     0600,
			expected: true,
		},
		{
			name:     "0400 permissions are acceptable",
			mode:     0400,
			expected: true,
		},
		{
			name:     "0644 permissions are too permissive",
			mode:     0644,
			expected: false,
		},
		{
			name:     "0755 permissions are too permissive",
			mode:     0755,
			expected: false,
		},
		{
			name:     "0700 permissions are too permissive",
			mode:     0700,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file with specific permissions
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.yaml")

			if err := os.WriteFile(testFile, []byte("test: value\n"), tt.mode); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			result := CheckFilePermissions(testFile)
			if result != tt.expected {
				t.Errorf("CheckFilePermissions() = %v, want %v for mode %o", result, tt.expected, tt.mode)
			}
		})
	}
}

func TestCheckFilePermissions_NonexistentFile(t *testing.T) {
	// Non-existent file should return true (safe default)
	result := CheckFilePermissions("/nonexistent/path/to/file.yaml")
	if !result {
		t.Error("CheckFilePermissions() should return true for nonexistent file")
	}
}

func TestSafeCompletionWrapper_Success(t *testing.T) {
	fn := func() ([]string, cobra.ShellCompDirective) {
		return []string{"result1", "result2"}, cobra.ShellCompDirectiveNoFileComp
	}

	results, directive := SafeCompletionWrapper(fn)
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("Expected NoFileComp directive, got %v", directive)
	}
}

func TestSafeCompletionWrapper_Panic(t *testing.T) {
	fn := func() ([]string, cobra.ShellCompDirective) {
		panic("test panic")
	}

	results, directive := SafeCompletionWrapper(fn)
	if len(results) != 0 {
		t.Errorf("Expected empty results on panic, got %d results", len(results))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("Expected NoFileComp directive on panic, got %v", directive)
	}
}

func TestSafeCompletionWrapper_NilResults(t *testing.T) {
	fn := func() ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	}

	results, directive := SafeCompletionWrapper(fn)
	if len(results) != 0 {
		t.Errorf("Expected empty results for nil return, got %d results", len(results))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("Expected NoFileComp directive, got %v", directive)
	}
}
