package notion

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
)

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
func (c *NotionIntegration) appendBlocks(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"page_id", "blocks"}); err != nil {
		return nil, err
	}

	pageID, _ := inputs["page_id"].(string)
	blocks, ok := inputs["blocks"].([]interface{})
	if !ok {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "blocks must be an array",
			Category:  ErrorCategoryValidation,
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

	// Validate block count
	if len(blocks) == 0 {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "blocks array cannot be empty",
			Category:  ErrorCategoryValidation,
		}
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
