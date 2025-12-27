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

package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/templates"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

var (
	initAdvanced bool
	initYes      bool
	initForce    bool
	initTemplate string
	initFile     string
	initList     bool
)

// workflowNameRegex validates workflow names to prevent path traversal and invalid filesystem characters
var workflowNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// NewInitCommand creates the init command
func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [name]",
		Annotations: map[string]string{
			"group": "workflow",
		},
		Short: "Initialize Conductor or create a new workflow",
		Long: `Initialize Conductor or create a new workflow from a template.

Without arguments: Runs the setup wizard to configure Conductor providers.
With a name argument: Creates a new workflow file from a template.

Examples:
  conductor init                       # Run setup wizard
  conductor init my-workflow           # Create my-workflow/workflow.yaml
  conductor init --file review.yaml    # Create single file in current directory
  conductor init --template code-review my-review  # Use code-review template
  conductor init --list                # List available templates`,
		RunE: runInit,
		Args: cobra.MaximumNArgs(1),
	}

	cmd.Flags().BoolVar(&initAdvanced, "advanced", false, "Advanced setup with API key configuration")
	cmd.Flags().BoolVar(&initYes, "yes", false, "Accept defaults without prompts (non-interactive)")
	cmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing files")
	cmd.Flags().StringVarP(&initTemplate, "template", "t", "blank", "Template to use for workflow creation")
	cmd.Flags().StringVarP(&initFile, "file", "f", "", "Create single file instead of directory")
	cmd.Flags().BoolVar(&initList, "list", false, "List available templates")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	// Handle --list flag
	if initList {
		return listTemplates()
	}

	// If a name is provided, this is workflow scaffolding
	if len(args) == 1 {
		return runInitWorkflow(args[0])
	}

	// If --file flag is set without a name, create workflow in current directory
	if initFile != "" {
		return runInitWorkflow("")
	}

	// In JSON mode, skip interactive setup
	if shared.GetJSON() {
		errors := []shared.JSONError{
			{
				Code:       shared.ErrorCodeMissingInput,
				Message:    "Interactive setup not supported in JSON mode",
				Suggestion: "Use 'conductor init <workflow-name>' to create a workflow, or run without --json for interactive setup",
			},
		}
		shared.EmitJSONError("init", errors)
		return &shared.ExitError{Code: 1, Message: ""}
	}

	// Otherwise, run provider setup
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Get config directory
	configDir, err := config.ConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		if !initYes {
			fmt.Printf("Configuration file already exists at: %s\n", configPath)
			fmt.Print("Overwrite? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Initialization cancelled.")
				return nil
			}
		} else if !initForce {
			// In non-interactive mode, require --force to overwrite
			return fmt.Errorf("configuration already exists at %s (use --force to overwrite)", configPath)
		}
	}

	if initAdvanced {
		return runAdvancedSetup(ctx, configPath)
	}

	return runSimpleSetup(ctx, configPath)
}

func runSimpleSetup(ctx context.Context, configPath string) error {
	fmt.Println("Conductor Setup Wizard")
	fmt.Println("======================")
	fmt.Println()

	// Step 1: Detect Claude Code CLI
	fmt.Print("Checking for Claude Code CLI... ")
	provider := claudecode.New()

	found, err := provider.Detect()
	if err != nil {
		fmt.Println("ERROR")
		return fmt.Errorf("detection failed: %w", err)
	}

	if !found {
		fmt.Println("NOT FOUND")
		fmt.Println()
		printClaudeInstallInstructions()
		return fmt.Errorf("claude CLI not found in PATH")
	}
	fmt.Println("✓ FOUND")

	// Step 2: Run health check
	fmt.Print("Checking authentication... ")
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result := provider.HealthCheck(ctx)

	if !result.Installed {
		fmt.Println("✗ FAILED")
		fmt.Println()
		fmt.Println(result.Message)
		return fmt.Errorf("health check failed at step: %s", result.ErrorStep)
	}
	fmt.Println("✓ OK")

	if !result.Authenticated {
		fmt.Println()
		fmt.Println(result.Message)
		return fmt.Errorf("authentication required")
	}
	fmt.Print("Testing connectivity... ")

	if !result.Working {
		fmt.Println("✗ FAILED")
		fmt.Println()
		fmt.Println(result.Message)
		return fmt.Errorf("connectivity check failed")
	}
	fmt.Println("✓ OK")

	// Display version if available
	if result.Version != "" {
		fmt.Printf("Claude CLI version: %s\n", result.Version)
	}
	fmt.Println()

	// Step 3: Create configuration
	fmt.Print("Creating configuration file... ")

	providers := config.ProvidersMap{
		"claude": config.ProviderConfig{
			Type: "claude-code",
		},
	}

	if err := config.WriteConfigMinimal("claude", providers, configPath); err != nil {
		fmt.Println("✗ FAILED")
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Println("✓ DONE")
	fmt.Printf("Configuration written to: %s\n", configPath)
	fmt.Println()

	// Step 4: Offer to run quickstart
	if !initYes {
		fmt.Println("Setup complete! 🎉")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  • Run a quickstart example: conductor quickstart")
		fmt.Println("  • List available examples:  conductor examples")
		fmt.Println("  • View configuration:       conductor config show")
		fmt.Println()

		fmt.Println("Try these commands:")
		fmt.Println("  conductor serve   # Start the workflow server")
		fmt.Println("  conductor --help  # View all available commands")
	} else {
		fmt.Println("Setup complete!")
	}

	return nil
}

func runAdvancedSetup(ctx context.Context, configPath string) error {
	fmt.Println("Advanced Setup")
	fmt.Println("==============")
	fmt.Println()
	fmt.Println("This feature will be implemented in a future phase.")
	fmt.Println("For now, please use the default setup (conductor init)")
	return nil
}

func printClaudeInstallInstructions() string {
	instructions := `Claude Code CLI is not installed.

To install Claude Code:

macOS/Linux:
  Visit https://claude.ai/download

After installation, authenticate with:
  claude auth login

Then run 'conductor init' again.`

	fmt.Println(instructions)
	return instructions
}

// validateWorkflowName validates a workflow name to prevent path traversal and invalid characters
func validateWorkflowName(name string) error {
	if name == "" {
		return fmt.Errorf("workflow name cannot be empty")
	}

	if name == "." || name == ".." {
		return fmt.Errorf("workflow name cannot be '.' or '..'")
	}

	if !workflowNameRegex.MatchString(name) {
		return fmt.Errorf("invalid workflow name: must start with a letter and contain only letters, numbers, hyphens, and underscores")
	}

	return nil
}

// listTemplates displays all available workflow templates
func listTemplates() error {
	tmplList, err := templates.List()
	if err != nil {
		return fmt.Errorf("failed to list templates: %w", err)
	}

	fmt.Println("Available workflow templates:")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tCATEGORY\tDESCRIPTION")
	fmt.Fprintln(w, "----\t--------\t-----------")
	for _, tmpl := range tmplList {
		fmt.Fprintf(w, "%s\t%s\t%s\n", tmpl.Name, tmpl.Category, tmpl.Description)
	}
	w.Flush()

	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  conductor init --template <name> <workflow-name>")
	fmt.Println("  conductor init --template code-review my-review")

	return nil
}

// runInitWorkflow creates a new workflow from a template
func runInitWorkflow(name string) error {
	// Validate template exists
	if !templates.Exists(initTemplate) {
		return fmt.Errorf("template %q not found (use --list to see available templates)", initTemplate)
	}

	var targetPath string
	var workflowName string

	// Determine target path and workflow name
	if initFile != "" {
		// Single file mode
		targetPath = initFile
		// Extract workflow name from filename (remove .yaml extension if present)
		workflowName = filepath.Base(targetPath)
		if ext := filepath.Ext(workflowName); ext == ".yaml" || ext == ".yml" {
			workflowName = workflowName[:len(workflowName)-len(ext)]
		}
	} else {
		// Directory mode
		if name == "" {
			name = "workflow"
		}

		// Validate workflow name
		if err := validateWorkflowName(name); err != nil {
			return err
		}

		workflowName = name
		targetPath = filepath.Join(name, "workflow.yaml")
	}

	// Validate workflow name for template rendering
	if err := validateWorkflowName(workflowName); err != nil {
		return fmt.Errorf("invalid workflow name derived from path: %w", err)
	}

	// Check if target already exists
	if _, err := os.Stat(targetPath); err == nil && !initForce {
		return fmt.Errorf("file already exists: %s (use --force to overwrite)", targetPath)
	}

	// Render template
	content, err := templates.Render(initTemplate, workflowName)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Create directory if needed (directory mode)
	if initFile == "" {
		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Write workflow file with restrictive permissions
	if err := os.WriteFile(targetPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}

	// Success output
	if shared.GetJSON() {
		type initResponse struct {
			shared.JSONResponse
			Created  []string `json:"created"`
			Template string   `json:"template"`
		}

		resp := initResponse{
			JSONResponse: shared.JSONResponse{
				Version: "1.0",
				Command: "init",
				Success: true,
			},
			Created:  []string{targetPath},
			Template: initTemplate,
		}

		return shared.EmitJSON(resp)
	}

	// Success message
	fmt.Printf("Created workflow: %s\n", targetPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  conductor validate %s\n", targetPath)
	fmt.Printf("  conductor run %s\n", targetPath)

	return nil
}
