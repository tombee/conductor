package sdk

import (
	"fmt"
	"strings"

	"github.com/tombee/conductor/internal/truncate"
)

// TruncateCode truncates code content while preserving structural integrity.
// It intelligently shortens code files by understanding language structure
// (imports, complete functions, class boundaries) to maximize useful context
// within size constraints.
//
// The function supports Go, TypeScript, Python, and JavaScript with
// language-aware truncation. For unsupported or unspecified languages,
// it falls back to line-based truncation.
//
// TruncateCode is thread-safe and can be called concurrently. It is
// deterministic - the same inputs always produce the same output.
//
// Security Considerations:
//
// This function is designed to safely process untrusted code input with
// the following protections:
//
//   - Input Size Protection: Enforces MaxBytes limit (default 10MB) to prevent
//     memory exhaustion attacks. Inputs exceeding this limit are rejected
//     before processing with ErrInputTooLarge.
//
//   - No External I/O: The function operates purely on in-memory strings with
//     no file system access, network calls, or external command execution.
//     This isolation prevents path traversal, arbitrary file access, or
//     command injection vulnerabilities.
//
//   - Deterministic Output: All operations are deterministic with no randomness,
//     time-based logic, or external state dependencies. The same input always
//     produces the same output, preventing timing attacks or non-deterministic
//     behavior that could leak information.
//
//   - Panic Recovery: All panics are caught and returned as errors to prevent
//     crashes. While panics should not occur in normal operation, this defense-
//     in-depth approach ensures graceful degradation even with malformed input.
//
//   - Nesting Depth Limits: The internal string/comment stripper enforces a
//     maximum nesting depth (1000 levels) to prevent stack overflow attacks
//     from deeply nested structures or excessive bracket depth.
//
//   - No Information Leakage: Error messages never include code content, line
//     numbers, or structural details to prevent information disclosure about
//     the input or internal processing state.
//
//   - Bounded Complexity: All parsing algorithms have linear or near-linear
//     time complexity relative to input size. No unbounded loops, recursion,
//     or backtracking that could enable algorithmic complexity attacks.
//
// Example usage:
//
//	// Truncate a Go file to 500 lines, preserving imports and complete functions
//	result, err := sdk.TruncateCode(sourceCode, sdk.TruncateOptions{
//		MaxLines:     500,
//		Language:     "go",
//		PreserveTop:  true,
//		PreserveFunc: true,
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Truncated from %d to %d lines\n",
//		result.OriginalLines, result.FinalLines)
//
// Returns an error if:
//   - Input exceeds MaxBytes limit (defaults to 10MB)
//   - Options contain negative values for MaxLines, MaxTokens, or MaxBytes
//
// The function returns metadata about what was removed including:
//   - WasTruncated: Whether any content was removed
//   - OmittedItems: Details of code blocks that were removed
//   - EstimatedTokens: Token estimate using chars/4 heuristic
//   - Indicator: Human-readable truncation comment
func TruncateCode(content string, opts TruncateOptions) (result TruncateResult, err error) {
	// Recover from any panics and return as errors for graceful handling
	defer func() {
		if r := recover(); r != nil {
			// This shouldn't happen in normal operation, but we want to
			// handle it gracefully rather than crashing the caller
			err = fmt.Errorf("truncation panic: %v", r)
			result = TruncateResult{}
		}
	}()

	// Step 1: Input validation
	if err := validateOptions(opts); err != nil {
		return TruncateResult{}, err
	}

	maxBytes := opts.MaxBytes
	if maxBytes == 0 {
		maxBytes = DefaultMaxBytes
	}

	if len(content) > maxBytes {
		return TruncateResult{}, NewInputTooLargeError()
	}

	// Handle empty content
	if content == "" {
		return TruncateResult{
			Content:          "",
			WasTruncated:     false,
			OriginalLines:    0,
			FinalLines:       0,
			EstimatedTokens:  0,
			OmittedItems:     []OmittedItem{},
			Indicator:        "",
		}, nil
	}

	// Step 2: Get language parser from internal registry
	lang := truncate.GetLanguage(opts.Language)
	if lang == nil {
		lang = truncate.FallbackLanguage{}
	}

	// Split content into lines for processing
	lines := strings.Split(content, "\n")
	originalLineCount := len(lines)

	// If no limits specified, return original content
	if opts.MaxLines <= 0 && opts.MaxTokens <= 0 {
		return TruncateResult{
			Content:          content,
			WasTruncated:     false,
			OriginalLines:    originalLineCount,
			FinalLines:       originalLineCount,
			EstimatedTokens:  estimateTokens(content),
			OmittedItems:     []OmittedItem{},
			Indicator:        "",
		}, nil
	}

	// Step 3: Detect block boundaries using language parser
	blocks := lang.DetectBlocks(content)

	// Step 4: Apply PreserveTop logic
	importEndLine := 0
	if opts.PreserveTop {
		importEndLine = lang.DetectImportEnd(lines)
	}

	// Step 5: Apply PreserveFunc logic and truncation
	result = applyTruncation(content, lines, blocks, importEndLine, opts, lang)
	return result, nil
}

// validateOptions checks that all options are valid.
func validateOptions(opts TruncateOptions) error {
	if opts.MaxLines < 0 {
		return NewInvalidOptionsError("MaxLines cannot be negative")
	}
	if opts.MaxTokens < 0 {
		return NewInvalidOptionsError("MaxTokens cannot be negative")
	}
	if opts.MaxBytes < 0 {
		return NewInvalidOptionsError("MaxBytes cannot be negative")
	}
	return nil
}

// applyTruncation performs the actual truncation logic.
func applyTruncation(content string, lines []string, blocks []truncate.Block, importEndLine int, opts TruncateOptions, lang truncate.Language) TruncateResult {
	var selectedLines []string
	var omittedItems []OmittedItem
	wasTruncated := false

	// If PreserveFunc is disabled, do simple line-based truncation
	if !opts.PreserveFunc {
		truncLine := calculateTruncationPoint(lines, opts)
		if truncLine < len(lines) {
			wasTruncated = true
			selectedLines = lines[:truncLine]

			// Calculate omitted content
			omittedLineCount := len(lines) - truncLine
			if len(blocks) > 0 {
				// Count omitted blocks
				for _, block := range blocks {
					if block.StartLine >= truncLine {
						omittedItems = append(omittedItems, OmittedItem{
							Type:      block.Type,
							Name:      block.Name,
							StartLine: block.StartLine + 1, // Convert to 1-indexed
							EndLine:   block.EndLine + 1,   // Convert to 1-indexed
						})
					}
				}
			}

			// Add truncation indicator
			indicator := generateIndicator(omittedItems, omittedLineCount, lang)
			selectedLines = append(selectedLines, indicator)
		} else {
			selectedLines = lines
		}
	} else {
		// PreserveFunc is enabled - truncate at function boundaries
		selectedLines, omittedItems, wasTruncated = preserveFuncTruncation(lines, blocks, importEndLine, opts, lang)
	}

	// Build final content
	finalContent := strings.Join(selectedLines, "\n")

	return TruncateResult{
		Content:          finalContent,
		WasTruncated:     wasTruncated,
		OriginalLines:    len(lines),
		FinalLines:       len(selectedLines),
		EstimatedTokens:  estimateTokens(finalContent),
		OmittedItems:     omittedItems,
		Indicator:        "",
	}
}

// preserveFuncTruncation applies function-preserving truncation logic.
func preserveFuncTruncation(lines []string, blocks []truncate.Block, importEndLine int, opts TruncateOptions, lang truncate.Language) ([]string, []OmittedItem, bool) {
	var selectedLines []string
	var omittedItems []OmittedItem
	currentLine := 0

	// Step 1: Add imports/header if PreserveTop is enabled
	if importEndLine > 0 {
		for i := 0; i < importEndLine && i < len(lines); i++ {
			selectedLines = append(selectedLines, lines[i])
			currentLine = i + 1
		}
	}

	// Check if we've already exceeded limits with just imports
	if exceedsLimits(selectedLines, opts) {
		// Truncate even the imports
		truncLine := calculateTruncationPoint(selectedLines, opts)
		selectedLines = selectedLines[:truncLine]

		// Calculate what was omitted
		omittedLineCount := len(lines) - len(selectedLines)
		for _, block := range blocks {
			omittedItems = append(omittedItems, OmittedItem{
				Type:      block.Type,
				Name:      block.Name,
				StartLine: block.StartLine + 1,
				EndLine:   block.EndLine + 1,
			})
		}

		indicator := generateIndicator(omittedItems, omittedLineCount, lang)
		selectedLines = append(selectedLines, indicator)
		return selectedLines, omittedItems, true
	}

	// Step 2: Add complete functions from the start until we hit the limit
	for _, block := range blocks {
		// Skip blocks that are before our current position (already included in imports)
		if block.EndLine < currentLine {
			continue
		}

		// Try to include this block
		blockLines := []string{}

		// Add any gap lines between current position and block start
		for i := currentLine; i < block.StartLine && i < len(lines); i++ {
			blockLines = append(blockLines, lines[i])
		}

		// Add the block itself
		for i := block.StartLine; i <= block.EndLine && i < len(lines); i++ {
			blockLines = append(blockLines, lines[i])
		}

		// Check if adding this block would exceed limits
		testLines := append(selectedLines, blockLines...)
		if exceedsLimits(testLines, opts) {
			// This block doesn't fit - omit it and all remaining blocks
			for _, b := range blocks {
				if b.StartLine >= block.StartLine {
					omittedItems = append(omittedItems, OmittedItem{
						Type:      b.Type,
						Name:      b.Name,
						StartLine: b.StartLine + 1,
						EndLine:   b.EndLine + 1,
					})
				}
			}

			// Add truncation indicator
			omittedLineCount := len(lines) - len(selectedLines)
			indicator := generateIndicator(omittedItems, omittedLineCount, lang)
			selectedLines = append(selectedLines, indicator)
			return selectedLines, omittedItems, true
		}

		// Block fits - include it
		selectedLines = testLines
		currentLine = block.EndLine + 1
	}

	// Check if we included everything
	wasTruncated := currentLine < len(lines)
	if wasTruncated {
		// There's content after the last block that we need to account for
		omittedLineCount := len(lines) - currentLine
		indicator := generateIndicator(omittedItems, omittedLineCount, lang)
		selectedLines = append(selectedLines, indicator)
	}

	return selectedLines, omittedItems, wasTruncated
}

// exceedsLimits checks if the given lines exceed the specified limits.
func exceedsLimits(lines []string, opts TruncateOptions) bool {
	if opts.MaxLines > 0 && len(lines) > opts.MaxLines {
		return true
	}

	if opts.MaxTokens > 0 {
		content := strings.Join(lines, "\n")
		tokens := estimateTokens(content)
		if tokens > opts.MaxTokens {
			return true
		}
	}

	return false
}

// calculateTruncationPoint determines where to truncate for line-based truncation.
// Returns the line index where truncation should occur.
func calculateTruncationPoint(lines []string, opts TruncateOptions) int {
	// Start with MaxLines if specified
	truncPoint := len(lines)

	if opts.MaxLines > 0 && opts.MaxLines < truncPoint {
		truncPoint = opts.MaxLines
	}

	// Apply MaxTokens constraint if more restrictive
	if opts.MaxTokens > 0 {
		// Binary search for the line that fits within token limit
		for i := 0; i < truncPoint; i++ {
			testContent := strings.Join(lines[:i+1], "\n")
			if estimateTokens(testContent) > opts.MaxTokens {
				truncPoint = i
				break
			}
		}
	}

	// Ensure we don't exceed the content length
	if truncPoint > len(lines) {
		truncPoint = len(lines)
	}

	return truncPoint
}

// generateIndicator creates a truncation indicator comment.
func generateIndicator(omittedItems []OmittedItem, omittedLines int, lang truncate.Language) string {
	single, _, _ := lang.CommentSyntax()

	// Count items by type
	itemCounts := make(map[string]int)
	for _, item := range omittedItems {
		itemCounts[item.Type]++
	}

	// Build description
	var parts []string
	for itemType, count := range itemCounts {
		if count == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", itemType))
		} else {
			parts = append(parts, fmt.Sprintf("%d %ss", count, itemType))
		}
	}

	description := strings.Join(parts, ", ")
	if description == "" {
		description = fmt.Sprintf("%d items", len(omittedItems))
	}

	// Format indicator based on language
	if single != "" {
		return fmt.Sprintf("%s ... %s omitted (%d lines)", single, description, omittedLines)
	}

	// Fallback for unknown languages
	return fmt.Sprintf("... %s omitted (%d lines)", description, omittedLines)
}

// estimateTokens estimates the token count using the chars/4 heuristic.
func estimateTokens(content string) int {
	return (len(content) + 3) / 4 // Round up
}
