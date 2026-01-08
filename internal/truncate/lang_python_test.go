package truncate

import (
	"testing"
)

func TestPythonLanguage_CommentSyntax(t *testing.T) {
	p := PythonLanguage{}

	single, multiOpen, multiClose := p.CommentSyntax()

	if single != "#" {
		t.Errorf("CommentSyntax() single = %q, want %q", single, "#")
	}
	if multiOpen != `"""` {
		t.Errorf("CommentSyntax() multiOpen = %q, want %q", multiOpen, `"""`)
	}
	if multiClose != `"""` {
		t.Errorf("CommentSyntax() multiClose = %q, want %q", multiClose, `"""`)
	}
}

func TestPythonLanguage_DetectImportEnd(t *testing.T) {
	p := PythonLanguage{}

	tests := []struct {
		name  string
		lines []string
		want  int
	}{
		{
			name:  "no imports",
			lines: []string{"# comment", "", "def main():", "    pass"},
			want:  0,
		},
		{
			name: "single import",
			lines: []string{
				"import os",
				"",
				"def main():",
				"    pass",
			},
			want: 2,
		},
		{
			name: "multiple imports",
			lines: []string{
				"import os",
				"import sys",
				"import json",
				"",
				"def main():",
				"    pass",
			},
			want: 4,
		},
		{
			name: "from imports",
			lines: []string{
				"from os import path",
				"from typing import Dict, List",
				"",
				"def main():",
				"    pass",
			},
			want: 3,
		},
		{
			name: "mixed import and from",
			lines: []string{
				"import os",
				"from typing import Dict",
				"import sys",
				"",
				"class Handler:",
				"    pass",
			},
			want: 4,
		},
		{
			name: "multiline import with backslash",
			lines: []string{
				"from package import \\",
				"    module1, \\",
				"    module2",
				"",
				"def main():",
				"    pass",
			},
			want: 4,
		},
		{
			name: "multiline import with parentheses",
			lines: []string{
				"from package import (",
				"    module1,",
				"    module2,",
				")",
				"",
				"def main():",
				"    pass",
			},
			want: 5,
		},
		{
			name: "import with comment after",
			lines: []string{
				"import os",
				"import sys  # system stuff",
				"# This is a comment",
				"",
				"def main():",
				"    pass",
			},
			want: 4,
		},
		{
			name: "import at EOF",
			lines: []string{
				"import os",
				"import sys",
			},
			want: 2,
		},
		{
			name: "docstring before import",
			lines: []string{
				`"""Module docstring"""`,
				"",
				"import os",
				"",
				"def main():",
				"    pass",
			},
			want: 4,
		},
		{
			name: "comment before import",
			lines: []string{
				"# File header",
				"# Copyright notice",
				"",
				"import os",
				"from typing import List",
				"",
				"def main():",
				"    pass",
			},
			want: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.DetectImportEnd(tt.lines)
			if got != tt.want {
				t.Errorf("DetectImportEnd() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPythonLanguage_DetectBlocks_Functions(t *testing.T) {
	p := PythonLanguage{}

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
			content: `def main():
    print("hello")
    return`,
			want: []Block{
				{
					Type:      "function",
					Name:      "main",
					StartLine: 0,
					EndLine:   2,
				},
			},
		},
		{
			name: "multiple functions",
			content: `def first():
    return 1

def second():
    return 2

def third():
    return 3`,
			want: []Block{
				{
					Type:      "function",
					Name:      "first",
					StartLine: 0,
					EndLine:   1,
				},
				{
					Type:      "function",
					Name:      "second",
					StartLine: 3,
					EndLine:   4,
				},
				{
					Type:      "function",
					Name:      "third",
					StartLine: 6,
					EndLine:   7,
				},
			},
		},
		{
			name: "function with parameters",
			content: `def add(a, b):
    return a + b`,
			want: []Block{
				{
					Type:      "function",
					Name:      "add",
					StartLine: 0,
					EndLine:   1,
				},
			},
		},
		{
			name: "async function",
			content: `async def fetch_data():
    await some_call()
    return data`,
			want: []Block{
				{
					Type:      "function",
					Name:      "fetch_data",
					StartLine: 0,
					EndLine:   2,
				},
			},
		},
		{
			name: "decorated function",
			content: `@decorator
def process():
    return "processed"`,
			want: []Block{
				{
					Type:      "function",
					Name:      "process",
					StartLine: 0,
					EndLine:   2,
				},
			},
		},
		{
			name: "multiple decorators",
			content: `@decorator1
@decorator2
@decorator3
def complex_function():
    return "result"`,
			want: []Block{
				{
					Type:      "function",
					Name:      "complex_function",
					StartLine: 0,
					EndLine:   4,
				},
			},
		},
		{
			name: "decorated async function",
			content: `@app.route('/api')
async def handler():
    return {"status": "ok"}`,
			want: []Block{
				{
					Type:      "function",
					Name:      "handler",
					StartLine: 0,
					EndLine:   2,
				},
			},
		},
		{
			name: "nested function",
			content: `def outer():
    def inner():
        return 1
    return inner()`,
			want: []Block{
				{
					Type:      "function",
					Name:      "outer",
					StartLine: 0,
					EndLine:   3,
				},
			},
		},
		{
			name: "function with nested structures",
			content: `def process():
    if True:
        for i in range(10):
            print(i)
    return`,
			want: []Block{
				{
					Type:      "function",
					Name:      "process",
					StartLine: 0,
					EndLine:   4,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.DetectBlocks(tt.content)

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

func TestPythonLanguage_DetectBlocks_Classes(t *testing.T) {
	p := PythonLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name: "simple class",
			content: `class MyClass:
    pass`,
			want: []Block{
				{
					Type:      "class",
					Name:      "MyClass",
					StartLine: 0,
					EndLine:   1,
				},
			},
		},
		{
			name: "class with methods",
			content: `class Server:
    def __init__(self):
        self.port = 8080

    def start(self):
        print("starting")`,
			want: []Block{
				{
					Type:      "class",
					Name:      "Server",
					StartLine: 0,
					EndLine:   5,
				},
			},
		},
		{
			name: "class with inheritance",
			content: `class Child(Parent):
    def method(self):
        return "child"`,
			want: []Block{
				{
					Type:      "class",
					Name:      "Child",
					StartLine: 0,
					EndLine:   2,
				},
			},
		},
		{
			name: "class with multiple inheritance",
			content: `class Multi(Base1, Base2, Base3):
    pass`,
			want: []Block{
				{
					Type:      "class",
					Name:      "Multi",
					StartLine: 0,
					EndLine:   1,
				},
			},
		},
		{
			name: "decorated class",
			content: `@dataclass
class Config:
    host: str
    port: int`,
			want: []Block{
				{
					Type:      "class",
					Name:      "Config",
					StartLine: 0,
					EndLine:   3,
				},
			},
		},
		{
			name: "nested class",
			content: `class Outer:
    class Inner:
        pass

    def method(self):
        return`,
			want: []Block{
				{
					Type:      "class",
					Name:      "Outer",
					StartLine: 0,
					EndLine:   5,
				},
			},
		},
		{
			name: "multiple classes",
			content: `class First:
    pass

class Second:
    pass

class Third:
    pass`,
			want: []Block{
				{
					Type:      "class",
					Name:      "First",
					StartLine: 0,
					EndLine:   1,
				},
				{
					Type:      "class",
					Name:      "Second",
					StartLine: 3,
					EndLine:   4,
				},
				{
					Type:      "class",
					Name:      "Third",
					StartLine: 6,
					EndLine:   7,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.DetectBlocks(tt.content)

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

func TestPythonLanguage_DetectBlocks_Mixed(t *testing.T) {
	p := PythonLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name: "class and functions",
			content: `def helper():
    return "help"

class MyClass:
    def method(self):
        return "method"

def another_function():
    return "function"`,
			want: []Block{
				{
					Type:      "function",
					Name:      "helper",
					StartLine: 0,
					EndLine:   1,
				},
				{
					Type:      "class",
					Name:      "MyClass",
					StartLine: 3,
					EndLine:   5,
				},
				{
					Type:      "function",
					Name:      "another_function",
					StartLine: 7,
					EndLine:   8,
				},
			},
		},
		{
			name: "decorated class with decorated methods",
			content: `@dataclass
class Config:
    @property
    def host(self):
        return self._host

    @host.setter
    def host(self, value):
        self._host = value`,
			want: []Block{
				{
					Type:      "class",
					Name:      "Config",
					StartLine: 0,
					EndLine:   8,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.DetectBlocks(tt.content)

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

func TestPythonLanguage_DetectBlocks_RealWorld(t *testing.T) {
	p := PythonLanguage{}

	content := `"""
Module for handling HTTP requests
"""

import os
from typing import Dict, List
from dataclasses import dataclass

@dataclass
class Config:
    host: str
    port: int
    debug: bool = False

class Handler:
    def __init__(self, config: Config):
        self.config = config

    async def handle_request(self, request):
        """Process incoming request"""
        if self.config.debug:
            print(f"Handling request: {request}")
        return {"status": "ok"}

    def shutdown(self):
        print("Shutting down")

def create_handler(host: str, port: int) -> Handler:
    config = Config(host=host, port=port)
    return Handler(config)

@app.route('/health')
async def health_check():
    return {"status": "healthy"}

if __name__ == "__main__":
    handler = create_handler("localhost", 8080)
    print("Server started")`

	want := []Block{
		{
			Type:      "class",
			Name:      "Config",
			StartLine: 8,
			EndLine:   12,
		},
		{
			Type:      "class",
			Name:      "Handler",
			StartLine: 14,
			EndLine:   25,
		},
		{
			Type:      "function",
			Name:      "create_handler",
			StartLine: 27,
			EndLine:   29,
		},
		{
			Type:      "function",
			Name:      "health_check",
			StartLine: 31,
			EndLine:   33,
		},
	}

	got := p.DetectBlocks(content)

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

func TestPythonLanguage_ImplementsInterface(t *testing.T) {
	// Compile-time check that PythonLanguage implements Language
	var _ Language = PythonLanguage{}
}

func TestPythonLanguage_ExtractFunctionName(t *testing.T) {
	p := PythonLanguage{}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple function",
			input: "def main():",
			want:  "main",
		},
		{
			name:  "function with parameters",
			input: "def add(a, b):",
			want:  "add",
		},
		{
			name:  "async function",
			input: "async def fetch_data():",
			want:  "fetch_data",
		},
		{
			name:  "function with type hints",
			input: "def process(data: str) -> bool:",
			want:  "process",
		},
		{
			name:  "async function with type hints",
			input: "async def fetch(url: str) -> Dict[str, Any]:",
			want:  "fetch",
		},
		{
			name:  "method with self",
			input: "def method(self, arg):",
			want:  "method",
		},
		{
			name:  "class method",
			input: "def method(cls, arg):",
			want:  "method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.extractFunctionName(tt.input)
			if got != tt.want {
				t.Errorf("extractFunctionName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPythonLanguage_ExtractClassName(t *testing.T) {
	p := PythonLanguage{}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple class",
			input: "class MyClass:",
			want:  "MyClass",
		},
		{
			name:  "class with inheritance",
			input: "class Child(Parent):",
			want:  "Child",
		},
		{
			name:  "class with multiple inheritance",
			input: "class Multi(Base1, Base2):",
			want:  "Multi",
		},
		{
			name:  "class with generic types",
			input: "class Handler(Generic[T]):",
			want:  "Handler",
		},
		{
			name:  "class with complex bases",
			input: "class Server(BaseServer, metaclass=ABCMeta):",
			want:  "Server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.extractClassName(tt.input)
			if got != tt.want {
				t.Errorf("extractClassName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPythonLanguage_GetIndentation(t *testing.T) {
	p := PythonLanguage{}

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "no indentation",
			input: "def main():",
			want:  0,
		},
		{
			name:  "4 spaces",
			input: "    return",
			want:  4,
		},
		{
			name:  "8 spaces",
			input: "        print('nested')",
			want:  8,
		},
		{
			name:  "1 tab",
			input: "\treturn",
			want:  4,
		},
		{
			name:  "2 tabs",
			input: "\t\tprint('nested')",
			want:  8,
		},
		{
			name:  "mixed tabs and spaces",
			input: "\t  return",
			want:  6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.getIndentation(tt.input)
			if got != tt.want {
				t.Errorf("getIndentation(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
