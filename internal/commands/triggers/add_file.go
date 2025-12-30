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

package triggers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/triggers"
)

var (
	filePath              string
	fileEvents            []string
	fileIncludePatterns   []string
	fileExcludePatterns   []string
	fileDebounce          string
	fileBatchMode         bool
	fileRateLimit         int
	fileInputs            []string
)

func newAddFileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file WORKFLOW",
		Short: "Add a file watcher trigger",
		Long: `Add a file watcher trigger that invokes a workflow when files change.

The file watcher monitors a filesystem path for changes and triggers the workflow
when matching events occur. Supports pattern matching, debouncing, and rate limiting.

Event types:
  - created:  File or directory created
  - modified: File or directory modified
  - deleted:  File or directory deleted
  - renamed:  File or directory renamed

Patterns support extended glob syntax via doublestar:
  - * matches any sequence of non-path-separators
  - ** matches any sequence including path separators (recursive)
  - ? matches a single non-path-separator
  - [class] matches any character in the class

Common exclude patterns are applied by default (editor temp files, .git, etc).`,
		Example: `  # Watch for new PDF files in downloads
  conductor triggers add file process-pdf.yaml \
    --path=~/Downloads \
    --events=created \
    --include="*.pdf"

  # Watch source code changes with debouncing
  conductor triggers add file run-tests.yaml \
    --path=./src \
    --events=modified \
    --include="**/*.go" \
    --exclude="**/*_test.go" \
    --debounce=2s

  # Batch mode for processing multiple files together
  conductor triggers add file batch-import.yaml \
    --path=./data/incoming \
    --events=created \
    --debounce=5s \
    --batch

  # Rate limiting to avoid overload
  conductor triggers add file backup.yaml \
    --path=/var/log/app \
    --events=modified \
    --include="*.log" \
    --rate-limit=10

  # Dry-run to preview changes
  conductor triggers add file cleanup.yaml \
    --path=/tmp/uploads \
    --events=created \
    --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: runAddFile,
	}

	cmd.Flags().StringVar(&filePath, "path", "", "Filesystem path to watch (required)")
	cmd.Flags().StringSliceVar(&fileEvents, "events", []string{"created", "modified", "deleted", "renamed"}, "Event types to watch")
	cmd.Flags().StringSliceVar(&fileIncludePatterns, "include", nil, "Include glob patterns (repeatable)")
	cmd.Flags().StringSliceVar(&fileExcludePatterns, "exclude", nil, "Exclude glob patterns (repeatable)")
	cmd.Flags().StringVar(&fileDebounce, "debounce", "", "Debounce window (e.g., 1s, 500ms)")
	cmd.Flags().BoolVar(&fileBatchMode, "batch", false, "Batch events during debounce window")
	cmd.Flags().IntVar(&fileRateLimit, "rate-limit", 0, "Max triggers per minute (0 = unlimited)")
	cmd.Flags().StringSliceVar(&fileInputs, "input", nil, "Static inputs: key=value (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")

	cmd.MarkFlagRequired("path")

	return cmd
}

func runAddFile(cmd *cobra.Command, args []string) error {
	workflow := args[0]

	// Parse debounce duration
	var debounceWindow time.Duration
	var err error
	if fileDebounce != "" {
		debounceWindow, err = time.ParseDuration(fileDebounce)
		if err != nil {
			return fmt.Errorf("invalid debounce duration: %w", err)
		}
	}

	// Parse input key-value pairs
	inputs := make(map[string]any)
	for _, input := range fileInputs {
		parts := strings.SplitN(input, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid input format: %s (expected key=value)", input)
		}
		inputs[parts[0]] = parts[1]
	}

	req := triggers.CreateFileWatcherRequest{
		Workflow:             workflow,
		Path:                 filePath,
		Events:               fileEvents,
		IncludePatterns:      fileIncludePatterns,
		ExcludePatterns:      fileExcludePatterns,
		DebounceWindow:       debounceWindow,
		BatchMode:            fileBatchMode,
		MaxTriggersPerMinute: fileRateLimit,
		Inputs:               inputs,
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry-run: Would add file watcher trigger:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Path: %s\n", req.Path)
		fmt.Fprintf(cmd.OutOrStdout(), "  Workflow: %s\n", req.Workflow)
		fmt.Fprintf(cmd.OutOrStdout(), "  Events: %s\n", strings.Join(req.Events, ", "))
		if len(req.IncludePatterns) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Include Patterns: %s\n", strings.Join(req.IncludePatterns, ", "))
		}
		if len(req.ExcludePatterns) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Exclude Patterns: %s\n", strings.Join(req.ExcludePatterns, ", "))
		}
		if req.DebounceWindow > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Debounce: %s\n", req.DebounceWindow)
		}
		if req.BatchMode {
			fmt.Fprintf(cmd.OutOrStdout(), "  Batch Mode: enabled\n")
		}
		if req.MaxTriggersPerMinute > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Rate Limit: %d triggers/minute\n", req.MaxTriggersPerMinute)
		}
		if len(req.Inputs) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Static Inputs:\n")
			for k, v := range req.Inputs {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s = %v\n", k, v)
			}
		}
		return nil
	}

	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	name, err := mgr.AddFileWatcher(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to add file watcher: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "File watcher trigger created: %s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "Watching: %s\n", req.Path)
	if req.DebounceWindow > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Debounce: %s\n", req.DebounceWindow)
	}
	if req.BatchMode {
		fmt.Fprintf(cmd.OutOrStdout(), "Batch mode enabled\n")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nRestart the controller for changes to take effect:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  conductor controller restart\n")

	return nil
}
