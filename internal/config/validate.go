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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tombee/conductor/pkg/workflow"
)

// ValidatePublicAPIRequirements validates that workflows requiring public API
// have the public API enabled in the controller configuration.
// Returns an error listing all workflows that require public API when it's disabled.
func ValidatePublicAPIRequirements(cfg *Config) error {
	// If public API is enabled, no validation needed
	if cfg.Controller.Listen.PublicAPI.Enabled {
		return nil
	}

	// If no workflows directory configured, skip validation
	if cfg.Controller.WorkflowsDir == "" {
		return nil
	}

	// Scan workflows for listen.webhook and listen.api configurations
	var workflowsRequiringPublicAPI []string

	err := filepath.Walk(cfg.Controller.WorkflowsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files we can't access
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process YAML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Read and parse workflow
		data, err := os.ReadFile(path)
		if err != nil {
			// Skip files we can't read
			return nil
		}

		def, err := workflow.ParseDefinition(data)
		if err != nil {
			// Skip invalid workflows - they'll be caught by other validation
			return nil
		}

		// Check if workflow has public API listeners
		if def.Trigger != nil {
			if def.Trigger.Webhook != nil {
				workflowsRequiringPublicAPI = append(workflowsRequiringPublicAPI,
					fmt.Sprintf("%s (has listen.webhook)", def.Name))
			}
			if def.Trigger.API != nil {
				workflowsRequiringPublicAPI = append(workflowsRequiringPublicAPI,
					fmt.Sprintf("%s (has listen.api)", def.Name))
			}
		}

		return nil
	})

	if err != nil {
		// Walk error shouldn't fail startup, log and continue
		return nil
	}

	// If any workflows require public API, return error
	if len(workflowsRequiringPublicAPI) > 0 {
		return fmt.Errorf(
			"public API is disabled but the following workflows require it:\n  %s\n\n"+
				"To fix this, either:\n"+
				"  1. Enable public API in controller config:\n"+
				"     controller:\n"+
				"       listen:\n"+
				"         public_api:\n"+
				"           enabled: true\n"+
				"           tcp: 127.0.0.1:8081\n"+
				"  2. Or set environment variables:\n"+
				"     CONDUCTOR_PUBLIC_API_ENABLED=true\n"+
				"     CONDUCTOR_PUBLIC_API_TCP=127.0.0.1:8081\n"+
				"  3. Or remove listen.webhook/listen.api from workflows that don't need external triggers",
			strings.Join(workflowsRequiringPublicAPI, "\n  "),
		)
	}

	return nil
}
