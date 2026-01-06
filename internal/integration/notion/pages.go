package notion

import (
	"context"
	"fmt"
	"strings"

	"github.com/tombee/conductor/internal/operation"
)

// createPage creates a new page under a parent page or database.
func (c *NotionIntegration) createPage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"parent_id", "title"}); err != nil {
		return nil, err
	}

	parentID, _ := inputs["parent_id"].(string)
	title, _ := inputs["title"].(string)

	// Validate parent ID format (32 characters alphanumeric)
	if !isValidNotionID(parentID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "parent_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Validate title length
	if len(title) < 1 || len(title) > 500 {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "title must be between 1 and 500 characters",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build request
	req := CreatePageRequest{
		Parent: Parent{
			Type:   "page_id",
			PageID: parentID,
		},
		Properties: map[string]interface{}{
			"title": map[string]interface{}{
				"title": []map[string]interface{}{
					{
						"text": map[string]interface{}{
							"content": title,
						},
					},
				},
			},
		},
	}

	// Include optional properties if provided
	if props, ok := inputs["properties"].(map[string]interface{}); ok {
		for k, v := range props {
			req.Properties[k] = v
		}
	}

	// Build URL
	url, err := c.BuildURL("/pages", inputs)
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
	var page Page
	if err := c.ParseJSONResponse(resp, &page); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"id":         page.ID,
		"url":        page.URL,
		"created_at": page.CreatedTime,
	}), nil
}

// getPage retrieves a page's properties.
func (c *NotionIntegration) getPage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"page_id"}); err != nil {
		return nil, err
	}

	pageID, _ := inputs["page_id"].(string)

	// Validate page ID format
	if !isValidNotionID(pageID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "page_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build URL
	url, err := c.BuildURL(fmt.Sprintf("/pages/%s", pageID), inputs)
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
	var page Page
	if err := c.ParseJSONResponse(resp, &page); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"id":         page.ID,
		"url":        page.URL,
		"properties": page.Properties,
		"parent":     page.Parent,
	}), nil
}

// updatePage updates page properties (title, icon, cover).
func (c *NotionIntegration) updatePage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"page_id"}); err != nil {
		return nil, err
	}

	pageID, _ := inputs["page_id"].(string)

	// Validate page ID format
	if !isValidNotionID(pageID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "page_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build request with optional properties
	req := UpdatePageRequest{}

	if props, ok := inputs["properties"].(map[string]interface{}); ok {
		req.Properties = props
	}

	if icon, ok := inputs["icon"].(map[string]interface{}); ok {
		iconType, _ := icon["type"].(string)
		emoji, _ := icon["emoji"].(string)
		req.Icon = &Icon{
			Type:  iconType,
			Emoji: emoji,
		}
	}

	if cover, ok := inputs["cover"].(map[string]interface{}); ok {
		coverType, _ := cover["type"].(string)
		if external, ok := cover["external"].(map[string]interface{}); ok {
			url, _ := external["url"].(string)
			req.Cover = &Cover{
				Type: coverType,
				External: &External{
					URL: url,
				},
			}
		}
	}

	// Build URL
	url, err := c.BuildURL(fmt.Sprintf("/pages/%s", pageID), inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "PATCH", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var page Page
	if err := c.ParseJSONResponse(resp, &page); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"id":  page.ID,
		"url": page.URL,
	}), nil
}

// upsertPage updates if exists by title match, creates if not.
// Uses block children API instead of search to avoid indexing lag.
// Optionally accepts "blocks" parameter to add content to the page.
func (c *NotionIntegration) upsertPage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"parent_id", "title"}); err != nil {
		return nil, err
	}

	parentID, _ := inputs["parent_id"].(string)
	title, _ := inputs["title"].(string)

	// Validate parent ID format
	if !isValidNotionID(parentID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "parent_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Get child blocks of parent to find existing pages with matching title
	// Use block children API instead of search to avoid indexing lag
	childrenURL, err := c.BuildURL(fmt.Sprintf("/blocks/%s/children", parentID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "GET", childrenURL, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse children results
	var childrenResults struct {
		Results []struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			ChildPage *struct {
				Title string `json:"title"`
			} `json:"child_page,omitempty"`
		} `json:"results"`
	}
	if err := c.ParseJSONResponse(resp, &childrenResults); err != nil {
		return nil, err
	}

	// Find child pages with matching title
	var matchingPageIDs []string
	for _, child := range childrenResults.Results {
		if child.Type == "child_page" && child.ChildPage != nil {
			if child.ChildPage.Title == title {
				matchingPageIDs = append(matchingPageIDs, normalizeNotionID(child.ID))
			}
		}
	}

	var pageID string
	var pageURL string
	var isNew bool

	switch len(matchingPageIDs) {
	case 0:
		// No match found - create new page
		result, err := c.createPage(ctx, inputs)
		if err != nil {
			return nil, err
		}
		if respMap, ok := result.Response.(map[string]interface{}); ok {
			pageID, _ = respMap["id"].(string)
			pageURL, _ = respMap["url"].(string)
		}
		if pageID == "" {
			return nil, fmt.Errorf("failed to get page ID from create response")
		}
		isNew = true

	case 1:
		// Exactly one match - get page info
		pageID = matchingPageIDs[0]
		getInputs := map[string]interface{}{
			"page_id": pageID,
		}
		result, err := c.getPage(ctx, getInputs)
		if err != nil {
			return nil, err
		}
		if respMap, ok := result.Response.(map[string]interface{}); ok {
			pageURL, _ = respMap["url"].(string)
		}
		isNew = false

	default:
		// Multiple matches - return error directing user to use explicit page_id
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   fmt.Sprintf("multiple pages found with title %q under parent. Use explicit page_id instead of upsert_page", title),
			Category:  ErrorCategoryValidation,
		}
	}

	// Handle blocks if provided
	if blocks, ok := inputs["blocks"]; ok && blocks != nil {
		var blocksList []interface{}
		switch b := blocks.(type) {
		case []interface{}:
			blocksList = b
		case []map[string]interface{}:
			for _, block := range b {
				blocksList = append(blocksList, block)
			}
		}

		if len(blocksList) > 0 {
			// Validate and transform blocks
			notionBlocks := make([]map[string]interface{}, 0, len(blocksList))
			for i, block := range blocksList {
				blockMap, ok := block.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("block at index %d is not a valid object", i)
				}

				blockType, ok := blockMap["type"].(string)
				if !ok {
					return nil, fmt.Errorf("block at index %d missing 'type' field", i)
				}

				notionBlock, err := buildNotionBlock(blockType, blockMap)
				if err != nil {
					return nil, err
				}
				notionBlocks = append(notionBlocks, notionBlock)
			}

			// For existing pages, first delete existing content blocks
			if !isNew {
				getURL, err := c.BuildURL(fmt.Sprintf("/blocks/%s/children", pageID), nil)
				if err != nil {
					return nil, err
				}

				resp, err := c.ExecuteRequest(ctx, "GET", getURL, c.defaultHeaders(), nil)
				if err != nil {
					return nil, err
				}

				if parseErr := ParseError(resp); parseErr != nil {
					return nil, parseErr
				}

				var childrenResp struct {
					Results []struct {
						ID   string `json:"id"`
						Type string `json:"type"`
					} `json:"results"`
				}
				if parseErr := c.ParseJSONResponse(resp, &childrenResp); parseErr != nil {
					return nil, parseErr
				}

				// Delete each existing block except child_page blocks
				for _, block := range childrenResp.Results {
					if block.Type == "child_page" || block.Type == "child_database" {
						continue
					}
					deleteURL, delErr := c.BuildURL(fmt.Sprintf("/blocks/%s", block.ID), nil)
					if delErr != nil {
						continue
					}
					delResp, delErr := c.ExecuteRequest(ctx, "DELETE", deleteURL, c.defaultHeaders(), nil)
					if delErr != nil {
						continue
					}
					ParseError(delResp) // Ignore errors
				}
			}

			// Append new blocks
			appendReq := AppendBlocksRequest{
				Children: notionBlocks,
			}

			appendURL, err := c.BuildURL(fmt.Sprintf("/blocks/%s/children", pageID), nil)
			if err != nil {
				return nil, fmt.Errorf("failed to build append URL: %w", err)
			}

			body, err := marshalRequest(appendReq)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal blocks request: %w", err)
			}

			appendResp, err := c.ExecuteRequest(ctx, "PATCH", appendURL, c.defaultHeaders(), body)
			if err != nil {
				return nil, fmt.Errorf("failed to append blocks: %w", err)
			}

			if parseErr := ParseError(appendResp); parseErr != nil {
				return nil, fmt.Errorf("failed to append blocks: %w", parseErr)
			}
		}
	}

	// Return a result without HTTP response metadata
	return &operation.Result{
		Response: map[string]interface{}{
			"id":     pageID,
			"url":    pageURL,
			"is_new": isNew,
		},
	}, nil
}

// extractPageTitle extracts the title from page properties.
func extractPageTitle(properties map[string]interface{}) string {
	titleProp, ok := properties["title"]
	if !ok {
		return ""
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

	textMap, ok := firstTitle["text"].(map[string]interface{})
	if !ok {
		return ""
	}

	content, _ := textMap["content"].(string)
	return content
}

// normalizeNotionID removes hyphens from a Notion ID.
func normalizeNotionID(id string) string {
	return strings.ReplaceAll(id, "-", "")
}

// isValidNotionID validates a Notion ID format (32 characters, alphanumeric).
func isValidNotionID(id string) bool {
	// Remove hyphens if present (Notion IDs can be formatted with or without hyphens)
	id = normalizeNotionID(id)

	if len(id) != 32 {
		return false
	}

	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}

	return true
}
