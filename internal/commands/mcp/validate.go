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

	fmt.Printf("%s %s\n\n", shared.Header.Render("Validating MCP server:"), name)

	issues := 0
	warnings := 0

	// Check name format
	fmt.Print("  Name format.............. ")
	if isValidServerName(name) {
		fmt.Println(shared.StatusOK.Render("OK"))
	} else {
		fmt.Println(shared.StatusError.Render("INVALID"))
		fmt.Printf("    %s Name must start with a letter and contain only letters, numbers, hyphens, and underscores\n", shared.StatusError.Render("Error:"))
		issues++
	}

	if server.Config != nil {
		// Check command
		fmt.Print("  Command.................. ")
		if server.Config.Command != "" {
			fmt.Printf("'%s' ", server.Config.Command)
			// Note: We can't check if command exists from CLI since controller runs it
			fmt.Printf("%s\n", shared.Muted.Render("(check on controller)"))
		} else {
			fmt.Println(shared.StatusError.Render("MISSING"))
			fmt.Printf("    %s Command is required\n", shared.StatusError.Render("Error:"))
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
			fmt.Println(shared.StatusWarn.Render("WARNING"))
			fmt.Printf("    %s %d argument(s) contain shell metacharacters\n", shared.StatusWarn.Render("Warning:"), argIssues)
			warnings++
		} else if len(server.Config.Args) > 0 {
			fmt.Printf("%s (%d args)\n", shared.StatusOK.Render("OK"), len(server.Config.Args))
		} else {
			fmt.Printf("%s %s\n", shared.StatusOK.Render("OK"), shared.Muted.Render("(none)"))
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
			fmt.Println(shared.StatusWarn.Render("WARNING"))
			fmt.Printf("    %s %d env var(s) missing '=' separator\n", shared.StatusWarn.Render("Warning:"), envIssues)
			warnings++
		} else if len(server.Config.Env) > 0 {
			fmt.Printf("%s (%d vars)\n", shared.StatusOK.Render("OK"), len(server.Config.Env))
		} else {
			fmt.Printf("%s %s\n", shared.StatusOK.Render("OK"), shared.Muted.Render("(none)"))
		}

		// Check timeout
		fmt.Print("  Timeout.................. ")
		if server.Config.Timeout > 0 {
			fmt.Printf("%s (%ds)\n", shared.StatusOK.Render("OK"), server.Config.Timeout)
		} else {
			fmt.Printf("%s %s\n", shared.StatusOK.Render("OK"), shared.Muted.Render("(default)"))
		}
	} else {
		fmt.Printf("  Config................... %s\n", shared.StatusError.Render("NOT FOUND"))
		issues++
	}

	// Summary
	fmt.Println()
	if issues > 0 {
		fmt.Println(shared.RenderError(fmt.Sprintf("Validation FAILED: %d error(s), %d warning(s)", issues, warnings)))
		return fmt.Errorf("validation failed")
	} else if warnings > 0 {
		fmt.Println(shared.RenderWarn(fmt.Sprintf("Validation PASSED with %d warning(s)", warnings)))
	} else {
		fmt.Println(shared.RenderOK("Validation PASSED"))
	}

	return nil
}
