package github

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// createRelease creates a new GitHub release.
func (c *GitHubConnector) createRelease(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo", "tag_name"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/releases", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body (exclude path parameters)
	body, err := c.BuildRequestBody(inputs, []string{"owner", "repo"})
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var release Release
	if err := c.ParseJSONResponse(resp, &release); err != nil {
		return nil, err
	}

	// Return connector result
	result := map[string]interface{}{
		"id":         release.ID,
		"html_url":   release.HTMLURL,
		"tag_name":   release.TagName,
		"name":       release.Name,
		"draft":      release.Draft,
		"prerelease": release.Prerelease,
		"created_at": release.CreatedAt,
	}

	return c.ToConnectorResult(resp, result), nil
}
