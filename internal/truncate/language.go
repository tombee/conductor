package truncate

// Block represents a code block (function, class, method, etc.) with its boundaries.
type Block struct {
	Type      string // "function", "class", "method", "block"
	Name      string // Identifier name if available
	StartLine int    // Line number where block starts (0-indexed)
	EndLine   int    // Line number where block ends (0-indexed, inclusive)
}

// Language defines the interface for language-specific parsing.
// Implementations provide language-aware detection of imports and code blocks
// to enable intelligent truncation at natural boundaries.
type Language interface {
	// DetectImportEnd returns the line index (0-indexed) where the import section ends.
	// Returns the first non-import, non-comment, non-blank line after imports.
	// Returns 0 if there are no imports.
	DetectImportEnd(lines []string) int

	// DetectBlocks returns all function/class/method boundaries in the content.
	// The content string contains the full file content.
	// Returns blocks in order of appearance (by StartLine).
	DetectBlocks(content string) []Block

	// CommentSyntax returns the comment syntax for this language.
	// Returns:
	//   - single: single-line comment prefix (e.g., "//", "#")
	//   - multiOpen: multi-line comment opening (e.g., "/*", `"""`)
	//   - multiClose: multi-line comment closing (e.g., "*/", `"""`)
	// Empty strings indicate no support for that comment type.
	CommentSyntax() (single string, multiOpen string, multiClose string)
}
