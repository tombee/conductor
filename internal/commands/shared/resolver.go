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
	"fmt"
	"os"
	"path/filepath"

	"github.com/tombee/conductor/internal/remote"
)

// ResolveWorkflowPath resolves a workflow argument to an actual file path.
// If arg is a remote reference (github:...), returns an empty string.
// Use IsRemoteWorkflow to check for remote references first.
// Resolution order for local paths:
// 1. If arg exists as file, return it
// 2. If arg is directory with workflow.yaml, return that
// 3. Try arg.yaml in current directory
// 4. Try arg/workflow.yaml
func ResolveWorkflowPath(arg string) (string, error) {
	// Check if this is a remote reference
	if remote.IsRemote(arg) {
		// Remote references are not resolved to file paths
		// The caller should handle remote references separately
		return "", fmt.Errorf("cannot resolve remote reference as local path: %s", arg)
	}
	// 1. Check if arg exists as-is
	info, err := os.Stat(arg)
	if err == nil {
		// Exists - check if it's a file or directory
		if info.IsDir() {
			// It's a directory - look for workflow.yaml inside
			workflowPath := filepath.Join(arg, "workflow.yaml")
			if _, err := os.Stat(workflowPath); err == nil {
				return workflowPath, nil
			}
			return "", fmt.Errorf("directory %q exists but does not contain workflow.yaml", arg)
		}
		// It's a file
		return arg, nil
	}

	// 2. Not found as-is. Try arg.yaml in current directory
	yamlPath := arg + ".yaml"
	if _, err := os.Stat(yamlPath); err == nil {
		return yamlPath, nil
	}

	// 3. Try arg/workflow.yaml
	dirWorkflowPath := filepath.Join(arg, "workflow.yaml")
	if _, err := os.Stat(dirWorkflowPath); err == nil {
		return dirWorkflowPath, nil
	}

	// 4. Nothing found - return helpful error
	return "", fmt.Errorf("workflow not found: tried %q, %q, and %q", arg, yamlPath, dirWorkflowPath)
}
