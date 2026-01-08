package truncate

import (
	"testing"
)

func TestFallbackLanguage_DetectImportEnd(t *testing.T) {
	fb := FallbackLanguage{}

	tests := []struct {
		name  string
		lines []string
		want  int
	}{
		{
			name:  "empty file",
			lines: []string{},
			want:  0,
		},
		{
			name:  "single line",
			lines: []string{"line1"},
			want:  0,
		},
		{
			name: "multiple lines",
			lines: []string{
				"import something",
				"",
				"code here",
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fb.DetectImportEnd(tt.lines)
			if got != tt.want {
				t.Errorf("DetectImportEnd() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFallbackLanguage_DetectBlocks(t *testing.T) {
	fb := FallbackLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name:    "empty content",
			content: "",
			want:    []Block{},
		},
		{
			name:    "single line",
			content: "single line",
			want: []Block{
				{
					Type:      "block",
					Name:      "",
					StartLine: 0,
					EndLine:   0,
				},
			},
		},
		{
			name:    "multiple lines",
			content: "line1\nline2\nline3",
			want: []Block{
				{
					Type:      "block",
					Name:      "",
					StartLine: 0,
					EndLine:   2,
				},
			},
		},
		{
			name:    "content with trailing newline",
			content: "line1\nline2\n",
			want: []Block{
				{
					Type:      "block",
					Name:      "",
					StartLine: 0,
					EndLine:   2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fb.DetectBlocks(tt.content)

			if len(got) != len(tt.want) {
				t.Errorf("DetectBlocks() returned %d blocks, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i].Type != tt.want[i].Type {
					t.Errorf("block[%d].Type = %q, want %q", i, got[i].Type, tt.want[i].Type)
				}
				if got[i].Name != tt.want[i].Name {
					t.Errorf("block[%d].Name = %q, want %q", i, got[i].Name, tt.want[i].Name)
				}
				if got[i].StartLine != tt.want[i].StartLine {
					t.Errorf("block[%d].StartLine = %d, want %d", i, got[i].StartLine, tt.want[i].StartLine)
				}
				if got[i].EndLine != tt.want[i].EndLine {
					t.Errorf("block[%d].EndLine = %d, want %d", i, got[i].EndLine, tt.want[i].EndLine)
				}
			}
		})
	}
}

func TestFallbackLanguage_CommentSyntax(t *testing.T) {
	fb := FallbackLanguage{}

	single, multiOpen, multiClose := fb.CommentSyntax()

	if single != "" {
		t.Errorf("CommentSyntax() single = %q, want empty string", single)
	}
	if multiOpen != "" {
		t.Errorf("CommentSyntax() multiOpen = %q, want empty string", multiOpen)
	}
	if multiClose != "" {
		t.Errorf("CommentSyntax() multiClose = %q, want empty string", multiClose)
	}
}

func TestFallbackLanguage_ImplementsInterface(t *testing.T) {
	// Compile-time check that FallbackLanguage implements Language
	var _ Language = FallbackLanguage{}
}
