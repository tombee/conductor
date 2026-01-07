package notion

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// CommentRequest represents a request to create a comment.
type CommentRequest struct {
	Parent      *CommentParent `json:"parent,omitempty"`
	DiscussionID string        `json:"discussion_id,omitempty"`
	RichText    []RichText     `json:"rich_text"`
}

// CommentParent identifies the parent of a comment.
type CommentParent struct {
	PageID string `json:"page_id,omitempty"`
}

// CommentResponse represents the response from the comments API.
type CommentResponse struct {
	Object       string     `json:"object"`
	ID           string     `json:"id"`
	DiscussionID string     `json:"discussion_id"`
	CreatedTime  string     `json:"created_time"`
	CreatedBy    User       `json:"created_by"`
	RichText     []RichText `json:"rich_text"`
}

// CommentsListResponse represents a list of comments.
type CommentsListResponse struct {
	Object     string            `json:"object"`
	Results    []CommentResponse `json:"results"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

// User represents a Notion user.
type User struct {
	Object    string `json:"object"`
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// getComments retrieves comments from a page or block.
func (c *NotionIntegration) getComments(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Get block_id (which can be a page_id)
	blockID, _ := inputs["block_id"].(string)
	pageID, _ := inputs["page_id"].(string)

	// Use page_id if block_id not provided
	if blockID == "" && pageID != "" {
		blockID = pageID
	}

	if blockID == "" {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "either block_id or page_id is required",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build URL with block_id parameter
	url, err := c.BuildURL("/comments", map[string]interface{}{
		"block_id": blockID,
	})
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

	var commentsResp CommentsListResponse
	if err := c.ParseJSONResponse(resp, &commentsResp); err != nil {
		return nil, err
	}

	// Transform to simplified format
	comments := make([]map[string]interface{}, 0, len(commentsResp.Results))
	for _, comment := range commentsResp.Results {
		// Extract plain text from rich_text
		content := ""
		for _, rt := range comment.RichText {
			content += rt.PlainText
		}

		comments = append(comments, map[string]interface{}{
			"id":            comment.ID,
			"discussion_id": comment.DiscussionID,
			"author":        comment.CreatedBy.Name,
			"author_id":     comment.CreatedBy.ID,
			"content":       content,
			"created_time":  comment.CreatedTime,
		})
	}

	return c.ToResult(resp, map[string]interface{}{
		"comments":    comments,
		"has_more":    commentsResp.HasMore,
		"next_cursor": commentsResp.NextCursor,
	}), nil
}

// addComment creates a new comment on a page or replies to a discussion.
func (c *NotionIntegration) addComment(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	content, ok := inputs["content"].(string)
	if !ok || content == "" {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "content is required",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build comment request
	req := CommentRequest{
		RichText: []RichText{
			{
				Type: "text",
				Text: &TextContent{
					Content: content,
				},
			},
		},
	}

	// Either page_id (new comment) or discussion_id (reply)
	if pageID, ok := inputs["page_id"].(string); ok && pageID != "" {
		req.Parent = &CommentParent{PageID: pageID}
	} else if discussionID, ok := inputs["discussion_id"].(string); ok && discussionID != "" {
		req.DiscussionID = discussionID
	} else {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "either page_id or discussion_id is required",
			Category:  ErrorCategoryValidation,
		}
	}

	url, err := c.BuildURL("/comments", nil)
	if err != nil {
		return nil, err
	}

	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var commentResp CommentResponse
	if err := c.ParseJSONResponse(resp, &commentResp); err != nil {
		return nil, err
	}

	return c.ToResult(resp, map[string]interface{}{
		"id":            commentResp.ID,
		"discussion_id": commentResp.DiscussionID,
		"created_time":  commentResp.CreatedTime,
	}), nil
}

// resolveComment marks a comment as resolved.
// Note: Notion API doesn't currently support resolving comments via API.
// This operation is a placeholder for future API support.
func (c *NotionIntegration) resolveComment(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Notion API doesn't support resolving comments yet
	// Return an error indicating this limitation
	return nil, &NotionError{
		ErrorCode: "not_implemented",
		Message:   "Notion API does not currently support resolving comments. This operation will be enabled when Notion adds API support.",
		Category:  ErrorCategoryValidation,
	}
}

// getCommentSchema returns the schema for get_comments operation.
func getCommentSchemas() map[string]interface{} {
	return map[string]interface{}{
		"get_comments": map[string]interface{}{
			"description": "Get comments from a page or block",
			"parameters": []map[string]interface{}{
				{"name": "page_id", "type": "string", "description": "Page ID to get comments from"},
				{"name": "block_id", "type": "string", "description": "Block ID to get comments from (alternative to page_id)"},
			},
			"response_fields": []map[string]interface{}{
				{"name": "comments", "type": "array", "description": "Array of comments"},
				{"name": "has_more", "type": "boolean", "description": "Whether more comments exist"},
				{"name": "next_cursor", "type": "string", "description": "Cursor for pagination"},
			},
		},
		"add_comment": map[string]interface{}{
			"description": "Add a comment to a page or reply to a discussion",
			"parameters": []map[string]interface{}{
				{"name": "page_id", "type": "string", "description": "Page ID to comment on (for new comments)"},
				{"name": "discussion_id", "type": "string", "description": "Discussion ID to reply to"},
				{"name": "content", "type": "string", "description": "Comment text content", "required": true},
			},
			"response_fields": []map[string]interface{}{
				{"name": "id", "type": "string", "description": "Created comment ID"},
				{"name": "discussion_id", "type": "string", "description": "Discussion thread ID"},
				{"name": "created_time", "type": "string", "description": "Creation timestamp"},
			},
		},
	}
}
