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
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	maxWorkflowFiles = 100
	maxSearchDepth   = 2
)

// workflowFile represents a discovered workflow file with metadata.
type workflowFile struct {
	path    string
	modTime int64
}

// CompleteWorkflowFiles provides dynamic completion for workflow file paths.
// Discovers .yaml and .yml files in the current directory and subdirectories (max 2 levels deep).
// Validates files contain a 'name:' key to confirm they are workflow files.
// Returns paths relative to current directory, limited to 100 files sorted by modification date.
func CompleteWorkflowFiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		// If user is typing "github:", suggest it and stop
		if strings.HasPrefix(toComplete, "github:") || toComplete == "g" || toComplete == "gi" {
			return []string{"github:"}, cobra.ShellCompDirectiveNoSpace
		}

		// Discover YAML files
		files, err := discoverWorkflowFiles(".", maxSearchDepth)
		if err != nil || len(files) == 0 {
			// No local files - suggest github: prefix
			return []string{"github:"}, cobra.ShellCompDirectiveNoSpace
		}

		// Sort by modification time (newest first)
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime > files[j].modTime
		})

		// Limit to maxWorkflowFiles
		if len(files) > maxWorkflowFiles {
			files = files[:maxWorkflowFiles]
		}

		// Extract paths
		var paths []string
		for _, f := range files {
			paths = append(paths, f.path)
		}

		return paths, cobra.ShellCompDirectiveDefault
	})
}

// discoverWorkflowFiles recursively searches for workflow files up to maxDepth levels.
// Returns files sorted by modification time (newest first).
func discoverWorkflowFiles(root string, maxDepth int) ([]workflowFile, error) {
	var files []workflowFile
	currentDepth := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip directories we can't read
			return nil
		}

		// Calculate depth
		relPath, _ := filepath.Rel(root, path)
		depth := strings.Count(relPath, string(filepath.Separator))

		// Skip if too deep
		if depth > maxDepth {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != root {
			return fs.SkipDir
		}

		// Only process YAML files
		if d.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}

		// Check for symlinks and resolve them
		if !isSafeFile(path) {
			return nil
		}

		// Validate it's a workflow file (has 'name:' key)
		if !isWorkflowFile(path) {
			return nil
		}

		// Get modification time
		info, err := d.Info()
		if err != nil {
			return nil
		}

		files = append(files, workflowFile{
			path:    path,
			modTime: info.ModTime().Unix(),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Update currentDepth tracking
	_ = currentDepth

	return files, nil
}

// isSafeFile checks if a file path is safe (no symlinks in the final component).
// Uses filepath.EvalSymlinks to detect and reject symlink paths.
func isSafeFile(path string) bool {
	// Check if the file itself is a symlink
	info, err := os.Lstat(path)
	if err != nil {
		// If we can't stat, assume unsafe
		return false
	}

	// If the file itself is a symlink, reject it
	if info.Mode()&os.ModeSymlink != 0 {
		return false
	}

	// The file itself is not a symlink, so it's safe
	// (We don't check parent directories as those are typically system-managed)
	return true
}

// isWorkflowFile validates that a YAML file contains a 'name:' key at the top level.
// Returns false if file cannot be parsed or doesn't have a name key.
func isWorkflowFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		// Can't read file - skip silently
		return false
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// Can't parse YAML - skip silently
		return false
	}

	// Check for 'name' key
	_, hasName := doc["name"]
	return hasName
}
