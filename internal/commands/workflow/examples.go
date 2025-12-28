package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/examples"
	"github.com/tombee/conductor/pkg/workflow"
)

// NewExamplesCommand creates the examples command
func NewExamplesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "examples",
		Annotations: map[string]string{
			"group": "workflow",
		},
		Short: "Manage example workflows",
		Long: `Browse, view, run, and copy example workflows.

Examples are embedded in the Conductor binary and work offline.
They demonstrate common workflow patterns and best practices.`,
	}

	cmd.AddCommand(newExamplesListCmd())
	cmd.AddCommand(newExamplesShowCmd())
	cmd.AddCommand(newExamplesRunCmd())
	cmd.AddCommand(newExamplesCopyCmd())

	// Default to list if no subcommand specified
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newExamplesListCmd().RunE(cmd, args)
	}

	return cmd
}

func newExamplesListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available example workflows",
		Long: `List all embedded example workflows with their descriptions.

See also: conductor examples show, conductor examples run`,
		Example: `  # Example 1: List all examples
  conductor examples list

  # Example 2: Get examples as JSON
  conductor examples list --json

  # Example 3: Extract example names for scripting
  conductor examples list --json | jq -r '.[].name'

  # Example 4: Find examples by description keyword
  conductor examples list --json | jq '.[] | select(.description | contains("API"))'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			examplesList, err := examples.List()
			if err != nil {
				return fmt.Errorf("failed to list examples: %w", err)
			}

			// Check global --json flag in addition to local flag
			useJSON := shared.GetJSON() || jsonOutput

			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(examplesList)
			}

			// Table output
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tDESCRIPTION")
			fmt.Fprintln(w, "────\t───────────")
			for _, ex := range examplesList {
				fmt.Fprintf(w, "%s\t%s\n", ex.Name, ex.Description)
			}
			w.Flush()

			fmt.Println()
			fmt.Println("Use 'conductor examples show <name>' to view an example")
			fmt.Println("Use 'conductor examples run <name>' to execute an example")
			fmt.Println("Use 'conductor examples copy <name> <dest>' to copy an example")

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func newExamplesShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Display an example workflow",
		Long: `Display the YAML content of an example workflow with syntax highlighting.

See also: conductor examples list, conductor examples copy, conductor validate`,
		Example: `  # Example 1: View an example workflow
  conductor examples show hello-world

  # Example 2: Show and pipe to a file
  conductor examples show api-request > my-workflow.yaml

  # Example 3: View example and extract step names
  conductor examples show data-pipeline | grep "id:"`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteExampleNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			content, err := examples.Get(name)
			if err != nil {
				return fmt.Errorf("failed to get example: %w", err)
			}

			// Display the content
			fmt.Printf("# Example: %s\n\n", name)
			fmt.Println(string(content))

			return nil
		},
	}

	return cmd
}

func newExamplesRunCmd() *cobra.Command {
	var (
		dryRun  bool
		quiet   bool
		verbose bool
	)

	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Run an example workflow",
		Long: `Run an embedded example workflow.

This command executes the example with default settings. You can pass
additional flags like --verbose or --dry-run.

See also: conductor examples list, conductor run, conductor validate`,
		Example: `  # Example 1: Run an example workflow
  conductor examples run hello-world

  # Example 2: Preview execution without running
  conductor examples run api-request --dry-run

  # Example 3: Run with verbose output
  conductor examples run data-pipeline --verbose

  # Example 4: Run with JSON output
  conductor examples run hello-world --json`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteExampleNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if !examples.Exists(name) {
				return fmt.Errorf("example %q not found (use 'conductor examples list' to see available examples)", name)
			}

			fmt.Printf("Running example: %s\n\n", name)

			// Load the example
			content, err := examples.Get(name)
			if err != nil {
				return fmt.Errorf("failed to load example: %w", err)
			}

			// Parse the workflow
			def, err := workflow.ParseDefinition(content)
			if err != nil {
				return fmt.Errorf("failed to parse example workflow: %w", err)
			}

			// Load config for provider resolution
			_, err = config.Load(shared.GetConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Show execution plan if verbose or dry-run
			if verbose || dryRun {
				fmt.Println("Execution Plan:")
				fmt.Printf("  Workflow: %s\n", def.Name)
				fmt.Printf("  Steps: %d\n", len(def.Steps))
				fmt.Println()
			}

			if dryRun {
				fmt.Println("Dry run complete. No workflow executed.")
				return nil
			}

			// TODO: Actual execution will be implemented in later phase
			fmt.Println("✓ Example validated successfully!")
			fmt.Println()
			fmt.Println("Note: Workflow execution not yet implemented")

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show execution plan without running")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress all warnings")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed execution logs")

	return cmd
}

func newExamplesCopyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy <name> [dest]",
		Short: "Copy an example to the filesystem",
		Long: `Copy an embedded example workflow to the local filesystem.

If no destination is specified, the example is copied to the current directory
with the name '<name>.yaml'.

See also: conductor examples show, conductor examples list, conductor init`,
		Example: `  # Example 1: Copy to current directory
  conductor examples copy hello-world

  # Example 2: Copy to specific file
  conductor examples copy api-request my-api-workflow.yaml

  # Example 3: Copy to a directory
  conductor examples copy data-pipeline ./workflows/

  # Example 4: Copy and immediately validate
  conductor examples copy hello-world && conductor validate hello-world.yaml`,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completion.CompleteExampleNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Determine destination path
			var destPath string
			if len(args) > 1 {
				destPath = args[1]
			} else {
				destPath = name + ".yaml"
			}

			// Check if example exists
			if !examples.Exists(name) {
				return fmt.Errorf("example %q not found (use 'conductor examples list' to see available examples)", name)
			}

			// If destination is a directory, append filename
			if stat, err := os.Stat(destPath); err == nil && stat.IsDir() {
				destPath = filepath.Join(destPath, name+".yaml")
			}

			// Check if destination exists
			if _, err := os.Stat(destPath); err == nil {
				fmt.Printf("File %s already exists. Overwrite? [y/N] ", destPath)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			// Get the example content
			content, err := examples.Get(name)
			if err != nil {
				return fmt.Errorf("failed to get example: %w", err)
			}

			// Ensure destination directory exists
			destDir := filepath.Dir(destPath)
			if err := os.MkdirAll(destDir, 0755); err != nil {
				return fmt.Errorf("failed to create destination directory: %w", err)
			}

			// Write the file
			if err := os.WriteFile(destPath, content, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			fmt.Printf("✓ Copied example %q to %s\n", name, destPath)

			return nil
		},
	}

	return cmd
}
