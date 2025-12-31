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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/pkg/llm"
)

// NewUsageCommand creates the usage command for viewing token usage statistics.
func NewUsageCommand() *cobra.Command {
	var (
		groupBy string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "usage",
		Annotations: map[string]string{
			"group": "workflow",
		},
		Short: "View token usage statistics",
		Long: `View aggregated token usage statistics.

Grouping Options:
  --by provider   Group usage by provider (anthropic, openai, etc.)
  --by model      Group usage by model

Examples:
  # Show total usage
  conductor usage

  # Usage by provider
  conductor usage --by provider

  # Usage by model as JSON
  conductor usage --by model --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsageCommand(groupBy, jsonOut)
		},
	}

	cmd.Flags().StringVar(&groupBy, "by", "", "Group by: provider or model")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func runUsageCommand(groupBy string, jsonOut bool) error {
	var data interface{}
	var err error

	switch groupBy {
	case "provider":
		data = llm.AggregateUsageByProvider()
	case "model":
		data = llm.AggregateUsageByModel()
	case "":
		data = getTotalUsage()
	default:
		return fmt.Errorf("invalid grouping: %s (use provider or model)", groupBy)
	}

	if err != nil {
		return err
	}

	if jsonOut {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	}

	return outputUsageHuman(data, groupBy)
}

func getTotalUsage() llm.UsageAggregate {
	var total llm.UsageAggregate
	for _, agg := range llm.AggregateUsageByProvider() {
		total.TotalRequests += agg.TotalRequests
		total.TotalTokens += agg.TotalTokens
		total.TotalPromptTokens += agg.TotalPromptTokens
		total.TotalCompletionTokens += agg.TotalCompletionTokens
		total.TotalCacheCreationTokens += agg.TotalCacheCreationTokens
		total.TotalCacheReadTokens += agg.TotalCacheReadTokens
	}
	return total
}

func outputUsageHuman(data interface{}, groupBy string) error {
	if groupBy != "" {
		return outputGroupedUsage(data, groupBy)
	}
	return outputTotalUsage(data)
}

func outputTotalUsage(data interface{}) error {
	agg, ok := data.(llm.UsageAggregate)
	if !ok {
		return fmt.Errorf("invalid data type")
	}

	fmt.Println("Token Usage Summary")
	fmt.Println("===================")
	fmt.Println()
	fmt.Printf("Total Requests:     %d\n", agg.TotalRequests)
	fmt.Printf("Total Tokens:       %s\n", llm.FormatTokens(agg.TotalTokens))
	fmt.Println()
	fmt.Println("Breakdown:")
	fmt.Printf("  Prompt:           %s\n", llm.FormatTokens(agg.TotalPromptTokens))
	fmt.Printf("  Completion:       %s\n", llm.FormatTokens(agg.TotalCompletionTokens))
	if agg.TotalCacheCreationTokens > 0 {
		fmt.Printf("  Cache Creation:   %s\n", llm.FormatTokens(agg.TotalCacheCreationTokens))
	}
	if agg.TotalCacheReadTokens > 0 {
		fmt.Printf("  Cache Read:       %s\n", llm.FormatTokens(agg.TotalCacheReadTokens))
	}

	return nil
}

func outputGroupedUsage(data interface{}, groupBy string) error {
	aggregates, ok := data.(map[string]llm.UsageAggregate)
	if !ok {
		return fmt.Errorf("invalid data type")
	}

	fmt.Printf("Usage by %s\n", groupBy)
	fmt.Println(strings.Repeat("=", len(groupBy)+9))
	fmt.Println()

	var totalRequests, totalTokens int
	for _, agg := range aggregates {
		totalRequests += agg.TotalRequests
		totalTokens += agg.TotalTokens
	}

	for name, agg := range aggregates {
		fmt.Printf("%s\n", name)
		fmt.Printf("  Requests:   %d\n", agg.TotalRequests)
		fmt.Printf("  Tokens:     %s\n", llm.FormatTokens(agg.TotalTokens))
		fmt.Println()
	}

	fmt.Println("Total")
	fmt.Printf("  Requests:   %d\n", totalRequests)
	fmt.Printf("  Tokens:     %s\n", llm.FormatTokens(totalTokens))

	return nil
}
