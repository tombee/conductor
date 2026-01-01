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
)

// Validate command

func newMCPValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <name>",
		Short: "Validate an MCP server configuration",
		Long: `Validate an MCP server configuration before starting.

Checks:
- Server name format
- Command exists and is executable
- Arguments don't contain shell injection patterns
- Environment variables are properly formatted

Examples:
  conductor mcp validate github
  conductor mcp validate my-server`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPValidate(args[0])
		},
	}

	return cmd
}

func runMCPValidate(name string) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	// Get server configuration
	data, err := client.get(ctx, "/v1/mcp/servers/"+name)
	if err != nil {
		return err
	}

	var server struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Config *struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
			Env     []string `json:"env"`
			Timeout int      `json:"timeout"`
		} `json:"config"`
	}

	if err := json.Unmarshal(data, &server); err != nil {
		return fmt.Errorf("failed to parse server info: %w", err)
	}

	fmt.Printf("Validating MCP server: %s\n\n", name)

	issues := 0
	warnings := 0

	// Check name format
	fmt.Print("  Name format.............. ")
	if isValidServerName(name) {
		fmt.Println("OK")
	} else {
		fmt.Println("INVALID")
		fmt.Println("    Error: Name must start with a letter and contain only letters, numbers, hyphens, and underscores")
		issues++
	}

	if server.Config != nil {
		// Check command
		fmt.Print("  Command.................. ")
		if server.Config.Command != "" {
			fmt.Printf("'%s' ", server.Config.Command)
			// Note: We can't check if command exists from CLI since controller runs it
			fmt.Println("(check on controller)")
		} else {
			fmt.Println("MISSING")
			fmt.Println("    Error: Command is required")
			issues++
		}

		// Check for shell injection in args
		fmt.Print("  Arguments................ ")
		argIssues := 0
		for _, arg := range server.Config.Args {
			if containsShellMeta(arg) {
				argIssues++
			}
		}
		if argIssues > 0 {
			fmt.Println("WARNING")
			fmt.Printf("    Warning: %d argument(s) contain shell metacharacters\n", argIssues)
			warnings++
		} else if len(server.Config.Args) > 0 {
			fmt.Printf("OK (%d args)\n", len(server.Config.Args))
		} else {
			fmt.Println("OK (none)")
		}

		// Check environment variables
		fmt.Print("  Environment variables.... ")
		envIssues := 0
		for _, env := range server.Config.Env {
			if !strings.Contains(env, "=") {
				envIssues++
			}
		}
		if envIssues > 0 {
			fmt.Println("WARNING")
			fmt.Printf("    Warning: %d env var(s) missing '=' separator\n", envIssues)
			warnings++
		} else if len(server.Config.Env) > 0 {
			fmt.Printf("OK (%d vars)\n", len(server.Config.Env))
		} else {
			fmt.Println("OK (none)")
		}

		// Check timeout
		fmt.Print("  Timeout.................. ")
		if server.Config.Timeout > 0 {
			fmt.Printf("OK (%ds)\n", server.Config.Timeout)
		} else {
			fmt.Println("OK (default)")
		}
	} else {
		fmt.Println("  Config................... NOT FOUND")
		issues++
	}

	// Summary
	fmt.Println()
	if issues > 0 {
		fmt.Printf("Validation FAILED: %d error(s), %d warning(s)\n", issues, warnings)
		return fmt.Errorf("validation failed")
	} else if warnings > 0 {
		fmt.Printf("Validation PASSED with %d warning(s)\n", warnings)
	} else {
		fmt.Println("Validation PASSED")
	}

	return nil
}
