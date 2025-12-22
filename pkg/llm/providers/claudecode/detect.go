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
	"regexp"
	"strings"
	"time"
)

// Detect checks if the Claude Code CLI is available in the system PATH
func (p *Provider) Detect() (bool, error) {
	// Try both 'claude' and 'claude-code' commands
	for _, cmd := range []string{"claude", "claude-code"} {
		if path, err := exec.LookPath(cmd); err == nil {
			p.cliCommand = cmd
			p.cliPath = path
			return true, nil
		}
	}
	return false, nil
}

// detectVersion attempts to get the Claude CLI version
func (p *Provider) detectVersion(ctx context.Context) (string, error) {
	if p.cliCommand == "" {
		if found, _ := p.Detect(); !found {
			return "", fmt.Errorf("claude CLI not found in PATH")
		}
	}

	// Create a context with timeout for version check
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.cliCommand, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get version: %w (stderr: %s)", err, stderr.String())
	}

	// Parse version from output
	// Expected formats:
	// - "claude version X.Y.Z"
	// - "X.Y.Z"
	// - "claude-code X.Y.Z"
	output := strings.TrimSpace(stdout.String())
	versionRegex := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// If we got output but couldn't parse version, return the full output
	if output != "" {
		return output, nil
	}

	return "unknown", nil
}
