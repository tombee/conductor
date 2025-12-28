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

package override

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/output"
)

type listFlags struct {
	json bool
}

func newListCommand() *cobra.Command {
	flags := &listFlags{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active security overrides",
		Long:  `List all currently active security overrides.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// List overrides via daemon API
			url := shared.BuildAPIURL("/v1/override", nil)
			body, err := shared.MakeAPIRequest("GET", url, nil)
			if err != nil {
				return fmt.Errorf("failed to list overrides: %w", err)
			}

			// Parse response
			var response struct {
				Overrides []struct {
					Type      string    `json:"type"`
					Reason    string    `json:"reason"`
					ExpiresAt time.Time `json:"expires_at"`
					CreatedAt time.Time `json:"created_at"`
				} `json:"overrides"`
			}
			if err := json.Unmarshal(body, &response); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			// Format output
			if flags.json {
				return output.EmitJSON(response)
			}

			if len(response.Overrides) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No active overrides")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Active overrides (%d):\n\n", len(response.Overrides))
			for _, override := range response.Overrides {
				fmt.Fprintf(cmd.OutOrStdout(), "Type:       %s\n", override.Type)
				fmt.Fprintf(cmd.OutOrStdout(), "Reason:     %s\n", override.Reason)
				fmt.Fprintf(cmd.OutOrStdout(), "Expires at: %s\n", override.ExpiresAt.Format(time.RFC3339))
				fmt.Fprintf(cmd.OutOrStdout(), "Created at: %s\n", override.CreatedAt.Format(time.RFC3339))
				fmt.Fprintln(cmd.OutOrStdout())
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&flags.json, "json", false, "Output in JSON format")

	return cmd
}
