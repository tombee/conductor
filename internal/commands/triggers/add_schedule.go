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

package triggers

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/triggers"
)

var (
	scheduleName     string
	scheduleCron     string
	scheduleEvery    string
	scheduleAt       string
	scheduleTimezone string
)

func newAddScheduleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule WORKFLOW",
		Short: "Add a schedule trigger",
		Long: `Add a schedule trigger that invokes a workflow on a recurring schedule.

You can specify the schedule in two ways:
1. Using cron syntax: --cron="0 9 * * *"
2. Using human-friendly syntax: --every=day --at=09:00

The --every flag accepts: hour, day, week, month
The --at flag uses 24-hour format: HH:MM`,
		Example: `  # Run daily at 9 AM
  conductor triggers add schedule daily-report.yaml \
    --name=daily-report \
    --every=day \
    --at=09:00 \
    --timezone="America/New_York"

  # Use cron syntax for complex schedules
  conductor triggers add schedule oncall-handoff.yaml \
    --name=oncall-handoff \
    --cron="0 9,21 * * *" \
    --timezone="America/New_York"

  # Run every hour
  conductor triggers add schedule health-check.yaml \
    --name=health-check \
    --every=hour

  # Run weekly on Monday at 10 AM
  conductor triggers add schedule weekly-summary.yaml \
    --name=weekly-summary \
    --every=week \
    --at=10:00

  # Dry-run to preview
  conductor triggers add schedule test.yaml \
    --name=test \
    --every=day \
    --at=12:00 \
    --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: runAddSchedule,
	}

	cmd.Flags().StringVar(&scheduleName, "name", "", "Unique schedule name (required)")
	cmd.Flags().StringVar(&scheduleCron, "cron", "", "Cron expression (e.g., '0 9 * * *')")
	cmd.Flags().StringVar(&scheduleEvery, "every", "", "Human-friendly schedule: hour, day, week, month")
	cmd.Flags().StringVar(&scheduleAt, "at", "", "Time for daily/weekly/monthly schedules (HH:MM)")
	cmd.Flags().StringVar(&scheduleTimezone, "timezone", "UTC", "IANA timezone (default: UTC)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")

	cmd.MarkFlagRequired("name")

	return cmd
}

func runAddSchedule(cmd *cobra.Command, args []string) error {
	workflow := args[0]

	if scheduleCron == "" && scheduleEvery == "" {
		return fmt.Errorf("either --cron or --every is required")
	}

	req := triggers.CreateScheduleRequest{
		Workflow: workflow,
		Name:     scheduleName,
		Cron:     scheduleCron,
		Every:    scheduleEvery,
		At:       scheduleAt,
		Timezone: scheduleTimezone,
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry-run: Would add schedule trigger:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Name: %s\n", req.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "  Workflow: %s\n", req.Workflow)
		if req.Cron != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Cron: %s\n", req.Cron)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  Every: %s\n", req.Every)
			if req.At != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  At: %s\n", req.At)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Timezone: %s\n", req.Timezone)
		return nil
	}

	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := mgr.AddSchedule(ctx, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Schedule trigger created!\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", scheduleName)
	fmt.Fprintf(cmd.OutOrStdout(), "\nRestart the controller for changes to take effect:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  conductor controller restart\n")

	return nil
}
