package github

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
)

// GitHubIntegration implements the Provider interface for GitHub API.
type GitHubIntegration struct {
	*api.BaseProvider
}

// NewGitHubIntegration creates a new GitHub integration.
func NewGitHubIntegration(config *api.ProviderConfig) (operation.Provider, error) {
	// Set default base URL if not provided
	if config.BaseURL == "" {
		config.BaseURL = "https://api.github.com"
	}

	base := api.NewBaseProvider("github", config)

	return &GitHubIntegration{
		BaseProvider: base,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *GitHubIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
	switch operation {
	// Issues
	case "create_issue":
		return c.createIssue(ctx, inputs)
	case "update_issue":
		return c.updateIssue(ctx, inputs)
	case "close_issue":
		return c.closeIssue(ctx, inputs)
	case "add_comment":
		return c.addComment(ctx, inputs)
	case "list_issues":
		return c.listIssues(ctx, inputs)

	// Pull Requests
	case "create_pr":
		return c.createPR(ctx, inputs)
	case "get_pull":
		return c.getPull(ctx, inputs)
	case "merge_pr":
		return c.mergePR(ctx, inputs)
	case "list_prs":
		return c.listPRs(ctx, inputs)

	// Repositories
	case "get_file":
		return c.getFile(ctx, inputs)
	case "list_repos":
		return c.listRepos(ctx, inputs)

	// Releases
	case "create_release":
		return c.createRelease(ctx, inputs)

	// Actions
	case "get_workflow_runs":
		return c.getWorkflowRuns(ctx, inputs)

	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (c *GitHubIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		// Issues
		{Name: "create_issue", Description: "Create a new issue", Category: "issues", Tags: []string{"write"}},
		{Name: "update_issue", Description: "Update an existing issue", Category: "issues", Tags: []string{"write"}},
		{Name: "close_issue", Description: "Close an issue", Category: "issues", Tags: []string{"write"}},
		{Name: "add_comment", Description: "Add a comment to an issue or PR", Category: "issues", Tags: []string{"write"}},
		{Name: "list_issues", Description: "List issues with filtering", Category: "issues", Tags: []string{"read", "paginated"}},

		// Pull Requests
		{Name: "create_pr", Description: "Create a pull request", Category: "pulls", Tags: []string{"write"}},
		{Name: "get_pull", Description: "Get details for a specific pull request", Category: "pulls", Tags: []string{"read"}},
		{Name: "merge_pr", Description: "Merge a pull request", Category: "pulls", Tags: []string{"write"}},
		{Name: "list_prs", Description: "List pull requests with filtering", Category: "pulls", Tags: []string{"read", "paginated"}},

		// Repositories
		{Name: "get_file", Description: "Get file contents from a repository", Category: "repos", Tags: []string{"read"}},
		{Name: "list_repos", Description: "List user's repositories", Category: "repos", Tags: []string{"read", "paginated"}},

		// Releases
		{Name: "create_release", Description: "Create a new release", Category: "releases", Tags: []string{"write"}},

		// Actions
		{Name: "get_workflow_runs", Description: "List workflow runs", Category: "actions", Tags: []string{"read"}},
	}
}

// OperationSchema returns the schema for an operation.
func (c *GitHubIntegration) OperationSchema(operation string) *api.OperationSchema {
	// This would return detailed schema information for each operation
	// For now, returning nil (would be implemented based on requirements)
	return nil
}

// defaultHeaders returns default headers for GitHub API requests.
func (c *GitHubIntegration) defaultHeaders() map[string]string {
	return map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
		"Content-Type":         "application/json",
	}
}
