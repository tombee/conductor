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

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tombee/conductor/internal/commands/shared"
)

const docsBaseURL = "https://tombee.github.io/conductor"

// CommandMetadata represents metadata about a command for JSON output
type CommandMetadata struct {
	Name        string         `json:"name"`
	Short       string         `json:"short"`
	Long        string         `json:"long,omitempty"`
	Usage       string         `json:"usage"`
	Flags       []FlagMetadata `json:"flags,omitempty"`
	Examples    string         `json:"examples,omitempty"`
	Subcommands []string       `json:"subcommands,omitempty"`
	Group       string         `json:"group,omitempty"`
	Aliases     []string       `json:"aliases,omitempty"`
}

// FlagMetadata represents metadata about a flag
type FlagMetadata struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Usage     string `json:"usage"`
	Default   string `json:"default,omitempty"`
	Required  bool   `json:"required"`
}

// HelpResponse is the JSON response for help command
type HelpResponse struct {
	shared.JSONResponse
	Commands    []CommandMetadata `json:"commands,omitempty"`
	Command     *CommandMetadata  `json:"command,omitempty"`
	GlobalFlags []FlagMetadata    `json:"global_flags,omitempty"`
	DocsURL     string            `json:"docs_url"`
}

// NewHelpCommand creates the help command
func NewHelpCommand(rootCmd *cobra.Command) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides detailed information about commands and their usage.

Run 'conductor help' to see all available commands.
Run 'conductor help <command>' to see detailed help for a specific command.
Use --json flag to get machine-readable output for LLM agents.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			useJSON := shared.GetJSON() || jsonOutput

			if len(args) == 0 {
				// Show all commands
				if useJSON {
					return outputAllCommandsJSON(cmd, rootCmd)
				}
				return rootCmd.Help()
			}

			// Find the specific command
			targetCmd, _, err := rootCmd.Find(args)
			if err != nil {
				return fmt.Errorf("command %q not found", args[0])
			}

			if useJSON {
				return outputCommandJSON(cmd, targetCmd, rootCmd)
			}

			return targetCmd.Help()
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

// outputAllCommandsJSON outputs all commands in JSON format
func outputAllCommandsJSON(cmd *cobra.Command, rootCmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	commands := []CommandMetadata{}
	for _, c := range rootCmd.Commands() {
		if c.Hidden {
			continue
		}
		commands = append(commands, extractCommandMetadata(c))
	}

	resp := HelpResponse{
		JSONResponse: shared.JSONResponse{
			Version: "1.0",
			Command: "help",
			Success: true,
		},
		Commands:    commands,
		GlobalFlags: extractGlobalFlags(rootCmd),
		DocsURL:     docsBaseURL + "/reference/cli/",
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resp)
}

// outputCommandJSON outputs a specific command in JSON format
func outputCommandJSON(cmd *cobra.Command, targetCmd *cobra.Command, rootCmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	metadata := extractCommandMetadata(targetCmd)

	resp := HelpResponse{
		JSONResponse: shared.JSONResponse{
			Version: "1.0",
			Command: "help " + targetCmd.Name(),
			Success: true,
		},
		Command:     &metadata,
		GlobalFlags: extractGlobalFlags(rootCmd),
		DocsURL:     docsBaseURL + "/reference/cli/",
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resp)
}

// extractCommandMetadata extracts metadata from a cobra command
func extractCommandMetadata(cmd *cobra.Command) CommandMetadata {
	metadata := CommandMetadata{
		Name:     cmd.Name(),
		Short:    cmd.Short,
		Long:     cmd.Long,
		Usage:    cmd.UseLine(),
		Examples: cmd.Example,
		Aliases:  cmd.Aliases,
	}

	// Extract group from annotations
	if cmd.Annotations != nil {
		if group, ok := cmd.Annotations["group"]; ok {
			metadata.Group = group
		}
	}

	// Extract flags
	flags := []FlagMetadata{}
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		flagMeta := FlagMetadata{
			Name:      flag.Name,
			Shorthand: flag.Shorthand,
			Usage:     flag.Usage,
			Default:   flag.DefValue,
		}
		// Check if flag is required (this is a heuristic, Cobra doesn't expose this directly)
		if flag.Value.Type() == "string" && flag.DefValue == "" {
			// This is a rough heuristic - in practice we'd need better tracking
			flagMeta.Required = false
		}
		flags = append(flags, flagMeta)
	})
	if len(flags) > 0 {
		metadata.Flags = flags
	}

	// Extract subcommands
	subcommands := []string{}
	for _, sub := range cmd.Commands() {
		if !sub.Hidden {
			subcommands = append(subcommands, sub.Name())
		}
	}
	if len(subcommands) > 0 {
		metadata.Subcommands = subcommands
	}

	return metadata
}

// extractGlobalFlags extracts global flags from root command
func extractGlobalFlags(rootCmd *cobra.Command) []FlagMetadata {
	flags := []FlagMetadata{}
	rootCmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		flags = append(flags, FlagMetadata{
			Name:      flag.Name,
			Shorthand: flag.Shorthand,
			Usage:     flag.Usage,
			Default:   flag.DefValue,
			Required:  false,
		})
	})
	return flags
}
