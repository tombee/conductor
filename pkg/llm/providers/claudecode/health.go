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

package claudecode

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/tombee/conductor/pkg/llm"
)

// HealthCheck performs a three-step verification of Claude CLI status
func (p *Provider) HealthCheck(ctx context.Context) llm.HealthCheckResult {
	result := llm.HealthCheckResult{}

	// Step 1: Check if CLI is installed
	found, err := p.Detect()
	if err != nil {
		result.Error = err
		result.ErrorStep = llm.HealthCheckStepInstalled
		result.Message = "Failed to detect Claude CLI"
		return result
	}
	if !found {
		result.Error = fmt.Errorf("claude CLI not found in PATH")
		result.ErrorStep = llm.HealthCheckStepInstalled
		result.Message = installationGuidance()
		return result
	}
	result.Installed = true

	// Get version for reporting
	version, err := p.detectVersion(ctx)
	if err == nil {
		result.Version = version
	}

	// Step 2 & 3: Check authentication and connectivity with a single API call
	// Claude CLI doesn't have an 'auth status' command, so we verify both
	// auth and connectivity by making a minimal API request.
	if err, isAuthError := p.checkWorking(ctx); err != nil {
		if isAuthError {
			result.Error = err
			result.ErrorStep = llm.HealthCheckStepAuthenticated
			result.Message = authenticationGuidance()
			return result
		}
		result.Authenticated = true // Auth passed but connectivity failed
		result.Error = err
		result.ErrorStep = llm.HealthCheckStepWorking
		result.Message = workingGuidance(err)
		return result
	}
	result.Authenticated = true
	result.Working = true

	result.Message = "Claude CLI is healthy and ready"
	return result
}

// checkWorking performs a lightweight connectivity test
// Returns: error, isAuthError
func (p *Provider) checkWorking(ctx context.Context) (error, bool) {
	// Use a very short prompt to test connectivity
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.cliCommand, "-p", "--max-budget-usd", "0.01", "respond with just: ok")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrOutput := stderr.String()
		// Check for authentication-related errors
		if strings.Contains(stderrOutput, "not authenticated") ||
			strings.Contains(stderrOutput, "not logged in") ||
			strings.Contains(stderrOutput, "authentication") ||
			strings.Contains(stderrOutput, "API key") ||
			strings.Contains(stderrOutput, "unauthorized") {
			return fmt.Errorf("authentication failed: %s", stderrOutput), true
		}
		return fmt.Errorf("connectivity test failed: %w (stderr: %s)", err, stderrOutput), false
	}

	return nil, false
}

// installationGuidance returns platform-specific installation instructions
func installationGuidance() string {
	return `Claude CLI not found. To install Claude Code:

macOS/Linux:
  Visit https://claude.ai/download or use your package manager

Verify installation:
  claude --version

After installation, authenticate with:
  claude auth login`
}

// authenticationGuidance returns instructions for authenticating the CLI
func authenticationGuidance() string {
	return `Claude CLI is not authenticated. To authenticate:

  claude auth login

This will open a browser window to complete authentication.
Once authenticated, try your command again.`
}

// workingGuidance returns troubleshooting guidance based on the error
func workingGuidance(err error) string {
	errStr := err.Error()

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
		return `Connection timeout. Possible issues:
  - Network connectivity problems
  - Anthropic API service may be unavailable
  - Firewall blocking requests

Try again in a moment, or check your network connection.`
	}

	if strings.Contains(errStr, "rate limit") {
		return `Rate limit reached. Please wait a moment and try again.`
	}

	return fmt.Sprintf(`Claude CLI test request failed: %v

Possible issues:
  - Network connectivity problems
  - Anthropic API service may be unavailable
  - Session may have expired

Try re-authenticating with:
  claude auth login`, err)
}
