package sdk_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/tombee/conductor/sdk"
)

// TestTruncateCode_US1_GoFilePreservation tests US1: Code Review Agent Truncation
// A 2000-line Go file should truncate to 500 lines without cutting mid-function,
// with imports preserved and a truncation indicator showing what was removed.
func TestTruncateCode_US1_GoFilePreservation(t *testing.T) {
	// Generate a large Go file with imports and many functions
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"fmt\"\n")
	sb.WriteString("\t\"log\"\n")
	sb.WriteString(")\n\n")

	// Add 400 functions (5 lines each = 2000 lines)
	for i := 1; i <= 400; i++ {
		sb.WriteString(fmt.Sprintf("func function%d() {\n", i))
		sb.WriteString(fmt.Sprintf("\tfmt.Println(\"function %d\")\n", i))
		sb.WriteString("}\n\n")
	}

	content := sb.String()
	originalLines := strings.Count(content, "\n") + 1

	// Truncate to 500 lines with preservation options
	result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
		MaxLines:     500,
		Language:     "go",
		PreserveTop:  true,
		PreserveFunc: true,
	})

	if err != nil {
		t.Fatalf("TruncateCode failed: %v", err)
	}

	// AC: Can truncate a 2000-line Go file to 500 lines without cutting mid-function
	if !result.WasTruncated {
		t.Error("Expected content to be truncated")
	}

	if result.FinalLines > 500 {
		t.Errorf("Expected at most 500 lines, got %d", result.FinalLines)
	}

	if result.OriginalLines != originalLines {
		t.Errorf("Expected original line count %d, got %d", originalLines, result.OriginalLines)
	}

	// AC: Import section is preserved when PreserveTop is enabled
	if !strings.Contains(result.Content, "import (") {
		t.Error("Expected imports to be preserved")
	}

	// AC: Truncation indicator shows what was removed
	if result.Indicator != "" {
		t.Errorf("Indicator should be empty in result struct, but got: %s", result.Indicator)
	}

	if !strings.Contains(result.Content, "//") && !strings.Contains(result.Content, "omitted") {
		t.Error("Expected truncation indicator in content")
	}

	// Verify no mid-function cuts by checking that all function declarations have closing braces
	lines := strings.Split(result.Content, "\n")
	openBraces := 0
	sawFunc := false
	for _, line := range lines {
		if strings.Contains(line, "func ") {
			sawFunc = true
		}
		openBraces += strings.Count(line, "{")
		openBraces -= strings.Count(line, "}")
	}

	if sawFunc && openBraces != 0 {
		t.Errorf("Expected balanced braces (no mid-function cut), got imbalance: %d", openBraces)
	}

	// AC: Output is parseable - basic check for valid Go structure
	if !strings.HasPrefix(result.Content, "package main") {
		t.Error("Expected output to start with package declaration")
	}
}

// TestTruncateCode_US2_MultiLanguageSupport tests US2: Multi-Language Support
// The function should support TypeScript, Go, Python, and JavaScript with
// language-aware truncation, and fall back gracefully for unknown languages.
func TestTruncateCode_US2_MultiLanguageSupport(t *testing.T) {
	tests := []struct {
		name     string
		language string
		content  string
		wantType string // Expected type in omitted items
	}{
		{
			name:     "TypeScript with interfaces and classes",
			language: "typescript",
			content: `interface User {
	name: string;
	age: number;
}

class UserService {
	getUser(): User {
		return { name: "test", age: 30 };
	}
}

function processUser() {
	console.log("processing");
}`,
			wantType: "function",
		},
		{
			name:     "Go with functions and methods",
			language: "go",
			content: `package main

func helper() {
	fmt.Println("helper")
}

type Service struct{}

func (s Service) Method() {
	fmt.Println("method")
}`,
			wantType: "function",
		},
		{
			name:     "Python with classes and decorators",
			language: "python",
			content: `import sys

@decorator
def decorated_func():
    print("decorated")

class MyClass:
    def method(self):
        print("method")`,
			wantType: "function",
		},
		{
			name:     "JavaScript with functions and classes",
			language: "javascript",
			content: `const helper = () => {
	console.log("helper");
};

class Service {
	method() {
		console.log("method");
	}
}

function process() {
	console.log("process");
}`,
			wantType: "function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sdk.TruncateCode(tt.content, sdk.TruncateOptions{
				MaxLines:     5,
				Language:     tt.language,
				PreserveTop:  true,
				PreserveFunc: true,
			})

			if err != nil {
				t.Fatalf("TruncateCode failed: %v", err)
			}

			// Should truncate and preserve structure
			if !result.WasTruncated {
				t.Error("Expected content to be truncated")
			}

			// Allow 6 lines (5 content + 1 indicator)
			if result.FinalLines > 6 {
				t.Errorf("Expected at most 6 lines (5 + indicator), got %d", result.FinalLines)
			}
		})
	}

	// AC: Falls back to line-based truncation for unsupported languages
	t.Run("Unsupported language fallback", func(t *testing.T) {
		content := strings.Repeat("line\n", 100)
		result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
			MaxLines: 50,
			Language: "ruby", // Unsupported
		})

		if err != nil {
			t.Fatalf("TruncateCode should not error for unsupported language: %v", err)
		}

		if !result.WasTruncated {
			t.Error("Expected content to be truncated")
		}

		if result.FinalLines > 51 { // 50 lines + indicator
			t.Errorf("Expected at most 51 lines (50 + indicator), got %d", result.FinalLines)
		}
	})

	// AC: Language matching is case-insensitive
	t.Run("Case-insensitive language", func(t *testing.T) {
		content := `package main
func test() {
	fmt.Println("test")
}`

		cases := []string{"GO", "Go", "go", "gO"}
		for _, lang := range cases {
			result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
				MaxLines: 2,
				Language: lang,
			})

			if err != nil {
				t.Errorf("TruncateCode failed for language %q: %v", lang, err)
			}

			if !result.WasTruncated {
				t.Errorf("Expected truncation for language %q", lang)
			}
		}
	})
}

// TestTruncateCode_US3_TokenBasedTruncation tests US3: Token-Based Truncation
// The function should support MaxTokens with reasonable accuracy (within 15%).
func TestTruncateCode_US3_TokenBasedTruncation(t *testing.T) {
	// Generate content with known character count
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	for i := 0; i < 100; i++ {
		// Each function is approximately 60 characters
		sb.WriteString(fmt.Sprintf("func f%d() {\n\tfmt.Println(%d)\n}\n\n", i, i))
	}
	content := sb.String()

	// Request specific token limit (chars/4 heuristic)
	maxTokens := 500
	result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
		MaxTokens:    maxTokens,
		Language:     "go",
		PreserveFunc: true,
	})

	if err != nil {
		t.Fatalf("TruncateCode failed: %v", err)
	}

	// AC: Can specify MaxTokens instead of MaxLines
	if !result.WasTruncated {
		t.Error("Expected content to be truncated")
	}

	// AC: Token estimation is reasonably accurate (within 15%)
	if result.EstimatedTokens > maxTokens {
		// Allow small overage for the indicator line (15% tolerance)
		maxAllowedWithOverage := maxTokens + (maxTokens * 15 / 100)
		if result.EstimatedTokens > maxAllowedWithOverage {
			t.Errorf("Token estimate %d exceeds MaxTokens %d by more than 15%%",
				result.EstimatedTokens, maxTokens)
		}
	}

	// Test: When both limits specified, more restrictive one applies
	t.Run("Both limits specified", func(t *testing.T) {
		result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
			MaxLines:     100, // More permissive
			MaxTokens:    200, // More restrictive
			Language:     "go",
			PreserveFunc: true,
		})

		if err != nil {
			t.Fatalf("TruncateCode failed: %v", err)
		}

		// Should respect the more restrictive limit (MaxTokens)
		// Allow 15% overage for indicator line
		maxAllowedTokens := 200 + (200 * 15 / 100)
		if result.EstimatedTokens > maxAllowedTokens {
			t.Errorf("Expected tokens to respect MaxTokens=200 with 15%% overage, got %d", result.EstimatedTokens)
		}
	})
}

// TestTruncateCode_US4_FallbackForUnknownLanguages tests US4: Fallback behavior
// Unknown languages should fall back to line-based truncation without errors.
func TestTruncateCode_US4_FallbackForUnknownLanguages(t *testing.T) {
	content := strings.Repeat("This is a line of text\n", 100)

	// AC: Unknown languages fall back to line-based truncation
	result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
		MaxLines: 50,
		Language: "unknown-language",
	})

	if err != nil {
		t.Errorf("TruncateCode should not error for unknown language: %v", err)
	}

	if !result.WasTruncated {
		t.Error("Expected content to be truncated")
	}

	// AC: Plaintext files truncate at line boundaries (not mid-line)
	lines := strings.Split(result.Content, "\n")
	for i, line := range lines[:len(lines)-1] { // Skip last line (indicator)
		if !strings.HasPrefix(line, "This is a line") && line != "" {
			t.Errorf("Line %d was cut mid-line: %q", i, line)
		}
	}

	// AC: No errors thrown for unsupported language identifiers
	unsupportedLangs := []string{"", "rust", "java", "c++", "ruby", "php"}
	for _, lang := range unsupportedLangs {
		_, err := sdk.TruncateCode(content, sdk.TruncateOptions{
			MaxLines: 10,
			Language: lang,
		})

		if err != nil {
			t.Errorf("TruncateCode should not error for language %q: %v", lang, err)
		}
	}
}

// TestTruncateCode_ConcurrentCalls tests NFR5: Thread Safety
// The function should be safe for concurrent use.
func TestTruncateCode_ConcurrentCalls(t *testing.T) {
	content := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}

func helper() {
	fmt.Println("helper")
}
`

	const numGoroutines = 50
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
				MaxLines:     5,
				Language:     "go",
				PreserveTop:  true,
				PreserveFunc: true,
			})

			if err != nil {
				errors <- fmt.Errorf("goroutine %d: %w", id, err)
				return
			}

			// Verify deterministic output
			if !result.WasTruncated {
				errors <- fmt.Errorf("goroutine %d: expected truncation", id)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors from goroutines
	for err := range errors {
		t.Error(err)
	}
}

// TestTruncateCode_MalformedInput tests error handling for invalid inputs.
func TestTruncateCode_MalformedInput(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		opts        sdk.TruncateOptions
		wantErr     bool
		errContains string
	}{
		{
			name:        "Negative MaxLines",
			content:     "test",
			opts:        sdk.TruncateOptions{MaxLines: -1},
			wantErr:     true,
			errContains: "MaxLines",
		},
		{
			name:        "Negative MaxTokens",
			content:     "test",
			opts:        sdk.TruncateOptions{MaxTokens: -1},
			wantErr:     true,
			errContains: "MaxTokens",
		},
		{
			name:        "Negative MaxBytes",
			content:     "test",
			opts:        sdk.TruncateOptions{MaxBytes: -1},
			wantErr:     true,
			errContains: "MaxBytes",
		},
		{
			name:        "Input exceeds MaxBytes",
			content:     strings.Repeat("x", 1000),
			opts:        sdk.TruncateOptions{MaxBytes: 100},
			wantErr:     true,
			errContains: "INPUT_TOO_LARGE",
		},
		{
			name:    "Empty content",
			content: "",
			opts:    sdk.TruncateOptions{MaxLines: 10},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sdk.TruncateCode(tt.content, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// For empty content, verify empty result
				if tt.content == "" {
					if result.WasTruncated {
						t.Error("Empty content should not be marked as truncated")
					}
					if result.Content != "" {
						t.Errorf("Expected empty content, got: %q", result.Content)
					}
				}
			}
		})
	}
}

// TestTruncateCode_Deterministic verifies that the function is deterministic.
func TestTruncateCode_Deterministic(t *testing.T) {
	content := `package main

func a() { fmt.Println("a") }
func b() { fmt.Println("b") }
func c() { fmt.Println("c") }
`

	opts := sdk.TruncateOptions{
		MaxLines:     3,
		Language:     "go",
		PreserveFunc: true,
	}

	// Run multiple times
	var results []sdk.TruncateResult
	for i := 0; i < 5; i++ {
		result, err := sdk.TruncateCode(content, opts)
		if err != nil {
			t.Fatalf("Run %d failed: %v", i, err)
		}
		results = append(results, result)
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		if results[i].Content != results[0].Content {
			t.Errorf("Run %d produced different content than run 0", i)
		}
		if results[i].FinalLines != results[0].FinalLines {
			t.Errorf("Run %d: FinalLines=%d, expected %d", i, results[i].FinalLines, results[0].FinalLines)
		}
	}
}

// TestTruncateCode_PreserveOptions tests the PreserveTop and PreserveFunc options.
func TestTruncateCode_PreserveOptions(t *testing.T) {
	content := `package main

import (
	"fmt"
	"log"
)

func first() {
	fmt.Println("first")
}

func second() {
	log.Println("second")
}
`

	t.Run("PreserveTop only", func(t *testing.T) {
		result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
			MaxLines:    8,
			Language:    "go",
			PreserveTop: true,
		})

		if err != nil {
			t.Fatalf("TruncateCode failed: %v", err)
		}

		// Should include imports
		if !strings.Contains(result.Content, "import") {
			t.Error("Expected imports to be preserved")
		}
	})

	t.Run("PreserveFunc only", func(t *testing.T) {
		result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
			MaxLines:     8,
			Language:     "go",
			PreserveFunc: true,
		})

		if err != nil {
			t.Fatalf("TruncateCode failed: %v", err)
		}

		// Should include at least one complete function
		if !strings.Contains(result.Content, "func") {
			t.Error("Expected at least one function")
		}
	})

	t.Run("Both preserve options", func(t *testing.T) {
		result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
			MaxLines:     15,
			Language:     "go",
			PreserveTop:  true,
			PreserveFunc: true,
		})

		if err != nil {
			t.Fatalf("TruncateCode failed: %v", err)
		}

		// Should include imports and complete functions
		if !strings.Contains(result.Content, "import") {
			t.Error("Expected imports to be preserved")
		}

		if !strings.Contains(result.Content, "func first") {
			t.Error("Expected first function to be preserved")
		}
	})

	t.Run("Neither preserve option", func(t *testing.T) {
		result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
			MaxLines: 5,
			Language: "go",
		})

		if err != nil {
			t.Fatalf("TruncateCode failed: %v", err)
		}

		// Just simple line-based truncation
		if result.FinalLines > 6 { // 5 + indicator
			t.Errorf("Expected at most 6 lines, got %d", result.FinalLines)
		}
	})
}

// TestTruncateCode_NoTruncationNeeded tests that content within limits is returned unchanged.
func TestTruncateCode_NoTruncationNeeded(t *testing.T) {
	content := `package main

func small() {
	fmt.Println("small")
}
`

	result, err := sdk.TruncateCode(content, sdk.TruncateOptions{
		MaxLines: 100,
		Language: "go",
	})

	if err != nil {
		t.Fatalf("TruncateCode failed: %v", err)
	}

	if result.WasTruncated {
		t.Error("Content should not be truncated when within limits")
	}

	if result.Content != content {
		t.Error("Content should be unchanged when within limits")
	}

	if len(result.OmittedItems) > 0 {
		t.Error("No items should be omitted when within limits")
	}
}
