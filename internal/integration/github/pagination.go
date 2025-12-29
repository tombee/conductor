package github

import (
	"context"
	"fmt"
	"regexp"

	op "github.com/tombee/conductor/internal/operation"
)

// ExecutePaginated implements paginated operations for GitHub connector.
// Supports list_issues, list_prs, and list_repos operations.
func (c *GitHubIntegration) ExecutePaginated(ctx context.Context, operation string, inputs map[string]interface{}) (<-chan *op.Result, error) {
	// Check if pagination is enabled
	paginate, _ := inputs["paginate"].(bool)
	if !paginate {
		// If pagination is not enabled, execute normally and return single result
		result, err := c.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}

		ch := make(chan *op.Result, 1)
		ch <- result
		close(ch)
		return ch, nil
	}

	// Validate operation supports pagination
	switch operation {
	case "list_issues", "list_prs", "list_repos":
		// Supported
	default:
		return nil, fmt.Errorf("operation %s does not support pagination", operation)
	}

	// Create results channel
	resultsChan := make(chan *op.Result)

	// Start pagination in goroutine
	go func() {
		defer close(resultsChan)

		// Get max results limit
		maxResults := 0
		if max, ok := inputs["max_results"].(int); ok {
			maxResults = max
		}

		// Set page size (default to 100, GitHub's max)
		pageSize := 100
		if perPage, ok := inputs["per_page"].(int); ok {
			pageSize = perPage
		}
		if pageSize > 100 {
			pageSize = 100
		}

		// Track total results sent
		totalSent := 0

		// Start with page 1
		page := 1
		inputs["per_page"] = pageSize

		for {
			// Check context cancellation
			if ctx.Err() != nil {
				return
			}

			// Set current page
			inputs["page"] = page

			// Execute request
			result, err := c.Execute(ctx, operation, inputs)
			if err != nil {
				// Send error in metadata
				errResult := &op.Result{
					Metadata: map[string]interface{}{
						"error": err.Error(),
					},
				}
				resultsChan <- errResult
				return
			}

			// Send result
			resultsChan <- result

			// Count results in this page
			var resultsInPage int
			if respSlice, ok := result.Response.([]map[string]interface{}); ok {
				resultsInPage = len(respSlice)
			} else if respSlice, ok := result.Response.([]interface{}); ok {
				resultsInPage = len(respSlice)
			}

			totalSent += resultsInPage

			// Check if we've reached max results
			if maxResults > 0 && totalSent >= maxResults {
				return
			}

			// Check if this is the last page (fewer results than page size)
			if resultsInPage < pageSize {
				return
			}

			// Check Link header for next page
			hasNextPage := false
			if linkHeaders, ok := result.Headers["Link"]; ok && len(linkHeaders) > 0 {
				hasNextPage = hasNextPageLink(linkHeaders[0])
			}

			if !hasNextPage {
				return
			}

			// Move to next page
			page++
		}
	}()

	return resultsChan, nil
}

// hasNextPageLink checks if the Link header contains a "next" relation.
// GitHub uses RFC 5988 Link headers for pagination.
func hasNextPageLink(linkHeader string) bool {
	// Parse Link header: <https://api.github.com/repos/...?page=2>; rel="next"
	nextLinkRegex := regexp.MustCompile(`<[^>]+>;\s*rel="next"`)
	return nextLinkRegex.MatchString(linkHeader)
}
