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

// Package endpoint provides CLI commands for managing API endpoints.
package endpoint

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the endpoint command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoint",
		Short: "Manage API endpoints",
		Long: `Manage API endpoints that expose workflows as callable HTTP endpoints.

Endpoints allow external applications to trigger workflows via simple API calls
without managing workflow files directly.

Examples:
  # List all endpoints
  conductor endpoint list

  # Show endpoint details
  conductor endpoint show review-pr

  # Add a new endpoint (interactive)
  conductor endpoint add

  # Add a new endpoint (direct)
  conductor endpoint add review-pr --workflow code-review.yaml --scope code-ops`,
	}

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewShowCommand())
	cmd.AddCommand(NewAddCommand())

	return cmd
}
