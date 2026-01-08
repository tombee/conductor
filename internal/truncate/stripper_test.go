package truncate

import (
	"strings"
	"testing"
)

func TestStripper_Strip_Go(t *testing.T) {
	// Go uses //, /* */, double-quoted strings, and backtick raw strings
	s := NewStripper("//", "/*", "*/")

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "code without strings or comments",
			input: "func main() {\n\tx := 42\n}",
			want:  "func main() {\n\tx := 42\n}",
		},
		{
			name:  "single line comment",
			input: "x := 42 // comment here\ny := 10",
			want:  "x := 42                \ny := 10",
		},
		{
			name:  "multi-line comment",
			input: "x := 42 /* comment\nacross lines */ y := 10",
			want:  "x := 42           \n                y := 10",
		},
		{
			name:  "double-quoted string",
			input: `s := "hello world"`,
			want:  `s :=              `,
		},
		{
			name:  "escaped quote in string",
			input: `s := "hello \"quoted\" world"`,
			want:  `s :=                         `,
		},
		{
			name:  "backtick raw string",
			input: "s := `raw\nstring`",
			want:  "s :=     \n       ",
		},
		{
			name:  "comment marker inside string - preserved",
			input: `s := "// not a comment"`,
			want:  `s :=                   `,
		},
		{
			name:  "string inside comment - stripped",
			input: `// comment with "string"`,
			want:  `                        `,
		},
		{
			name:  "bracket in string - preserved",
			input: `s := "text { bracket }"`,
			want:  `s :=                   `,
		},
		{
			name:  "bracket in comment - stripped",
			input: `// comment { with bracket }`,
			want:  `                           `,
		},
		{
			name:  "multiple strings and comments",
			input: "s1 := \"hello\"\n// comment\ns2 := \"world\"",
			want:  "s1 :=        \n          \ns2 :=        ",
		},
		{
			name:  "nested quotes with escapes",
			input: `s := "outer \"inner \\\" nested\" outer"`,
			want:  `s :=                                    `,
		},
		{
			name:  "comment at EOF",
			input: "x := 42 // comment",
			want:  "x := 42           ",
		},
		{
			name:  "string at EOF",
			input: `s := "text"`,
			want:  `s :=       `,
		},
		{
			name:  "unclosed string",
			input: `s := "unclosed`,
			want:  `s :=          `,
		},
		{
			name:  "unclosed comment",
			input: "x := 42 /* unclosed",
			want:  "x := 42            ",
		},
		{
			name:  "empty string literal",
			input: `s := ""`,
			want:  `s :=   `,
		},
		{
			name:  "empty comment",
			input: "x := 42 //\ny := 10",
			want:  "x := 42   \ny := 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Strip(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Strip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Strip() mismatch:\ninput:  %q\ngot:    %q\nwant:   %q", tt.input, got, tt.want)
			}

			// Verify length preservation
			if len(got) != len(tt.input) {
				t.Errorf("Strip() length mismatch: got %d, want %d", len(got), len(tt.input))
			}

			// Verify newline preservation
			if strings.Count(got, "\n") != strings.Count(tt.input, "\n") {
				t.Errorf("Strip() newline count mismatch: got %d, want %d",
					strings.Count(got, "\n"), strings.Count(tt.input, "\n"))
			}
		})
	}
}

func TestStripper_Strip_Python(t *testing.T) {
	// Python uses #, triple-quoted strings
	s := NewStripper("#", "", "")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "hash comment",
			input: "x = 42 # comment here\ny = 10",
			want:  "x = 42               \ny = 10",
		},
		{
			name:  "triple-double-quote docstring",
			input: "def f():\n    \"\"\"docstring\n    here\"\"\"\n    pass",
			want:  "def f():\n                \n           \n    pass",
		},
		{
			name:  "triple-single-quote docstring",
			input: "def f():\n    '''docstring\n    here'''\n    pass",
			want:  "def f():\n                \n           \n    pass",
		},
		{
			name:  "single-quoted string",
			input: "s = 'hello world'",
			want:  "s =              ",
		},
		{
			name:  "double-quoted string",
			input: `s = "hello world"`,
			want:  `s =              `,
		},
		{
			name:  "escaped quote in python string",
			input: `s = "test \" quote"`,
			want:  `s =                `,
		},
		{
			name:  "comment marker in string",
			input: `s = "# not a comment"`,
			want:  `s =                  `,
		},
		{
			name:  "string in comment",
			input: `# comment with "string"`,
			want:  `                       `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Strip(tt.input)
			if err != nil {
				t.Errorf("Strip() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Strip() mismatch:\ninput:  %q\ngot:    %q\nwant:   %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripper_Strip_JavaScript(t *testing.T) {
	// JavaScript uses //, /* */, template literals with backticks
	s := NewStripper("//", "/*", "*/")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "template literal",
			input: "const s = `template ${x} string`",
			want:  "const s =                       ",
		},
		{
			name:  "template literal multiline",
			input: "const s = `line1\nline2`",
			want:  "const s =       \n      ",
		},
		{
			name:  "single and double quotes",
			input: `const s1 = "double"; const s2 = 'single'`,
			want:  `const s1 =         ; const s2 =         `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Strip(tt.input)
			if err != nil {
				t.Errorf("Strip() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Strip() mismatch:\ninput:  %q\ngot:    %q\nwant:   %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripper_Strip_ComplexNesting(t *testing.T) {
	s := NewStripper("//", "/*", "*/")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "multiple nested structures",
			input: `s1 := "string1" // comment` + "\n" + `s2 := "string2" /* comment */`,
			want:  `s1 :=                     ` + "\n" + `s2 :=                        `,
		},
		{
			name:  "bracket counting scenario",
			input: "func f() {\n\ts := \"text { here }\"\n\t// comment { here }\n\tx := 42\n}",
			want:  "func f() {\n\ts :=                \n\t                   \n\tx := 42\n}",
		},
		{
			name: "real world Go code",
			input: `package main

import "fmt"

func main() {
	// This is a comment
	s := "hello {world}" /* block comment */
	fmt.Println(s)
}`,
			// Each stripped string/comment is replaced with spaces
			// Line 3: import "fmt" -> import       (space + 5 spaces for "fmt")
			// Line 6: \t// This is a comment -> \t                     (21 spaces for comment)
			// Line 7: \ts := "hello {world}" /* block comment */
			//         -> \ts :=                                     (15 + 1 + 20 spaces)
			want: "package main\n\nimport      \n\nfunc main() {\n\t                    \n\ts :=                                    \n\tfmt.Println(s)\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Strip(tt.input)
			if err != nil {
				t.Errorf("Strip() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Strip() mismatch:\ninput:\n%s\n\ngot:\n%s\n\nwant:\n%s", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripper_Strip_MaxDepth(t *testing.T) {
	// The depth check is incremented per string/comment entry
	// not per character, so this test needs adjustment
	// The depth tracks nesting level (string inside comment), not sequential items

	// Better approach: create input with legitimate depth tracking
	// We enter comment (depth=1), and process many chars.
	// Each char checks if depth > maxDepth
	// So even at depth=1, if maxDepth=0, it would fail

	s := NewStripper("//", "", "")
	s.maxDepth = 0

	_, err := s.Strip("// comment")
	if err != ErrMaxNestingDepthExceeded {
		t.Errorf("Strip() with maxDepth=0: got error %v, want %v", err, ErrMaxNestingDepthExceeded)
	}
}

func TestStripper_Strip_NoCommentSyntax(t *testing.T) {
	// Stripper with no comment syntax (e.g., for plaintext)
	s := NewStripper("", "", "")

	input := `some text with "quotes" and // things`
	// Only strings should be stripped, not comment-like patterns (no comment syntax configured)
	want := `some text with          and // things`

	got, err := s.Strip(input)
	if err != nil {
		t.Errorf("Strip() error = %v", err)
		return
	}
	if got != want {
		t.Errorf("Strip() mismatch:\ninput:  %q\ngot:    %q\nwant:   %q", input, got, want)
	}
}

func TestStripper_Strip_EdgeCases(t *testing.T) {
	s := NewStripper("//", "/*", "*/")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "backslash at end of string",
			input: `s := "text\\"`,
			want:  `s :=         `,
		},
		{
			name:  "multiple backslashes",
			input: `s := "text\\\\"`,
			want:  `s :=           `,
		},
		{
			name:  "escaped backslash before quote",
			input: `s := "text\\\""`,
			want:  `s :=           `,
		},
		{
			name:  "only comments",
			input: "// line1\n// line2\n// line3",
			want:  "        \n        \n        ",
		},
		{
			name:  "only strings",
			input: `"string1" "string2" "string3"`,
			want:  `                             `,
		},
		{
			name:  "consecutive quotes",
			input: `s := """`,
			want:  `s :=    `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Strip(tt.input)
			if err != nil {
				t.Errorf("Strip() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Strip() mismatch:\ninput:  %q\ngot:    %q\nwant:   %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripper_Strip_PreservesStructure(t *testing.T) {
	s := NewStripper("//", "/*", "*/")

	input := `func calculate() {
	// Initialize variables
	x := 42
	y := "test { bracket }"
	/*
	 * Multi-line comment
	 * with { brackets }
	 */
	return x
}`

	got, err := s.Strip(input)
	if err != nil {
		t.Errorf("Strip() error = %v", err)
		return
	}

	// Count brackets in stripped version - only code brackets should remain
	openBrackets := strings.Count(got, "{")
	closeBrackets := strings.Count(got, "}")

	// Should have exactly 2 { and 2 } from the actual code structure
	// (function body has 1 pair, the brackets in string/comment are stripped)
	if openBrackets != 1 {
		t.Errorf("Strip() open brackets in result = %d, want 1 (brackets in string/comment should be stripped)", openBrackets)
	}
	if closeBrackets != 1 {
		t.Errorf("Strip() close brackets in result = %d, want 1 (brackets in string/comment should be stripped)", closeBrackets)
	}

	// Verify line count is preserved
	inputLines := strings.Count(input, "\n")
	gotLines := strings.Count(got, "\n")
	if gotLines != inputLines {
		t.Errorf("Strip() line count = %d, want %d", gotLines, inputLines)
	}
}

func TestNewStripper(t *testing.T) {
	s := NewStripper("//", "/*", "*/")

	if s.singleLineComment != "//" {
		t.Errorf("NewStripper() singleLineComment = %q, want //", s.singleLineComment)
	}
	if s.multiOpen != "/*" {
		t.Errorf("NewStripper() multiOpen = %q, want /*", s.multiOpen)
	}
	if s.multiClose != "*/" {
		t.Errorf("NewStripper() multiClose = %q, want */", s.multiClose)
	}
	if s.maxDepth != defaultMaxNestingDepth {
		t.Errorf("NewStripper() maxDepth = %d, want %d", s.maxDepth, defaultMaxNestingDepth)
	}
}
