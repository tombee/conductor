package sdk

// TruncateOptions configures code truncation behavior.
type TruncateOptions struct {
	// MaxLines is the maximum number of lines in the output.
	// If 0, no line limit is applied.
	MaxLines int

	// MaxTokens is the maximum estimated token count.
	// Uses chars/4 heuristic. If 0, no token limit is applied.
	MaxTokens int

	// MaxBytes is the maximum input size in bytes.
	// If 0, defaults to 10MB. Inputs exceeding this are rejected.
	MaxBytes int

	// Language specifies the programming language for structure-aware truncation.
	// Supported: "go", "typescript", "python", "javascript".
	// Required for structure-aware truncation; empty string uses line-based fallback.
	Language string

	// PreserveTop keeps import statements and file headers when true.
	PreserveTop bool

	// PreserveFunc avoids cutting in the middle of functions when true.
	// Functions are kept from the beginning; omitted from the end.
	PreserveFunc bool
}

// TruncateResult contains the truncation output and metadata.
type TruncateResult struct {
	// Content is the truncated code.
	Content string

	// WasTruncated indicates whether any content was removed.
	WasTruncated bool

	// OriginalLines is the line count of the input.
	OriginalLines int

	// FinalLines is the line count of the output.
	FinalLines int

	// EstimatedTokens is the estimated token count of the output (chars/4).
	EstimatedTokens int

	// OmittedItems lists the code blocks that were removed.
	OmittedItems []OmittedItem

	// Indicator is the truncation comment added to the output.
	Indicator string
}

// OmittedItem describes a code block that was removed during truncation.
type OmittedItem struct {
	// Type is the kind of block: "function", "method", "class", "interface", "type", "const", "var", "block".
	Type string

	// Name is the identifier of the omitted block.
	Name string

	// StartLine is the original line number where the block started.
	StartLine int

	// EndLine is the original line number where the block ended.
	EndLine int
}

// DefaultMaxBytes is the default maximum input size (10MB).
const DefaultMaxBytes = 10 * 1024 * 1024

// DefaultMaxNestingDepth is the maximum bracket nesting depth for parsing.
const DefaultMaxNestingDepth = 1000
