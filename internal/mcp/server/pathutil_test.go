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

package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath_RejectDirectoryTraversal(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "simple traversal",
			path:    "../etc/passwd",
			wantErr: true,
		},
		{
			name:    "nested traversal",
			path:    "foo/../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "hidden traversal",
			path:    "./foo/../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "valid relative path",
			path:    "./workflow.yaml",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    "workflows/test.yaml",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath_EmptyPath(t *testing.T) {
	err := ValidatePath("")
	if err == nil {
		t.Errorf("Expected error for empty path, got nil")
	}
}

func TestValidatePath_CurrentDirectory(t *testing.T) {
	// Create a temporary file in current directory
	tmpFile := filepath.Join(".", "test-workflow.yaml")

	// Should accept path in current directory
	err := ValidatePath(tmpFile)
	if err != nil {
		t.Errorf("ValidatePath() rejected current directory path: %v", err)
	}
}

func TestValidatePath_AllowedPaths(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "conductor-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "workflow.yaml")

	// Without CONDUCTOR_ALLOWED_PATHS, should be rejected
	err = ValidatePath(testFile)
	if err == nil {
		t.Errorf("Expected error for path outside current directory, got nil")
	}

	// Set CONDUCTOR_ALLOWED_PATHS
	oldAllowedPaths := os.Getenv("CONDUCTOR_ALLOWED_PATHS")
	defer os.Setenv("CONDUCTOR_ALLOWED_PATHS", oldAllowedPaths)

	os.Setenv("CONDUCTOR_ALLOWED_PATHS", tmpDir)

	// Now should be accepted
	err = ValidatePath(testFile)
	if err != nil {
		t.Errorf("ValidatePath() rejected path in CONDUCTOR_ALLOWED_PATHS: %v", err)
	}
}

func TestIsPathWithinDir(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		dir      string
		expected bool
	}{
		{
			name:     "exact match",
			path:     "/foo/bar",
			dir:      "/foo/bar",
			expected: true,
		},
		{
			name:     "subdirectory",
			path:     "/foo/bar/baz",
			dir:      "/foo/bar",
			expected: true,
		},
		{
			name:     "parent directory",
			path:     "/foo",
			dir:      "/foo/bar",
			expected: false,
		},
		{
			name:     "different branch",
			path:     "/foo/baz",
			dir:      "/foo/bar",
			expected: false,
		},
		{
			name:     "prefix false match",
			path:     "/foobar",
			dir:      "/foo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPathWithinDir(tt.path, tt.dir)
			if result != tt.expected {
				t.Errorf("isPathWithinDir(%q, %q) = %v, expected %v", tt.path, tt.dir, result, tt.expected)
			}
		})
	}
}
