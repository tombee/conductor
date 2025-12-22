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

package daemon

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

// NewCommand creates the daemon command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the conductor daemon",
		Long: `Commands for managing the conductor daemon (conductord).

The daemon is the central service that executes workflows. The CLI
communicates with the daemon to run workflows, check status, and more.`,
	}

	cmd.AddCommand(newDaemonStatusCommand())
	cmd.AddCommand(newDaemonPingCommand())

	return cmd
}

func newDaemonStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status and version",
		Long:  `Display the status, version, and health of the conductor daemon.`,
		RunE:  runDaemonStatus,
	}
}

func newDaemonPingCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Check if daemon is reachable",
		Long:  `Quickly check if the conductor daemon is running and reachable.`,
		RunE:  runDaemonPing,
	}
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
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
		if client.IsDaemonNotRunning(healthResult.err) {
			dnr := &client.DaemonNotRunningError{}
			fmt.Fprintln(os.Stderr, dnr.Guidance())
			os.Exit(10)
		}
		return fmt.Errorf("failed to get daemon health: %w", healthResult.err)
	}

	if versionResult.err != nil {
		return fmt.Errorf("failed to get daemon version: %w", versionResult.err)
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

	fmt.Println("Conductor Daemon Status")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Printf("Status:     %s\n", health.Status)
	fmt.Printf("Version:    %s\n", version.Version)
	fmt.Printf("Commit:     %s\n", version.Commit)
	fmt.Printf("Build Date: %s\n", version.BuildDate)
	fmt.Printf("Go Version: %s\n", version.GoVersion)
	fmt.Printf("Platform:   %s/%s\n", version.OS, version.Arch)
	fmt.Printf("Uptime:     %s\n", health.Uptime)

	if len(health.Checks) > 0 {
		fmt.Println()
		fmt.Println("Health Checks:")
		for name, status := range health.Checks {
			fmt.Printf("  %s: %s\n", name, status)
		}
	}

	return nil
}

func runDaemonPing(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	start := time.Now()
	if err := c.Ping(ctx); err != nil {
		if client.IsDaemonNotRunning(err) {
			if !shared.GetQuiet() {
				fmt.Println("Daemon is not running")
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
		fmt.Printf("Daemon is running (latency: %v)\n", latency.Round(time.Millisecond))
	}

	return nil
}
