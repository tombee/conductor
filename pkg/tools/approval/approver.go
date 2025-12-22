// Package approval provides tool execution approval mechanisms.
package approval

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// ExecutionMode determines how tool approvals are handled.
type ExecutionMode string

const (
	// ModeInteractive prompts the user for approval
	ModeInteractive ExecutionMode = "interactive"

	// ModeUnattended only allows auto-approved tools
	ModeUnattended ExecutionMode = "unattended"
)

// Approver handles tool execution approval decisions.
type Approver interface {
	// Approve returns true if the tool execution should proceed.
	// toolName is the name of the tool being invoked.
	// toolDescription describes what the tool does.
	// inputs are the parameters being passed to the tool.
	Approve(ctx context.Context, toolName string, toolDescription string, inputs map[string]interface{}) (bool, error)
}

// CLIApprover prompts the user for approval via command line.
type CLIApprover struct {
	reader        io.Reader
	writer        io.Writer
	alwaysApprove map[string]bool // Tools the user said "always" to this run
}

// NewCLIApprover creates a new CLI-based approver.
func NewCLIApprover() *CLIApprover {
	return &CLIApprover{
		reader:        os.Stdin,
		writer:        os.Stdout,
		alwaysApprove: make(map[string]bool),
	}
}

// NewCLIApproverWithIO creates a CLI approver with custom IO (for testing).
func NewCLIApproverWithIO(reader io.Reader, writer io.Writer) *CLIApprover {
	return &CLIApprover{
		reader:        reader,
		writer:        writer,
		alwaysApprove: make(map[string]bool),
	}
}

// Approve prompts the user for approval.
// Returns true if approved, false if denied.
func (c *CLIApprover) Approve(ctx context.Context, toolName string, toolDescription string, inputs map[string]interface{}) (bool, error) {
	// Check if user previously said "always" for this tool
	if c.alwaysApprove[toolName] {
		return true, nil
	}

	// Display approval prompt
	fmt.Fprintf(c.writer, "\n")
	fmt.Fprintf(c.writer, "Tool approval required:\n")
	fmt.Fprintf(c.writer, "  Tool: %s\n", toolName)
	fmt.Fprintf(c.writer, "  Description: %s\n", toolDescription)
	if len(inputs) > 0 {
		fmt.Fprintf(c.writer, "  Inputs:\n")
		for k, v := range inputs {
			fmt.Fprintf(c.writer, "    %s: %v\n", k, v)
		}
	}
	fmt.Fprintf(c.writer, "\n")
	fmt.Fprintf(c.writer, "Approve execution? [y/N/always]: ")

	// Read user response
	scanner := bufio.NewScanner(c.reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("failed to read input: %w", err)
		}
		// EOF or no input - default to deny
		return false, nil
	}

	response := strings.ToLower(strings.TrimSpace(scanner.Text()))

	switch response {
	case "y", "yes":
		return true, nil
	case "always":
		// Remember this approval for the rest of the run
		c.alwaysApprove[toolName] = true
		return true, nil
	default:
		// "n", "no", or empty/unknown input - deny
		return false, nil
	}
}

// UnattendedApprover only allows auto-approved tools.
type UnattendedApprover struct {
	autoApprovedTools map[string]bool
}

// NewUnattendedApprover creates an approver for unattended mode.
// It accepts a set of tool names that are auto-approved.
func NewUnattendedApprover(autoApprovedTools map[string]bool) *UnattendedApprover {
	return &UnattendedApprover{
		autoApprovedTools: autoApprovedTools,
	}
}

// Approve returns true only if the tool is in the auto-approved list.
func (u *UnattendedApprover) Approve(ctx context.Context, toolName string, toolDescription string, inputs map[string]interface{}) (bool, error) {
	if u.autoApprovedTools[toolName] {
		return true, nil
	}
	return false, fmt.Errorf("tool %s requires approval but running in unattended mode", toolName)
}
