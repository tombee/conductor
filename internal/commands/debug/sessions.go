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

	"github.com/spf13/cobra"
)

// NewSessionsCmd creates the debug sessions command.
func NewSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage debug sessions",
		Long: `List and manage active debug sessions on the controller.

NOTE: Remote debugging is not yet implemented. For now, use the --breakpoint
flag with 'conductor run' for local debugging.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("remote debugging is not yet implemented; use 'conductor run --breakpoint <step>' for local debugging")
		},
	}

	cmd.AddCommand(NewSessionsListCmd())
	cmd.AddCommand(NewSessionsKillCmd())

	return cmd
}

// NewSessionsListCmd creates the debug sessions list command.
func NewSessionsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active debug sessions",
		Long: `List all active debug sessions on the controller.

NOTE: Remote debugging is not yet implemented. For now, use the --breakpoint
flag with 'conductor run' for local debugging.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("remote debugging is not yet implemented; use 'conductor run --breakpoint <step>' for local debugging")
		},
	}

	return cmd
}

// NewSessionsKillCmd creates the debug sessions kill command.
func NewSessionsKillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kill <session-id>",
		Short: "Kill a debug session",
		Long: `Terminate a debug session and its associated run.

NOTE: Remote debugging is not yet implemented. For now, use the --breakpoint
flag with 'conductor run' for local debugging.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("remote debugging is not yet implemented; use 'conductor run --breakpoint <step>' for local debugging")
		},
	}

	return cmd
}
