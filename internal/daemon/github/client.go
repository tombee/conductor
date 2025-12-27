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

// Package github provides a GitHub API client for fetching repository content.
package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/remote"
	"github.com/tombee/conductor/pkg/httpclient"
)

// Client is a GitHub API client for fetching repository content.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Config contains configuration for the GitHub client.
type Config struct {
	// Token is the GitHub personal access token (optional for public repos)
	Token string

	// Host is the GitHub API host (default: api.github.com)
	// For GitHub Enterprise, use your custom host (e.g., github.company.com)
	Host string

	// HTTPClient is an optional custom HTTP client
	HTTPClient *http.Client
}

// NewClient creates a new GitHub API client.
func NewClient(cfg Config) *Client {
	// Default to GitHub.com API
	baseURL := "https://api.github.com"
	if cfg.Host != "" && cfg.Host != "github.com" {
		// GitHub Enterprise uses /api/v3 path prefix
		baseURL = fmt.Sprintf("https://%s/api/v3", cfg.Host)
	}

	// Use default HTTP client if not provided
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpCfg := httpclient.DefaultConfig()
		httpCfg.Timeout = 30 * time.Second
		httpCfg.UserAgent = "conductor-github-client/1.0"

		client, err := httpclient.New(httpCfg)
		if err != nil {
			// Fallback to basic client
			httpClient = &http.Client{Timeout: 30 * time.Second}
		} else {
			httpClient = client
		}
	}

	return &Client{
		baseURL:    baseURL,
		token:      cfg.Token,
		httpClient: httpClient,
	}
}

// ResolveToken attempts to resolve a GitHub token from various sources.
// Resolution order:
// 1. GITHUB_TOKEN environment variable
// 2. CONDUCTOR_GITHUB_TOKEN environment variable
// 3. GitHub CLI (gh auth token)
// Returns empty string if no token found (anonymous access).
func ResolveToken() string {
	// 1. GITHUB_TOKEN (standard for CI/CD)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	// 2. CONDUCTOR_GITHUB_TOKEN (conductor-specific)
	if token := os.Getenv("CONDUCTOR_GITHUB_TOKEN"); token != "" {
		return token
	}

	// 3. Try GitHub CLI
	if token := getGHCLIToken(); token != "" {
		return token
	}

	return ""
}

// getGHCLIToken attempts to get a token from GitHub CLI.
func getGHCLIToken() string {
	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// ContentResponse represents a GitHub API content response.
type ContentResponse struct {
	Type        string `json:"type"`
	Encoding    string `json:"encoding"`
	Size        int    `json:"size"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	SHA         string `json:"sha"`
	URL         string `json:"url"`
	GitURL      string `json:"git_url"`
	HTMLURL     string `json:"html_url"`
	DownloadURL string `json:"download_url"`
}

// RefResponse represents a GitHub API git reference response.
type RefResponse struct {
	Ref    string `json:"ref"`
	NodeID string `json:"node_id"`
	URL    string `json:"url"`
	Object struct {
		Type string `json:"type"`
		SHA  string `json:"sha"`
		URL  string `json:"url"`
	} `json:"object"`
}

// GetContent fetches file content from a GitHub repository.
// ref can be a branch name, tag, or commit SHA.
func (c *Client) GetContent(ctx context.Context, owner, repo, path, ref string) (*ContentResponse, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, owner, repo, path)
	if ref != "" {
		url += fmt.Sprintf("?ref=%s", ref)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		// Provide user-friendly error messages
		switch resp.StatusCode {
		case http.StatusNotFound:
			return nil, fmt.Errorf("workflow not found: %s/%s/%s (check repository name and path)", owner, repo, path)
		case http.StatusUnauthorized, http.StatusForbidden:
			msg := "authentication required for private repository"
			if c.token != "" {
				msg = "access denied (check token permissions)"
			}
			return nil, fmt.Errorf("%s: %s/%s (set GITHUB_TOKEN or use 'gh auth login')", msg, owner, repo)
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("GitHub API rate limit exceeded (try again later or set GITHUB_TOKEN for higher limits)")
		default:
			return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
		}
	}

	var content ContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &content, nil
}

// DecodeContent decodes base64-encoded content from GitHub API response.
func DecodeContent(encoded string) ([]byte, error) {
	// GitHub API returns base64 content with newlines - need to remove them
	encoded = strings.ReplaceAll(encoded, "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 content: %w", err)
	}
	return decoded, nil
}

// ResolveRef resolves a git reference (branch, tag) to a commit SHA.
// For commit SHAs, returns the SHA as-is.
func (c *Client) ResolveRef(ctx context.Context, owner, repo string, ref *remote.Reference) (string, error) {
	// If no version specified, use default branch (fetch it via repo API)
	if ref.Version == "" {
		return c.getDefaultBranch(ctx, owner, repo)
	}

	// If it's already a commit SHA, return it
	if ref.RefType == remote.RefTypeCommit {
		return ref.Version, nil
	}

	// Try to resolve as branch first
	if ref.RefType == remote.RefTypeBranch {
		sha, err := c.resolveRefPath(ctx, owner, repo, "heads/"+ref.Version)
		if err == nil {
			return sha, nil
		}
		// If branch not found, might be a tag - fall through
	}

	// Try to resolve as tag
	if ref.RefType == remote.RefTypeTag {
		sha, err := c.resolveRefPath(ctx, owner, repo, "tags/"+ref.Version)
		if err == nil {
			return sha, nil
		}
	}

	// Last resort: try as commit SHA directly
	return ref.Version, nil
}

// getDefaultBranch fetches the default branch for a repository.
func (c *Client) getDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch repository info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Now resolve the default branch to a commit SHA
	return c.resolveRefPath(ctx, owner, repo, "heads/"+repoInfo.DefaultBranch)
}

// resolveRefPath resolves a git ref path to a commit SHA.
func (c *Client) resolveRefPath(ctx context.Context, owner, repo, refPath string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs/%s", c.baseURL, owner, repo, refPath)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to resolve ref: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	var refResp RefResponse
	if err := json.NewDecoder(resp.Body).Decode(&refResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return refResp.Object.SHA, nil
}

// FetchWorkflow fetches a workflow from a GitHub repository.
// This is a convenience method that combines ref resolution and content fetching.
func (c *Client) FetchWorkflow(ctx context.Context, ref *remote.Reference) ([]byte, string, error) {
	// Resolve ref to commit SHA
	sha, err := c.ResolveRef(ctx, ref.Owner, ref.Repo, ref)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve ref: %w", err)
	}

	// Fetch content at the resolved SHA
	content, err := c.GetContent(ctx, ref.Owner, ref.Repo, ref.FullPath(), sha)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch content: %w", err)
	}

	// Decode base64 content
	data, err := DecodeContent(content.Content)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode content: %w", err)
	}

	return data, sha, nil
}
