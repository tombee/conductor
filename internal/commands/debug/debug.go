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

// Package debug provides CLI commands for debugging workflows.
package debug

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewDebugCommand creates the debug command for workflow debugging.
func NewDebugCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug workflow executions",
		Long: `Debug workflow executions running on the controller.

The debug command group provides tools for interactive workflow debugging,
including attaching to debug sessions and managing active sessions.

NOTE: Remote debugging is not yet implemented. For now, use the --breakpoint
flag with 'conductor run' for local debugging.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("remote debugging is not yet implemented; use 'conductor run --breakpoint <step>' for local debugging")
		},
	}

	cmd.AddCommand(NewAttachCmd())
	cmd.AddCommand(NewSessionsCmd())

	return cmd
}
