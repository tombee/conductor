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

package debug

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestShell_ParseCommand(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	shell := NewShell(adapter)

	tests := []struct {
		name    string
		input   string
		wantCmd CommandType
		wantErr bool
	}{
		{
			name:    "continue command",
			input:   "continue",
			wantCmd: CommandContinue,
			wantErr: false,
		},
		{
			name:    "continue shorthand",
			input:   "c",
			wantCmd: CommandContinue,
			wantErr: false,
		},
		{
			name:    "next command",
			input:   "next",
			wantCmd: CommandNext,
			wantErr: false,
		},
		{
			name:    "next shorthand",
			input:   "n",
			wantCmd: CommandNext,
			wantErr: false,
		},
		{
			name:    "skip command",
			input:   "skip",
			wantCmd: CommandSkip,
			wantErr: false,
		},
		{
			name:    "abort command",
			input:   "abort",
			wantCmd: CommandAbort,
			wantErr: false,
		},
		{
			name:    "inspect command with arg",
			input:   "inspect myvar",
			wantCmd: CommandInspect,
			wantErr: false,
		},
		{
			name:    "inspect without arg",
			input:   "inspect",
			wantCmd: CommandInspect,
			wantErr: true,
		},
		{
			name:    "context command",
			input:   "context",
			wantCmd: CommandContext,
			wantErr: false,
		},
		{
			name:    "help command",
			input:   "help",
			wantCmd: "",
			wantErr: true, // help displays help and returns error
		},
		{
			name:    "unknown command",
			input:   "unknown",
			wantCmd: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := shell.parseCommand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cmd.Type != tt.wantCmd {
				t.Errorf("parseCommand() cmd.Type = %v, want %v", cmd.Type, tt.wantCmd)
			}
		})
	}
}

func TestShell_HandleEvent_StepStart(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		Type:      EventStepStart,
		StepID:    "test_step",
		StepIndex: 0,
		Timestamp: time.Now(),
	}

	err := shell.handleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("handleEvent() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "test_step") {
		t.Errorf("Expected output to contain step ID, got: %s", outputStr)
	}
}

func TestShell_HandleEvent_Resumed(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		Type:      EventResumed,
		StepID:    "test_step",
		Timestamp: time.Now(),
	}

	err := shell.handleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("handleEvent() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Resuming") {
		t.Errorf("Expected output to contain 'Resuming', got: %s", outputStr)
	}
}

func TestShell_HandleEvent_Skipped(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		Type:      EventSkipped,
		StepID:    "test_step",
		Timestamp: time.Now(),
	}

	err := shell.handleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("handleEvent() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Skipped") {
		t.Errorf("Expected output to contain 'Skipped', got: %s", outputStr)
	}
}

func TestShell_HandleEvent_Completed(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		Type:      EventCompleted,
		Timestamp: time.Now(),
	}

	err := shell.handleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("handleEvent() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "completed") {
		t.Errorf("Expected output to contain 'completed', got: %s", outputStr)
	}

	// Verify quit channel was closed
	select {
	case <-shell.quit:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected quit channel to be closed")
	}
}

func TestShell_HandleEvent_Aborted(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		Type:      EventAborted,
		Timestamp: time.Now(),
	}

	err := shell.handleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("handleEvent() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "aborted") {
		t.Errorf("Expected output to contain 'aborted', got: %s", outputStr)
	}

	// Verify quit channel was closed
	select {
	case <-shell.quit:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected quit channel to be closed")
	}
}

func TestShell_HandleInspect(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		Snapshot: map[string]interface{}{
			"test_key": "test_value",
		},
	}

	shell.handleInspect(event, []string{"test_key"})

	outputStr := output.String()
	if !strings.Contains(outputStr, "test_key") {
		t.Errorf("Expected output to contain 'test_key', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "test_value") {
		t.Errorf("Expected output to contain 'test_value', got: %s", outputStr)
	}
}

func TestShell_HandleInspect_MissingKey(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		Snapshot: map[string]interface{}{
			"test_key": "test_value",
		},
	}

	shell.handleInspect(event, []string{"missing_key"})

	outputStr := output.String()
	if !strings.Contains(outputStr, "not found") {
		t.Errorf("Expected output to contain 'not found', got: %s", outputStr)
	}
}

func TestShell_HandleContext(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		Snapshot: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}

	shell.handleContext(event)

	outputStr := output.String()
	if !strings.Contains(outputStr, "key1") {
		t.Errorf("Expected output to contain 'key1', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "key2") {
		t.Errorf("Expected output to contain 'key2', got: %s", outputStr)
	}
}

func TestShell_HandleContext_Empty(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		Snapshot: map[string]interface{}{},
	}

	shell.handleContext(event)

	outputStr := output.String()
	if !strings.Contains(outputStr, "empty") {
		t.Errorf("Expected output to contain 'empty', got: %s", outputStr)
	}
}

func TestShell_ShowHelp(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	shell.showHelp()

	outputStr := output.String()
	expectedCommands := []string{"continue", "next", "skip", "abort", "inspect", "context", "help"}
	for _, cmd := range expectedCommands {
		if !strings.Contains(outputStr, cmd) {
			t.Errorf("Expected help output to contain '%s', got: %s", cmd, outputStr)
		}
	}
}

func TestShell_DisplayStepInfo(t *testing.T) {
	config := New([]string{}, "info")
	adapter := NewAdapter(config, slog.Default())
	defer adapter.Close()

	var output bytes.Buffer
	shell := NewShell(adapter)
	shell.output = &output

	event := &Event{
		StepID:    "test_step",
		StepIndex: 2,
		Snapshot: map[string]interface{}{
			"var1": "value1",
			"var2": "value2",
		},
	}

	shell.displayStepInfo(event)

	outputStr := output.String()
	if !strings.Contains(outputStr, "test_step") {
		t.Errorf("Expected output to contain step ID, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "var1") {
		t.Errorf("Expected output to contain 'var1', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "var2") {
		t.Errorf("Expected output to contain 'var2', got: %s", outputStr)
	}
}
