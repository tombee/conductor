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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// Shell provides an interactive debugging interface for workflow execution.
type Shell struct {
	adapter *Adapter
	input   io.Reader
	output  io.Writer
	quit    chan struct{}
}

// NewShell creates a new debug shell connected to the given adapter.
func NewShell(adapter *Adapter) *Shell {
	return &Shell{
		adapter: adapter,
		input:   os.Stdin,
		output:  os.Stdout,
		quit:    make(chan struct{}),
	}
}

// Run starts the interactive debugging shell.
// It listens for debug events and provides a command prompt when paused.
func (s *Shell) Run(ctx context.Context) error {
	// Set up signal handling for Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-s.quit:
			return nil

		case <-sigCh:
			fmt.Fprintln(s.output, "\nInterrupt received. Type 'abort' to stop execution, or 'continue' to resume.")

		case event := <-s.adapter.EventChan():
			if event == nil {
				// Channel closed, execution complete
				return nil
			}

			if err := s.handleEvent(ctx, event); err != nil {
				return err
			}
		}
	}
}

// handleEvent processes a debug event and takes appropriate action.
func (s *Shell) handleEvent(ctx context.Context, event *Event) error {
	switch event.Type {
	case EventStepStart:
		fmt.Fprintf(s.output, "→ Step: %s (index: %d)\n", event.StepID, event.StepIndex)

	case EventPaused:
		return s.promptForCommand(ctx, event)

	case EventResumed:
		fmt.Fprintln(s.output, "Resuming execution...")

	case EventSkipped:
		fmt.Fprintf(s.output, "⊘ Skipped: %s\n", event.StepID)

	case EventCompleted:
		fmt.Fprintln(s.output, "✓ Workflow completed")
		close(s.quit)

	case EventAborted:
		fmt.Fprintln(s.output, "✗ Execution aborted")
		close(s.quit)
	}

	return nil
}

// promptForCommand displays a prompt and waits for user input.
func (s *Shell) promptForCommand(ctx context.Context, event *Event) error {
	s.displayStepInfo(event)

	scanner := bufio.NewScanner(s.input)

	for {
		fmt.Fprint(s.output, "debug> ")

		if !scanner.Scan() {
			// EOF or error
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("input error: %w", err)
			}
			// EOF - treat as abort
			s.adapter.CommandChan() <- &Command{Type: CommandAbort}
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		cmd, err := s.parseCommand(line)
		if err != nil {
			fmt.Fprintf(s.output, "Error: %v\n", err)
			continue
		}

		// Handle inspect and context commands locally
		if cmd.Type == CommandInspect {
			s.handleInspect(event, cmd.Args)
			continue
		}

		if cmd.Type == CommandContext {
			s.handleContext(event)
			continue
		}

		// Send command to adapter
		select {
		case s.adapter.CommandChan() <- cmd:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// parseCommand parses a command string into a Command struct.
func (s *Shell) parseCommand(line string) (*Command, error) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmdStr := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmdStr {
	case "c", "continue":
		return &Command{Type: CommandContinue}, nil

	case "n", "next":
		return &Command{Type: CommandNext}, nil

	case "s", "skip":
		return &Command{Type: CommandSkip}, nil

	case "a", "abort":
		return &Command{Type: CommandAbort}, nil

	case "i", "inspect":
		if len(args) == 0 {
			return nil, fmt.Errorf("inspect requires an expression argument")
		}
		return &Command{Type: CommandInspect, Args: args}, nil

	case "ctx", "context":
		return &Command{Type: CommandContext}, nil

	case "h", "help", "?":
		s.showHelp()
		return nil, fmt.Errorf("help displayed")

	default:
		return nil, fmt.Errorf("unknown command: %s (type 'help' for commands)", cmdStr)
	}
}

// displayStepInfo shows information about the current step.
func (s *Shell) displayStepInfo(event *Event) {
	fmt.Fprintln(s.output, "\n═══════════════════════════════════════════════════════════")
	fmt.Fprintf(s.output, "Paused at step: %s (index: %d)\n", event.StepID, event.StepIndex)
	fmt.Fprintln(s.output, "───────────────────────────────────────────────────────────")

	// Show current context snapshot
	if len(event.Snapshot) > 0 {
		fmt.Fprintln(s.output, "Context:")
		for key := range event.Snapshot {
			fmt.Fprintf(s.output, "  - %s\n", key)
		}
	} else {
		fmt.Fprintln(s.output, "Context: (empty)")
	}

	fmt.Fprintln(s.output, "═══════════════════════════════════════════════════════════")
	fmt.Fprintln(s.output, "Commands: continue, next, skip, abort, inspect <expr>, context, help")
	fmt.Fprintln(s.output)
}

// handleInspect evaluates an expression against the current context.
func (s *Shell) handleInspect(event *Event, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(s.output, "Error: inspect requires an expression")
		return
	}

	key := args[0]
	value, ok := event.Snapshot[key]
	if !ok {
		fmt.Fprintf(s.output, "Key '%s' not found in context\n", key)
		return
	}

	// Pretty-print the value as JSON
	jsonBytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(s.output, "Error formatting value: %v\n", err)
		return
	}

	fmt.Fprintf(s.output, "%s = %s\n", key, string(jsonBytes))
}

// handleContext dumps the full workflow context.
func (s *Shell) handleContext(event *Event) {
	if len(event.Snapshot) == 0 {
		fmt.Fprintln(s.output, "Context is empty")
		return
	}

	jsonBytes, err := json.MarshalIndent(event.Snapshot, "", "  ")
	if err != nil {
		fmt.Fprintf(s.output, "Error formatting context: %v\n", err)
		return
	}

	fmt.Fprintln(s.output, "Full Context:")
	fmt.Fprintln(s.output, string(jsonBytes))
}

// showHelp displays available commands.
func (s *Shell) showHelp() {
	help := `
Debug Commands:
  continue, c      Resume execution until next breakpoint or completion
  next, n          Step to the next step
  skip, s          Skip the current step and proceed
  abort, a         Cancel execution immediately
  inspect <key>, i Show the value of a context variable
  context, ctx     Dump the full workflow context as JSON
  help, h, ?       Show this help message

Press Ctrl+C to interrupt (then choose to abort or continue)
`
	fmt.Fprintln(s.output, help)
}

// Close closes the shell and releases resources.
func (s *Shell) Close() {
	close(s.quit)
}
