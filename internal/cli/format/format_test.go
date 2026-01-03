package format

import (
	"strings"
	"testing"
)

func TestFormatMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		isTTY    bool
		wantErr  bool
		contains string // String that should be in output
	}{
		{
			name:     "simple markdown with TTY",
			content:  "# Heading\n\nSome text",
			isTTY:    true,
			wantErr:  false,
			contains: "Heading",
		},
		{
			name:     "simple markdown without TTY",
			content:  "# Heading\n\nSome text",
			isTTY:    false,
			wantErr:  false,
			contains: "# Heading",
		},
		{
			name:    "empty markdown",
			content: "",
			isTTY:   true,
			wantErr: false,
		},
		{
			name:     "markdown with lists",
			content:  "- Item 1\n- Item 2",
			isTTY:    false,
			wantErr:  false,
			contains: "Item 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatMarkdown(tt.content, tt.isTTY)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatMarkdown() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("FormatMarkdown() output should contain %q, got %q", tt.contains, got)
			}
		})
	}
}

func TestFormatJSON(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		isTTY    bool
		wantErr  bool
		contains string
	}{
		{
			name:     "valid JSON object",
			content:  `{"key":"value"}`,
			isTTY:    true,
			wantErr:  false,
			contains: "\"key\": \"value\"",
		},
		{
			name:    "invalid JSON",
			content: `{invalid}`,
			isTTY:   true,
			wantErr: true,
		},
		{
			name:     "valid JSON array",
			content:  `["a","b","c"]`,
			isTTY:    false,
			wantErr:  false,
			contains: "\"a\"",
		},
		{
			name:     "nested JSON",
			content:  `{"outer":{"inner":"value"}}`,
			isTTY:    true,
			wantErr:  false,
			contains: "\"outer\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatJSON(tt.content, tt.isTTY)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("FormatJSON() output should contain %q, got %q", tt.contains, got)
			}
		})
	}
}

func TestFormatCode(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		format   string
		isTTY    bool
		wantErr  bool
		contains string
	}{
		{
			name:     "code without language with TTY",
			content:  "print('hello')",
			format:   "code",
			isTTY:    true,
			wantErr:  false,
			contains: "print('hello')",
		},
		{
			name:     "code with python language and TTY",
			content:  "def foo():\n    return 42",
			format:   "code:python",
			isTTY:    true,
			wantErr:  false,
			contains: "def foo()",
		},
		{
			name:     "code with unknown language",
			content:  "some code",
			format:   "code:unknownlang",
			isTTY:    true,
			wantErr:  false,
			contains: "some code",
		},
		{
			name:     "code without TTY",
			content:  "console.log('hi')",
			format:   "code:javascript",
			isTTY:    false,
			wantErr:  false,
			contains: "console.log('hi')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatCode(tt.content, tt.format, tt.isTTY)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("FormatCode() output should contain %q, got %q", tt.contains, got)
			}
		})
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name    string
		content string
		isTTY   bool
		wantErr bool
		want    string
	}{
		{
			name:    "integer",
			content: "123",
			isTTY:   true,
			wantErr: false,
			want:    "123",
		},
		{
			name:    "float",
			content: "123.45",
			isTTY:   false,
			wantErr: false,
			want:    "123.45",
		},
		{
			name:    "scientific notation",
			content: "1.5e10",
			isTTY:   true,
			wantErr: false,
			want:    "1.5e10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatNumber(tt.content, tt.isTTY)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FormatNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatString(t *testing.T) {
	tests := []struct {
		name    string
		content string
		isTTY   bool
		wantErr bool
		want    string
	}{
		{
			name:    "simple string",
			content: "hello world",
			isTTY:   true,
			wantErr: false,
			want:    "hello world",
		},
		{
			name:    "empty string",
			content: "",
			isTTY:   false,
			wantErr: false,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatString(tt.content, tt.isTTY)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FormatString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		format   string
		isTTY    bool
		wantErr  bool
		contains string
	}{
		{
			name:     "markdown format",
			content:  "# Title",
			format:   "markdown",
			isTTY:    false,
			wantErr:  false,
			contains: "# Title",
		},
		{
			name:     "json format",
			content:  `{"key":"value"}`,
			format:   "json",
			isTTY:    true,
			wantErr:  false,
			contains: "\"key\"",
		},
		{
			name:     "code format with language",
			content:  "print('hi')",
			format:   "code:python",
			isTTY:    false,
			wantErr:  false,
			contains: "print('hi')",
		},
		{
			name:    "unknown format",
			content: "content",
			format:  "unknown",
			isTTY:   true,
			wantErr: true,
		},
		{
			name:     "empty format defaults to string",
			content:  "text",
			format:   "",
			isTTY:    true,
			wantErr:  false,
			contains: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Format(tt.content, tt.format, tt.isTTY)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("Format() output should contain %q, got %q", tt.contains, got)
			}
		})
	}
}

func TestSanitizeANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no ANSI codes",
			input: "plain text",
			want:  "plain text",
		},
		{
			name:  "with ANSI color codes",
			input: "\x1b[31mred text\x1b[0m",
			want:  "red text",
		},
		{
			name:  "with multiple ANSI codes",
			input: "\x1b[1m\x1b[32mbold green\x1b[0m\x1b[0m",
			want:  "bold green",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeANSI(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeANSI() = %q, want %q", got, tt.want)
			}
		})
	}
}
