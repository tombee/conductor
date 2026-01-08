package truncate

import (
	"errors"
	"strings"
)

const (
	defaultMaxNestingDepth = 1000
)

var (
	ErrMaxNestingDepthExceeded = errors.New("maximum nesting depth exceeded")
)

// stripState represents the current parsing state in the stripper state machine.
type stripState int

const (
	stateNormal stripState = iota
	stateSingleLineComment
	stateMultiLineComment
	stateDoubleQuoteString
	stateSingleQuoteString
	stateBacktickString
	stateTripleDoubleQuoteString
	stateTripleSingleQuoteString
)

// Stripper removes string literals and comments from code while preserving
// structure (line breaks, character positions) for bracket counting.
type Stripper struct {
	singleLineComment string
	multiOpen         string
	multiClose        string
	maxDepth          int
}

// NewStripper creates a stripper for the given comment syntax.
// Pass empty strings for unsupported comment types.
func NewStripper(singleLineComment, multiOpen, multiClose string) *Stripper {
	return &Stripper{
		singleLineComment: singleLineComment,
		multiOpen:         multiOpen,
		multiClose:        multiClose,
		maxDepth:          defaultMaxNestingDepth,
	}
}

// Strip removes strings and comments from content, replacing them with spaces.
// Line structure and character positions are preserved for accurate bracket counting.
// Returns error if nesting depth exceeds safety limit.
func (s *Stripper) Strip(content string) (string, error) {
	if content == "" {
		return "", nil
	}

	var result strings.Builder
	result.Grow(len(content))

	state := stateNormal
	escaped := false
	depth := 0
	i := 0

	for i < len(content) {
		ch := content[i]

		// Check nesting depth to prevent stack overflow attacks
		if depth > s.maxDepth {
			return "", ErrMaxNestingDepthExceeded
		}

		switch state {
		case stateNormal:
			// Check for triple-quoted strings first (Python)
			if i+2 < len(content) && content[i:i+3] == `"""` {
				result.WriteString("   ")
				i += 3
				state = stateTripleDoubleQuoteString
				depth++
				continue
			}
			if i+2 < len(content) && content[i:i+3] == "'''" {
				result.WriteString("   ")
				i += 3
				state = stateTripleSingleQuoteString
				depth++
				continue
			}

			// Check for multi-line comment
			if s.multiOpen != "" && strings.HasPrefix(content[i:], s.multiOpen) {
				result.WriteString(strings.Repeat(" ", len(s.multiOpen)))
				i += len(s.multiOpen)
				state = stateMultiLineComment
				depth++
				continue
			}

			// Check for single-line comment
			if s.singleLineComment != "" && strings.HasPrefix(content[i:], s.singleLineComment) {
				result.WriteString(strings.Repeat(" ", len(s.singleLineComment)))
				i += len(s.singleLineComment)
				state = stateSingleLineComment
				depth++
				continue
			}

			// Check for string literals
			if ch == '"' {
				result.WriteByte(' ')
				i++
				state = stateDoubleQuoteString
				depth++
				continue
			}
			if ch == '\'' {
				result.WriteByte(' ')
				i++
				state = stateSingleQuoteString
				depth++
				continue
			}
			if ch == '`' {
				result.WriteByte(' ')
				i++
				state = stateBacktickString
				depth++
				continue
			}

			// Normal code character
			result.WriteByte(ch)
			i++

		case stateSingleLineComment:
			if ch == '\n' {
				result.WriteByte('\n')
				state = stateNormal
				depth--
			} else {
				result.WriteByte(' ')
			}
			i++

		case stateMultiLineComment:
			if s.multiClose != "" && strings.HasPrefix(content[i:], s.multiClose) {
				result.WriteString(strings.Repeat(" ", len(s.multiClose)))
				i += len(s.multiClose)
				state = stateNormal
				depth--
				continue
			}

			// Preserve newlines for line structure
			if ch == '\n' {
				result.WriteByte('\n')
			} else {
				result.WriteByte(' ')
			}
			i++

		case stateDoubleQuoteString:
			if escaped {
				result.WriteByte(' ')
				escaped = false
				i++
				continue
			}

			if ch == '\\' {
				result.WriteByte(' ')
				escaped = true
				i++
				continue
			}

			if ch == '"' {
				result.WriteByte(' ')
				state = stateNormal
				depth--
				i++
				continue
			}

			// Preserve newlines (though uncommon in non-raw strings)
			if ch == '\n' {
				result.WriteByte('\n')
			} else {
				result.WriteByte(' ')
			}
			i++

		case stateSingleQuoteString:
			if escaped {
				result.WriteByte(' ')
				escaped = false
				i++
				continue
			}

			if ch == '\\' {
				result.WriteByte(' ')
				escaped = true
				i++
				continue
			}

			if ch == '\'' {
				result.WriteByte(' ')
				state = stateNormal
				depth--
				i++
				continue
			}

			// Preserve newlines
			if ch == '\n' {
				result.WriteByte('\n')
			} else {
				result.WriteByte(' ')
			}
			i++

		case stateBacktickString:
			// Backquote strings (Go raw strings, JS template literals)
			// No escape sequences in Go raw strings
			// JS template literals can have ${} but we're just stripping
			if ch == '`' {
				result.WriteByte(' ')
				state = stateNormal
				depth--
				i++
				continue
			}

			// Preserve newlines
			if ch == '\n' {
				result.WriteByte('\n')
			} else {
				result.WriteByte(' ')
			}
			i++

		case stateTripleDoubleQuoteString:
			if strings.HasPrefix(content[i:], `"""`) {
				result.WriteString("   ")
				i += 3
				state = stateNormal
				depth--
				continue
			}

			// Preserve newlines
			if ch == '\n' {
				result.WriteByte('\n')
			} else {
				result.WriteByte(' ')
			}
			i++

		case stateTripleSingleQuoteString:
			if strings.HasPrefix(content[i:], "'''") {
				result.WriteString("   ")
				i += 3
				state = stateNormal
				depth--
				continue
			}

			// Preserve newlines
			if ch == '\n' {
				result.WriteByte('\n')
			} else {
				result.WriteByte(' ')
			}
			i++
		}
	}

	return result.String(), nil
}
