package notion

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// search performs a search across pages and databases accessible to the integration.
func (c *NotionIntegration) search(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Build search request
	req := SearchRequest{}

	// Optional query string
	if query, ok := inputs["query"].(string); ok {
		req.Query = query
	}

	// Optional filter (object type: page or database)
	if filter, ok := inputs["filter"]; ok {
		req.Filter = filter
	}

	// Optional sort configuration
	if sort, ok := inputs["sort"]; ok {
		req.Sort = sort
	}

	// Optional pagination
	if startCursor, ok := inputs["start_cursor"].(string); ok {
		req.StartCursor = startCursor
	}
	if pageSize, ok := inputs["page_size"].(float64); ok {
		req.PageSize = int(pageSize)
	}

	// Build URL
	url, err := c.BuildURL("/search", inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var searchResp SearchResponse
	if err := c.ParseJSONResponse(resp, &searchResp); err != nil {
		return nil, err
	}

	// Transform results to simplified format
	results := make([]map[string]interface{}, 0, len(searchResp.Results))
	for _, result := range searchResp.Results {
		item := map[string]interface{}{
			"id":   result.ID,
			"type": result.Object,
			"url":  result.URL,
		}

		// Extract title based on object type
		if result.Object == "database" {
			// Database title is in the title field
			if titleArr, ok := result.Title.([]interface{}); ok && len(titleArr) > 0 {
				if titleObj, ok := titleArr[0].(map[string]interface{}); ok {
					if plainText, ok := titleObj["plain_text"].(string); ok {
						item["title"] = plainText
					}
				}
			}
		} else if result.Object == "page" {
			// Page title is in properties.title
			item["title"] = extractSearchResultTitle(result.Properties)
		}

		results = append(results, item)
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"results":     results,
		"has_more":    searchResp.HasMore,
		"next_cursor": searchResp.NextCursor,
	}), nil
}

// extractSearchResultTitle extracts the title from page properties in search results.
func extractSearchResultTitle(properties interface{}) string {
	propsMap, ok := properties.(map[string]interface{})
	if !ok {
		return ""
	}

	titleProp, ok := propsMap["title"]
	if !ok {
		// Try "Name" as common database property name
		titleProp, ok = propsMap["Name"]
		if !ok {
			return ""
		}
	}

	titleMap, ok := titleProp.(map[string]interface{})
	if !ok {
		return ""
	}

	titleArray, ok := titleMap["title"].([]interface{})
	if !ok || len(titleArray) == 0 {
		return ""
	}

	firstTitle, ok := titleArray[0].(map[string]interface{})
	if !ok {
		return ""
	}

	if plainText, ok := firstTitle["plain_text"].(string); ok {
		return plainText
	}

	return ""
}
