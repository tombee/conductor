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
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
)

// newMCPAddCommand creates the 'mcp add' command.
func newMCPAddCommand() *cobra.Command {
	var (
		command   string
		args      []string
		env       []string
		timeout   int
		autoStart bool
		dryRun    bool
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register a new global MCP server",
		Long: `Register a new global MCP server.

The server configuration is saved to ~/.config/conductor/mcp.yaml.

Examples:
  conductor mcp add github --command npx --args "-y" --args "@modelcontextprotocol/server-github"
  conductor mcp add my-server --command python --args "server.py" --env "DEBUG=true"
  conductor mcp add db --command ./db-server --timeout 60 --auto-start`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			return runMCPAdd(cmdArgs[0], command, args, env, timeout, autoStart, dryRun)
		},
	}

	cmd.Flags().StringVar(&command, "command", "", "Command to run (required)")
	cmd.Flags().StringArrayVar(&args, "args", nil, "Command arguments (can be repeated)")
	cmd.Flags().StringArrayVar(&env, "env", nil, "Environment variables in KEY=VALUE format (can be repeated)")
	cmd.Flags().IntVar(&timeout, "timeout", 30, "Timeout for tool calls in seconds")
	cmd.Flags().BoolVar(&autoStart, "auto-start", false, "Start automatically when controller starts")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be added without executing")

	cmd.MarkFlagRequired("command")

	return cmd
}

func runMCPAdd(name, command string, args, env []string, timeout int, autoStart bool, dryRun bool) error {
	// Handle dry-run mode
	if dryRun {
		return mcpAddDryRun(name, command, args, env, timeout, autoStart)
	}

	client := newMCPAPIClient()
	ctx := context.Background()

	reqBody := map[string]any{
		"name":       name,
		"command":    command,
		"args":       args,
		"env":        env,
		"timeout":    timeout,
		"auto_start": autoStart,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	_, err = client.post(ctx, "/v1/mcp/servers", strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	fmt.Printf("Registered MCP server: %s\n", name)
	fmt.Println("\nTo start the server:")
	fmt.Printf("  conductor mcp start %s\n", name)

	return nil
}

// newMCPRemoveCommand creates the 'mcp remove' command.
func newMCPRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a global MCP server",
		Long: `Remove a global MCP server.

If the server is running, it will be stopped first.

Examples:
  conductor mcp remove github`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPRemove(args[0])
		},
	}

	return cmd
}

func runMCPRemove(name string) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	if err := client.delete(ctx, "/v1/mcp/servers/"+name); err != nil {
		return err
	}

	fmt.Printf("Removed MCP server: %s\n", name)
	return nil
}

// mcpAddDryRun shows what would be added when registering an MCP server
func mcpAddDryRun(name, command string, args, env []string, timeout int, autoStart bool) error {
	// Get config directory for placeholder
	configDir, err := config.ConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	output := shared.NewDryRunOutput()

	// Build MCP config path placeholder
	mcpConfigPath := "<config-dir>/mcp.yaml"

	// Build description of what would be added
	description := fmt.Sprintf("register MCP server '%s' (command: %s", name, command)
	if len(args) > 0 {
		description += fmt.Sprintf(", args: %v", args)
	}
	if len(env) > 0 {
		// Mask environment variables that might contain sensitive data
		maskedEnv := make([]string, len(env))
		for i, e := range env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				maskedValue := shared.MaskSensitiveData(parts[0], parts[1])
				maskedEnv[i] = parts[0] + "=" + maskedValue
			} else {
				maskedEnv[i] = e
			}
		}
		description += fmt.Sprintf(", env: %v", maskedEnv)
	}
	if timeout != 30 {
		description += fmt.Sprintf(", timeout: %ds", timeout)
	}
	if autoStart {
		description += ", auto-start: true"
	}
	description += ")"

	// Since the config might not exist, use MODIFY (updates or creates)
	output.DryRunModify(mcpConfigPath, description)

	// Print the output
	fmt.Println(output.String())

	// Suppress unused variable warning
	_ = configDir

	return nil
}
