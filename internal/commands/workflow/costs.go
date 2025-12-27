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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/pricing"
)

// NewCostsCommand creates the costs command for viewing cost and usage statistics.
func NewCostsCommand() *cobra.Command {
	var (
		groupBy   string
		since     string
		until     string
		export    string
		jsonOut   bool
		provider  string
		model     string
		workflow  string
	)

	cmd := &cobra.Command{
		Use:   "costs",
		Annotations: map[string]string{
			"group": "workflow",
		},
		Short: "View cost and usage statistics",
		Long: `View aggregated cost and token usage statistics.

Grouping Options:
  --by provider   Group costs by provider (anthropic, openai, etc.)
  --by model      Group costs by model
  --by workflow   Group costs by workflow

Time Filters:
  --since         Start time (e.g., "2024-01-01", "24h ago", "7 days")
  --until         End time (e.g., "2024-01-31", "now")

Export Formats:
  --export json   Export to JSON file
  --export csv    Export to CSV file
  --json          Output as JSON to stdout

Examples:
  # Show total costs
  conductor costs

  # Costs by provider in the last 7 days
  conductor costs --by provider --since "7 days ago"

  # Export model costs to CSV
  conductor costs --by model --export costs.csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCostsCommand(costsOptions{
				groupBy:  groupBy,
				since:    since,
				until:    until,
				export:   export,
				jsonOut:  jsonOut,
				provider: provider,
				model:    model,
				workflow: workflow,
			})
		},
	}

	cmd.Flags().StringVar(&groupBy, "by", "", "Group by: provider, model, or workflow")
	cmd.Flags().StringVar(&since, "since", "", "Start time for cost query")
	cmd.Flags().StringVar(&until, "until", "", "End time for cost query")
	cmd.Flags().StringVar(&export, "export", "", "Export to file (json or csv)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&provider, "provider", "", "Filter by provider")
	cmd.Flags().StringVar(&model, "model", "", "Filter by model")
	cmd.Flags().StringVar(&workflow, "workflow", "", "Filter by workflow")

	return cmd
}

type costsOptions struct {
	groupBy  string
	since    string
	until    string
	export   string
	jsonOut  bool
	provider string
	model    string
	workflow string
}

func runCostsCommand(opts costsOptions) error {
	ctx := context.Background()

	// Parse time range
	start, end, err := parseTimeRange(opts.since, opts.until)
	if err != nil {
		return fmt.Errorf("invalid time range: %w", err)
	}

	// Get cost data from global tracker
	// In a real implementation, this would query the cost store
	tracker := getGlobalCostTracker()

	// Apply filters and grouping
	var data interface{}
	if opts.groupBy != "" {
		data, err = getGroupedCosts(ctx, tracker, opts.groupBy, start, end)
		if err != nil {
			return err
		}
	} else {
		data, err = getTotalCosts(ctx, tracker, start, end)
		if err != nil {
			return err
		}
	}

	// Handle export
	if opts.export != "" {
		return exportCosts(data, opts.export, opts.groupBy)
	}

	// Handle JSON output
	if opts.jsonOut {
		return outputJSON(data)
	}

	// Default: human-readable output
	return outputHumanReadable(data, opts.groupBy)
}

func parseTimeRange(since, until string) (*time.Time, *time.Time, error) {
	var start, end *time.Time

	if since != "" {
		t, err := parseTimeString(since)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --since value: %w", err)
		}
		start = &t
	}

	if until != "" {
		t, err := parseTimeString(until)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --until value: %w", err)
		}
		end = &t
	}

	return start, end, nil
}

func parseTimeString(s string) (time.Time, error) {
	// Try RFC3339 format first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try date-only format
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}

	// Try relative time formats
	if s == "now" {
		return time.Now(), nil
	}

	// Parse "X days ago", "X hours ago", etc.
	// For MVP, support simple formats
	// TODO: Add more sophisticated time parsing

	return time.Time{}, fmt.Errorf("unsupported time format: %s (use RFC3339 or YYYY-MM-DD)", s)
}

func getGroupedCosts(ctx context.Context, tracker *llm.CostTracker, groupBy string, start, end *time.Time) (interface{}, error) {
	switch groupBy {
	case "provider":
		return getByProvider(tracker, start, end), nil
	case "model":
		return getByModel(tracker, start, end), nil
	case "workflow":
		// For now, return empty since workflow grouping needs store implementation
		return map[string]llm.CostAggregate{}, nil
	default:
		return nil, fmt.Errorf("invalid grouping: %s (use provider, model, or workflow)", groupBy)
	}
}

func getByProvider(tracker *llm.CostTracker, start, end *time.Time) map[string]llm.CostAggregate {
	// If time range specified, filter records first
	if start != nil || end != nil {
		// For MVP, just return all aggregates
		// TODO: Implement time filtering
	}
	return tracker.AggregateByProvider()
}

func getByModel(tracker *llm.CostTracker, start, end *time.Time) map[string]llm.CostAggregate {
	// If time range specified, filter records first
	if start != nil || end != nil {
		// For MVP, just return all aggregates
		// TODO: Implement time filtering
	}
	return tracker.AggregateByModel()
}

func getTotalCosts(ctx context.Context, tracker *llm.CostTracker, start, end *time.Time) (interface{}, error) {
	// Sum up all costs
	var total llm.CostAggregate

	records := tracker.GetRecords()
	for _, record := range records {
		// Apply time filtering if specified
		if start != nil && record.Timestamp.Before(*start) {
			continue
		}
		if end != nil && record.Timestamp.After(*end) {
			continue
		}

		if record.Cost != nil {
			total.TotalCost += record.Cost.Amount

			// Track accuracy breakdown
			switch record.Cost.Accuracy {
			case llm.CostMeasured:
				total.AccuracyBreakdown.Measured++
			case llm.CostEstimated:
				total.AccuracyBreakdown.Estimated++
			case llm.CostUnavailable:
				total.AccuracyBreakdown.Unavailable++
			}
		}

		total.TotalRequests++
		total.TotalTokens += record.Usage.TotalTokens
		total.TotalPromptTokens += record.Usage.PromptTokens
		total.TotalCompletionTokens += record.Usage.CompletionTokens
		total.TotalCacheCreationTokens += record.Usage.CacheCreationTokens
		total.TotalCacheReadTokens += record.Usage.CacheReadTokens
	}

	// Determine overall accuracy
	totalAccuracy := total.AccuracyBreakdown.Measured + total.AccuracyBreakdown.Estimated + total.AccuracyBreakdown.Unavailable
	if totalAccuracy > 0 {
		if total.AccuracyBreakdown.Measured == totalAccuracy {
			total.Accuracy = llm.CostMeasured
		} else if total.AccuracyBreakdown.Unavailable == totalAccuracy {
			total.Accuracy = llm.CostUnavailable
		} else {
			total.Accuracy = llm.CostEstimated
		}
	}

	return total, nil
}

func exportCosts(data interface{}, filename string, groupBy string) error {
	// Determine format from filename extension
	isCSV := len(filename) > 4 && filename[len(filename)-4:] == ".csv"

	if isCSV {
		return exportCSV(data, filename, groupBy)
	}
	return exportJSON(data, filename)
}

func exportJSON(data interface{}, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	fmt.Printf("Exported costs to %s\n", filename)
	return nil
}

func exportCSV(data interface{}, filename string, groupBy string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Write header based on grouping
	var header []string
	if groupBy != "" {
		header = []string{"Group", "Requests", "Total Tokens", "Prompt Tokens", "Completion Tokens", "Cache Creation", "Cache Read", "Total Cost", "Accuracy"}
	} else {
		header = []string{"Total Requests", "Total Tokens", "Prompt Tokens", "Completion Tokens", "Cache Creation", "Cache Read", "Total Cost", "Accuracy"}
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	if groupBy != "" {
		// Grouped data
		if aggregates, ok := data.(map[string]llm.CostAggregate); ok {
			for name, agg := range aggregates {
				row := []string{
					name,
					fmt.Sprintf("%d", agg.TotalRequests),
					fmt.Sprintf("%d", agg.TotalTokens),
					fmt.Sprintf("%d", agg.TotalPromptTokens),
					fmt.Sprintf("%d", agg.TotalCompletionTokens),
					fmt.Sprintf("%d", agg.TotalCacheCreationTokens),
					fmt.Sprintf("%d", agg.TotalCacheReadTokens),
					fmt.Sprintf("%.4f", agg.TotalCost),
					string(agg.Accuracy),
				}
				if err := writer.Write(row); err != nil {
					return fmt.Errorf("failed to write CSV row: %w", err)
				}
			}
		}
	} else {
		// Total data
		if agg, ok := data.(llm.CostAggregate); ok {
			row := []string{
				fmt.Sprintf("%d", agg.TotalRequests),
				fmt.Sprintf("%d", agg.TotalTokens),
				fmt.Sprintf("%d", agg.TotalPromptTokens),
				fmt.Sprintf("%d", agg.TotalCompletionTokens),
				fmt.Sprintf("%d", agg.TotalCacheCreationTokens),
				fmt.Sprintf("%d", agg.TotalCacheReadTokens),
				fmt.Sprintf("%.4f", agg.TotalCost),
				string(agg.Accuracy),
			}
			if err := writer.Write(row); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
		}
	}

	fmt.Printf("Exported costs to %s\n", filename)
	return nil
}

func outputJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func outputHumanReadable(data interface{}, groupBy string) error {
	if groupBy != "" {
		return outputGroupedHuman(data, groupBy)
	}
	return outputTotalHuman(data)
}

func outputTotalHuman(data interface{}) error {
	agg, ok := data.(llm.CostAggregate)
	if !ok {
		return fmt.Errorf("invalid data type for total costs")
	}

	fmt.Println("Cost Summary")
	fmt.Println("============")
	fmt.Println()

	// Format cost with accuracy indicator
	costStr := formatCostWithAccuracy(agg.TotalCost, agg.Accuracy)
	fmt.Printf("Total Cost:         %s\n", costStr)
	fmt.Printf("Total Requests:     %d\n", agg.TotalRequests)
	fmt.Printf("Total Tokens:       %s\n", pricing.FormatTokens(agg.TotalTokens))
	fmt.Println()

	// Token breakdown
	fmt.Println("Token Breakdown:")
	fmt.Printf("  Prompt:           %s\n", pricing.FormatTokens(agg.TotalPromptTokens))
	fmt.Printf("  Completion:       %s\n", pricing.FormatTokens(agg.TotalCompletionTokens))
	if agg.TotalCacheCreationTokens > 0 {
		fmt.Printf("  Cache Creation:   %s\n", pricing.FormatTokens(agg.TotalCacheCreationTokens))
	}
	if agg.TotalCacheReadTokens > 0 {
		fmt.Printf("  Cache Read:       %s\n", pricing.FormatTokens(agg.TotalCacheReadTokens))
	}
	fmt.Println()

	// Accuracy breakdown
	if agg.AccuracyBreakdown.Measured > 0 || agg.AccuracyBreakdown.Estimated > 0 || agg.AccuracyBreakdown.Unavailable > 0 {
		fmt.Println("Accuracy Breakdown:")
		if agg.AccuracyBreakdown.Measured > 0 {
			fmt.Printf("  Measured:         %d requests\n", agg.AccuracyBreakdown.Measured)
		}
		if agg.AccuracyBreakdown.Estimated > 0 {
			fmt.Printf("  Estimated:        %d requests\n", agg.AccuracyBreakdown.Estimated)
		}
		if agg.AccuracyBreakdown.Unavailable > 0 {
			fmt.Printf("  Unavailable:      %d requests\n", agg.AccuracyBreakdown.Unavailable)
		}
	}

	return nil
}

func outputGroupedHuman(data interface{}, groupBy string) error {
	aggregates, ok := data.(map[string]llm.CostAggregate)
	if !ok {
		return fmt.Errorf("invalid data type for grouped costs")
	}

	fmt.Printf("Costs by %s\n", groupBy)
	fmt.Println(strings.Repeat("=", len(groupBy)+9))
	fmt.Println()

	// Calculate totals
	var totalCost float64
	var totalRequests int
	var totalTokens int

	for _, agg := range aggregates {
		totalCost += agg.TotalCost
		totalRequests += agg.TotalRequests
		totalTokens += agg.TotalTokens
	}

	// Print each group
	for name, agg := range aggregates {
		costStr := formatCostWithAccuracy(agg.TotalCost, agg.Accuracy)
		percentage := 0.0
		if totalCost > 0 {
			percentage = (agg.TotalCost / totalCost) * 100
		}

		fmt.Printf("%s\n", name)
		fmt.Printf("  Cost:       %s (%.1f%%)\n", costStr, percentage)
		fmt.Printf("  Requests:   %d\n", agg.TotalRequests)
		fmt.Printf("  Tokens:     %s\n", pricing.FormatTokens(agg.TotalTokens))
		fmt.Println()
	}

	// Print totals
	fmt.Println("Total")
	fmt.Printf("  Cost:       $%.4f\n", totalCost)
	fmt.Printf("  Requests:   %d\n", totalRequests)
	fmt.Printf("  Tokens:     %s\n", pricing.FormatTokens(totalTokens))

	return nil
}

func formatCostWithAccuracy(cost float64, accuracy llm.CostAccuracy) string {
	// Convert llm.CostAccuracy to pricing.CostAccuracy
	var pricingAccuracy pricing.CostAccuracy
	switch accuracy {
	case llm.CostMeasured:
		pricingAccuracy = pricing.CostMeasured
	case llm.CostEstimated:
		pricingAccuracy = pricing.CostEstimated
	case llm.CostUnavailable:
		pricingAccuracy = pricing.CostUnavailable
	}

	costInfo := &pricing.CostInfo{
		Amount:   cost,
		Currency: "USD",
		Accuracy: pricingAccuracy,
		Source:   pricing.SourcePricingTable,
	}
	return pricing.FormatCost(costInfo)
}

// getGlobalCostTracker returns the global cost tracker instance.
// This is a temporary helper until we have proper cost store integration.
func getGlobalCostTracker() *llm.CostTracker {
	// For now, return a new tracker
	// In production, this would return the actual global tracker
	// or query the cost store
	return llm.NewCostTracker()
}

