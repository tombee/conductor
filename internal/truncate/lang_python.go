package truncate

import (
	"strings"
)

// PythonLanguage implements the Language interface for Python source files.
type PythonLanguage struct{}

func init() {
	RegisterLanguage("python", PythonLanguage{})
}

// CommentSyntax returns Python's comment syntax: # for single-line, """ and ''' for multi-line.
func (p PythonLanguage) CommentSyntax() (single string, multiOpen string, multiClose string) {
	return "#", `"""`, `"""`
}

// DetectImportEnd returns the line index where imports end.
// Returns the first non-import, non-comment, non-blank line after the import section.
// Returns 0 if there are no imports.
func (p PythonLanguage) DetectImportEnd(lines []string) int {
	sawImport := false
	lastImportLine := -1
	inMultilineImport := false
	usingParens := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments before we've seen imports
		if !sawImport && (trimmed == "" || strings.HasPrefix(trimmed, "#")) {
			continue
		}

		// Check for import statements
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
			sawImport = true
			lastImportLine = i

			// Check if it's a multiline import (ends with backslash or has opening paren)
			if strings.Contains(trimmed, "(") && !strings.Contains(trimmed, ")") {
				inMultilineImport = true
				usingParens = true
			} else if strings.HasSuffix(trimmed, "\\") {
				inMultilineImport = true
				usingParens = false
			}
			continue
		}

		// Handle continuation of multiline imports
		if inMultilineImport {
			lastImportLine = i
			// Check if the multiline import ends
			if usingParens {
				if strings.Contains(trimmed, ")") {
					inMultilineImport = false
				}
			} else {
				// Backslash continuation - ends when no backslash at end
				if !strings.HasSuffix(trimmed, "\\") {
					inMultilineImport = false
				}
			}
			continue
		}

		// After seeing imports, skip blank lines and comments
		if sawImport {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			// Hit first non-blank, non-comment, non-import line
			return i
		}
	}

	// If we only saw imports until EOF, return the line after the last import
	if sawImport {
		return lastImportLine + 1
	}

	return 0
}

// DetectBlocks returns all function and class boundaries in the content.
// Uses indentation tracking for Python's block structure.
func (p PythonLanguage) DetectBlocks(content string) []Block {
	if content == "" {
		return []Block{}
	}

	// Strip strings and comments for accurate detection
	single, multiOpen, multiClose := p.CommentSyntax()
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
	i := 0

	for i < len(strippedLines) {
		trimmed := strings.TrimSpace(strippedLines[i])

		// Check for decorators (@decorator) - they're part of the function/class block
		if strings.HasPrefix(trimmed, "@") {
			decoratorStart := i
			// Find the def/class that follows the decorator(s)
			i++
			for i < len(strippedLines) {
				nextTrimmed := strings.TrimSpace(strippedLines[i])
				if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "@") {
					i++
					continue
				}
				// Check if this is a def or class
				if strings.HasPrefix(nextTrimmed, "def ") || strings.HasPrefix(nextTrimmed, "async def ") {
					block := p.detectFunctionBlock(lines, strippedLines, decoratorStart)
					if block != nil {
						blocks = append(blocks, *block)
						i = block.EndLine + 1
					}
					break
				}
				if strings.HasPrefix(nextTrimmed, "class ") {
					block := p.detectClassBlock(lines, strippedLines, decoratorStart)
					if block != nil {
						blocks = append(blocks, *block)
						i = block.EndLine + 1
					}
					break
				}
				// Not a def or class after decorator, skip the decorator
				i = decoratorStart + 1
				break
			}
			continue
		}

		// Detect function definitions (including async)
		if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "async def ") {
			block := p.detectFunctionBlock(lines, strippedLines, i)
			if block != nil {
				blocks = append(blocks, *block)
				i = block.EndLine + 1
				continue
			}
		}

		// Detect class definitions
		if strings.HasPrefix(trimmed, "class ") {
			block := p.detectClassBlock(lines, strippedLines, i)
			if block != nil {
				blocks = append(blocks, *block)
				i = block.EndLine + 1
				continue
			}
		}

		i++
	}

	return blocks
}

// detectFunctionBlock detects a function or method block starting at or before the given line.
// The startLine may point to a decorator; we'll find the actual def line.
func (p PythonLanguage) detectFunctionBlock(lines []string, strippedLines []string, startLine int) *Block {
	if startLine >= len(strippedLines) {
		return nil
	}

	// Find the actual def line (might be after decorators)
	defLine := startLine
	for defLine < len(strippedLines) {
		trimmed := strings.TrimSpace(strippedLines[defLine])
		if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "async def ") {
			break
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "@") {
			// Not a decorator or def, bail
			return nil
		}
		defLine++
	}

	if defLine >= len(strippedLines) {
		return nil
	}

	// Extract function name
	line := strings.TrimSpace(strippedLines[defLine])
	name := p.extractFunctionName(line)

	// Get base indentation of the def line
	baseIndent := p.getIndentation(strippedLines[defLine])

	// Find the end of the function by tracking indentation
	endLine := defLine
	foundBody := false

	for i := defLine + 1; i < len(strippedLines); i++ {
		trimmed := strings.TrimSpace(strippedLines[i])

		// Skip empty lines and comments (don't update endLine - might be trailing)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := p.getIndentation(strippedLines[i])

		// If indentation is greater than base, we're inside the function
		if indent > baseIndent {
			foundBody = true
			endLine = i
			continue
		}

		// If indentation is <= base and we've seen the body, the function ends
		if foundBody && indent <= baseIndent {
			break
		}

		// If we haven't seen a body yet and hit same/lower indent, might be empty function
		if !foundBody && indent <= baseIndent {
			break
		}
	}

	return &Block{
		Type:      "function",
		Name:      name,
		StartLine: startLine,
		EndLine:   endLine,
	}
}

// detectClassBlock detects a class block starting at or before the given line.
// The startLine may point to a decorator; we'll find the actual class line.
func (p PythonLanguage) detectClassBlock(lines []string, strippedLines []string, startLine int) *Block {
	if startLine >= len(strippedLines) {
		return nil
	}

	// Find the actual class line (might be after decorators)
	classLine := startLine
	for classLine < len(strippedLines) {
		trimmed := strings.TrimSpace(strippedLines[classLine])
		if strings.HasPrefix(trimmed, "class ") {
			break
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "@") {
			// Not a decorator or class, bail
			return nil
		}
		classLine++
	}

	if classLine >= len(strippedLines) {
		return nil
	}

	// Extract class name
	line := strings.TrimSpace(strippedLines[classLine])
	name := p.extractClassName(line)

	// Get base indentation of the class line
	baseIndent := p.getIndentation(strippedLines[classLine])

	// Find the end of the class by tracking indentation
	endLine := classLine
	foundBody := false

	for i := classLine + 1; i < len(strippedLines); i++ {
		trimmed := strings.TrimSpace(strippedLines[i])

		// Skip empty lines and comments (don't update endLine - might be trailing)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := p.getIndentation(strippedLines[i])

		// If indentation is greater than base, we're inside the class
		if indent > baseIndent {
			foundBody = true
			endLine = i
			continue
		}

		// If indentation is <= base and we've seen the body, the class ends
		if foundBody && indent <= baseIndent {
			break
		}

		// If we haven't seen a body yet and hit same/lower indent, might be empty class
		if !foundBody && indent <= baseIndent {
			break
		}
	}

	return &Block{
		Type:      "class",
		Name:      name,
		StartLine: startLine,
		EndLine:   endLine,
	}
}

// getIndentation returns the number of leading spaces/tabs in a line.
// Tabs are counted as 4 spaces for consistency.
func (p PythonLanguage) getIndentation(line string) int {
	indent := 0
	for _, ch := range line {
		if ch == ' ' {
			indent++
		} else if ch == '\t' {
			indent += 4
		} else {
			break
		}
	}
	return indent
}

// extractFunctionName extracts the function name from a def line.
// Handles: def name(), async def name()
func (p PythonLanguage) extractFunctionName(line string) string {
	// Remove "async def " or "def " prefix
	line = strings.TrimPrefix(strings.TrimSpace(line), "async ")
	line = strings.TrimPrefix(strings.TrimSpace(line), "def ")

	// Extract name (everything before the opening parenthesis)
	parenIdx := strings.Index(line, "(")
	if parenIdx > 0 {
		return strings.TrimSpace(line[:parenIdx])
	}

	// No parameters found - might be malformed, return what we have
	fields := strings.Fields(line)
	if len(fields) > 0 {
		return fields[0]
	}

	return ""
}

// extractClassName extracts the class name from a class line.
// Handles: class Name:, class Name(Base):, class Name(Base1, Base2):
func (p PythonLanguage) extractClassName(line string) string {
	// Remove "class " prefix
	line = strings.TrimPrefix(strings.TrimSpace(line), "class ")

	// Extract name (everything before : or ( )
	colonIdx := strings.Index(line, ":")
	parenIdx := strings.Index(line, "(")

	if parenIdx > 0 && (colonIdx == -1 || parenIdx < colonIdx) {
		// Has base classes
		return strings.TrimSpace(line[:parenIdx])
	}

	if colonIdx > 0 {
		// No base classes, just name:
		return strings.TrimSpace(line[:colonIdx])
	}

	// No : or ( found - might be malformed, return what we have
	fields := strings.Fields(line)
	if len(fields) > 0 {
		return fields[0]
	}

	return ""
}
