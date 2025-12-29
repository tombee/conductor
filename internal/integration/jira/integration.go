package jira

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
)

// JiraIntegration implements the Connector interface for Jira API.
type JiraIntegration struct {
	*api.BaseConnector
	email     string
	apiToken  string
	transport transport.Transport
}

// NewJiraIntegration creates a new Jira integration.
func NewJiraIntegration(config *api.ConnectorConfig) (operation.Connector, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("jira connector requires base_url configuration (e.g., https://your-domain.atlassian.net)")
	}

	// Jira requires email and API token for Basic auth
	email := ""
	if config.AdditionalAuth != nil {
		email = config.AdditionalAuth["email"]
	}
	if email == "" {
		return nil, fmt.Errorf("jira connector requires email in additional_auth configuration")
	}

	if config.Token == "" {
		return nil, fmt.Errorf("jira connector requires API token")
	}

	// Ensure base URL ends with /rest/api/3
	baseURL := config.BaseURL
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	config.BaseURL = baseURL + "/rest/api/3"

	base := api.NewBaseConnector("jira", config)

	return &JiraIntegration{
		BaseConnector: base,
		email:         email,
		apiToken:      config.Token,
		transport:     config.Transport,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *JiraIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
	switch operation {
	// Issues
	case "create_issue":
		return c.createIssue(ctx, inputs)
	case "update_issue":
		return c.updateIssue(ctx, inputs)
	case "get_issue":
		return c.getIssue(ctx, inputs)
	case "transition_issue":
		return c.transitionIssue(ctx, inputs)
	case "get_transitions":
		return c.getTransitions(ctx, inputs)

	// Comments
	case "add_comment":
		return c.addComment(ctx, inputs)

	// Assignment
	case "assign_issue":
		return c.assignIssue(ctx, inputs)

	// Search
	case "search_issues":
		return c.searchIssues(ctx, inputs)

	// Projects
	case "list_projects":
		return c.listProjects(ctx, inputs)

	// Attachments
	case "add_attachment":
		return c.addAttachment(ctx, inputs)

	// Links
	case "link_issues":
		return c.linkIssues(ctx, inputs)

	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (c *JiraIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		// Issues
		{Name: "create_issue", Description: "Create a new issue", Category: "issues", Tags: []string{"write"}},
		{Name: "update_issue", Description: "Update an existing issue", Category: "issues", Tags: []string{"write"}},
		{Name: "get_issue", Description: "Get issue details", Category: "issues", Tags: []string{"read"}},
		{Name: "transition_issue", Description: "Change issue status", Category: "issues", Tags: []string{"write"}},
		{Name: "get_transitions", Description: "Get available transitions for an issue", Category: "issues", Tags: []string{"read"}},

		// Comments
		{Name: "add_comment", Description: "Add a comment to an issue", Category: "comments", Tags: []string{"write"}},

		// Assignment
		{Name: "assign_issue", Description: "Assign an issue to a user", Category: "assignment", Tags: []string{"write"}},

		// Search
		{Name: "search_issues", Description: "Search issues with JQL", Category: "search", Tags: []string{"read", "paginated"}},

		// Projects
		{Name: "list_projects", Description: "List accessible projects", Category: "projects", Tags: []string{"read"}},

		// Attachments
		{Name: "add_attachment", Description: "Add an attachment to an issue", Category: "attachments", Tags: []string{"write"}},

		// Links
		{Name: "link_issues", Description: "Link two issues together", Category: "links", Tags: []string{"write"}},
	}
}

// OperationSchema returns the schema for an operation.
func (c *JiraIntegration) OperationSchema(operation string) *api.OperationSchema {
	// This would return detailed schema information for each operation
	// For now, returning nil (would be implemented based on requirements)
	return nil
}

// defaultHeaders returns default headers for Jira API requests.
func (c *JiraIntegration) defaultHeaders() map[string]string {
	return map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}
}

// ExecuteRequest overrides BaseConnector's method to use Basic authentication.
func (c *JiraIntegration) ExecuteRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (*transport.Response, error) {
	// Add Basic authentication header
	if c.email != "" && c.apiToken != "" {
		if headers == nil {
			headers = make(map[string]string)
		}
		credentials := c.email + ":" + c.apiToken
		encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
		headers["Authorization"] = "Basic " + encoded
	}

	req := &transport.Request{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}

	return c.transport.Execute(ctx, req)
}
