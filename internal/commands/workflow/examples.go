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
	"github.com/tombee/conductor/internal/examples"
)

// NewExamplesCommand creates the examples command
func NewExamplesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "examples",
		Annotations: map[string]string{
			"group": "workflow",
		},
		Short: "Manage example workflows",
		Long: `Browse, view, and copy example workflows.

Examples are embedded in the Conductor binary and work offline.
They demonstrate common workflow patterns and best practices.`,
	}

	cmd.AddCommand(newExamplesListCmd())
	cmd.AddCommand(newExamplesShowCmd())
	cmd.AddCommand(newExamplesCopyCmd())

	// Default to list if no subcommand specified
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newExamplesListCmd().RunE(cmd, args)
	}

	return cmd
}

func newExamplesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available example workflows",
		Long: `List all embedded example workflows with their descriptions.

See also: conductor examples show, conductor examples copy`,
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

			if shared.GetJSON() {
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
			fmt.Println("Use 'conductor examples copy <name> <dest>' to copy an example")

			return nil
		},
	}

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

			fmt.Println(shared.RenderOK(fmt.Sprintf("Copied example %q to %s", name, destPath)))

			return nil
		},
	}

	return cmd
}
