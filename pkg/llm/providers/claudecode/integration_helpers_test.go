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

//go:build integration

package claudecode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/operation"
)

// InstrumentedRegistry wraps an operation.Registry to record all executions.
// It implements the claudecode.OperationRegistry interface for use with the Claude Code provider.
type InstrumentedRegistry struct {
	inner      *operation.Registry
	executions []ExecutionRecord
	mu         sync.Mutex
}

// ExecutionRecord captures details of a single operation execution.
type ExecutionRecord struct {
	Reference string
	Inputs    map[string]interface{}
	Result    *operation.Result
	Error     error
	Timestamp time.Time
}

// NewInstrumentedRegistry creates a new instrumented registry wrapping the given registry.
func NewInstrumentedRegistry(inner *operation.Registry) *InstrumentedRegistry {
	return &InstrumentedRegistry{
		inner:      inner,
		executions: make([]ExecutionRecord, 0),
	}
}

// Execute delegates to the inner registry and records the execution.
func (r *InstrumentedRegistry) Execute(ctx context.Context, reference string, inputs map[string]interface{}) (*operation.Result, error) {
	r.mu.Lock()
	record := ExecutionRecord{
		Reference: reference,
		Inputs:    inputs,
		Timestamp: time.Now(),
	}
	r.mu.Unlock()

	result, err := r.inner.Execute(ctx, reference, inputs)

	r.mu.Lock()
	record.Result = result
	record.Error = err
	r.executions = append(r.executions, record)
	r.mu.Unlock()

	return result, err
}

// List delegates to the inner registry.
func (r *InstrumentedRegistry) List() []string {
	return r.inner.List()
}

// Records returns a copy of all execution records.
func (r *InstrumentedRegistry) Records() []ExecutionRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	records := make([]ExecutionRecord, len(r.executions))
	copy(records, r.executions)
	return records
}

// Reset clears all recorded executions.
func (r *InstrumentedRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executions = make([]ExecutionRecord, 0)
}

// skipClaudeCLITests checks if Claude CLI integration tests should run.
// Tests are skipped if:
// - CONDUCTOR_CLAUDE_CLI environment variable is not set to "1"
// - Claude CLI is not installed or not available
func skipClaudeCLITests(t *testing.T) {
	t.Helper()

	// Check environment variable
	if os.Getenv("CONDUCTOR_CLAUDE_CLI") != "1" {
		t.Skip("Claude CLI integration tests skipped. Run 'make test-claude-cli' or set CONDUCTOR_CLAUDE_CLI=1")
	}

	// Check if Claude CLI is available
	_, err := exec.LookPath("claude")
	if err != nil {
		// Also try claude-code
		_, err = exec.LookPath("claude-code")
		if err != nil {
			t.Skip("Claude CLI not found. Install Claude Code to enable integration tests")
		}
	}
}

// testTimeout returns the test timeout from environment or default (60 seconds).
func testTimeout() time.Duration {
	timeoutStr := os.Getenv("CONDUCTOR_TEST_TIMEOUT")
	if timeoutStr != "" {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return 60 * time.Second
}

// setupTestProvider creates a Claude Code provider with an instrumented registry
// containing all builtin actions.
func setupTestProvider(t *testing.T, workDir string) (*Provider, *InstrumentedRegistry) {
	t.Helper()

	// Create builtin registry with the test working directory
	config := &operation.BuiltinConfig{
		WorkflowDir:   workDir,
		AllowAbsolute: true, // Allow absolute paths for temp files
	}

	registry, err := operation.NewBuiltinRegistry(config)
	if err != nil {
		t.Fatalf("Failed to create builtin registry: %v", err)
	}

	// Wrap with instrumentation
	instrumented := NewInstrumentedRegistry(registry)

	// Create provider with instrumented registry
	provider := NewWithRegistry(instrumented)

	// Detect CLI
	if found, err := provider.Detect(); !found || err != nil {
		t.Fatalf("Claude CLI not available: %v", err)
	}

	return provider, instrumented
}

// createTestFile creates a temporary file with the given content.
// Returns the file path. The file is automatically cleaned up when the test completes.
func createTestFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()

	path := fmt.Sprintf("%s/%s", dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return path
}

// claudeBuiltinTools lists all of Claude's built-in tool names that should NOT appear
// in tool_use blocks when using Conductor's tool routing.
var claudeBuiltinTools = []string{
	"Read", "Write", "Edit", "Bash", "Glob", "Grep",
	"LS", "Task", "TodoWrite", "NotebookEdit", "WebFetch", "WebSearch",
}

// conductorToolPattern matches Conductor's operation reference format (namespace.operation).
var conductorToolPattern = regexp.MustCompile(`^[a-z]+\.[a-z_]+$`)

// assertNoBuiltinTools verifies that none of the tool names match Claude's built-in tools.
func assertNoBuiltinTools(t *testing.T, toolNames []string) {
	t.Helper()

	for _, name := range toolNames {
		for _, builtin := range claudeBuiltinTools {
			if name == builtin {
				t.Errorf("Found Claude built-in tool %q in tool calls - tool routing is broken", name)
			}
		}
	}
}

// assertConductorToolFormat verifies that all tool names follow Conductor's namespace.operation format.
func assertConductorToolFormat(t *testing.T, toolNames []string) {
	t.Helper()

	for _, name := range toolNames {
		if !conductorToolPattern.MatchString(name) {
			t.Errorf("Tool name %q does not match Conductor format (namespace.operation)", name)
		}
	}
}
