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
	"fmt"
	"slices"

	"github.com/tombee/conductor/pkg/workflow"
)

// Config holds debugging configuration for workflow execution.
type Config struct {
	// Breakpoints is a list of step IDs where execution should pause.
	Breakpoints []string

	// LogLevel overrides the default log level for this execution.
	// Valid values: trace, debug, info, warn, error
	LogLevel string

	// Enabled indicates whether debugging is active.
	Enabled bool
}

// New creates a new debug configuration.
func New(breakpoints []string, logLevel string) *Config {
	return &Config{
		Breakpoints: breakpoints,
		LogLevel:    logLevel,
		Enabled:     len(breakpoints) > 0,
	}
}

// Validate checks that the debug configuration is valid for the given workflow.
func (c *Config) Validate(def *workflow.Definition) error {
	if !c.Enabled {
		return nil
	}

	// Build a set of valid step IDs from the workflow
	validSteps := make(map[string]bool)
	for _, step := range def.Steps {
		validSteps[step.ID] = true
	}

	// Check that all breakpoints reference valid steps
	var invalidSteps []string
	for _, bp := range c.Breakpoints {
		if !validSteps[bp] {
			invalidSteps = append(invalidSteps, bp)
		}
	}

	if len(invalidSteps) > 0 {
		return fmt.Errorf("invalid breakpoint step IDs: %v", invalidSteps)
	}

	return nil
}

// ShouldPauseAt returns true if execution should pause at the given step.
func (c *Config) ShouldPauseAt(stepID string) bool {
	if !c.Enabled {
		return false
	}
	return slices.Contains(c.Breakpoints, stepID)
}

// AddBreakpoint adds a new breakpoint to the configuration.
func (c *Config) AddBreakpoint(stepID string) {
	if !slices.Contains(c.Breakpoints, stepID) {
		c.Breakpoints = append(c.Breakpoints, stepID)
		c.Enabled = true
	}
}

// RemoveBreakpoint removes a breakpoint from the configuration.
func (c *Config) RemoveBreakpoint(stepID string) {
	c.Breakpoints = slices.DeleteFunc(c.Breakpoints, func(s string) bool {
		return s == stepID
	})
	if len(c.Breakpoints) == 0 {
		c.Enabled = false
	}
}

// ClearBreakpoints removes all breakpoints.
func (c *Config) ClearBreakpoints() {
	c.Breakpoints = nil
	c.Enabled = false
}
