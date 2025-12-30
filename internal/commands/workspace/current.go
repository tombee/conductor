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

package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewCurrentCommand creates the workspace current command.
func NewCurrentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Show the current workspace",
		Long: `Show the current active workspace.

The current workspace is used by default for all commands that accept a --workspace flag.

Examples:
  # Show current workspace
  conductor workspace current`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			storage, err := getStorage(ctx)
			if err != nil {
				return err
			}
			defer storage.Close()

			// Get current workspace
			workspaceName, err := storage.GetCurrentWorkspace(ctx)
			if err != nil {
				return fmt.Errorf("failed to get current workspace: %w", err)
			}

			// Get workspace details
			ws, err := storage.GetWorkspace(ctx, workspaceName)
			if err != nil {
				return fmt.Errorf("failed to get workspace details: %w", err)
			}

			if shared.GetJSON() {
				output := map[string]interface{}{
					"name":        ws.Name,
					"description": ws.Description,
					"created_at":  ws.CreatedAt,
					"updated_at":  ws.UpdatedAt,
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			fmt.Printf("Current workspace: %s\n", ws.Name)
			if ws.Description != "" {
				fmt.Printf("Description: %s\n", ws.Description)
			}

			// Show integration count
			integrations, err := storage.ListIntegrations(ctx, ws.Name)
			if err == nil {
				fmt.Printf("Integrations: %d\n", len(integrations))
			}

			return nil
		},
	}

	return cmd
}
