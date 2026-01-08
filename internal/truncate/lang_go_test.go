package truncate

import (
	"testing"
)

func TestGoLanguage_CommentSyntax(t *testing.T) {
	g := GoLanguage{}

	single, multiOpen, multiClose := g.CommentSyntax()

	if single != "//" {
		t.Errorf("CommentSyntax() single = %q, want %q", single, "//")
	}
	if multiOpen != "/*" {
		t.Errorf("CommentSyntax() multiOpen = %q, want %q", multiOpen, "/*")
	}
	if multiClose != "*/" {
		t.Errorf("CommentSyntax() multiClose = %q, want %q", multiClose, "*/")
	}
}

func TestGoLanguage_DetectImportEnd(t *testing.T) {
	g := GoLanguage{}

	tests := []struct {
		name  string
		lines []string
		want  int
	}{
		{
			name:  "no imports",
			lines: []string{"package main", "", "func main() {}"},
			want:  0,
		},
		{
			name: "single import",
			lines: []string{
				"package main",
				"",
				"import \"fmt\"",
				"",
				"func main() {}",
			},
			want: 4,
		},
		{
			name: "multiple single imports",
			lines: []string{
				"package main",
				"",
				"import \"fmt\"",
				"import \"os\"",
				"import \"strings\"",
				"",
				"func main() {}",
			},
			want: 6,
		},
		{
			name: "grouped import block",
			lines: []string{
				"package main",
				"",
				"import (",
				"\t\"fmt\"",
				"\t\"os\"",
				"\t\"strings\"",
				")",
				"",
				"func main() {}",
			},
			want: 8,
		},
		{
			name: "grouped import with comments",
			lines: []string{
				"package main",
				"",
				"import (",
				"\t\"fmt\" // for printing",
				"\t\"os\"",
				"\t// strings package",
				"\t\"strings\"",
				")",
				"",
				"func main() {}",
			},
			want: 9,
		},
		{
			name: "import followed by const",
			lines: []string{
				"package main",
				"",
				"import \"fmt\"",
				"",
				"const Version = \"1.0\"",
				"",
				"func main() {}",
			},
			want: 4,
		},
		{
			name: "import followed by var",
			lines: []string{
				"package main",
				"",
				"import \"fmt\"",
				"",
				"var logger = fmt.Println",
			},
			want: 4,
		},
		{
			name: "import followed by type",
			lines: []string{
				"package main",
				"",
				"import \"fmt\"",
				"",
				"type MyType struct {}",
			},
			want: 4,
		},
		{
			name: "import at EOF",
			lines: []string{
				"package main",
				"",
				"import \"fmt\"",
			},
			want: 3,
		},
		{
			name: "grouped import at EOF",
			lines: []string{
				"package main",
				"",
				"import (",
				"\t\"fmt\"",
				")",
			},
			want: 5,
		},
		{
			name: "imports with blank lines",
			lines: []string{
				"package main",
				"",
				"import (",
				"\t\"fmt\"",
				"",
				"\t\"os\"",
				")",
				"",
				"func main() {}",
			},
			want: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.DetectImportEnd(tt.lines)
			if got != tt.want {
				t.Errorf("DetectImportEnd() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGoLanguage_DetectBlocks_Functions(t *testing.T) {
	g := GoLanguage{}

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
			name: "single function",
			content: `package main

func main() {
	fmt.Println("hello")
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "main",
					StartLine: 2,
					EndLine:   4,
				},
			},
		},
		{
			name: "multiple functions",
			content: `package main

func first() {
	return
}

func second() {
	return
}

func third() {
	return
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "first",
					StartLine: 2,
					EndLine:   4,
				},
				{
					Type:      "function",
					Name:      "second",
					StartLine: 6,
					EndLine:   8,
				},
				{
					Type:      "function",
					Name:      "third",
					StartLine: 10,
					EndLine:   12,
				},
			},
		},
		{
			name: "function with parameters",
			content: `package main

func add(a int, b int) int {
	return a + b
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "add",
					StartLine: 2,
					EndLine:   4,
				},
			},
		},
		{
			name: "function with return type",
			content: `package main

func getName() string {
	return "test"
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "getName",
					StartLine: 2,
					EndLine:   4,
				},
			},
		},
		{
			name: "nested braces in function",
			content: `package main

func process() {
	if true {
		for i := 0; i < 10; i++ {
			fmt.Println(i)
		}
	}
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "process",
					StartLine: 2,
					EndLine:   8,
				},
			},
		},
		{
			name: "function with string containing braces",
			content: `package main

func template() {
	s := "text { with braces }"
	fmt.Println(s)
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "template",
					StartLine: 2,
					EndLine:   5,
				},
			},
		},
		{
			name: "function with comment containing braces",
			content: `package main

func example() {
	// comment { with braces }
	x := 42
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "example",
					StartLine: 2,
					EndLine:   5,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.DetectBlocks(tt.content)

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

func TestGoLanguage_DetectBlocks_Methods(t *testing.T) {
	g := GoLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name: "method with value receiver",
			content: `package main

func (s Server) Start() {
	fmt.Println("starting")
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "Start",
					StartLine: 2,
					EndLine:   4,
				},
			},
		},
		{
			name: "method with pointer receiver",
			content: `package main

func (s *Server) Stop() {
	fmt.Println("stopping")
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "Stop",
					StartLine: 2,
					EndLine:   4,
				},
			},
		},
		{
			name: "multiple methods on same type",
			content: `package main

func (s *Server) Start() {
	return
}

func (s *Server) Stop() {
	return
}

func (s *Server) Restart() {
	s.Stop()
	s.Start()
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "Start",
					StartLine: 2,
					EndLine:   4,
				},
				{
					Type:      "function",
					Name:      "Stop",
					StartLine: 6,
					EndLine:   8,
				},
				{
					Type:      "function",
					Name:      "Restart",
					StartLine: 10,
					EndLine:   13,
				},
			},
		},
		{
			name: "methods and functions mixed",
			content: `package main

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Start() {
	fmt.Println("starting")
}

func main() {
	s := NewServer()
	s.Start()
}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "NewServer",
					StartLine: 2,
					EndLine:   4,
				},
				{
					Type:      "function",
					Name:      "Start",
					StartLine: 6,
					EndLine:   8,
				},
				{
					Type:      "function",
					Name:      "main",
					StartLine: 10,
					EndLine:   13,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.DetectBlocks(tt.content)

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

func TestGoLanguage_DetectBlocks_Types(t *testing.T) {
	g := GoLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name: "simple type alias",
			content: `package main

type MyInt int`,
			want: []Block{
				{
					Type:      "type",
					Name:      "MyInt",
					StartLine: 2,
					EndLine:   2,
				},
			},
		},
		{
			name: "struct type",
			content: `package main

type Server struct {
	Port int
	Host string
}`,
			want: []Block{
				{
					Type:      "type",
					Name:      "Server",
					StartLine: 2,
					EndLine:   5,
				},
			},
		},
		{
			name: "interface type",
			content: `package main

type Handler interface {
	Handle() error
	Stop()
}`,
			want: []Block{
				{
					Type:      "type",
					Name:      "Handler",
					StartLine: 2,
					EndLine:   2,
				},
			},
		},
		{
			name: "multiple types and functions",
			content: `package main

type Config struct {
	Port int
}

func NewConfig() *Config {
	return &Config{Port: 8080}
}

type Server struct {
	config *Config
}

func (s *Server) Start() {
	fmt.Printf("Starting on port %d\n", s.config.Port)
}`,
			want: []Block{
				{
					Type:      "type",
					Name:      "Config",
					StartLine: 2,
					EndLine:   4,
				},
				{
					Type:      "function",
					Name:      "NewConfig",
					StartLine: 6,
					EndLine:   8,
				},
				{
					Type:      "type",
					Name:      "Server",
					StartLine: 10,
					EndLine:   12,
				},
				{
					Type:      "function",
					Name:      "Start",
					StartLine: 14,
					EndLine:   16,
				},
			},
		},
		{
			name: "nested struct",
			content: `package main

type Outer struct {
	Inner struct {
		Value int
	}
	Name string
}`,
			want: []Block{
				{
					Type:      "type",
					Name:      "Outer",
					StartLine: 2,
					EndLine:   7,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.DetectBlocks(tt.content)

			if len(got) != len(tt.want) {
				t.Errorf("DetectBlocks() returned %d blocks, want %d", len(got), len(tt.want))
				for i, b := range got {
					t.Logf("  got[%d]: %+v", i, b)
				}
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

func TestGoLanguage_DetectBlocks_RealWorldCode(t *testing.T) {
	g := GoLanguage{}

	content := `package main

import (
	"fmt"
	"os"
)

const Version = "1.0.0"

type Config struct {
	Host string
	Port int
}

func NewConfig() *Config {
	return &Config{
		Host: "localhost",
		Port: 8080,
	}
}

type Server struct {
	config *Config
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	fmt.Printf("Server starting on %s\n", addr)
	return nil
}

func (s *Server) Stop() error {
	fmt.Println("Server stopping")
	return nil
}

func main() {
	config := NewConfig()
	server := &Server{config: config}
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}`

	want := []Block{
		{
			Type:      "type",
			Name:      "Config",
			StartLine: 9,
			EndLine:   12,
		},
		{
			Type:      "function",
			Name:      "NewConfig",
			StartLine: 14,
			EndLine:   19,
		},
		{
			Type:      "type",
			Name:      "Server",
			StartLine: 21,
			EndLine:   23,
		},
		{
			Type:      "function",
			Name:      "Start",
			StartLine: 25,
			EndLine:   29,
		},
		{
			Type:      "function",
			Name:      "Stop",
			StartLine: 31,
			EndLine:   34,
		},
		{
			Type:      "function",
			Name:      "main",
			StartLine: 36,
			EndLine:   43,
		},
	}

	got := g.DetectBlocks(content)

	if len(got) != len(want) {
		t.Errorf("DetectBlocks() returned %d blocks, want %d", len(got), len(want))
		for i, b := range got {
			t.Logf("  got[%d]: %+v", i, b)
		}
		return
	}

	for i := range got {
		if got[i].Type != want[i].Type {
			t.Errorf("block[%d].Type = %q, want %q", i, got[i].Type, want[i].Type)
		}
		if got[i].Name != want[i].Name {
			t.Errorf("block[%d].Name = %q, want %q", i, got[i].Name, want[i].Name)
		}
		if got[i].StartLine != want[i].StartLine {
			t.Errorf("block[%d].StartLine = %d, want %d", i, got[i].StartLine, want[i].StartLine)
		}
		if got[i].EndLine != want[i].EndLine {
			t.Errorf("block[%d].EndLine = %d, want %d", i, got[i].EndLine, want[i].EndLine)
		}
	}
}

func TestGoLanguage_ImplementsInterface(t *testing.T) {
	// Compile-time check that GoLanguage implements Language
	var _ Language = GoLanguage{}
}

func TestGoLanguage_ExtractFunctionName(t *testing.T) {
	g := GoLanguage{}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple function",
			input: "func main() {",
			want:  "main",
		},
		{
			name:  "function with parameters",
			input: "func add(a int, b int) int {",
			want:  "add",
		},
		{
			name:  "method with value receiver",
			input: "func (s Server) Start() {",
			want:  "Start",
		},
		{
			name:  "method with pointer receiver",
			input: "func (s *Server) Stop() error {",
			want:  "Stop",
		},
		{
			name:  "method with complex receiver",
			input: "func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {",
			want:  "ServeHTTP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.extractFunctionName(tt.input)
			if got != tt.want {
				t.Errorf("extractFunctionName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGoLanguage_ExtractTypeName(t *testing.T) {
	g := GoLanguage{}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple type alias",
			input: "type MyInt int",
			want:  "MyInt",
		},
		{
			name:  "struct type",
			input: "type Server struct {",
			want:  "Server",
		},
		{
			name:  "interface type",
			input: "type Handler interface {",
			want:  "Handler",
		},
		{
			name:  "type alias with equals",
			input: "type StringAlias = string",
			want:  "StringAlias",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.extractTypeName(tt.input)
			if got != tt.want {
				t.Errorf("extractTypeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
