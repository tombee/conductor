package truncate

import (
	"strings"
)

// JavaScriptLanguage implements Language interface for JavaScript files.
// JavaScript is essentially TypeScript without type annotations and interfaces.
// This implementation wraps TypeScriptLanguage and filters out TypeScript-specific constructs.
type JavaScriptLanguage struct {
	ts TypeScriptLanguage
}

func init() {
	RegisterLanguage("javascript", JavaScriptLanguage{})
}

// CommentSyntax returns JavaScript's comment syntax.
// JavaScript uses the same comment syntax as TypeScript.
func (js JavaScriptLanguage) CommentSyntax() (single string, multiOpen string, multiClose string) {
	return js.ts.CommentSyntax()
}

// DetectImportEnd returns the line index where the import section ends.
// JavaScript uses the same import/export syntax as TypeScript (ES6 modules).
func (js JavaScriptLanguage) DetectImportEnd(lines []string) int {
	return js.ts.DetectImportEnd(lines)
}

// DetectBlocks identifies function and class boundaries in JavaScript.
// Filters out TypeScript-specific constructs (interface, type) from the TypeScript parser.
func (js JavaScriptLanguage) DetectBlocks(content string) []Block {
	// Use TypeScript parser to detect all blocks
	blocks := js.ts.DetectBlocks(content)

	// Filter out TypeScript-specific block types
	var jsBlocks []Block
	for _, block := range blocks {
		// Exclude interface and type blocks (TypeScript-only)
		if block.Type != "interface" && block.Type != "type" {
			jsBlocks = append(jsBlocks, block)
		}
	}

	return jsBlocks
}

// isTypeScriptOnlyLine checks if a line contains TypeScript-specific syntax.
// Used to filter out type annotations and interfaces when parsing as JavaScript.
func (js JavaScriptLanguage) isTypeScriptOnlyLine(line string) bool {
	trimmed := strings.TrimSpace(line)

	// Check for interface declarations
	if strings.HasPrefix(trimmed, "interface ") || strings.Contains(trimmed, " interface ") {
		return true
	}

	// Check for type alias declarations (but not typeof operator)
	if strings.HasPrefix(trimmed, "type ") && !strings.Contains(trimmed, "typeof") {
		return true
	}

	return false
}
