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

package completion

import (
	"context"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/client"
)

const (
	runCacheTTL       = 2 * time.Second
	controllerTimeout = 500 * time.Millisecond
	maxCompletionMS   = 200 * time.Millisecond
)

// runCacheEntry holds cached run completions with expiry.
type runCacheEntry struct {
	runs      []runInfo
	expiresAt time.Time
}

// runInfo represents a run ID with optional description.
type runInfo struct {
	id          string
	workflow    string
	status      string
	description string
}

var (
	runCache   *runCacheEntry
	runCacheMu sync.RWMutex
)

// CompleteRunIDs provides dynamic completion for workflow run IDs.
// Queries the controller API for recent runs and caches results for 2 seconds.
// Returns run IDs with workflow names as descriptions.
// Applies 500ms timeout for controller queries and aims for <200ms overall.
func CompleteRunIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		runs, err := getRunCompletions(false)
		if err != nil || len(runs) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		completions := make([]string, 0, len(runs))
		for _, r := range runs {
			// Format: "runID\tworkflow (status)"
			completions = append(completions, r.id+"\t"+r.description)
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	})
}

// CompleteActiveRunIDs provides completion for active (running/pending) run IDs.
// Used by 'runs cancel' command to only show cancelable runs.
func CompleteActiveRunIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		runs, err := getRunCompletions(true)
		if err != nil || len(runs) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		completions := make([]string, 0, len(runs))
		for _, r := range runs {
			completions = append(completions, r.id+"\t"+r.description)
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	})
}

// getRunCompletions fetches run completions from the controller with caching.
// If activeOnly is true, only returns running or pending runs.
func getRunCompletions(activeOnly bool) ([]runInfo, error) {
	// Check cache first
	runCacheMu.RLock()
	if runCache != nil && time.Now().Before(runCache.expiresAt) {
		cached := runCache.runs
		runCacheMu.RUnlock()

		// Filter for active runs if requested
		if activeOnly {
			return filterActiveRuns(cached), nil
		}
		return cached, nil
	}
	runCacheMu.RUnlock()

	// Cache miss - fetch from controller
	runs, err := fetchRunsFromController()
	if err != nil {
		return nil, err
	}

	// Update cache
	runCacheMu.Lock()
	runCache = &runCacheEntry{
		runs:      runs,
		expiresAt: time.Now().Add(runCacheTTL),
	}
	runCacheMu.Unlock()

	// Filter for active runs if requested
	if activeOnly {
		return filterActiveRuns(runs), nil
	}
	return runs, nil
}

// fetchRunsFromController queries the controller API for runs with a timeout.
func fetchRunsFromController() ([]runInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), controllerTimeout)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return nil, err
	}

	resp, err := c.Get(ctx, "/v1/runs")
	if err != nil {
		return nil, err
	}

	// Parse response with defensive type assertions
	runsData, ok := resp["runs"].([]interface{})
	if !ok {
		return nil, nil
	}

	completions := make([]runInfo, 0, len(runsData))
	for _, r := range runsData {
		runMap, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		id, ok := runMap["id"].(string)
		if !ok || id == "" {
			continue
		}

		workflow, _ := runMap["workflow"].(string)
		status, _ := runMap["status"].(string)

		// Build description: "workflow (status)"
		description := workflow
		if status != "" {
			if description != "" {
				description += " (" + status + ")"
			} else {
				description = status
			}
		}

		completions = append(completions, runInfo{
			id:          id,
			workflow:    workflow,
			status:      status,
			description: description,
		})
	}

	return completions, nil
}

// filterActiveRuns returns only runs with status "running" or "pending".
func filterActiveRuns(runs []runInfo) []runInfo {
	active := make([]runInfo, 0, len(runs))
	for _, r := range runs {
		if r.status == "running" || r.status == "pending" {
			active = append(active, r)
		}
	}
	return active
}
