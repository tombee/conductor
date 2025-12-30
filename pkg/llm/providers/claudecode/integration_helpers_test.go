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
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

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
