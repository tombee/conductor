package workflow

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/examples"
	"github.com/tombee/conductor/pkg/workflow"
)

// NewQuickstartCommand creates the quickstart command
func NewQuickstartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quickstart",
		Annotations: map[string]string{
			"group": "workflow",
		},
		Short: "Run the quickstart workflow",
		Long: `Run a simple hello world workflow to verify Conductor is working correctly.

This command runs an embedded example workflow that requires no additional
setup or configuration. It's the fastest way to see Conductor in action.`,
		RunE: runQuickstart,
	}

	return cmd
}

func runQuickstart(cmd *cobra.Command, args []string) error {
	fmt.Println("Running quickstart workflow...")
	fmt.Println()

	// Load the embedded quickstart workflow
	content, err := examples.Get("quickstart")
	if err != nil {
		return fmt.Errorf("failed to load quickstart workflow: %w", err)
	}

	// Parse the workflow
	def, err := workflow.ParseDefinition(content)
	if err != nil {
		return fmt.Errorf("failed to parse quickstart workflow: %w", err)
	}

	// Load config for provider resolution
	_, err = config.Load(shared.GetConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// TODO: Actual execution will be implemented in later phase
	// For now, just show that it validates correctly
	fmt.Printf("✓ Quickstart workflow validated successfully!\n")
	fmt.Printf("  Workflow: %s\n", def.Name)
	fmt.Printf("  Steps: %d\n", len(def.Steps))
	fmt.Println()
	fmt.Println("Note: Workflow execution not yet implemented")
	fmt.Println()

	// Show next steps
	fmt.Println("Next steps:")
	fmt.Println("  • conductor examples list        - Browse more examples")
	fmt.Println("  • conductor examples show <name> - View an example")
	fmt.Println("  • conductor run <workflow>       - Run your own workflow")
	fmt.Println("  • conductor --help               - See all available commands")
	fmt.Println()

	return nil
}
