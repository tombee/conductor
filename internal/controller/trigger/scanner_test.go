// Package trigger provides workflow trigger scanning and registration.
package trigger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewScanner(t *testing.T) {
	s := NewScanner("/path/to/workflows")
	if s == nil {
		t.Fatal("NewScanner() returned nil")
	}
	if s.workflowsDir != "/path/to/workflows" {
		t.Errorf("workflowsDir = %v, want /path/to/workflows", s.workflowsDir)
	}
}

func TestScanner_Scan_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.WebhookTriggers) != 0 {
		t.Errorf("WebhookTriggers = %d, want 0", len(result.WebhookTriggers))
	}
	if len(result.ScheduleTriggers) != 0 {
		t.Errorf("ScheduleTriggers = %d, want 0", len(result.ScheduleTriggers))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors = %d, want 0", len(result.Errors))
	}
}

func TestScanner_Scan_WorkflowWithWebhook(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a workflow with a webhook trigger
	workflowContent := `
name: webhook-handler
description: Handles webhook events

listen:
  webhook:
    path: /webhooks/test
    secret: ${WEBHOOK_SECRET}

steps:
  - id: process
    type: llm
    prompt: "Process: {{.inputs.payload}}"
`
	err := os.WriteFile(filepath.Join(tmpDir, "webhook.yaml"), []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.WebhookTriggers) != 1 {
		t.Errorf("WebhookTriggers = %d, want 1", len(result.WebhookTriggers))
	}
	if len(result.ScheduleTriggers) != 0 {
		t.Errorf("ScheduleTriggers = %d, want 0", len(result.ScheduleTriggers))
	}

	if len(result.WebhookTriggers) > 0 {
		trigger := result.WebhookTriggers[0]
		if trigger.WorkflowName != "webhook-handler" {
			t.Errorf("WorkflowName = %v, want webhook-handler", trigger.WorkflowName)
		}
		if trigger.Trigger.Webhook == nil || trigger.Trigger.Webhook.Path != "/webhooks/test" {
			t.Errorf("Path = %v, want /webhooks/test", trigger.Trigger.Webhook)
		}
	}
}

func TestScanner_Scan_WorkflowWithSchedule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a workflow with a schedule trigger
	workflowContent := `
name: scheduled-task
description: Runs on schedule

listen:
  schedule:
    cron: "0 * * * *"
    timezone: UTC

steps:
  - id: run
    type: llm
    prompt: "Run scheduled task"
`
	err := os.WriteFile(filepath.Join(tmpDir, "scheduled.yaml"), []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.ScheduleTriggers) != 1 {
		t.Errorf("ScheduleTriggers = %d, want 1", len(result.ScheduleTriggers))
	}

	if len(result.ScheduleTriggers) > 0 {
		trigger := result.ScheduleTriggers[0]
		if trigger.WorkflowName != "scheduled-task" {
			t.Errorf("WorkflowName = %v, want scheduled-task", trigger.WorkflowName)
		}
		if trigger.Trigger.Schedule == nil || trigger.Trigger.Schedule.Cron != "0 * * * *" {
			t.Errorf("Cron = %v, want 0 * * * *", trigger.Trigger.Schedule)
		}
	}
}

func TestScanner_Scan_MultipleTriggers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create separate workflows for each trigger type
	// (workflows can only have one trigger type each)
	webhookWorkflow := `
name: webhook-multi
description: Webhook trigger

listen:
  webhook:
    path: /webhooks/multi

steps:
  - id: run
    type: llm
    prompt: "Run task"
`
	scheduleWorkflow := `
name: schedule-multi
description: Schedule trigger

listen:
  schedule:
    cron: "0 0 * * *"

steps:
  - id: run
    type: llm
    prompt: "Run task"
`
	err := os.WriteFile(filepath.Join(tmpDir, "webhook.yaml"), []byte(webhookWorkflow), 0644)
	if err != nil {
		t.Fatalf("Failed to write webhook workflow: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "schedule.yaml"), []byte(scheduleWorkflow), 0644)
	if err != nil {
		t.Fatalf("Failed to write schedule workflow: %v", err)
	}

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.WebhookTriggers) != 1 {
		t.Errorf("WebhookTriggers = %d, want 1", len(result.WebhookTriggers))
	}
	if len(result.ScheduleTriggers) != 1 {
		t.Errorf("ScheduleTriggers = %d, want 1", len(result.ScheduleTriggers))
	}
}

func TestScanner_Scan_NoTriggers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a workflow without triggers
	workflowContent := `
name: no-trigger
description: No automatic triggers

steps:
  - id: run
    type: llm
    prompt: "Run task"
`
	err := os.WriteFile(filepath.Join(tmpDir, "no-trigger.yaml"), []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.WebhookTriggers) != 0 {
		t.Errorf("WebhookTriggers = %d, want 0", len(result.WebhookTriggers))
	}
	if len(result.ScheduleTriggers) != 0 {
		t.Errorf("ScheduleTriggers = %d, want 0", len(result.ScheduleTriggers))
	}
}

func TestScanner_Scan_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid YAML file
	err := os.WriteFile(filepath.Join(tmpDir, "invalid.yaml"), []byte("invalid: yaml: syntax:"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v (should not fail)", err)
	}

	// Should have an error recorded but not fail the scan
	if len(result.Errors) == 0 {
		t.Error("Should have recorded an error for invalid YAML")
	}
}

func TestScanner_Scan_NonYAMLFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create non-YAML files
	err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("text file"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "script.sh"), []byte("#!/bin/bash"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Also add a valid YAML workflow
	workflowContent := `
name: test
steps:
  - id: run
    type: llm
    prompt: test
`
	err = os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Should have no errors for non-YAML files
	if len(result.Errors) != 0 {
		t.Errorf("Should not have errors for non-YAML files: %v", result.Errors)
	}
}

func TestScanner_Scan_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	nestedDir := filepath.Join(tmpDir, "subdir", "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	// Create workflow in nested directory
	workflowContent := `
name: nested-workflow
listen:
  webhook:
    path: /webhooks/nested
steps:
  - id: run
    type: llm
    prompt: test
`
	err := os.WriteFile(filepath.Join(nestedDir, "workflow.yaml"), []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.WebhookTriggers) != 1 {
		t.Errorf("WebhookTriggers = %d, want 1", len(result.WebhookTriggers))
	}
}

func TestScanner_Scan_YMLExtension(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workflow with .yml extension
	workflowContent := `
name: yml-workflow
listen:
  schedule:
    cron: "0 0 * * *"
steps:
  - id: run
    type: llm
    prompt: test
`
	err := os.WriteFile(filepath.Join(tmpDir, "workflow.yml"), []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.ScheduleTriggers) != 1 {
		t.Errorf("ScheduleTriggers = %d, want 1", len(result.ScheduleTriggers))
	}
}

func TestScanner_Scan_NonExistentDir(t *testing.T) {
	s := NewScanner("/non/existent/directory")
	result, err := s.Scan()
	// filepath.Walk returns error when dir doesn't exist
	// Check that we get an error either as return or in result
	if err == nil && len(result.Errors) == 0 {
		t.Error("Scan() should fail or record error for non-existent directory")
	}
}

func TestExpandSecret(t *testing.T) {
	// Save and restore environment
	savedValue := os.Getenv("TEST_SECRET")
	defer os.Setenv("TEST_SECRET", savedValue)

	os.Setenv("TEST_SECRET", "my-secret-value")

	tests := []struct {
		name   string
		secret string
		want   string
	}{
		{
			name:   "env var reference",
			secret: "${TEST_SECRET}",
			want:   "my-secret-value",
		},
		{
			name:   "plain string",
			secret: "plain-secret",
			want:   "plain-secret",
		},
		{
			name:   "partial match - no prefix",
			secret: "TEST_SECRET}",
			want:   "TEST_SECRET}",
		},
		{
			name:   "partial match - no suffix",
			secret: "${TEST_SECRET",
			want:   "${TEST_SECRET",
		},
		{
			name:   "empty string",
			secret: "",
			want:   "",
		},
		{
			name:   "unset env var",
			secret: "${UNSET_VAR}",
			want:   "", // os.Getenv returns empty for unset vars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandSecret(tt.secret)
			if got != tt.want {
				t.Errorf("ExpandSecret(%q) = %q, want %q", tt.secret, got, tt.want)
			}
		})
	}
}

func TestWorkflowTrigger_Fields(t *testing.T) {
	trigger := WorkflowTrigger{
		WorkflowPath: "/path/to/workflow.yaml",
		WorkflowName: "test-workflow",
	}

	if trigger.WorkflowPath != "/path/to/workflow.yaml" {
		t.Errorf("WorkflowPath = %v, want /path/to/workflow.yaml", trigger.WorkflowPath)
	}
	if trigger.WorkflowName != "test-workflow" {
		t.Errorf("WorkflowName = %v, want test-workflow", trigger.WorkflowName)
	}
}

func TestScanResult_Fields(t *testing.T) {
	result := &ScanResult{
		WebhookTriggers:  make([]WorkflowTrigger, 0),
		ScheduleTriggers: make([]WorkflowTrigger, 0),
		FileTriggers:     make([]WorkflowTrigger, 0),
		Errors:           make([]error, 0),
	}

	if result.WebhookTriggers == nil {
		t.Error("WebhookTriggers should not be nil")
	}
	if result.ScheduleTriggers == nil {
		t.Error("ScheduleTriggers should not be nil")
	}
	if result.FileTriggers == nil {
		t.Error("FileTriggers should not be nil")
	}
	if result.Errors == nil {
		t.Error("Errors should not be nil")
	}
}

func TestScanner_Scan_WorkflowWithFileTrigger(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a workflow with a file trigger
	workflowContent := `
name: file-watcher
description: Handles file events

listen:
  file:
    paths:
      - /tmp/watch
    events:
      - created
      - modified
    include_patterns:
      - "*.txt"
    exclude_patterns:
      - "*.log"
    debounce: 500ms
    batch_mode: true
    max_triggers_per_minute: 60

steps:
  - id: process
    type: llm
    prompt: "Process file: {{.trigger.file.path}}"
`
	err := os.WriteFile(filepath.Join(tmpDir, "file-watcher.yaml"), []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	s := NewScanner(tmpDir)
	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.FileTriggers) != 1 {
		t.Errorf("FileTriggers = %d, want 1", len(result.FileTriggers))
	}
	if len(result.WebhookTriggers) != 0 {
		t.Errorf("WebhookTriggers = %d, want 0", len(result.WebhookTriggers))
	}
	if len(result.ScheduleTriggers) != 0 {
		t.Errorf("ScheduleTriggers = %d, want 0", len(result.ScheduleTriggers))
	}

	if len(result.FileTriggers) > 0 {
		trigger := result.FileTriggers[0]
		if trigger.WorkflowName != "file-watcher" {
			t.Errorf("WorkflowName = %v, want file-watcher", trigger.WorkflowName)
		}
		if trigger.Trigger.File == nil {
			t.Fatal("File trigger is nil")
		}
		if len(trigger.Trigger.File.Paths) != 1 || trigger.Trigger.File.Paths[0] != "/tmp/watch" {
			t.Errorf("Paths = %v, want [/tmp/watch]", trigger.Trigger.File.Paths)
		}
		if trigger.Trigger.File.Debounce != "500ms" {
			t.Errorf("Debounce = %v, want 500ms", trigger.Trigger.File.Debounce)
		}
		if !trigger.Trigger.File.BatchMode {
			t.Error("BatchMode should be true")
		}
		if trigger.Trigger.File.MaxTriggersPerMinute != 60 {
			t.Errorf("MaxTriggersPerMinute = %d, want 60", trigger.Trigger.File.MaxTriggersPerMinute)
		}
	}
}
