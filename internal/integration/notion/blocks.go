package notion

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/tombee/conductor/internal/operation"
)

// getBlocks retrieves content blocks from a page and returns the content.
// Supports format parameter: "blocks" (default), "markdown", or "text"
func (c *NotionIntegration) getBlocks(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
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

	// Get format parameter (default: "blocks")
	format := "blocks"
	if f, ok := inputs["format"].(string); ok {
		format = f
	}

	// Get child blocks
	url, err := c.BuildURL(fmt.Sprintf("/blocks/%s/children", pageID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var blocksResp struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := c.ParseJSONResponse(resp, &blocksResp); err != nil {
		return nil, err
	}

	// Build response based on format
	result := map[string]interface{}{
		"block_count": len(blocksResp.Results),
		"raw_blocks":  blocksResp.Results,
	}

	switch format {
	case "markdown":
		// Convert blocks to markdown
		result["content"] = blocksToMarkdown(blocksResp.Results)
	case "text":
		// Extract plain text content
		var textParts []string
		for _, block := range blocksResp.Results {
			text := extractBlockText(block)
			if text != "" {
				textParts = append(textParts, text)
			}
		}
		result["content"] = strings.Join(textParts, "\n")
	default: // "blocks"
		// Extract text content (legacy behavior)
		var textParts []string
		for _, block := range blocksResp.Results {
			text := extractBlockText(block)
			if text != "" {
				textParts = append(textParts, text)
			}
		}
		result["content"] = strings.Join(textParts, "\n")
	}

	return c.ToResult(resp, result), nil
}

// extractBlockText extracts text content from a Notion block.
func extractBlockText(block map[string]interface{}) string {
	blockType, _ := block["type"].(string)
	if blockType == "" {
		return ""
	}

	// Get the block content based on type
	content, ok := block[blockType].(map[string]interface{})
	if !ok {
		return ""
	}

	// Most text blocks have a "rich_text" array
	richText, ok := content["rich_text"].([]interface{})
	if !ok {
		return ""
	}

	var parts []string
	for _, rt := range richText {
		rtMap, ok := rt.(map[string]interface{})
		if !ok {
			continue
		}
		if text, ok := rtMap["plain_text"].(string); ok {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, "")
}

// Maximum blocks per append request
const maxBlocksPerAppend = 100

// Block content character limits
const (
	maxParagraphChars = 2000
	maxCodeChars      = 2000
	maxHeadingChars   = 200
)

// Supported block types
var supportedBlockTypes = map[string]bool{
	"paragraph":           true,
	"heading_1":           true,
	"heading_2":           true,
	"heading_3":           true,
	"bulleted_list_item":  true,
	"numbered_list_item":  true,
	"to_do":               true,
	"code":                true,
	"quote":               true,
	"divider":             true,
}

// appendBlocks appends content blocks to an existing page.
// Supports either `blocks` array or `markdown` string parameter.
func (c *NotionIntegration) appendBlocks(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate page_id
	if err := c.ValidateRequired(inputs, []string{"page_id"}); err != nil {
		return nil, err
	}

	pageID, _ := inputs["page_id"].(string)

	// Check for markdown parameter first
	var blocks []interface{}
	if md, ok := inputs["markdown"].(string); ok && md != "" {
		// Convert markdown to blocks
		mdBlocks, err := markdownToBlocks(md)
		if err != nil {
			return nil, &NotionError{
				ErrorCode: "validation_error",
				Message:   fmt.Sprintf("failed to parse markdown: %v", err),
				Category:  ErrorCategoryValidation,
			}
		}
		// Convert []map[string]interface{} to []interface{}
		for _, b := range mdBlocks {
			blocks = append(blocks, b)
		}
	} else {
		// Use blocks parameter
		var ok bool
		blocks, ok = inputs["blocks"].([]interface{})
		if !ok {
			return nil, &NotionError{
				ErrorCode: "validation_error",
				Message:   "either 'blocks' array or 'markdown' string is required",
				Category:  ErrorCategoryValidation,
			}
		}
	}

	// Validate page ID format
	if !isValidNotionID(pageID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "page_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Validate block count (allow empty for markdown which may produce no blocks)
	if len(blocks) == 0 {
		// Return success with 0 blocks added for empty markdown
		return &operation.Result{
			Response: map[string]interface{}{
				"blocks_added": 0,
			},
		}, nil
	}

	if len(blocks) > maxBlocksPerAppend {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   fmt.Sprintf("cannot append more than %d blocks at once. Split into multiple calls", maxBlocksPerAppend),
			Category:  ErrorCategoryValidation,
		}
	}

	// Validate and transform blocks
	notionBlocks := make([]map[string]interface{}, 0, len(blocks))
	for i, block := range blocks {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			return nil, &NotionError{
				ErrorCode: "validation_error",
				Message:   fmt.Sprintf("block at index %d is not a valid object", i),
				Category:  ErrorCategoryValidation,
			}
		}

		blockType, ok := blockMap["type"].(string)
		if !ok {
			return nil, &NotionError{
				ErrorCode: "validation_error",
				Message:   fmt.Sprintf("block at index %d missing 'type' field", i),
				Category:  ErrorCategoryValidation,
			}
		}

		// Validate block type
		if !supportedBlockTypes[blockType] {
			return nil, &NotionError{
				ErrorCode: "validation_error",
				Message:   fmt.Sprintf("unsupported block type '%s'. Supported types: paragraph, heading_1, heading_2, heading_3, bulleted_list_item, numbered_list_item, to_do, code, quote, divider", blockType),
				Category:  ErrorCategoryValidation,
			}
		}

		// Build Notion block format
		notionBlock, err := buildNotionBlock(blockType, blockMap)
		if err != nil {
			return nil, err
		}

		notionBlocks = append(notionBlocks, notionBlock)
	}

	// Build request
	req := AppendBlocksRequest{
		Children: notionBlocks,
	}

	// Build URL
	url, err := c.BuildURL(fmt.Sprintf("/blocks/%s/children", pageID), inputs)
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
	var appendResp AppendBlocksResponse
	if err := c.ParseJSONResponse(resp, &appendResp); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"blocks_added": len(appendResp.Results),
	}), nil
}

// buildNotionBlock constructs a Notion API block from workflow inputs.
func buildNotionBlock(blockType string, blockMap map[string]interface{}) (map[string]interface{}, error) {
	notionBlock := map[string]interface{}{
		"object": "block",
		"type":   blockType,
	}

	// Handle divider (no content)
	if blockType == "divider" {
		notionBlock[blockType] = map[string]interface{}{}
		return notionBlock, nil
	}

	// Get text content
	text, ok := blockMap["text"].(string)
	if !ok && blockType != "divider" {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   fmt.Sprintf("block of type '%s' requires 'text' field", blockType),
			Category:  ErrorCategoryValidation,
		}
	}

	// Validate content length
	if err := validateBlockContentLength(blockType, text); err != nil {
		return nil, err
	}

	// Validate checked field is only used with to_do blocks
	if _, hasChecked := blockMap["checked"]; hasChecked && blockType != "to_do" {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   fmt.Sprintf("'checked' field is only valid for 'to_do' blocks, not '%s'", blockType),
			Category:  ErrorCategoryValidation,
		}
	}

	// Build rich text array
	richText := []map[string]interface{}{
		{
			"type": "text",
			"text": map[string]interface{}{
				"content": text,
			},
		},
	}

	// Build block content based on type
	switch blockType {
	case "paragraph":
		notionBlock["paragraph"] = map[string]interface{}{
			"rich_text": richText,
		}
	case "heading_1":
		notionBlock["heading_1"] = map[string]interface{}{
			"rich_text": richText,
		}
	case "heading_2":
		notionBlock["heading_2"] = map[string]interface{}{
			"rich_text": richText,
		}
	case "heading_3":
		notionBlock["heading_3"] = map[string]interface{}{
			"rich_text": richText,
		}
	case "bulleted_list_item":
		notionBlock["bulleted_list_item"] = map[string]interface{}{
			"rich_text": richText,
		}
	case "numbered_list_item":
		notionBlock["numbered_list_item"] = map[string]interface{}{
			"rich_text": richText,
		}
	case "to_do":
		checked := false
		if c, ok := blockMap["checked"].(bool); ok {
			checked = c
		}
		notionBlock["to_do"] = map[string]interface{}{
			"rich_text": richText,
			"checked":   checked,
		}
	case "code":
		language := "plain text"
		if lang, ok := blockMap["language"].(string); ok {
			language = lang
		}
		notionBlock["code"] = map[string]interface{}{
			"rich_text": richText,
			"language":  language,
		}
	case "quote":
		notionBlock["quote"] = map[string]interface{}{
			"rich_text": richText,
		}
	}

	return notionBlock, nil
}

// replaceContent replaces all content on a page with new blocks.
// This deletes existing blocks and appends new ones.
// Supports either `blocks` array or `markdown` string parameter.
func (c *NotionIntegration) replaceContent(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate page_id
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

	// Check for markdown parameter and convert to blocks
	if md, ok := inputs["markdown"].(string); ok && md != "" {
		mdBlocks, err := markdownToBlocks(md)
		if err != nil {
			return nil, &NotionError{
				ErrorCode: "validation_error",
				Message:   fmt.Sprintf("failed to parse markdown: %v", err),
				Category:  ErrorCategoryValidation,
			}
		}
		// Convert to []interface{} for the blocks parameter
		blocks := make([]interface{}, len(mdBlocks))
		for i, b := range mdBlocks {
			blocks[i] = b
		}
		inputs["blocks"] = blocks
	}

	// Now validate blocks is present
	if _, ok := inputs["blocks"]; !ok {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "either 'blocks' array or 'markdown' string is required",
			Category:  ErrorCategoryValidation,
		}
	}

	// Step 1: Get existing child blocks
	getURL, err := c.BuildURL(fmt.Sprintf("/blocks/%s/children", pageID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "GET", getURL, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var childrenResp struct {
		Results []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"results"`
	}
	if err := c.ParseJSONResponse(resp, &childrenResp); err != nil {
		return nil, err
	}

	// Step 2: Delete each existing block EXCEPT child_page blocks (preserve nested pages)
	for _, block := range childrenResp.Results {
		// Skip child_page blocks - these are links to nested pages that should be preserved
		if block.Type == "child_page" || block.Type == "child_database" {
			continue
		}

		deleteURL, err := c.BuildURL(fmt.Sprintf("/blocks/%s", block.ID), nil)
		if err != nil {
			slog.Warn("failed to build delete URL for block", "block_id", block.ID, "error", err)
			continue
		}

		resp, err := c.ExecuteRequest(ctx, "DELETE", deleteURL, c.defaultHeaders(), nil)
		if err != nil {
			slog.Warn("failed to delete block during replace", "block_id", block.ID, "error", err)
			continue
		}

		if err := ParseError(resp); err != nil {
			// Log warning for blocks that may have been deleted already or have other issues
			slog.Warn("block deletion returned error", "block_id", block.ID, "error", err)
			continue
		}
	}

	// Step 3: Append new blocks using the existing append logic
	return c.appendBlocks(ctx, inputs)
}

// validateBlockContentLength validates block content against character limits.
func validateBlockContentLength(blockType, text string) error {
	var limit int
	switch blockType {
	case "paragraph", "code", "quote", "bulleted_list_item", "numbered_list_item", "to_do":
		limit = maxParagraphChars
	case "heading_1", "heading_2", "heading_3":
		limit = maxHeadingChars
	default:
		return nil
	}

	if len(text) > limit {
		return &NotionError{
			ErrorCode: "validation_error",
			Message:   fmt.Sprintf("block type '%s' text exceeds %d character limit (got %d characters)", blockType, limit, len(text)),
			Category:  ErrorCategoryValidation,
		}
	}

	return nil
}
