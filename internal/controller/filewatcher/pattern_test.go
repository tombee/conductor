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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatternMatcher_SimplePatterns(t *testing.T) {
	tests := []struct {
		name            string
		includePatterns []string
		excludePatterns []string
		testPaths       map[string]bool // path -> should match
	}{
		{
			name:            "no patterns matches everything",
			includePatterns: nil,
			excludePatterns: nil,
			testPaths: map[string]bool{
				"/tmp/test.txt":       true,
				"/tmp/test.pdf":       true,
				"/tmp/subdir/file.go": true,
			},
		},
		{
			name:            "simple extension include",
			includePatterns: []string{"*.txt"},
			excludePatterns: nil,
			testPaths: map[string]bool{
				"/tmp/test.txt":       true,
				"/tmp/test.pdf":       false,
				"/tmp/subdir/file.txt": true,
			},
		},
		{
			name:            "simple extension exclude",
			includePatterns: nil,
			excludePatterns: []string{"*.log"},
			testPaths: map[string]bool{
				"/tmp/test.txt": true,
				"/tmp/test.log": false,
				"/var/log/app.log": false,
			},
		},
		{
			name:            "include and exclude",
			includePatterns: []string{"*.txt"},
			excludePatterns: []string{"temp*"},
			testPaths: map[string]bool{
				"/tmp/test.txt":    true,
				"/tmp/temp.txt":    false,
				"/tmp/tempfile.txt": false,
				"/tmp/test.pdf":    false,
			},
		},
		{
			name:            "vim swap files excluded",
			includePatterns: nil,
			excludePatterns: []string{"*.swp", "*.swo", ".*.sw?"},
			testPaths: map[string]bool{
				"/tmp/test.txt":     true,
				"/tmp/.test.txt.swp": false,
				"/tmp/test.txt.swp": false,
				"/tmp/test.txt.swo": false,
				"/tmp/.test.swx":    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, err := NewPatternMatcher(tt.includePatterns, tt.excludePatterns)
			require.NoError(t, err)

			for path, shouldMatch := range tt.testPaths {
				matched := pm.Match(path)
				assert.Equal(t, shouldMatch, matched,
					"path %q should match=%v but got %v", path, shouldMatch, matched)
			}
		})
	}
}

func TestPatternMatcher_RecursivePatterns(t *testing.T) {
	tests := []struct {
		name            string
		includePatterns []string
		excludePatterns []string
		testPaths       map[string]bool
	}{
		{
			name:            "recursive directory pattern",
			includePatterns: []string{"**/src/**/*.go"},
			excludePatterns: nil,
			testPaths: map[string]bool{
				"/project/src/main.go":            true,
				"/project/src/pkg/util.go":        true,
				"/project/test/test.go":           false,
				"/project/vendor/src/lib.go":      true,
			},
		},
		{
			name:            "exclude recursive directory",
			includePatterns: []string{"*.go"},
			excludePatterns: []string{"**/vendor/**"},
			testPaths: map[string]bool{
				"/project/main.go":                true,
				"/project/pkg/util.go":            true,
				"/project/vendor/lib/pkg.go":      false,
				"/project/vendor/src/main.go":     false,
			},
		},
		{
			name:            "exclude dot directories",
			includePatterns: nil,
			excludePatterns: []string{"**/.git/**", "**/.vscode/**"},
			testPaths: map[string]bool{
				"/project/src/main.go":       true,
				"/project/.git/config":       false,
				"/project/.vscode/settings.json": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, err := NewPatternMatcher(tt.includePatterns, tt.excludePatterns)
			require.NoError(t, err)

			for path, shouldMatch := range tt.testPaths {
				matched := pm.Match(path)
				assert.Equal(t, shouldMatch, matched,
					"path %q should match=%v but got %v", path, shouldMatch, matched)
			}
		})
	}
}

func TestPatternMatcher_InvalidPatterns(t *testing.T) {
	tests := []struct {
		name            string
		includePatterns []string
		excludePatterns []string
	}{
		{
			name:            "invalid include pattern",
			includePatterns: []string{"[invalid"},
			excludePatterns: nil,
		},
		{
			name:            "invalid exclude pattern",
			includePatterns: nil,
			excludePatterns: []string{"[invalid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPatternMatcher(tt.includePatterns, tt.excludePatterns)
			assert.Error(t, err)
		})
	}
}

func TestPatternMatcher_DefaultExcludePatterns(t *testing.T) {
	patterns := DefaultExcludePatterns()
	pm, err := NewPatternMatcher(nil, patterns)
	require.NoError(t, err)

	// Test common temporary files are excluded
	tempFiles := []string{
		"/tmp/test.txt.swp",
		"/tmp/.test.txt.swp",
		"/tmp/test.txt~",
		"/tmp/#test.txt#",
		"/tmp/.#test.txt",
		"/tmp/.DS_Store",
		"/tmp/Thumbs.db",
		"/tmp/.idea/workspace.xml",
		"/tmp/.vscode/settings.json",
		"/tmp/test.tmp",
		"/tmp/data.temp",
	}

	for _, path := range tempFiles {
		matched := pm.Match(path)
		assert.False(t, matched, "temporary file %q should be excluded", path)
	}

	// Test normal files are included
	normalFiles := []string{
		"/tmp/test.txt",
		"/tmp/document.pdf",
		"/tmp/src/main.go",
	}

	for _, path := range normalFiles {
		matched := pm.Match(path)
		assert.True(t, matched, "normal file %q should be included", path)
	}
}

func TestPatternMatcher_EdgeCases(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		pm, err := NewPatternMatcher([]string{"*.txt"}, nil)
		require.NoError(t, err)
		assert.False(t, pm.Match(""))
	})

	t.Run("multiple includes match", func(t *testing.T) {
		pm, err := NewPatternMatcher([]string{"*.txt", "*.md"}, nil)
		require.NoError(t, err)
		assert.True(t, pm.Match("/tmp/readme.md"))
		assert.True(t, pm.Match("/tmp/notes.txt"))
	})

	t.Run("exclude overrides include", func(t *testing.T) {
		pm, err := NewPatternMatcher([]string{"*.txt"}, []string{"secret*"})
		require.NoError(t, err)
		assert.True(t, pm.Match("/tmp/notes.txt"))
		assert.False(t, pm.Match("/tmp/secret.txt"))
	})
}
