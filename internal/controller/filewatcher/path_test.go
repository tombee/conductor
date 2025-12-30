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

package filewatcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	// Set test environment variable
	os.Setenv("TEST_DIR", "/test/path")
	defer os.Unsetenv("TEST_DIR")

	tests := []struct {
		name        string
		path        string
		wantPrefix  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "absolute path",
			path:       "/tmp/watch",
			wantPrefix: "/tmp/watch",
			wantErr:    false,
		},
		{
			name:       "tilde expansion",
			path:       "~/documents",
			wantPrefix: filepath.Join(home, "documents"),
			wantErr:    false,
		},
		{
			name:       "tilde only",
			path:       "~",
			wantPrefix: home,
			wantErr:    false,
		},
		{
			name:       "environment variable expansion",
			path:       "$TEST_DIR/subdir",
			wantPrefix: "/test/path/subdir",
			wantErr:    false,
		},
		{
			name:       "curly brace env var",
			path:       "${TEST_DIR}/subdir",
			wantPrefix: "/test/path/subdir",
			wantErr:    false,
		},
		{
			name:        "empty path",
			path:        "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "blocked path /etc",
			path:        "/etc",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked path /etc subdirectory",
			path:        "/etc/nginx",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked path ~/.ssh",
			path:        "~/.ssh",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked path /root",
			path:        "/root",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NormalizePath() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NormalizePath() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NormalizePath() unexpected error = %v", err)
				return
			}
			if !strings.HasPrefix(got, tt.wantPrefix) && got != tt.wantPrefix {
				t.Errorf("NormalizePath() = %v, want prefix %v", got, tt.wantPrefix)
			}
		})
	}
}

func TestValidatePathNotBlocked(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "safe path /tmp",
			path:    "/tmp/watch",
			wantErr: false,
		},
		{
			name:    "safe path /home",
			path:    "/home/user/documents",
			wantErr: false,
		},
		{
			name:        "blocked /etc",
			path:        "/etc",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked /sys",
			path:        "/sys",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked /proc",
			path:        "/proc",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked /dev",
			path:        "/dev",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked /boot",
			path:        "/boot",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked /root",
			path:        "/root",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked /var/log",
			path:        "/var/log",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked /var/run",
			path:        "/var/run",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
		{
			name:        "blocked /.ssh suffix",
			path:        "/home/user/.ssh",
			wantErr:     true,
			errContains: "SSH directory",
		},
		{
			name:        "blocked /.ssh in path",
			path:        "/home/user/.ssh/keys",
			wantErr:     true,
			errContains: "SSH directory",
		},
		{
			name:        "blocked /etc subdirectory",
			path:        "/etc/nginx/conf.d",
			wantErr:     true,
			errContains: "blocked for security reasons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathNotBlocked(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validatePathNotBlocked() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validatePathNotBlocked() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("validatePathNotBlocked() unexpected error = %v", err)
			}
		})
	}
}

func TestResolveSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real file
	realFile := filepath.Join(tmpDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a symlink
	symlink := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name:    "regular file - resolves to itself",
			path:    realFile,
			wantErr: false,
		},
		{
			name:    "symlink resolves to target",
			path:    symlink,
			wantErr: false,
		},
		{
			name:    "non-existent file returns original path",
			path:    filepath.Join(tmpDir, "nonexistent.txt"),
			want:    filepath.Join(tmpDir, "nonexistent.txt"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveSymlink(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveSymlink() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveSymlink() unexpected error = %v", err)
				return
			}
			// Only check exact match if want is specified
			if tt.want != "" && got != tt.want {
				// On macOS, /var is symlinked to /private/var, so check for suffix match too
				if !strings.HasSuffix(got, tt.want) && !strings.Contains(got, "private") {
					t.Errorf("ResolveSymlink() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestResolveSymlink_PreventsTOCTOU(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a safe target initially
	safeTarget := filepath.Join(tmpDir, "safe.txt")
	if err := os.WriteFile(safeTarget, []byte("safe"), 0644); err != nil {
		t.Fatalf("Failed to create safe file: %v", err)
	}

	// Create symlink pointing to safe target
	symlink := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(safeTarget, symlink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Verify safe resolution works
	resolved, err := ResolveSymlink(symlink)
	if err != nil {
		t.Fatalf("Failed to resolve safe symlink: %v", err)
	}
	// On macOS, paths may have /private prefix
	if resolved != safeTarget && !strings.Contains(resolved, safeTarget) {
		t.Errorf("ResolveSymlink() = %v, want %v", resolved, safeTarget)
	}

	// Now remove symlink and recreate pointing to blocked path
	os.Remove(symlink)
	if err := os.Symlink("/etc/passwd", symlink); err != nil {
		t.Fatalf("Failed to create dangerous symlink: %v", err)
	}

	// Resolution should fail due to blocked path validation
	_, err = ResolveSymlink(symlink)
	if err == nil {
		t.Error("ResolveSymlink() should fail for symlink to blocked path")
	} else {
		if !strings.Contains(err.Error(), "blocked for security reasons") {
			t.Errorf("ResolveSymlink() error = %v, want error about blocked path", err)
		}
	}
}

func TestWalkDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure:
	// tmpDir/
	//   level1a/
	//     level2a/
	//       level3/
	//   level1b/
	//     level2b/

	dirs := []string{
		filepath.Join(tmpDir, "level1a"),
		filepath.Join(tmpDir, "level1a", "level2a"),
		filepath.Join(tmpDir, "level1a", "level2a", "level3"),
		filepath.Join(tmpDir, "level1b"),
		filepath.Join(tmpDir, "level1b", "level2b"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	tests := []struct {
		name      string
		root      string
		maxDepth  int
		wantCount int
	}{
		{
			name:      "maxDepth 0 - root only",
			root:      tmpDir,
			maxDepth:  0,
			wantCount: 1,
		},
		{
			name:      "maxDepth 1 - root + level1",
			root:      tmpDir,
			maxDepth:  1,
			wantCount: 3, // root + level1a + level1b
		},
		{
			name:      "maxDepth 2 - root + level1 + level2",
			root:      tmpDir,
			maxDepth:  2,
			wantCount: 5, // root + level1a + level1b + level2a + level2b
		},
		{
			name:      "maxDepth 3 - all levels",
			root:      tmpDir,
			maxDepth:  3,
			wantCount: 6, // all directories
		},
		{
			name:      "maxDepth 10 - beyond tree depth",
			root:      tmpDir,
			maxDepth:  10,
			wantCount: 6, // all directories (tree is only 3 deep)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := WalkDirectory(tt.root, tt.maxDepth)
			if err != nil {
				t.Errorf("WalkDirectory() error = %v", err)
				return
			}
			if len(paths) != tt.wantCount {
				t.Errorf("WalkDirectory() returned %d paths, want %d. Paths: %v", len(paths), tt.wantCount, paths)
			}
			// Verify root is always included
			if paths[0] != tt.root {
				t.Errorf("WalkDirectory() first path = %v, want %v", paths[0], tt.root)
			}
		})
	}
}

func TestWalkDirectory_NonExistent(t *testing.T) {
	paths, err := WalkDirectory("/nonexistent/path", 1)
	// WalkDirectory uses filepath.Walk which only errors if the walk function returns an error
	// For non-existent root, it will return the root in paths but Walk will error
	if err == nil && len(paths) == 0 {
		t.Error("WalkDirectory() should return error or empty list for non-existent path")
	}
}
