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

package controller

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

// NewRestartCommand creates the controller restart command.
func NewRestartCommand() *cobra.Command {
	var (
		timeout time.Duration
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the conductor controller",
		Long: `Restart the conductor controller by stopping and starting it.

This is equivalent to running 'conductor controller stop' followed by
'conductor controller start'. Use this after configuration changes.`,
		Example: `  # Restart controller
  conductor controller restart

  # Restart with force stop
  conductor controller restart --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Stop the controller (ignore errors if not running)
			_ = runStop(cmd.Context(), stopOptions{
				timeout: timeout,
				force:   force,
			})

			// Give it a moment to fully stop
			time.Sleep(100 * time.Millisecond)

			// Start the controller with default timeout
			return runStart(context.Background(), startOptions{
				foreground: false,
				timeout:    30 * time.Second,
			})
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Graceful shutdown timeout before SIGKILL")
	cmd.Flags().BoolVar(&force, "force", false, "Skip graceful shutdown, send SIGKILL immediately")

	return cmd
}
