package truncate

import (
	"strings"
)

// GoLanguage implements the Language interface for Go source files.
type GoLanguage struct{}

func init() {
	RegisterLanguage("go", GoLanguage{})
}

// CommentSyntax returns Go's comment syntax: // for single-line, /* */ for multi-line.
func (g GoLanguage) CommentSyntax() (single string, multiOpen string, multiClose string) {
	return "//", "/*", "*/"
}

// DetectImportEnd returns the line index where imports end.
// Returns the first non-import, non-comment, non-blank line after the import section.
// Returns 0 if there are no imports.
func (g GoLanguage) DetectImportEnd(lines []string) int {
	inImportBlock := false
	sawImport := false
	lastImportLine := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for import statement
		if strings.HasPrefix(trimmed, "import") {
			sawImport = true
			lastImportLine = i
			// Check if it's a grouped import block
			if strings.Contains(trimmed, "(") {
				inImportBlock = true
			}
			continue
		}

		// Check if we're in an import block (lines between import ( and ))
		if inImportBlock {
			lastImportLine = i
			if strings.Contains(trimmed, ")") {
				inImportBlock = false
			}
			continue
		}

		// If we've seen imports, skip blank lines and comments after them
		if sawImport {
			if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
				continue
			}
			// Hit first non-blank, non-comment, non-import line after imports
			return i
		}
	}

	// If we only saw imports until EOF, return the line after the last import
	if sawImport {
		return lastImportLine + 1
	}

	return 0
}

// DetectBlocks returns all function, method, and type boundaries in the content.
// Uses bracket counting after detecting func/type keywords.
func (g GoLanguage) DetectBlocks(content string) []Block {
	if content == "" {
		return []Block{}
	}

	// Strip strings and comments for accurate bracket counting
	single, multiOpen, multiClose := g.CommentSyntax()
	stripper := NewStripper(single, multiOpen, multiClose)
	stripped, err := stripper.Strip(content)
	if err != nil {
		// On error, fall back to treating entire content as one block
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

	lines := strings.Split(content, "\n")
	strippedLines := strings.Split(stripped, "\n")

	var blocks []Block

	for i := 0; i < len(strippedLines); i++ {
		trimmed := strings.TrimSpace(strippedLines[i])

		// Detect function declarations
		if strings.HasPrefix(trimmed, "func ") {
			block := g.detectFunctionBlock(lines, strippedLines, i)
			if block != nil {
				blocks = append(blocks, *block)
				i = block.EndLine // Skip to end of this block
			}
			continue
		}

		// Detect type declarations
		if strings.HasPrefix(trimmed, "type ") {
			block := g.detectTypeBlock(lines, strippedLines, i)
			if block != nil {
				blocks = append(blocks, *block)
				i = block.EndLine // Skip to end of this block
			}
			continue
		}
	}

	return blocks
}

// detectFunctionBlock detects a function or method block starting at the given line.
// Handles both functions and methods with receivers: func (r *Type) Method()
func (g GoLanguage) detectFunctionBlock(lines []string, strippedLines []string, startLine int) *Block {
	if startLine >= len(strippedLines) {
		return nil
	}

	// Extract function name
	line := strings.TrimSpace(strippedLines[startLine])
	name := g.extractFunctionName(line)

	// Find the opening brace
	braceStart := -1
	for i := startLine; i < len(strippedLines); i++ {
		if strings.Contains(strippedLines[i], "{") {
			braceStart = i
			break
		}
		// If we hit a line with just a semicolon or another func, it's a declaration without body
		trimmed := strings.TrimSpace(strippedLines[i])
		if strings.HasSuffix(trimmed, ";") || (i > startLine && strings.HasPrefix(trimmed, "func ")) {
			return nil
		}
	}

	if braceStart == -1 {
		// No opening brace found - might be an interface method or forward declaration
		return nil
	}

	// Track brace depth to find the closing brace
	depth := 0
	endLine := braceStart

	for i := braceStart; i < len(strippedLines); i++ {
		for _, ch := range strippedLines[i] {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					endLine = i
					return &Block{
						Type:      "function",
						Name:      name,
						StartLine: startLine,
						EndLine:   endLine,
					}
				}
			}
		}
	}

	// Unclosed brace - treat as extending to end of file
	return &Block{
		Type:      "function",
		Name:      name,
		StartLine: startLine,
		EndLine:   len(strippedLines) - 1,
	}
}

// detectTypeBlock detects a type declaration starting at the given line.
// Handles struct types with braces.
func (g GoLanguage) detectTypeBlock(lines []string, strippedLines []string, startLine int) *Block {
	if startLine >= len(strippedLines) {
		return nil
	}

	// Extract type name
	line := strings.TrimSpace(strippedLines[startLine])
	name := g.extractTypeName(line)

	// Check if this is a struct with braces
	if !strings.Contains(line, "struct") {
		// Simple type alias - single line
		return &Block{
			Type:      "type",
			Name:      name,
			StartLine: startLine,
			EndLine:   startLine,
		}
	}

	// Find the opening brace for struct
	braceStart := -1
	for i := startLine; i < len(strippedLines); i++ {
		if strings.Contains(strippedLines[i], "{") {
			braceStart = i
			break
		}
		// If we hit another type or func, stop
		trimmed := strings.TrimSpace(strippedLines[i])
		if i > startLine && (strings.HasPrefix(trimmed, "type ") || strings.HasPrefix(trimmed, "func ")) {
			return &Block{
				Type:      "type",
				Name:      name,
				StartLine: startLine,
				EndLine:   i - 1,
			}
		}
	}

	if braceStart == -1 {
		// No opening brace found
		return &Block{
			Type:      "type",
			Name:      name,
			StartLine: startLine,
			EndLine:   startLine,
		}
	}

	// Track brace depth to find the closing brace
	depth := 0
	endLine := braceStart

	for i := braceStart; i < len(strippedLines); i++ {
		for _, ch := range strippedLines[i] {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					endLine = i
					return &Block{
						Type:      "type",
						Name:      name,
						StartLine: startLine,
						EndLine:   endLine,
					}
				}
			}
		}
	}

	// Unclosed brace - treat as extending to end of file
	return &Block{
		Type:      "type",
		Name:      name,
		StartLine: startLine,
		EndLine:   len(strippedLines) - 1,
	}
}

// extractFunctionName extracts the function or method name from a func declaration line.
// Handles: func Name(), func (r Receiver) Name(), func (r *Receiver) Name()
func (g GoLanguage) extractFunctionName(line string) string {
	// Remove "func " prefix
	line = strings.TrimPrefix(strings.TrimSpace(line), "func ")

	// Check for method receiver: (r Receiver) or (r *Receiver)
	if strings.HasPrefix(line, "(") {
		// Find the closing parenthesis of the receiver
		closeIdx := strings.Index(line, ")")
		if closeIdx > 0 && closeIdx < len(line)-1 {
			line = line[closeIdx+1:]
		}
	}

	// Now extract the function name (everything before the opening parenthesis)
	line = strings.TrimSpace(line)
	parenIdx := strings.Index(line, "(")
	if parenIdx > 0 {
		return strings.TrimSpace(line[:parenIdx])
	}

	// No parameters found - might be malformed, return what we have
	return strings.Fields(line)[0]
}

// extractTypeName extracts the type name from a type declaration line.
// Handles: type Name struct, type Name interface, type Name = OtherType
func (g GoLanguage) extractTypeName(line string) string {
	// Remove "type " prefix
	line = strings.TrimPrefix(strings.TrimSpace(line), "type ")

	// Extract the first word (the type name)
	fields := strings.Fields(line)
	if len(fields) > 0 {
		return fields[0]
	}

	return ""
}
