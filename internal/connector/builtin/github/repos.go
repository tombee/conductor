package github

import (
	"context"

	"github.com/tombee/conductor/internal/connector"
)

// getFile gets file contents from a GitHub repository.
func (c *GitHubConnector) getFile(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo", "path"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/contents/{path}", inputs)
	if err != nil {
		return nil, err
	}

	// Add query parameters (e.g., ref for branch/tag/commit)
	pathParams := []string{"owner", "repo", "path"}
	queryString := c.BuildQueryString(inputs, pathParams)
	url += queryString

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var fileContent FileContent
	if err := c.ParseJSONResponse(resp, &fileContent); err != nil {
		return nil, err
	}

	// Return connector result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"name":         fileContent.Name,
		"path":         fileContent.Path,
		"sha":          fileContent.SHA,
		"size":         fileContent.Size,
		"content":      fileContent.Content,
		"encoding":     fileContent.Encoding,
		"download_url": fileContent.DownloadURL,
	}), nil
}

// listRepos lists repositories for a GitHub user.
func (c *GitHubConnector) listRepos(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"username"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/users/{username}/repos", inputs)
	if err != nil {
		return nil, err
	}

	// Add query parameters
	pathParams := []string{"username"}
	queryString := c.BuildQueryString(inputs, pathParams)
	url += queryString

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var repos []Repository
	if err := c.ParseJSONResponse(resp, &repos); err != nil {
		return nil, err
	}

	// Transform to simplified format
	result := make([]map[string]interface{}, len(repos))
	for i, repo := range repos {
		item := map[string]interface{}{
			"name":        repo.Name,
			"full_name":   repo.FullName,
			"description": repo.Description,
			"html_url":    repo.HTMLURL,
			"created_at":  repo.CreatedAt,
		}

		// Note: language and stargazers_count would be in the full repo response
		// These fields may not be present in all responses, so we'd need to add them to Repository type
		// For now, keeping the basic fields

		result[i] = item
	}

	return c.ToConnectorResult(resp, result), nil
}
