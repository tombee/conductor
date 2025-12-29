package github

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// createIssue creates a new GitHub issue.
func (c *GitHubIntegration) createIssue(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo", "title"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/issues", inputs)
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
	var issue Issue
	if err := c.ParseJSONResponse(resp, &issue); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"number":   issue.Number,
		"html_url": issue.HTMLURL,
		"state":    issue.State,
	}), nil
}

// updateIssue updates an existing GitHub issue.
func (c *GitHubIntegration) updateIssue(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo", "issue_number"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/issues/{issue_number}", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body (exclude path parameters)
	body, err := c.BuildRequestBody(inputs, []string{"owner", "repo", "issue_number"})
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "PATCH", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var issue Issue
	if err := c.ParseJSONResponse(resp, &issue); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"number":     issue.Number,
		"html_url":   issue.HTMLURL,
		"state":      issue.State,
		"updated_at": issue.UpdatedAt,
	}), nil
}

// closeIssue closes a GitHub issue.
func (c *GitHubIntegration) closeIssue(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo", "issue_number"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/issues/{issue_number}", inputs)
	if err != nil {
		return nil, err
	}

	// Hardcode state to closed
	body := []byte(`{"state":"closed"}`)

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "PATCH", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var issue Issue
	if err := c.ParseJSONResponse(resp, &issue); err != nil {
		return nil, err
	}

	// Return operation result
	result := map[string]interface{}{
		"number": issue.Number,
		"state":  issue.State,
	}
	if issue.ClosedAt != nil {
		result["closed_at"] = issue.ClosedAt
	}

	return c.ToConnectorResult(resp, result), nil
}

// addComment adds a comment to a GitHub issue or PR.
func (c *GitHubIntegration) addComment(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo", "issue_number", "body"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/issues/{issue_number}/comments", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body (exclude path parameters)
	body, err := c.BuildRequestBody(inputs, []string{"owner", "repo", "issue_number"})
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
	var comment Comment
	if err := c.ParseJSONResponse(resp, &comment); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"id":         comment.ID,
		"html_url":   comment.HTMLURL,
		"created_at": comment.CreatedAt,
	}), nil
}

// listIssues lists issues for a repository.
func (c *GitHubIntegration) listIssues(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/issues", inputs)
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
	var issues []Issue
	if err := c.ParseJSONResponse(resp, &issues); err != nil {
		return nil, err
	}

	// Transform to simplified format
	result := make([]map[string]interface{}, len(issues))
	for i, issue := range issues {
		labelNames := make([]string, len(issue.Labels))
		for j, label := range issue.Labels {
			labelNames[j] = label.Name
		}

		result[i] = map[string]interface{}{
			"number":     issue.Number,
			"title":      issue.Title,
			"state":      issue.State,
			"html_url":   issue.HTMLURL,
			"created_at": issue.CreatedAt,
			"labels":     labelNames,
		}
	}

	return c.ToConnectorResult(resp, result), nil
}
