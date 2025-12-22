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

triggers:
  - type: webhook
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

triggers:
  - type: schedule
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

	// Create a workflow with multiple triggers
	workflowContent := `
name: multi-trigger
description: Has both webhook and schedule

triggers:
  - type: webhook
    webhook:
      path: /webhooks/multi
  - type: schedule
    schedule:
      cron: "0 0 * * *"

steps:
  - id: run
    type: llm
    prompt: "Run task"
`
	err := os.WriteFile(filepath.Join(tmpDir, "multi.yaml"), []byte(workflowContent), 0644)
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
triggers:
  - type: webhook
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
triggers:
  - type: schedule
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
		Errors:           make([]error, 0),
	}

	if result.WebhookTriggers == nil {
		t.Error("WebhookTriggers should not be nil")
	}
	if result.ScheduleTriggers == nil {
		t.Error("ScheduleTriggers should not be nil")
	}
	if result.Errors == nil {
		t.Error("Errors should not be nil")
	}
}
