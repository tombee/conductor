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

// Package remote provides remote workflow fetching with caching.
package remote

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tombee/conductor/internal/daemon/cache"
	"github.com/tombee/conductor/internal/daemon/github"
	internallog "github.com/tombee/conductor/internal/log"
	"github.com/tombee/conductor/internal/remote"
)

// Fetcher orchestrates remote workflow fetching with caching.
type Fetcher struct {
	client  *github.Client
	cache   *cache.WorkflowCache
	logger  *slog.Logger
	verbose bool
}

// Config contains configuration for the remote fetcher.
type Config struct {
	// GitHubToken is the GitHub API token (optional for public repos)
	GitHubToken string

	// GitHubHost is the GitHub API host (default: api.github.com)
	GitHubHost string

	// CacheBasePath is the cache directory (default: ~/.cache/conductor/remote-workflows)
	CacheBasePath string

	// DisableCache disables workflow caching
	DisableCache bool

	// Verbose enables verbose logging
	Verbose bool
}

// FetchResult contains the fetched workflow and metadata.
type FetchResult struct {
	// Content is the workflow YAML content
	Content []byte

	// CommitSHA is the git commit SHA this workflow was fetched from
	CommitSHA string

	// SourceURL is the original remote reference
	SourceURL string

	// CacheHit indicates whether the workflow was served from cache
	CacheHit bool
}

// NewFetcher creates a new remote workflow fetcher.
func NewFetcher(cfg Config) (*Fetcher, error) {
	// Create logger with component context
	logger := internallog.WithComponent(internallog.New(internallog.FromEnv()), "remote_fetcher")

	// Initialize GitHub client
	client := github.NewClient(github.Config{
		Token: cfg.GitHubToken,
		Host:  cfg.GitHubHost,
	})

	// Initialize cache (skip if disabled)
	var workflowCache *cache.WorkflowCache
	if !cfg.DisableCache {
		var err error
		workflowCache, err = cache.NewWorkflowCache(cache.Config{
			BasePath: cfg.CacheBasePath,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize cache: %w", err)
		}
	}

	return &Fetcher{
		client:  client,
		cache:   workflowCache,
		logger:  logger,
		verbose: cfg.Verbose,
	}, nil
}

// Fetch fetches a remote workflow with cache-first resolution.
// If noCache is true, bypasses cache and forces a fresh fetch.
func (f *Fetcher) Fetch(ctx context.Context, refStr string, noCache bool) (*FetchResult, error) {
	// Parse remote reference
	ref, err := remote.ParseReference(refStr)
	if err != nil {
		return nil, fmt.Errorf("invalid remote reference: %w", err)
	}

	if f.verbose {
		f.logger.Debug("fetching remote workflow",
			slog.String("reference", refStr))
	}

	// Resolve ref to commit SHA
	sha, err := f.client.ResolveRef(ctx, ref.Owner, ref.Repo, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve reference: %w", err)
	}

	if f.verbose {
		f.logger.Debug("resolved to commit SHA",
			slog.String("sha", sha))
	}

	// Try cache first (unless noCache is true or cache is disabled)
	if !noCache && f.cache != nil {
		cached, err := f.cache.Get(ref.Owner, ref.Repo, sha)
		if err != nil {
			// Log cache error but continue with fetch
			if f.verbose {
				f.logger.Debug("cache read error (continuing with fetch)",
					internallog.Error(err))
			}
		} else if cached != nil {
			// Cache hit
			if f.verbose {
				f.logger.Debug("cache hit",
					slog.String("reference", refStr),
					slog.String("sha", sha[:7]))
			}
			return &FetchResult{
				Content:   cached.Content,
				CommitSHA: cached.CommitSHA,
				SourceURL: cached.SourceURL,
				CacheHit:  true,
			}, nil
		}
	}

	if f.verbose {
		f.logger.Debug("cache miss - fetching from GitHub")
	}

	// Cache miss or cache disabled - fetch from GitHub
	content, fetchedSHA, err := f.client.FetchWorkflow(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workflow: %w", err)
	}

	// Validate that content is not empty
	if len(content) == 0 {
		return nil, fmt.Errorf("fetched workflow is empty")
	}

	if f.verbose {
		f.logger.Debug("downloaded workflow",
			slog.Int("bytes", len(content)))
	}

	// Store in cache (if enabled)
	if f.cache != nil {
		if err := f.cache.Put(ref.Owner, ref.Repo, fetchedSHA, ref.String(), content); err != nil {
			// Log cache write error but continue
			if f.verbose {
				f.logger.Debug("cache write error (continuing)",
					internallog.Error(err))
			}
		} else if f.verbose {
			f.logger.Debug("cached workflow",
				slog.String("reference", refStr),
				slog.String("sha", fetchedSHA[:7]))
		}
	}

	return &FetchResult{
		Content:   content,
		CommitSHA: fetchedSHA,
		SourceURL: ref.String(),
		CacheHit:  false,
	}, nil
}

// ClearCache clears cached workflows for a repository.
// If repo is empty, clears all workflows for the owner.
// If owner is also empty, clears the entire cache.
func (f *Fetcher) ClearCache(owner, repo string) error {
	if f.cache == nil {
		return fmt.Errorf("cache is disabled")
	}
	return f.cache.Clear(owner, repo)
}

// ListCache returns all cached entries for a repository.
func (f *Fetcher) ListCache(owner, repo string) ([]cache.Metadata, error) {
	if f.cache == nil {
		return nil, fmt.Errorf("cache is disabled")
	}
	return f.cache.List(owner, repo)
}
