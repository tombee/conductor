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

package triggers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/triggers"
	"gopkg.in/yaml.v3"
)

// getManager creates a trigger manager from the current config.
func getManager() (*triggers.Manager, error) {
	cfgPath := shared.GetConfigPath()
	if cfgPath == "" {
		var err error
		cfgPath, err = config.ConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to determine config path: %w", err)
		}
	}

	// Load config to get workflows directory
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	workflowsDir := cfg.Controller.WorkflowsDir
	if workflowsDir == "" {
		// Default to current directory if not specified
		workflowsDir = "."
	}

	// Resolve to absolute path
	if !filepath.IsAbs(workflowsDir) {
		absPath, err := filepath.Abs(workflowsDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve workflows directory: %w", err)
		}
		workflowsDir = absPath
	}

	return triggers.NewManager(cfgPath, workflowsDir), nil
}

// getWebhookURL constructs the webhook URL from the path.
// This is a placeholder - in production, this would use the controller's configured host.
func getWebhookURL(path string) string {
	// TODO: Get actual host from controller config
	return fmt.Sprintf("https://<controller-host>%s", path)
}

// getEndpointURL constructs the API endpoint URL from the name.
// This is a placeholder - in production, this would use the controller's configured host.
func getEndpointURL(name string) string {
	// TODO: Get actual host from controller config
	return fmt.Sprintf("https://<controller-host>/v1/endpoints/%s", name)
}
