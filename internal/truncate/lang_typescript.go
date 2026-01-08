package truncate

import (
	"strings"
	"unicode"
)

// TypeScriptLanguage implements Language interface for TypeScript files.
// Recognizes TypeScript-specific constructs including interfaces, type aliases,
// classes, functions (both traditional and arrow), and ES6 module syntax.
type TypeScriptLanguage struct{}

func init() {
	RegisterLanguage("typescript", TypeScriptLanguage{})
}

// CommentSyntax returns TypeScript's comment syntax.
// TypeScript uses // for single-line and /* */ for multi-line comments.
func (ts TypeScriptLanguage) CommentSyntax() (single string, multiOpen string, multiClose string) {
	return "//", "/*", "*/"
}

// DetectImportEnd returns the line index where the import section ends.
// Includes import statements and export declarations at the top of the file.
// Returns the first non-import, non-export, non-comment, non-blank line.
func (ts TypeScriptLanguage) DetectImportEnd(lines []string) int {
	lastImportLine := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip blank lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		// Check for import or export statements
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "import{") ||
			strings.HasPrefix(trimmed, "export ") || strings.HasPrefix(trimmed, "export{") {
			lastImportLine = i
			continue
		}

		// If we hit a non-import/export line, stop
		break
	}

	// Return the line after the last import/export
	if lastImportLine >= 0 {
		return lastImportLine + 1
	}

	return 0
}

// DetectBlocks identifies function, class, interface, and type boundaries in TypeScript.
// Uses brace depth tracking after stripping strings and comments for accuracy.
// Arrow functions with simple expressions (no braces) are treated as single-line blocks.
func (ts TypeScriptLanguage) DetectBlocks(content string) []Block {
	if content == "" {
		return []Block{}
	}

	// Strip strings and comments for accurate brace counting
	single, multiOpen, multiClose := ts.CommentSyntax()
	stripper := NewStripper(single, multiOpen, multiClose)
	stripped, err := stripper.Strip(content)
	if err != nil {
		// If stripping fails, return the entire content as one block
		lines := strings.Split(content, "\n")
		return []Block{
			{
				Type:      "block",
				Name:      "",
				StartLine: 0,
				EndLine:   len(lines) - 1,
			},
		}
	}

	// Keep both original and stripped lines for name extraction
	originalLines := strings.Split(content, "\n")
	strippedLines := strings.Split(stripped, "\n")
	var blocks []Block

	for i := 0; i < len(strippedLines); i++ {
		strippedLine := strings.TrimSpace(strippedLines[i])
		originalLine := strings.TrimSpace(originalLines[i])

		// Check for class declarations (use stripped line for detection)
		if strings.HasPrefix(strippedLine, "class ") || strings.Contains(strippedLine, " class ") {
			if block := ts.detectBraceBlock(originalLines, strippedLines, i, "class", stripped); block != nil {
				blocks = append(blocks, *block)
			}
			continue
		}

		// Check for interface declarations (use stripped line for detection)
		if strings.HasPrefix(strippedLine, "interface ") || strings.Contains(strippedLine, " interface ") {
			if block := ts.detectBraceBlock(originalLines, strippedLines, i, "interface", stripped); block != nil {
				blocks = append(blocks, *block)
			}
			continue
		}

		// Check for type declarations (use stripped line for detection)
		if strings.HasPrefix(strippedLine, "type ") || strings.Contains(strippedLine, " type ") {
			// Type aliases may or may not have braces
			if block := ts.detectTypeBlock(originalLines, strippedLines, i, stripped); block != nil {
				blocks = append(blocks, *block)
			}
			continue
		}

		// Check for function declarations (use stripped line for detection)
		if strings.HasPrefix(strippedLine, "function ") || strings.Contains(strippedLine, " function ") ||
			strings.HasPrefix(strippedLine, "async function ") {
			if block := ts.detectBraceBlock(originalLines, strippedLines, i, "function", stripped); block != nil {
				blocks = append(blocks, *block)
			}
			continue
		}

		// Check for arrow functions - use ORIGINAL line for detection since names are stripped
		if (strings.HasPrefix(originalLine, "const ") || strings.HasPrefix(originalLine, "let ") ||
			strings.HasPrefix(originalLine, "var ") || strings.HasPrefix(originalLine, "export const ") ||
			strings.HasPrefix(originalLine, "export let ")) && strings.Contains(strippedLine, "=>") {
			if block := ts.detectArrowFunction(originalLines, strippedLines, i, stripped); block != nil {
				blocks = append(blocks, *block)
			}
			continue
		}
	}

	return blocks
}

// detectBraceBlock finds a block that starts with a keyword and is delimited by braces.
// Used for classes, interfaces, and traditional functions.
func (ts TypeScriptLanguage) detectBraceBlock(originalLines []string, strippedLines []string, startLine int, blockType string, stripped string) *Block {
	name := ts.extractName(originalLines[startLine], blockType)

	// Find the opening brace in stripped lines
	openBraceLine := -1
	for i := startLine; i < len(strippedLines); i++ {
		if strings.Contains(strippedLines[i], "{") {
			openBraceLine = i
			break
		}
		// If we hit a semicolon or another statement before finding a brace, this isn't a block
		if strings.Contains(strippedLines[i], ";") {
			return nil
		}
	}

	if openBraceLine == -1 {
		return nil
	}

	// Track brace depth from the opening brace
	depth := 0
	allStrippedLines := strings.Split(stripped, "\n")

	for i := openBraceLine; i < len(allStrippedLines); i++ {
		for _, ch := range allStrippedLines[i] {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					return &Block{
						Type:      blockType,
						Name:      name,
						StartLine: startLine,
						EndLine:   i,
					}
				}
			}
		}
	}

	// If we never found the closing brace, treat up to the end as the block
	return &Block{
		Type:      blockType,
		Name:      name,
		StartLine: startLine,
		EndLine:   len(originalLines) - 1,
	}
}

// detectTypeBlock handles type alias declarations which may span multiple lines.
// Type aliases can be simple (type X = Y;) or complex with braces (type X = { ... }).
func (ts TypeScriptLanguage) detectTypeBlock(originalLines []string, strippedLines []string, startLine int, stripped string) *Block {
	name := ts.extractName(originalLines[startLine], "type")

	// Check if the type has braces (object type) in stripped line
	if strings.Contains(strippedLines[startLine], "{") {
		return ts.detectBraceBlock(originalLines, strippedLines, startLine, "type", stripped)
	}

	// Simple type alias - find the semicolon in stripped lines
	for i := startLine; i < len(strippedLines); i++ {
		if strings.Contains(strippedLines[i], ";") {
			return &Block{
				Type:      "type",
				Name:      name,
				StartLine: startLine,
				EndLine:   i,
			}
		}
		// If we see another statement start, the previous line was the end
		trimmed := strings.TrimSpace(strippedLines[i])
		if i > startLine && (strings.HasPrefix(trimmed, "import ") ||
			strings.HasPrefix(trimmed, "export ") ||
			strings.HasPrefix(trimmed, "const ") ||
			strings.HasPrefix(trimmed, "let ") ||
			strings.HasPrefix(trimmed, "var ") ||
			strings.HasPrefix(trimmed, "function ") ||
			strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "interface ") ||
			strings.HasPrefix(trimmed, "type ")) {
			return &Block{
				Type:      "type",
				Name:      name,
				StartLine: startLine,
				EndLine:   i - 1,
			}
		}
	}

	// If no terminator found, treat as single line
	return &Block{
		Type:      "type",
		Name:      name,
		StartLine: startLine,
		EndLine:   startLine,
	}
}

// detectArrowFunction handles arrow function expressions.
// Arrow functions can be simple expressions (x => x + 1) or block bodies (x => { ... }).
// Only block-body arrows are tracked as separate blocks.
func (ts TypeScriptLanguage) detectArrowFunction(originalLines []string, strippedLines []string, startLine int, stripped string) *Block {
	originalLine := originalLines[startLine]
	strippedLine := strippedLines[startLine]
	name := ts.extractArrowFunctionName(originalLine)

	// Check if this is a block-body arrow function (has braces after =>) using stripped line
	arrowIdx := strings.Index(strippedLine, "=>")
	openBraceLine := startLine

	if arrowIdx == -1 {
		// Multi-line arrow definition; look ahead in stripped lines
		for i := startLine; i < len(strippedLines) && i < startLine+5; i++ {
			if strings.Contains(strippedLines[i], "=>") {
				arrowIdx = strings.Index(strippedLines[i], "=>")
				strippedLine = strippedLines[i]
				openBraceLine = i
				break
			}
		}
		if arrowIdx == -1 {
			return nil
		}
	}

	afterArrow := strings.TrimSpace(strippedLine[arrowIdx+2:])

	// If there's an opening brace after the arrow, it's a block body
	if strings.HasPrefix(afterArrow, "{") || strings.Contains(strippedLine, "=> {") {
		// Track brace depth to find the end of the function
		depth := 0
		allStrippedLines := strings.Split(stripped, "\n")

		for i := openBraceLine; i < len(allStrippedLines); i++ {
			for _, ch := range allStrippedLines[i] {
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
					if depth == 0 {
						return &Block{
							Type:      "function",
							Name:      name,
							StartLine: startLine,
							EndLine:   i,
						}
					}
				}
			}
		}

		// If we never found the closing brace, treat up to the end as the block
		return &Block{
			Type:      "function",
			Name:      name,
			StartLine: startLine,
			EndLine:   len(originalLines) - 1,
		}
	}

	// Simple expression arrow function - treat as single line
	return &Block{
		Type:      "function",
		Name:      name,
		StartLine: startLine,
		EndLine:   startLine,
	}
}

// extractName extracts the identifier name from a declaration line.
// Handles class, interface, type, and function declarations.
func (ts TypeScriptLanguage) extractName(line string, blockType string) string {
	// Remove leading keywords and modifiers
	line = strings.TrimSpace(line)
	tokens := strings.Fields(line)

	// Find the keyword and extract the next token as the name
	for i, token := range tokens {
		if token == blockType {
			if i+1 < len(tokens) {
				name := tokens[i+1]
				// Clean up any trailing characters
				name = strings.TrimFunc(name, func(r rune) bool {
					return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
				})
				return name
			}
		}
	}

	return ""
}

// extractArrowFunctionName extracts the variable name from an arrow function declaration.
// Handles const/let/var declarations with export modifiers.
func (ts TypeScriptLanguage) extractArrowFunctionName(line string) string {
	line = strings.TrimSpace(line)

	// Remove export modifier if present
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimSpace(line)

	// Find the variable name after const/let/var
	for _, keyword := range []string{"const ", "let ", "var "} {
		if strings.HasPrefix(line, keyword) {
			rest := strings.TrimSpace(line[len(keyword):])
			// Split by whitespace and get the first token
			tokens := strings.Fields(rest)
			if len(tokens) > 0 {
				// The first token should be the variable name
				// Remove any trailing punctuation like : or =
				name := tokens[0]
				name = strings.TrimFunc(name, func(r rune) bool {
					return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$'
				})
				return name
			}
		}
	}

	return ""
}
