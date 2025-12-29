package jira

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
)

// createIssue creates a new Jira issue.
func (c *JiraIntegration) createIssue(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"project", "summary", "issuetype"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/issue", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	fields := make(map[string]interface{})

	// Required fields
	fields["project"] = map[string]string{"key": fmt.Sprint(inputs["project"])}
	fields["summary"] = inputs["summary"]
	fields["issuetype"] = map[string]string{"name": fmt.Sprint(inputs["issuetype"])}

	// Optional fields
	if description, ok := inputs["description"]; ok {
		fields["description"] = description
	}
	if assignee, ok := inputs["assignee"]; ok {
		fields["assignee"] = map[string]string{"accountId": fmt.Sprint(assignee)}
	}
	if priority, ok := inputs["priority"]; ok {
		fields["priority"] = map[string]string{"name": fmt.Sprint(priority)}
	}
	if labels, ok := inputs["labels"]; ok {
		fields["labels"] = labels
	}

	// Allow arbitrary additional fields
	for key, value := range inputs {
		if key != "project" && key != "summary" && key != "issuetype" &&
		   key != "description" && key != "assignee" && key != "priority" && key != "labels" {
			fields[key] = value
		}
	}

	requestBody := map[string]interface{}{
		"fields": fields,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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
	return c.ToResult(resp, map[string]interface{}{
		"id":   issue.ID,
		"key":  issue.Key,
		"self": issue.Self,
	}), nil
}

// updateIssue updates an existing Jira issue.
func (c *JiraIntegration) updateIssue(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"issue_key"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/issue/{issue_key}", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body - only include fields to update
	fields := make(map[string]interface{})

	for key, value := range inputs {
		if key == "issue_key" {
			continue
		}

		// Handle special fields that need object wrapping
		switch key {
		case "assignee":
			fields["assignee"] = map[string]string{"accountId": fmt.Sprint(value)}
		case "priority":
			fields["priority"] = map[string]string{"name": fmt.Sprint(value)}
		default:
			fields[key] = value
		}
	}

	requestBody := map[string]interface{}{
		"fields": fields,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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

	// Jira returns 204 No Content on successful update
	return c.ToResult(resp, map[string]interface{}{
		"success": true,
	}), nil
}

// getIssue retrieves details of a Jira issue.
func (c *JiraIntegration) getIssue(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"issue_key"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/issue/{issue_key}", inputs)
	if err != nil {
		return nil, err
	}

	// Add optional query parameters (fields, expand, etc.)
	pathParams := []string{"issue_key"}
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
	var issue Issue
	if err := c.ParseJSONResponse(resp, &issue); err != nil {
		return nil, err
	}

	// Transform to simplified format
	result := map[string]interface{}{
		"id":      issue.ID,
		"key":     issue.Key,
		"self":    issue.Self,
		"summary": issue.Fields.Summary,
	}

	if issue.Fields.Description != nil {
		result["description"] = issue.Fields.Description
	}
	if issue.Fields.Status != nil {
		result["status"] = issue.Fields.Status.Name
	}
	if issue.Fields.IssueType != nil {
		result["issuetype"] = issue.Fields.IssueType.Name
	}
	if issue.Fields.Assignee != nil {
		result["assignee"] = map[string]interface{}{
			"accountId":   issue.Fields.Assignee.AccountID,
			"displayName": issue.Fields.Assignee.DisplayName,
		}
	}
	if issue.Fields.Priority != nil {
		result["priority"] = issue.Fields.Priority.Name
	}
	if len(issue.Fields.Labels) > 0 {
		result["labels"] = issue.Fields.Labels
	}

	return c.ToResult(resp, result), nil
}

// transitionIssue changes the status of a Jira issue.
func (c *JiraIntegration) transitionIssue(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"issue_key", "transition"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/issue/{issue_key}/transitions", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	requestBody := map[string]interface{}{
		"transition": map[string]string{
			"id": fmt.Sprint(inputs["transition"]),
		},
	}

	// Add optional fields for the transition
	fields := make(map[string]interface{})
	for key, value := range inputs {
		if key != "issue_key" && key != "transition" {
			fields[key] = value
		}
	}
	if len(fields) > 0 {
		requestBody["fields"] = fields
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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

	// Jira returns 204 No Content on successful transition
	return c.ToResult(resp, map[string]interface{}{
		"success": true,
	}), nil
}

// getTransitions retrieves available transitions for a Jira issue.
func (c *JiraIntegration) getTransitions(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"issue_key"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/issue/{issue_key}/transitions", inputs)
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
	var transitionsResp TransitionsResponse
	if err := c.ParseJSONResponse(resp, &transitionsResp); err != nil {
		return nil, err
	}

	// Transform to simplified format
	result := make([]map[string]interface{}, len(transitionsResp.Transitions))
	for i, t := range transitionsResp.Transitions {
		result[i] = map[string]interface{}{
			"id":   t.ID,
			"name": t.Name,
			"to": map[string]interface{}{
				"id":   t.To.ID,
				"name": t.To.Name,
			},
		}
	}

	return c.ToResult(resp, map[string]interface{}{
		"transitions": result,
	}), nil
}

// addComment adds a comment to a Jira issue.
func (c *JiraIntegration) addComment(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"issue_key", "body"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/issue/{issue_key}/comment", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	requestBody := map[string]interface{}{
		"body": inputs["body"],
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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
	return c.ToResult(resp, map[string]interface{}{
		"id":      comment.ID,
		"self":    comment.Self,
		"created": comment.Created,
	}), nil
}

// assignIssue assigns a Jira issue to a user.
func (c *JiraIntegration) assignIssue(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"issue_key", "accountId"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/issue/{issue_key}/assignee", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	requestBody := map[string]interface{}{
		"accountId": inputs["accountId"],
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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

	// Jira returns 204 No Content on successful assignment
	return c.ToResult(resp, map[string]interface{}{
		"success": true,
	}), nil
}

// searchIssues searches for issues using JQL.
func (c *JiraIntegration) searchIssues(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"jql"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/search", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body with JQL and pagination parameters
	requestBody := map[string]interface{}{
		"jql": inputs["jql"],
	}

	// Handle pagination parameters
	if startAt, ok := inputs["startAt"]; ok {
		requestBody["startAt"] = startAt
	}
	if maxResults, ok := inputs["maxResults"]; ok {
		requestBody["maxResults"] = maxResults
	}
	if fields, ok := inputs["fields"]; ok {
		requestBody["fields"] = fields
	}
	if expand, ok := inputs["expand"]; ok {
		requestBody["expand"] = expand
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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
	var searchResults SearchResults
	if err := c.ParseJSONResponse(resp, &searchResults); err != nil {
		return nil, err
	}

	// Transform issues to simplified format
	issues := make([]map[string]interface{}, len(searchResults.Issues))
	for i, issue := range searchResults.Issues {
		issueMap := map[string]interface{}{
			"id":      issue.ID,
			"key":     issue.Key,
			"self":    issue.Self,
			"summary": issue.Fields.Summary,
		}

		if issue.Fields.Status != nil {
			issueMap["status"] = issue.Fields.Status.Name
		}
		if issue.Fields.IssueType != nil {
			issueMap["issuetype"] = issue.Fields.IssueType.Name
		}
		if issue.Fields.Assignee != nil {
			issueMap["assignee"] = issue.Fields.Assignee.DisplayName
		}

		issues[i] = issueMap
	}

	// Return operation result with pagination info
	return c.ToResult(resp, map[string]interface{}{
		"issues":     issues,
		"startAt":    searchResults.StartAt,
		"maxResults": searchResults.MaxResults,
		"total":      searchResults.Total,
	}), nil
}

// addAttachment adds an attachment to a Jira issue.
func (c *JiraIntegration) addAttachment(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"issue_key", "file"}); err != nil {
		return nil, err
	}

	// Build URL
	_, err := c.BuildURL("/issue/{issue_key}/attachments", inputs)
	if err != nil {
		return nil, err
	}

	// For attachments, we need multipart/form-data
	// This is a simplified implementation - would need proper multipart handling
	// For now, return an error indicating this needs implementation
	return nil, fmt.Errorf("add_attachment operation requires multipart/form-data support - not yet implemented")
}

// linkIssues creates a link between two Jira issues.
func (c *JiraIntegration) linkIssues(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"type", "inwardIssue", "outwardIssue"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/issueLink", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	requestBody := map[string]interface{}{
		"type": map[string]string{
			"name": fmt.Sprint(inputs["type"]),
		},
		"inwardIssue": map[string]string{
			"key": fmt.Sprint(inputs["inwardIssue"]),
		},
		"outwardIssue": map[string]string{
			"key": fmt.Sprint(inputs["outwardIssue"]),
		},
	}

	// Optional comment
	if comment, ok := inputs["comment"]; ok {
		requestBody["comment"] = map[string]interface{}{
			"body": comment,
		}
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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

	// Jira returns 201 Created on successful link creation
	return c.ToResult(resp, map[string]interface{}{
		"success": true,
	}), nil
}
