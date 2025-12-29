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

// Package cache provides workflow caching for remote workflows.
package cache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	daemonmetrics "github.com/tombee/conductor/internal/controller/metrics"
	"github.com/tombee/conductor/pkg/security"
)

// WorkflowCache provides content-addressable storage for remote workflows.
// Workflows are cached by commit SHA for reproducible builds.
type WorkflowCache struct {
	basePath string
	ttl      time.Duration
	logger   *slog.Logger
}

// Config contains cache configuration.
type Config struct {
	// BasePath is the directory where cached workflows are stored
	// Default: ~/.cache/conductor/remote-workflows
	BasePath string

	// TTL is the time-to-live for cached entries
	// Zero means no expiration
	TTL time.Duration

	// Logger for cache operations. If nil, uses slog.Default()
	Logger *slog.Logger
}

// CachedWorkflow represents a cached workflow entry.
type CachedWorkflow struct {
	// SourceURL is the original remote reference (e.g., github:user/repo@v1.0)
	SourceURL string `json:"source_url"`

	// CommitSHA is the git commit SHA this workflow was fetched from
	CommitSHA string `json:"commit_sha"`

	// FetchedAt is when the workflow was downloaded
	FetchedAt time.Time `json:"fetched_at"`

	// Content is the workflow YAML content
	Content []byte `json:"content"`

	// Size is the size of the content in bytes
	Size int `json:"size"`
}

// Metadata represents cached workflow metadata.
type Metadata struct {
	SourceURL string    `json:"source_url"`
	CommitSHA string    `json:"commit_sha"`
	FetchedAt time.Time `json:"fetched_at"`
	Size      int       `json:"size"`
}

// NewWorkflowCache creates a new workflow cache.
func NewWorkflowCache(cfg Config) (*WorkflowCache, error) {
	basePath := cfg.BasePath
	if basePath == "" {
		// Default to XDG cache directory
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		basePath = filepath.Join(home, ".cache", "conductor", "remote-workflows")
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &WorkflowCache{
		basePath: basePath,
		ttl:      cfg.TTL,
		logger:   logger,
	}, nil
}

// Get retrieves a cached workflow by owner, repo, and commit SHA.
// Returns nil if not found or expired.
func (c *WorkflowCache) Get(owner, repo, sha string) (*CachedWorkflow, error) {
	// Build cache path
	cachePath := c.getCachePath(owner, repo, sha)

	// Check if cached files exist
	contentPath := filepath.Join(cachePath, "workflow.yaml")
	metadataPath := filepath.Join(cachePath, "metadata.json")

	if !fileExists(contentPath) || !fileExists(metadataPath) {
		return nil, nil // Cache miss
	}

	// Load metadata
	metadata, err := c.loadMetadata(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	// Check if expired
	if c.ttl > 0 && time.Since(metadata.FetchedAt) > c.ttl {
		// Expired - delete and return nil
		if err := os.RemoveAll(cachePath); err != nil {
			cacheIdentifier := fmt.Sprintf("%s/%s@%s", owner, repo, sha)
			c.logger.Warn("failed to cleanup expired cache", "cache", cacheIdentifier, "error", err)
			daemonmetrics.RecordPersistenceError("CleanupCache", categorizeError(err))
		}
		return nil, nil
	}

	// Load content
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cached content: %w", err)
	}

	return &CachedWorkflow{
		SourceURL: metadata.SourceURL,
		CommitSHA: metadata.CommitSHA,
		FetchedAt: metadata.FetchedAt,
		Content:   content,
		Size:      metadata.Size,
	}, nil
}

// Put stores a workflow in the cache.
func (c *WorkflowCache) Put(owner, repo, sha, sourceURL string, content []byte) error {
	// Build cache path
	cachePath := c.getCachePath(owner, repo, sha)

	// Create directory structure with appropriate permissions
	contentPath := filepath.Join(cachePath, "workflow.yaml")
	fileMode, dirMode := security.DeterminePermissions(contentPath)
	if err := os.MkdirAll(cachePath, dirMode); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Write content with computed permissions
	if err := os.WriteFile(contentPath, content, fileMode); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	// Write metadata
	metadata := Metadata{
		SourceURL: sourceURL,
		CommitSHA: sha,
		FetchedAt: time.Now(),
		Size:      len(content),
	}

	metadataPath := filepath.Join(cachePath, "metadata.json")
	if err := c.saveMetadata(metadataPath, metadata); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// Delete removes a cached workflow.
func (c *WorkflowCache) Delete(owner, repo, sha string) error {
	cachePath := c.getCachePath(owner, repo, sha)
	if err := os.RemoveAll(cachePath); err != nil {
		return fmt.Errorf("failed to delete cache entry: %w", err)
	}
	return nil
}

// Clear removes all cached workflows for a repository.
// If repo is empty, clears all workflows for the owner.
// If owner is also empty, clears the entire cache.
func (c *WorkflowCache) Clear(owner, repo string) error {
	var path string
	if owner == "" {
		// Clear entire cache
		path = filepath.Join(c.basePath, "github.com")
	} else if repo == "" {
		// Clear all repos for owner
		path = filepath.Join(c.basePath, "github.com", owner)
	} else {
		// Clear specific repo
		path = filepath.Join(c.basePath, "github.com", owner, repo)
	}

	if !fileExists(path) {
		return nil // Nothing to clear
	}

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	return nil
}

// List returns all cached entries for a repository.
func (c *WorkflowCache) List(owner, repo string) ([]Metadata, error) {
	repoPath := filepath.Join(c.basePath, "github.com", owner, repo)

	if !fileExists(repoPath) {
		return []Metadata{}, nil
	}

	var entries []Metadata

	// Iterate through SHA directories
	shaEntries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, shaEntry := range shaEntries {
		if !shaEntry.IsDir() {
			continue
		}

		metadataPath := filepath.Join(repoPath, shaEntry.Name(), "metadata.json")
		if !fileExists(metadataPath) {
			continue
		}

		metadata, err := c.loadMetadata(metadataPath)
		if err != nil {
			continue // Skip invalid entries
		}

		entries = append(entries, metadata)
	}

	return entries, nil
}

// getCachePath returns the cache directory path for a specific workflow.
// Structure: basePath/github.com/owner/repo/sha/
func (c *WorkflowCache) getCachePath(owner, repo, sha string) string {
	return filepath.Join(c.basePath, "github.com", owner, repo, sha)
}

// loadMetadata loads metadata from a JSON file.
func (c *WorkflowCache) loadMetadata(path string) (Metadata, error) {
	var metadata Metadata

	data, err := os.ReadFile(path)
	if err != nil {
		return metadata, err
	}

	if err := json.Unmarshal(data, &metadata); err != nil {
		return metadata, err
	}

	return metadata, nil
}

// saveMetadata saves metadata to a JSON file.
func (c *WorkflowCache) saveMetadata(path string, metadata Metadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	fileMode, _ := security.DeterminePermissions(path)
	return os.WriteFile(path, data, fileMode)
}

// fileExists checks if a file or directory exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// categorizeError returns a simplified error type for metrics.
func categorizeError(err error) string {
	if err == nil {
		return "unknown"
	}
	if os.IsNotExist(err) {
		return "not_found"
	}
	if os.IsPermission(err) {
		return "permission_denied"
	}
	return "unknown"
}
