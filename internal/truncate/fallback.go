package truncate

import (
	"strings"
)

// FallbackLanguage provides line-based truncation for unknown or unsupported languages.
// It treats the entire file as a single block and provides no import detection.
// This ensures graceful degradation when language-specific parsing is unavailable.
type FallbackLanguage struct{}

// DetectImportEnd returns 0 for fallback as there's no language-specific import detection.
// The fallback treats the entire file uniformly without distinguishing imports.
func (f FallbackLanguage) DetectImportEnd(lines []string) int {
	return 0
}

// DetectBlocks returns the entire content as a single block.
// Since we don't understand the language structure, we treat everything as one unit.
// This allows line-based truncation while maintaining the Block interface contract.
func (f FallbackLanguage) DetectBlocks(content string) []Block {
	if content == "" {
		return []Block{}
	}

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

// CommentSyntax returns no comment syntax for fallback.
// Truncation indicators will use plaintext format without comment markers.
func (f FallbackLanguage) CommentSyntax() (single string, multiOpen string, multiClose string) {
	return "", "", ""
}
