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

package run

import (
	"fmt"
	"strings"
)

// parseStats parses statistics from an event map
func parseStats(event map[string]any) *RunStats {
	stats := &RunStats{}

	if cost, ok := event["cost_usd"].(float64); ok {
		stats.CostUSD = cost
	}
	if tokensIn, ok := event["tokens_in"].(float64); ok {
		stats.TokensIn = int(tokensIn)
	}
	if tokensOut, ok := event["tokens_out"].(float64); ok {
		stats.TokensOut = int(tokensOut)
	}
	if duration, ok := event["duration_ms"].(float64); ok {
		stats.DurationMs = int64(duration)
	}

	return stats
}

// parseStatsFromRun parses statistics from a run response
func parseStatsFromRun(run map[string]any) *RunStats {
	// Check if there's a stats object
	if statsData, ok := run["stats"].(map[string]any); ok {
		return parseStats(statsData)
	}

	// Try to extract from top-level fields
	return parseStats(run)
}

// displayStats displays execution statistics
func displayStats(stats *RunStats) {
	// Don't display anything if we have no meaningful stats
	hasStats := stats.CostUSD > 0 || stats.TokensIn > 0 || stats.TokensOut > 0 || stats.DurationMs > 0 || len(stats.StepCosts) > 0
	if !hasStats {
		return
	}

	fmt.Println("\n---")

	var parts []string

	if stats.CostUSD > 0 {
		prefix := ""
		if stats.Accuracy == "estimated" {
			prefix = "~"
		}
		parts = append(parts, fmt.Sprintf("Cost: %s$%.4f", prefix, stats.CostUSD))
	}

	if stats.TokensIn > 0 || stats.TokensOut > 0 {
		parts = append(parts, fmt.Sprintf("Tokens: %d in / %d out", stats.TokensIn, stats.TokensOut))
	}

	if stats.DurationMs > 0 {
		parts = append(parts, fmt.Sprintf("Time: %.1fs", float64(stats.DurationMs)/1000.0))
	}

	if len(parts) > 0 {
		fmt.Println(strings.Join(parts, " | "))
	}

	// Show per-step breakdown if available
	if len(stats.StepCosts) > 0 {
		fmt.Println("\nPer-step costs:")
		for stepName, stepCost := range stats.StepCosts {
			stepCostStr := formatStepCost(stepCost)
			fmt.Printf("  %s: %s (%d tokens)\n", stepName, stepCostStr, stepCost.Tokens)
		}
	}
}

// displayStepCost displays cost information for a completed step in real-time
func displayStepCost(event map[string]any) {
	stepName, _ := event["step_name"].(string)
	if stepName == "" {
		return
	}

	cost, _ := event["cost_usd"].(float64)
	tokens, _ := event["tokens"].(float64)
	accuracy, _ := event["accuracy"].(string)
	runningTotal, _ := event["running_total"].(float64)

	costStr := formatCost(cost, accuracy)

	fmt.Printf("  [✓] %s: %s (%d tokens)\n", stepName, costStr, int(tokens))

	// Show running total if available
	if runningTotal > 0 {
		totalStr := formatCost(runningTotal, accuracy)
		fmt.Printf("      Running total: %s\n", totalStr)
	}
}

// updateStatsWithStep updates running stats with step completion data
func updateStatsWithStep(stats *RunStats, event map[string]any) {
	stepName, _ := event["step_name"].(string)
	if stepName == "" {
		return
	}

	cost, _ := event["cost_usd"].(float64)
	tokens, _ := event["tokens"].(float64)
	accuracy, _ := event["accuracy"].(string)

	// Update step costs
	if stats.StepCosts == nil {
		stats.StepCosts = make(map[string]StepCost)
	}

	stats.StepCosts[stepName] = StepCost{
		CostUSD:  cost,
		Tokens:   int(tokens),
		Accuracy: accuracy,
	}

	// Update totals
	stats.CostUSD += cost
	stats.TokensIn += int(tokens) // Simplified - would need to separate in/out

	// Update overall accuracy (use most conservative)
	if accuracy == "unavailable" || stats.Accuracy == "unavailable" {
		stats.Accuracy = "unavailable"
	} else if accuracy == "estimated" || stats.Accuracy == "estimated" {
		stats.Accuracy = "estimated"
	} else {
		stats.Accuracy = "measured"
	}

	stats.RunningTotal = stats.CostUSD
}

// formatCost formats a cost value with accuracy indicator
func formatCost(cost float64, accuracy string) string {
	if accuracy == "unavailable" {
		return "--"
	}

	prefix := ""
	if accuracy == "estimated" {
		prefix = "~"
	}

	return fmt.Sprintf("%s$%.4f", prefix, cost)
}

// formatStepCost formats a step cost with accuracy indicator
func formatStepCost(stepCost StepCost) string {
	return formatCost(stepCost.CostUSD, stepCost.Accuracy)
}
