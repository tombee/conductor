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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/commands/shared"
)

// newMCPToolsCommand creates the 'mcp tools' command.
func newMCPToolsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools <name>",
		Short: "List tools available from an MCP server",
		Long: `List all tools exposed by an MCP server.

The server must be running to list its tools.

Examples:
  conductor mcp tools github
  conductor mcp tools github --json`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteMCPServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPTools(args[0])
		},
	}

	return cmd
}

func runMCPTools(name string) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	data, err := client.get(ctx, "/v1/mcp/servers/"+name+"/tools")
	if err != nil {
		return err
	}

	if shared.GetJSON() {
		fmt.Println(string(data))
		return nil
	}

	var resp struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Tools) == 0 {
		fmt.Println("No tools available from this server.")
		return nil
	}

	fmt.Printf("Tools from %s:\n\n", name)
	for _, t := range resp.Tools {
		fmt.Printf("  %s.%s\n", name, t.Name)
		if t.Description != "" {
			// Wrap description at 60 chars
			desc := wrapText(t.Description, 60)
			for _, line := range strings.Split(desc, "\n") {
				fmt.Printf("    %s\n", line)
			}
		}
		fmt.Println()
	}

	return nil
}
