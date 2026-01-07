package notion

import (
	"bytes"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// markdownToBlocks converts markdown text to Notion block structures.
// Supports: headings, paragraphs, lists, checkboxes, code blocks, quotes, dividers, images, callouts.
// Rich text: **bold**, *italic*, ~~strikethrough~~, `code`, [links](url)
func markdownToBlocks(md string) ([]map[string]interface{}, error) {
	// Handle empty/whitespace-only markdown
	md = strings.TrimSpace(md)
	if md == "" {
		return []map[string]interface{}{}, nil
	}

	// Create goldmark parser with GFM extensions
	gm := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (tables, strikethrough, task lists)
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	// Parse markdown to AST
	reader := text.NewReader([]byte(md))
	doc := gm.Parser().Parse(reader)

	// Convert AST to Notion blocks
	var blocks []map[string]interface{}
	source := []byte(md)

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		block := nodeToBlock(n, source)
		if block != nil {
			blocks = append(blocks, block)
			// Skip children for block-level nodes we've handled
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk markdown AST: %w", err)
	}

	return blocks, nil
}

// nodeToBlock converts a goldmark AST node to a Notion block.
// Returns nil if the node should not produce a block (e.g., inline elements handled by parent).
func nodeToBlock(n ast.Node, source []byte) map[string]interface{} {
	switch node := n.(type) {
	case *ast.Heading:
		return headingToBlock(node, source)
	case *ast.Paragraph:
		return paragraphToBlock(node, source)
	case *ast.List:
		// Lists are handled by collecting list items
		return nil
	case *ast.ListItem:
		return listItemToBlock(node, source)
	case *ast.FencedCodeBlock:
		return codeBlockToBlock(node, source)
	case *ast.CodeBlock:
		return codeBlockToBlock(node, source)
	case *ast.Blockquote:
		return blockquoteToBlock(node, source)
	case *ast.ThematicBreak:
		return map[string]interface{}{
			"object":  "block",
			"type":    "divider",
			"divider": map[string]interface{}{},
		}
	case *ast.Image:
		return imageToBlock(node, source)
	}
	return nil
}

// headingToBlock converts a heading node to a Notion heading block.
func headingToBlock(node *ast.Heading, source []byte) map[string]interface{} {
	level := node.Level
	if level > 3 {
		level = 3 // Notion only supports h1-h3
	}

	blockType := fmt.Sprintf("heading_%d", level)
	richText := extractRichText(node, source)

	return map[string]interface{}{
		"object": "block",
		"type":   blockType,
		blockType: map[string]interface{}{
			"rich_text": richText,
		},
	}
}

// paragraphToBlock converts a paragraph node to a Notion paragraph block.
// Also handles callout syntax: > [!NOTE] or > [!WARNING]
func paragraphToBlock(node *ast.Paragraph, source []byte) map[string]interface{} {
	richText := extractRichText(node, source)

	// Check if this is a callout (first text starts with [!NOTE] or [!WARNING])
	if len(richText) > 0 {
		if textContent, ok := richText[0]["text"].(map[string]interface{}); ok {
			if content, ok := textContent["content"].(string); ok {
				if callout := parseCallout(content, richText); callout != nil {
					return callout
				}
			}
		}
	}

	return map[string]interface{}{
		"object": "block",
		"type":   "paragraph",
		"paragraph": map[string]interface{}{
			"rich_text": richText,
		},
	}
}

// parseCallout checks if content starts with [!NOTE] or [!WARNING] and returns a callout block.
func parseCallout(content string, richText []map[string]interface{}) map[string]interface{} {
	calloutPattern := regexp.MustCompile(`^\[!(NOTE|WARNING|TIP|IMPORTANT|CAUTION)\]\s*`)
	matches := calloutPattern.FindStringSubmatch(content)
	if matches == nil {
		return nil
	}

	calloutType := matches[1]
	icon := "💡" // default
	switch calloutType {
	case "NOTE":
		icon = "ℹ️"
	case "WARNING", "CAUTION":
		icon = "⚠️"
	case "TIP":
		icon = "💡"
	case "IMPORTANT":
		icon = "❗"
	}

	// Remove the callout marker from the content
	newContent := calloutPattern.ReplaceAllString(content, "")
	if len(richText) > 0 {
		if textContent, ok := richText[0]["text"].(map[string]interface{}); ok {
			textContent["content"] = newContent
		}
	}

	return map[string]interface{}{
		"object": "block",
		"type":   "callout",
		"callout": map[string]interface{}{
			"rich_text": richText,
			"icon": map[string]interface{}{
				"type":  "emoji",
				"emoji": icon,
			},
		},
	}
}

// listItemToBlock converts a list item to the appropriate Notion block type.
func listItemToBlock(node *ast.ListItem, source []byte) map[string]interface{} {
	parent := node.Parent()
	list, ok := parent.(*ast.List)
	if !ok {
		return nil
	}

	// Check if it's a task list item (checkbox)
	// Walk through children to find TaskCheckBox
	var taskCheckbox *extast.TaskCheckBox
	var found bool
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if cb, ok := n.(*extast.TaskCheckBox); ok {
			taskCheckbox = cb
			found = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})

	if found && taskCheckbox != nil {
		// Extract text content after the checkbox
		richText := extractRichTextFromListItem(node, source)
		return map[string]interface{}{
			"object": "block",
			"type":   "to_do",
			"to_do": map[string]interface{}{
				"rich_text": richText,
				"checked":   taskCheckbox.IsChecked,
			},
		}
	}

	// Regular list item
	richText := extractRichTextFromListItem(node, source)
	if list.IsOrdered() {
		return map[string]interface{}{
			"object": "block",
			"type":   "numbered_list_item",
			"numbered_list_item": map[string]interface{}{
				"rich_text": richText,
			},
		}
	}

	return map[string]interface{}{
		"object": "block",
		"type":   "bulleted_list_item",
		"bulleted_list_item": map[string]interface{}{
			"rich_text": richText,
		},
	}
}

// codeBlockToBlock converts a code block to a Notion code block.
func codeBlockToBlock(node ast.Node, source []byte) map[string]interface{} {
	var language string
	var content strings.Builder

	switch n := node.(type) {
	case *ast.FencedCodeBlock:
		language = string(n.Language(source))
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			content.Write(line.Value(source))
		}
	case *ast.CodeBlock:
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			content.Write(line.Value(source))
		}
	}

	if language == "" {
		language = "plain text"
	}

	// Map common language aliases to Notion's expected names
	language = normalizeLanguage(language)

	text := strings.TrimSuffix(content.String(), "\n")

	return map[string]interface{}{
		"object": "block",
		"type":   "code",
		"code": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]interface{}{
						"content": text,
					},
				},
			},
			"language": language,
		},
	}
}

// normalizeLanguage maps common language aliases to Notion's expected names.
func normalizeLanguage(lang string) string {
	lang = strings.ToLower(lang)
	langMap := map[string]string{
		"js":         "javascript",
		"ts":         "typescript",
		"py":         "python",
		"rb":         "ruby",
		"sh":         "shell",
		"bash":       "shell",
		"zsh":        "shell",
		"yml":        "yaml",
		"dockerfile": "docker",
		"md":         "markdown",
	}
	if mapped, ok := langMap[lang]; ok {
		return mapped
	}
	return lang
}

// blockquoteToBlock converts a blockquote to a Notion quote block.
func blockquoteToBlock(node *ast.Blockquote, source []byte) map[string]interface{} {
	// Collect all text from the blockquote
	var texts []string
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if para, ok := child.(*ast.Paragraph); ok {
			text := extractPlainText(para, source)
			// Check for callout syntax at start of blockquote
			if len(texts) == 0 {
				calloutPattern := regexp.MustCompile(`^\[!(NOTE|WARNING|TIP|IMPORTANT|CAUTION)\]\s*`)
				if matches := calloutPattern.FindStringSubmatch(text); matches != nil {
					// This is a callout, not a quote
					calloutType := matches[1]
					icon := "💡"
					switch calloutType {
					case "NOTE":
						icon = "ℹ️"
					case "WARNING", "CAUTION":
						icon = "⚠️"
					case "TIP":
						icon = "💡"
					case "IMPORTANT":
						icon = "❗"
					}
					content := calloutPattern.ReplaceAllString(text, "")
					return map[string]interface{}{
						"object": "block",
						"type":   "callout",
						"callout": map[string]interface{}{
							"rich_text": []map[string]interface{}{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": content,
									},
								},
							},
							"icon": map[string]interface{}{
								"type":  "emoji",
								"emoji": icon,
							},
						},
					}
				}
			}
			texts = append(texts, text)
		}
	}

	content := strings.Join(texts, "\n")
	return map[string]interface{}{
		"object": "block",
		"type":   "quote",
		"quote": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]interface{}{
						"content": content,
					},
				},
			},
		},
	}
}

// imageToBlock converts an image to a Notion image block.
func imageToBlock(node *ast.Image, source []byte) map[string]interface{} {
	url := string(node.Destination)
	// Notion only supports external URLs for images via API
	return map[string]interface{}{
		"object": "block",
		"type":   "image",
		"image": map[string]interface{}{
			"type": "external",
			"external": map[string]interface{}{
				"url": url,
			},
		},
	}
}

// extractRichText extracts rich text content from an AST node with formatting.
func extractRichText(node ast.Node, source []byte) []map[string]interface{} {
	var richText []map[string]interface{}

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		items := nodeToRichText(child, source, &Annotations{})
		richText = append(richText, items...)
	}

	if len(richText) == 0 {
		// Return empty text to avoid nil
		richText = append(richText, map[string]interface{}{
			"type": "text",
			"text": map[string]interface{}{
				"content": "",
			},
		})
	}

	return richText
}

// extractRichTextSkipFirst extracts rich text but skips the first child (used for task list items).
func extractRichTextSkipFirst(node ast.Node, source []byte) []map[string]interface{} {
	var richText []map[string]interface{}
	first := true

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if first {
			first = false
			continue
		}
		items := nodeToRichText(child, source, &Annotations{})
		richText = append(richText, items...)
	}

	if len(richText) == 0 {
		richText = append(richText, map[string]interface{}{
			"type": "text",
			"text": map[string]interface{}{
				"content": "",
			},
		})
	}

	return richText
}

// extractRichTextFromListItem extracts text from a list item's children.
func extractRichTextFromListItem(node *ast.ListItem, source []byte) []map[string]interface{} {
	var richText []map[string]interface{}

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if para, ok := child.(*ast.Paragraph); ok {
			items := extractRichText(para, source)
			richText = append(richText, items...)
		}
	}

	if len(richText) == 0 {
		richText = append(richText, map[string]interface{}{
			"type": "text",
			"text": map[string]interface{}{
				"content": "",
			},
		})
	}

	return richText
}

// extractPlainText extracts plain text content from a node.
func extractPlainText(node ast.Node, source []byte) string {
	var buf bytes.Buffer
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		extractPlainTextRecursive(child, source, &buf)
	}
	return buf.String()
}

func extractPlainTextRecursive(node ast.Node, source []byte, buf *bytes.Buffer) {
	switch n := node.(type) {
	case *ast.Text:
		buf.Write(n.Segment.Value(source))
	case *ast.String:
		buf.Write(n.Value)
	default:
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			extractPlainTextRecursive(child, source, buf)
		}
	}
}

// nodeToRichText converts an inline AST node to Notion rich text format.
func nodeToRichText(node ast.Node, source []byte, annotations *Annotations) []map[string]interface{} {
	var items []map[string]interface{}

	switch n := node.(type) {
	case *ast.Text:
		text := string(n.Segment.Value(source))
		if text != "" {
			items = append(items, createRichTextItem(text, annotations, nil))
		}
	case *ast.String:
		text := string(n.Value)
		if text != "" {
			items = append(items, createRichTextItem(text, annotations, nil))
		}
	case *ast.Emphasis:
		// Single * or _ is italic, double ** or __ is bold
		newAnnotations := *annotations
		if n.Level == 1 {
			newAnnotations.Italic = true
		} else {
			newAnnotations.Bold = true
		}
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			items = append(items, nodeToRichText(child, source, &newAnnotations)...)
		}
	case *ast.CodeSpan:
		newAnnotations := *annotations
		newAnnotations.Code = true
		// For code spans, extract text from children
		var buf bytes.Buffer
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if textNode, ok := child.(*ast.Text); ok {
				buf.Write(textNode.Segment.Value(source))
			}
		}
		text := buf.String()
		if text != "" {
			items = append(items, createRichTextItem(text, &newAnnotations, nil))
		}
	case *ast.Link:
		url := string(n.Destination)
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if textNode, ok := child.(*ast.Text); ok {
				text := string(textNode.Segment.Value(source))
				items = append(items, createRichTextItem(text, annotations, &url))
			}
		}
	case *extast.Strikethrough:
		newAnnotations := *annotations
		newAnnotations.Strikethrough = true
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			items = append(items, nodeToRichText(child, source, &newAnnotations)...)
		}
	case *extast.TaskCheckBox:
		// Skip checkboxes, they're handled at the list item level
	default:
		// Recurse into children for unknown node types
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			items = append(items, nodeToRichText(child, source, annotations)...)
		}
	}

	return items
}

// createRichTextItem creates a Notion rich text item.
func createRichTextItem(text string, annotations *Annotations, link *string) map[string]interface{} {
	item := map[string]interface{}{
		"type": "text",
		"text": map[string]interface{}{
			"content": text,
		},
	}

	// Add link if present
	if link != nil {
		item["text"].(map[string]interface{})["link"] = map[string]interface{}{
			"url": *link,
		}
	}

	// Add annotations if any are set
	if annotations.Bold || annotations.Italic || annotations.Strikethrough || annotations.Code {
		item["annotations"] = map[string]interface{}{
			"bold":          annotations.Bold,
			"italic":        annotations.Italic,
			"strikethrough": annotations.Strikethrough,
			"underline":     false,
			"code":          annotations.Code,
			"color":         "default",
		}
	}

	return item
}

// blocksToMarkdown converts Notion blocks back to markdown text.
func blocksToMarkdown(blocks []map[string]interface{}) string {
	var lines []string

	for _, block := range blocks {
		line := blockToMarkdown(block)
		if line != "" {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n\n")
}

// blockToMarkdown converts a single Notion block to markdown.
func blockToMarkdown(block map[string]interface{}) string {
	blockType, _ := block["type"].(string)
	content, _ := block[blockType].(map[string]interface{})

	switch blockType {
	case "heading_1":
		return "# " + richTextToMarkdown(content)
	case "heading_2":
		return "## " + richTextToMarkdown(content)
	case "heading_3":
		return "### " + richTextToMarkdown(content)
	case "paragraph":
		return richTextToMarkdown(content)
	case "bulleted_list_item":
		return "- " + richTextToMarkdown(content)
	case "numbered_list_item":
		return "1. " + richTextToMarkdown(content)
	case "to_do":
		checked, _ := content["checked"].(bool)
		checkbox := "[ ]"
		if checked {
			checkbox = "[x]"
		}
		return "- " + checkbox + " " + richTextToMarkdown(content)
	case "quote":
		text := richTextToMarkdown(content)
		// Prefix each line with >
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			lines[i] = "> " + line
		}
		return strings.Join(lines, "\n")
	case "code":
		lang, _ := content["language"].(string)
		text := richTextToMarkdown(content)
		return fmt.Sprintf("```%s\n%s\n```", lang, text)
	case "divider":
		return "---"
	case "callout":
		icon, _ := content["icon"].(map[string]interface{})
		emoji, _ := icon["emoji"].(string)
		text := richTextToMarkdown(content)
		// Try to determine callout type from icon
		calloutType := "NOTE"
		switch emoji {
		case "⚠️":
			calloutType = "WARNING"
		case "💡":
			calloutType = "TIP"
		case "❗":
			calloutType = "IMPORTANT"
		}
		return fmt.Sprintf("> [!%s] %s", calloutType, text)
	case "image":
		if ext, ok := content["external"].(map[string]interface{}); ok {
			url, _ := ext["url"].(string)
			return fmt.Sprintf("![image](%s)", url)
		}
		return ""
	}

	// For unsupported block types, try to extract text
	slog.Warn("unsupported block type for markdown conversion", "type", blockType)
	return richTextToMarkdown(content)
}

// richTextToMarkdown converts Notion rich text array to markdown string.
func richTextToMarkdown(content map[string]interface{}) string {
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

		text := ""
		if textContent, ok := rtMap["text"].(map[string]interface{}); ok {
			text, _ = textContent["content"].(string)
		} else if plainText, ok := rtMap["plain_text"].(string); ok {
			text = plainText
		}

		// Apply formatting based on annotations
		if annotations, ok := rtMap["annotations"].(map[string]interface{}); ok {
			if bold, _ := annotations["bold"].(bool); bold {
				text = "**" + text + "**"
			}
			if italic, _ := annotations["italic"].(bool); italic {
				text = "*" + text + "*"
			}
			if strikethrough, _ := annotations["strikethrough"].(bool); strikethrough {
				text = "~~" + text + "~~"
			}
			if code, _ := annotations["code"].(bool); code {
				text = "`" + text + "`"
			}
		}

		// Handle links
		if textContent, ok := rtMap["text"].(map[string]interface{}); ok {
			if link, ok := textContent["link"].(map[string]interface{}); ok {
				if url, ok := link["url"].(string); ok {
					text = fmt.Sprintf("[%s](%s)", text, url)
				}
			}
		}

		parts = append(parts, text)
	}

	return strings.Join(parts, "")
}
