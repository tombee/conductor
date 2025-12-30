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
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestConfig_New(t *testing.T) {
	cfg := New([]string{"step1", "step2"}, "debug")

	if !cfg.Enabled {
		t.Error("Expected config to be enabled with breakpoints")
	}

	if len(cfg.Breakpoints) != 2 {
		t.Errorf("Expected 2 breakpoints, got %d", len(cfg.Breakpoints))
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got %s", cfg.LogLevel)
	}
}

func TestConfig_Validate(t *testing.T) {
	def := &workflow.Definition{
		Steps: []workflow.StepDefinition{
			{ID: "step1"},
			{ID: "step2"},
			{ID: "step3"},
		},
	}

	tests := []struct {
		name        string
		breakpoints []string
		wantErr     bool
	}{
		{
			name:        "valid breakpoints",
			breakpoints: []string{"step1", "step2"},
			wantErr:     false,
		},
		{
			name:        "invalid breakpoint",
			breakpoints: []string{"step1", "invalid"},
			wantErr:     true,
		},
		{
			name:        "all invalid",
			breakpoints: []string{"invalid1", "invalid2"},
			wantErr:     true,
		},
		{
			name:        "no breakpoints",
			breakpoints: []string{},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New(tt.breakpoints, "debug")
			err := cfg.Validate(def)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_ShouldPauseAt(t *testing.T) {
	cfg := New([]string{"step1", "step3"}, "debug")

	tests := []struct {
		stepID string
		want   bool
	}{
		{"step1", true},
		{"step2", false},
		{"step3", true},
		{"step4", false},
	}

	for _, tt := range tests {
		t.Run(tt.stepID, func(t *testing.T) {
			if got := cfg.ShouldPauseAt(tt.stepID); got != tt.want {
				t.Errorf("ShouldPauseAt(%s) = %v, want %v", tt.stepID, got, tt.want)
			}
		})
	}
}

func TestConfig_AddBreakpoint(t *testing.T) {
	cfg := New([]string{}, "debug")

	if cfg.Enabled {
		t.Error("Expected config to be disabled with no breakpoints")
	}

	cfg.AddBreakpoint("step1")

	if !cfg.Enabled {
		t.Error("Expected config to be enabled after adding breakpoint")
	}

	if !cfg.ShouldPauseAt("step1") {
		t.Error("Expected to pause at step1 after adding breakpoint")
	}

	// Adding duplicate should not change count
	cfg.AddBreakpoint("step1")
	if len(cfg.Breakpoints) != 1 {
		t.Errorf("Expected 1 breakpoint, got %d", len(cfg.Breakpoints))
	}
}

func TestConfig_RemoveBreakpoint(t *testing.T) {
	cfg := New([]string{"step1", "step2"}, "debug")

	cfg.RemoveBreakpoint("step1")

	if cfg.ShouldPauseAt("step1") {
		t.Error("Expected not to pause at step1 after removal")
	}

	if !cfg.Enabled {
		t.Error("Expected config to still be enabled with remaining breakpoint")
	}

	cfg.RemoveBreakpoint("step2")

	if cfg.Enabled {
		t.Error("Expected config to be disabled after removing all breakpoints")
	}
}

func TestConfig_ClearBreakpoints(t *testing.T) {
	cfg := New([]string{"step1", "step2", "step3"}, "debug")

	cfg.ClearBreakpoints()

	if cfg.Enabled {
		t.Error("Expected config to be disabled after clearing breakpoints")
	}

	if len(cfg.Breakpoints) != 0 {
		t.Errorf("Expected 0 breakpoints, got %d", len(cfg.Breakpoints))
	}
}
