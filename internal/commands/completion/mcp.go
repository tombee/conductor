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

package completion

import (
	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/mcp"
)

// CompleteMCPServerNames provides completion for MCP server names.
// Reads server names from the global MCP configuration file.
func CompleteMCPServerNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		// Get the MCP config path
		configPath, err := mcp.MCPConfigPath()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Check file permissions before loading
		if !CheckFilePermissions(configPath) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Load the MCP configuration
		cfg, err := mcp.LoadMCPConfig()
		if err != nil || cfg == nil || len(cfg.Servers) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Extract server names
		names := make([]string, 0, len(cfg.Servers))
		for name := range cfg.Servers {
			names = append(names, name)
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	})
}
