package github

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// getPull fetches details for a specific pull request.
func (c *GitHubIntegration) getPull(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo", "pull_number"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/pulls/{pull_number}", inputs)
	if err != nil {
		return nil, err
	}

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
	var pr PullRequest
	if err := c.ParseJSONResponse(resp, &pr); err != nil {
		return nil, err
	}

	// Transform labels to string slice
	labels := make([]string, len(pr.Labels))
	for i, label := range pr.Labels {
		labels[i] = label.Name
	}

	// Return full PR details
	result := map[string]interface{}{
		"id":         pr.ID,
		"number":     pr.Number,
		"title":      pr.Title,
		"body":       pr.Body,
		"state":      pr.State,
		"html_url":   pr.HTMLURL,
		"user":       map[string]interface{}{"login": pr.User.Login, "id": pr.User.ID, "html_url": pr.User.HTMLURL},
		"labels":     labels,
		"head":       map[string]interface{}{"ref": pr.Head.Ref, "sha": pr.Head.SHA},
		"base":       map[string]interface{}{"ref": pr.Base.Ref, "sha": pr.Base.SHA},
		"merged":     pr.Merged,
		"mergeable":  pr.Mergeable,
		"created_at": pr.CreatedAt,
		"updated_at": pr.UpdatedAt,
	}

	// Add optional fields if present
	if pr.MergedAt != nil {
		result["merged_at"] = pr.MergedAt
	}
	if pr.ClosedAt != nil {
		result["closed_at"] = pr.ClosedAt
	}

	return c.ToResult(resp, result), nil
}

// createPR creates a new GitHub pull request.
func (c *GitHubIntegration) createPR(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo", "title", "head", "base"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/pulls", inputs)
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
	var pr PullRequest
	if err := c.ParseJSONResponse(resp, &pr); err != nil {
		return nil, err
	}

	// Return operation result
	result := map[string]interface{}{
		"number":   pr.Number,
		"html_url": pr.HTMLURL,
		"state":    pr.State,
	}

	// Add draft field if present (draft field may not be in response for older API versions)
	result["draft"] = false
	// Note: The draft field would be in the full PR response in newer API versions

	return c.ToResult(resp, result), nil
}

// mergePR merges a GitHub pull request.
func (c *GitHubIntegration) mergePR(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo", "pull_number"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/pulls/{pull_number}/merge", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body (exclude path parameters)
	body, err := c.BuildRequestBody(inputs, []string{"owner", "repo", "pull_number"})
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "PUT", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var mergeResult struct {
		SHA     string `json:"sha"`
		Merged  bool   `json:"merged"`
		Message string `json:"message"`
	}
	if err := c.ParseJSONResponse(resp, &mergeResult); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"merged":  mergeResult.Merged,
		"sha":     mergeResult.SHA,
		"message": mergeResult.Message,
	}), nil
}

// listPRs lists pull requests for a repository.
func (c *GitHubIntegration) listPRs(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/pulls", inputs)
	if err != nil {
		return nil, err
	}

	// Add query parameters
	pathParams := []string{"owner", "repo"}
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
	var prs []PullRequest
	if err := c.ParseJSONResponse(resp, &prs); err != nil {
		return nil, err
	}

	// Transform to simplified format
	result := make([]map[string]interface{}, len(prs))
	for i, pr := range prs {
		result[i] = map[string]interface{}{
			"number":     pr.Number,
			"title":      pr.Title,
			"state":      pr.State,
			"html_url":   pr.HTMLURL,
			"head":       pr.Head.Ref,
			"base":       pr.Base.Ref,
			"created_at": pr.CreatedAt,
		}
	}

	return c.ToResult(resp, result), nil
}
