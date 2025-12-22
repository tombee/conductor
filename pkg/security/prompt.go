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

package security

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// PermissionResponse represents user's response to permission prompt.
type PermissionResponse string

const (
	// PermissionYes allows permission for this run only
	PermissionYes PermissionResponse = "y"

	// PermissionNo denies permission and aborts
	PermissionNo PermissionResponse = "n"

	// PermissionAlways grants and saves permission for future runs
	PermissionAlways PermissionResponse = "always"

	// PermissionNever denies and remembers to always deny
	PermissionNever PermissionResponse = "never"
)

// PermissionRequest describes permissions being requested.
type PermissionRequest struct {
	WorkflowName string
	Filesystem   *FilesystemPermissions
	Network      *NetworkPermissions
	Commands     *CommandPermissions
}

// FilesystemPermissions describes filesystem access requests.
type FilesystemPermissions struct {
	Read  []string
	Write []string
}

// NetworkPermissions describes network access requests.
type NetworkPermissions struct {
	Hosts []string
}

// CommandPermissions describes command execution requests.
type CommandPermissions struct {
	Commands []string
}

// Prompter handles interactive permission prompts.
type Prompter interface {
	// Prompt displays a permission request and returns the user's response.
	Prompt(req PermissionRequest) (PermissionResponse, error)
}

// StdioPrompter prompts user via stdin/stdout.
type StdioPrompter struct {
	reader io.Reader
	writer io.Writer
}

// NewStdioPrompter creates a new prompter using stdin/stdout.
func NewStdioPrompter() *StdioPrompter {
	return &StdioPrompter{
		reader: os.Stdin,
		writer: os.Stdout,
	}
}

// NewPrompterWithIO creates a prompter with custom IO streams (for testing).
func NewPrompterWithIO(reader io.Reader, writer io.Writer) *StdioPrompter {
	return &StdioPrompter{
		reader: reader,
		writer: writer,
	}
}

// Prompt displays permission request and waits for user response.
func (p *StdioPrompter) Prompt(req PermissionRequest) (PermissionResponse, error) {
	// Check if running in non-interactive mode
	if !isInteractive() {
		return PermissionNo, fmt.Errorf("permission prompt in non-interactive mode: defaulting to deny")
	}

	// Build prompt message
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\nWorkflow '%s' requests additional permissions:\n\n", req.WorkflowName))

	hasPermissions := false

	// Filesystem permissions
	if req.Filesystem != nil && (len(req.Filesystem.Read) > 0 || len(req.Filesystem.Write) > 0) {
		hasPermissions = true
		sb.WriteString("  Filesystem:\n")
		for _, path := range req.Filesystem.Read {
			sb.WriteString(fmt.Sprintf("    [+] Read: %s\n", path))
		}
		for _, path := range req.Filesystem.Write {
			sb.WriteString(fmt.Sprintf("    [+] Write: %s\n", path))
		}
		sb.WriteString("\n")
	}

	// Network permissions
	if req.Network != nil && len(req.Network.Hosts) > 0 {
		hasPermissions = true
		sb.WriteString("  Network:\n")
		for _, host := range req.Network.Hosts {
			sb.WriteString(fmt.Sprintf("    [+] %s\n", host))
		}
		sb.WriteString("\n")
	}

	// Command permissions
	if req.Commands != nil && len(req.Commands.Commands) > 0 {
		hasPermissions = true
		sb.WriteString("  Commands:\n")
		for _, cmd := range req.Commands.Commands {
			sb.WriteString(fmt.Sprintf("    [+] %s\n", cmd))
		}
		sb.WriteString("\n")
	}

	if !hasPermissions {
		return PermissionNo, fmt.Errorf("no permissions requested")
	}

	sb.WriteString("Allow these permissions? [y/N/always/never]: ")

	// Write prompt
	if _, err := p.writer.Write([]byte(sb.String())); err != nil {
		return PermissionNo, fmt.Errorf("failed to write prompt: %w", err)
	}

	// Read response
	scanner := bufio.NewScanner(p.reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return PermissionNo, fmt.Errorf("failed to read response: %w", err)
		}
		// EOF or no input - default to No
		return PermissionNo, nil
	}

	response := strings.TrimSpace(strings.ToLower(scanner.Text()))

	switch response {
	case "y", "yes":
		return PermissionYes, nil
	case "n", "no", "":
		// Default is No
		return PermissionNo, nil
	case "always", "a":
		return PermissionAlways, nil
	case "never", "nev":
		return PermissionNever, nil
	default:
		return PermissionNo, fmt.Errorf("invalid response: %s (expected y/N/always/never)", response)
	}
}

// NonInteractivePrompter always denies permission requests (for CI/automation).
type NonInteractivePrompter struct{}

// NewNonInteractivePrompter creates a prompter that always denies.
func NewNonInteractivePrompter() *NonInteractivePrompter {
	return &NonInteractivePrompter{}
}

// Prompt always returns PermissionNo for non-interactive environments.
func (p *NonInteractivePrompter) Prompt(req PermissionRequest) (PermissionResponse, error) {
	return PermissionNo, fmt.Errorf("permission prompt in non-interactive mode: workflow '%s' requires additional permissions", req.WorkflowName)
}

// isInteractive checks if stdin is a terminal.
func isInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
