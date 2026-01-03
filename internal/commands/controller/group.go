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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/client"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewCommand creates the controller command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "controller",
		Annotations: map[string]string{
			"group": "system",
		},
		Short: "Manage the conductor controller",
		Long: `Commands for managing the conductor controller.

The controller is the central service that executes workflows. The CLI
communicates with the controller to run workflows, check status, and more.`,
	}

	cmd.AddCommand(NewStartCommand())
	cmd.AddCommand(NewStopCommand())
	cmd.AddCommand(newControllerStatusCommand())
	cmd.AddCommand(newControllerPingCommand())

	return cmd
}

func newControllerStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show controller status and version",
		Long: `Display the status, version, and health of the conductor controller.

See also: conductor controller ping, conductor doctor, conductor history list`,
		Example: `  # Example 1: Check controller status
  conductor controller status

  # Example 2: Get controller info as JSON
  conductor controller status --json

  # Example 3: Extract controller version
  conductor controller status --json | jq -r '.version'

  # Example 4: Check controller uptime
  conductor controller status --json | jq -r '.uptime'`,
		RunE: runControllerStatus,
	}
}

func newControllerPingCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Check if controller is reachable",
		Long:  `Quickly check if the conductor controller is running and reachable.`,
		RunE:  runControllerPing,
	}
}

func runControllerStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Get health and version in parallel
	type result struct {
		health  *client.HealthResponse
		version *client.VersionResponse
		err     error
	}

	healthCh := make(chan result, 1)
	versionCh := make(chan result, 1)

	go func() {
		health, err := c.Health(ctx)
		healthCh <- result{health: health, err: err}
	}()

	go func() {
		version, err := c.Version(ctx)
		versionCh <- result{version: version, err: err}
	}()

	healthResult := <-healthCh
	versionResult := <-versionCh

	// Check for connection errors
	if healthResult.err != nil {
		if client.IsControllerNotRunning(healthResult.err) {
			cnr := &client.ControllerNotRunningError{}
			fmt.Fprintln(os.Stderr, cnr.Guidance())
			os.Exit(10)
		}
		return fmt.Errorf("failed to get controller health: %w", healthResult.err)
	}

	if versionResult.err != nil {
		return fmt.Errorf("failed to get controller version: %w", versionResult.err)
	}

	health := healthResult.health
	version := versionResult.version

	if shared.GetJSON() {
		output := map[string]any{
			"status":     health.Status,
			"version":    version.Version,
			"commit":     version.Commit,
			"build_date": version.BuildDate,
			"go_version": version.GoVersion,
			"os":         version.OS,
			"arch":       version.Arch,
			"uptime":     health.Uptime,
			"timestamp":  health.Timestamp,
			"checks":     health.Checks,
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	fmt.Println(shared.Header.Render("Controller Status"))
	fmt.Println()

	// Status with color based on health
	statusStyle := shared.StatusOK
	if health.Status != "healthy" {
		statusStyle = shared.StatusError
	}
	fmt.Printf("%s %s\n", shared.Muted.Render("Status:"), statusStyle.Render(health.Status))

	// Version info
	if version.Version != "" {
		fmt.Printf("%s %s\n", shared.Muted.Render("Version:"), shared.Bold.Render(version.Version))
	}
	if version.Commit != "" {
		fmt.Printf("%s %s\n", shared.Muted.Render("Commit:"), version.Commit)
	}
	if version.BuildDate != "" {
		fmt.Printf("%s %s\n", shared.Muted.Render("Build Date:"), version.BuildDate)
	}
	fmt.Printf("%s %s\n", shared.Muted.Render("Go Version:"), version.GoVersion)
	fmt.Printf("%s %s/%s\n", shared.Muted.Render("Platform:"), version.OS, version.Arch)
	fmt.Printf("%s %s\n", shared.Muted.Render("Uptime:"), health.Uptime)

	if len(health.Checks) > 0 {
		fmt.Println()
		fmt.Println(shared.Bold.Render("Health Checks:"))
		for name, status := range health.Checks {
			checkStyle := shared.StatusOK
			symbol := shared.SymbolOK
			if status != "ok" && status != "healthy" && status != "none" && status != "disabled" {
				checkStyle = shared.StatusWarn
				symbol = shared.SymbolWarn
			}
			fmt.Printf("  %s %s %s\n", checkStyle.Render(symbol), name+":", shared.Muted.Render(status))
		}
	}

	return nil
}

func runControllerPing(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	start := time.Now()
	if err := c.Ping(ctx); err != nil {
		if client.IsControllerNotRunning(err) {
			if !shared.GetQuiet() {
				fmt.Println(shared.RenderError("Controller is not running"))
			}
			os.Exit(1)
		}
		return fmt.Errorf("ping failed: %w", err)
	}

	latency := time.Since(start)

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"status":     "ok",
			"latency_ms": latency.Milliseconds(),
		})
	}

	if !shared.GetQuiet() {
		fmt.Printf("%s %s\n",
			shared.RenderOK("Controller is running"),
			shared.Muted.Render(fmt.Sprintf("(latency: %v)", latency.Round(time.Millisecond))))
	}

	return nil
}
