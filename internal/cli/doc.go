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

/*
Package cli provides the root command and shared configuration for Conductor's CLI.

This package creates the main Cobra command tree and handles global concerns like
version information, persistent flags, and error handling. Individual commands
are implemented in the internal/commands subpackages.

# Command Tree

The CLI is organized as:

	conductor
	├── run           Run a workflow
	├── init          Interactive setup
	├── quickstart    Try an example workflow
	├── validate      Validate workflow YAML
	├── schema        Output JSON schema
	├── config        Configuration management
	├── secrets       Secret management
	├── daemon        Daemon control
	├── runs          Manage workflow runs
	├── mcp           MCP server management
	├── security      Security profile management
	├── version       Show version
	└── help          Show help

# Usage

From main.go:

	cli.SetVersion(version, commit, date)
	rootCmd := cli.NewRootCommand()
	// ... add commands ...
	if err := rootCmd.Execute(); err != nil {
	    cli.HandleExitError(err)
	}

# Global Flags

All commands inherit these flags:

	--verbose, -v    Enable verbose output
	--quiet, -q      Suppress non-error output
	--json           Output in JSON format
	--config         Path to config file

# Error Handling

Errors are handled centrally to ensure proper exit codes:

  - Exit 0: Success
  - Exit 1: General error
  - Exit 2: Invalid usage

Use HandleExitError for consistent error handling:

	if err := cmd.Execute(); err != nil {
	    cli.HandleExitError(err)
	}

# Command Registration

Commands are registered in their respective packages:

	// In internal/commands/run/run.go
	func init() {
	    shared.RegisterCommand(NewRunCommand())
	}
*/
package cli
