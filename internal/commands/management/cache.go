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

package management

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/daemon/cache"
)

// NewCacheCommand creates the cache management command.
func NewCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Annotations: map[string]string{
			"group": "management",
		},
		Short: "Manage remote workflow cache",
		Long: `Manage the cache for remote workflows.

Remote workflows fetched from GitHub are cached locally for faster
subsequent runs. This command helps manage the cache.`,
	}

	cmd.AddCommand(newCacheClearCommand())
	cmd.AddCommand(newCacheListCommand())

	return cmd
}

// newCacheClearCommand creates the cache clear subcommand.
func newCacheClearCommand() *cobra.Command {
	var (
		owner string
		repo  string
	)

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear cached workflows",
		Long: `Clear cached remote workflows.

See also: conductor cache list, conductor run`,
		Example: `  # Example 1: Clear entire cache
  conductor cache clear

  # Example 2: Clear all repos for a user
  conductor cache clear --owner myuser

  # Example 3: Clear specific repository cache
  conductor cache clear --owner myuser --repo myrepo

  # Example 4: Clear and confirm with JSON output
  conductor cache clear --owner myuser --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return clearCache(owner, repo)
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Repository owner/organization")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name (requires --owner)")

	return cmd
}

// newCacheListCommand creates the cache list subcommand.
func newCacheListCommand() *cobra.Command {
	var (
		owner string
		repo  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List cached workflows",
		Long: `List cached remote workflows for a repository.

See also: conductor cache clear, conductor run`,
		Example: `  # Example 1: List cached workflows for a repository
  conductor cache list --owner myuser --repo myrepo

  # Example 2: Get cache list as JSON
  conductor cache list --owner myuser --repo myrepo --json

  # Example 3: Extract commit SHAs from cache
  conductor cache list --owner myuser --repo myrepo --json | jq -r '.entries[].CommitSHA'

  # Example 4: Check cache size
  conductor cache list --owner myuser --repo myrepo --json | jq '.count'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if owner == "" || repo == "" {
				return fmt.Errorf("both --owner and --repo are required")
			}
			return listCache(owner, repo)
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Repository owner/organization (required)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name (required)")
	cmd.MarkFlagRequired("owner")
	cmd.MarkFlagRequired("repo")

	return cmd
}

// clearCache clears the workflow cache.
func clearCache(owner, repo string) error {
	// Initialize cache
	workflowCache, err := cache.NewWorkflowCache(cache.Config{})
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	// Clear cache
	if err := workflowCache.Clear(owner, repo); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	// Display result
	if shared.GetJSON() {
		result := map[string]string{
			"status": "cleared",
		}
		if owner != "" {
			result["owner"] = owner
		}
		if repo != "" {
			result["repo"] = repo
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	}

	// Human-readable output
	if owner == "" && repo == "" {
		fmt.Println("Cleared entire workflow cache")
	} else if repo == "" {
		fmt.Printf("Cleared cache for all repositories owned by %s\n", owner)
	} else {
		fmt.Printf("Cleared cache for %s/%s\n", owner, repo)
	}

	return nil
}

// listCache lists cached workflows for a repository.
func listCache(owner, repo string) error {
	// Initialize cache
	workflowCache, err := cache.NewWorkflowCache(cache.Config{})
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	// List entries
	entries, err := workflowCache.List(owner, repo)
	if err != nil {
		return fmt.Errorf("failed to list cache: %w", err)
	}

	// Display results
	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"owner":   owner,
			"repo":    repo,
			"entries": entries,
			"count":   len(entries),
		})
	}

	// Human-readable output
	if len(entries) == 0 {
		fmt.Printf("No cached workflows for %s/%s\n", owner, repo)
		return nil
	}

	fmt.Printf("Cached workflows for %s/%s:\n\n", owner, repo)
	for _, entry := range entries {
		fmt.Printf("  Commit SHA:  %s\n", entry.CommitSHA)
		fmt.Printf("  Source:      %s\n", entry.SourceURL)
		fmt.Printf("  Fetched:     %s\n", entry.FetchedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Size:        %d bytes\n", entry.Size)
		fmt.Println()
	}

	return nil
}
