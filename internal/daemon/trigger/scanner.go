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

// Package trigger provides workflow trigger scanning and registration.
package trigger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tombee/conductor/pkg/workflow"
)

// WorkflowTrigger represents a trigger found in a workflow file.
type WorkflowTrigger struct {
	// WorkflowPath is the path to the workflow file
	WorkflowPath string

	// WorkflowName is the name of the workflow
	WorkflowName string

	// Trigger is the trigger definition
	Trigger workflow.TriggerDefinition
}

// ScanResult contains the results of scanning workflows for triggers.
type ScanResult struct {
	// WebhookTriggers are all webhook triggers found
	WebhookTriggers []WorkflowTrigger

	// ScheduleTriggers are all schedule triggers found
	ScheduleTriggers []WorkflowTrigger

	// Errors are any errors encountered while scanning
	Errors []error
}

// Scanner scans workflow files for trigger definitions.
type Scanner struct {
	workflowsDir string
}

// NewScanner creates a new trigger scanner.
func NewScanner(workflowsDir string) *Scanner {
	return &Scanner{
		workflowsDir: workflowsDir,
	}
}

// Scan scans all workflow files in the configured directory.
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{
		WebhookTriggers:  make([]WorkflowTrigger, 0),
		ScheduleTriggers: make([]WorkflowTrigger, 0),
		Errors:           make([]error, 0),
	}

	// Walk the workflows directory
	err := filepath.Walk(s.workflowsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("error accessing %s: %w", path, err))
			return nil // Continue scanning
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

		// Scan the workflow file
		triggers, err := s.scanWorkflow(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("error scanning %s: %w", path, err))
			return nil // Continue scanning
		}

		// Categorize triggers
		for _, t := range triggers {
			switch t.Trigger.Type {
			case workflow.TriggerTypeWebhook:
				result.WebhookTriggers = append(result.WebhookTriggers, t)
			case workflow.TriggerTypeSchedule:
				result.ScheduleTriggers = append(result.ScheduleTriggers, t)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk workflows directory: %w", err)
	}

	return result, nil
}

// scanWorkflow scans a single workflow file for triggers.
func (s *Scanner) scanWorkflow(path string) ([]WorkflowTrigger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	def, err := workflow.ParseDefinition(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	if len(def.Triggers) == 0 {
		return nil, nil
	}

	triggers := make([]WorkflowTrigger, 0, len(def.Triggers))
	for _, t := range def.Triggers {
		triggers = append(triggers, WorkflowTrigger{
			WorkflowPath: path,
			WorkflowName: def.Name,
			Trigger:      t,
		})
	}

	return triggers, nil
}

// ExpandSecret expands environment variable references in a secret string.
// Supports ${VAR_NAME} syntax.
func ExpandSecret(secret string) string {
	if !strings.HasPrefix(secret, "${") || !strings.HasSuffix(secret, "}") {
		return secret
	}

	envVar := strings.TrimSuffix(strings.TrimPrefix(secret, "${"), "}")
	return os.Getenv(envVar)
}
