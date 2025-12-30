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
	"fmt"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

// PatternMatcher handles include and exclude glob pattern matching for file paths.
// It uses doublestar for extended glob pattern support including ** for recursive matching.
type PatternMatcher struct {
	includePatterns []string
	excludePatterns []string
}

// NewPatternMatcher creates a new pattern matcher with the specified include and exclude patterns.
// Patterns support extended glob syntax via doublestar:
//   - * matches any sequence of non-path-separators
//   - ** matches any sequence of characters including path separators
//   - ? matches a single non-path-separator character
//   - [class] matches any single character in the class
//
// If includePatterns is empty, all files are included by default.
// excludePatterns are applied after includePatterns.
func NewPatternMatcher(includePatterns, excludePatterns []string) (*PatternMatcher, error) {
	// Validate patterns compile correctly
	for _, pattern := range includePatterns {
		if _, err := doublestar.Match(pattern, "test"); err != nil {
			return nil, fmt.Errorf("invalid include pattern %q: %w", pattern, err)
		}
	}
	for _, pattern := range excludePatterns {
		if _, err := doublestar.Match(pattern, "test"); err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}
	}

	return &PatternMatcher{
		includePatterns: includePatterns,
		excludePatterns: excludePatterns,
	}, nil
}

// Match returns true if the path matches the include patterns and doesn't match any exclude patterns.
// Path matching is performed against both the full absolute path and the base filename.
func (pm *PatternMatcher) Match(path string) bool {
	// If no include patterns, include everything by default
	included := len(pm.includePatterns) == 0

	// Check include patterns
	if !included {
		for _, pattern := range pm.includePatterns {
			if pm.matchPattern(pattern, path) {
				included = true
				break
			}
		}
	}

	if !included {
		return false
	}

	// Check exclude patterns
	for _, pattern := range pm.excludePatterns {
		if pm.matchPattern(pattern, path) {
			return false
		}
	}

	return true
}

// matchPattern checks if a path matches a pattern.
// It tries matching against both the full path and just the base filename.
func (pm *PatternMatcher) matchPattern(pattern, path string) bool {
	// Try matching against full path
	if matched, _ := doublestar.PathMatch(pattern, path); matched {
		return true
	}

	// Try matching against base filename
	base := filepath.Base(path)
	if matched, _ := doublestar.Match(pattern, base); matched {
		return true
	}

	return false
}

// DefaultExcludePatterns returns common editor temporary files and system files
// that should typically be excluded from file watching.
func DefaultExcludePatterns() []string {
	return []string{
		// Vim
		"*.swp",
		"*.swo",
		"*.swn",
		".*.sw?",
		// Emacs
		"*~",
		"#*#",
		".#*",
		// System files
		".DS_Store",
		"Thumbs.db",
		// IDE - use path patterns to match anywhere in tree
		"**/.idea/**",
		"**/.vscode/**",
		"*.tmp",
		"*.temp",
	}
}
