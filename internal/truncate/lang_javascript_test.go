package truncate

import (
	"testing"
)

func TestJavaScriptLanguage_CommentSyntax(t *testing.T) {
	js := JavaScriptLanguage{}

	single, multiOpen, multiClose := js.CommentSyntax()

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

func TestJavaScriptLanguage_DetectImportEnd(t *testing.T) {
	js := JavaScriptLanguage{}

	tests := []struct {
		name  string
		lines []string
		want  int
	}{
		{
			name:  "no imports",
			lines: []string{"const x = 1;", "function foo() {}"},
			want:  0,
		},
		{
			name: "single import",
			lines: []string{
				"import { foo } from 'bar';",
				"",
				"const x = 1;",
			},
			want: 1,
		},
		{
			name: "multiple imports",
			lines: []string{
				"import { foo } from 'bar';",
				"import { baz } from 'qux';",
				"",
				"const x = 1;",
			},
			want: 2,
		},
		{
			name: "export statements",
			lines: []string{
				"export { foo } from 'bar';",
				"export const x = 1;",
				"",
				"function doStuff() {}",
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := js.DetectImportEnd(tt.lines)
			if got != tt.want {
				t.Errorf("DetectImportEnd() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJavaScriptLanguage_DetectBlocks_Classes(t *testing.T) {
	js := JavaScriptLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name: "simple class",
			content: `class MyClass {
  constructor() {}
  method() {}
}`,
			want: []Block{
				{Type: "class", Name: "MyClass", StartLine: 0, EndLine: 3},
			},
		},
		{
			name: "class with export",
			content: `export class MyClass {
  method() {}
}`,
			want: []Block{
				{Type: "class", Name: "MyClass", StartLine: 0, EndLine: 2},
			},
		},
		{
			name: "multiple classes",
			content: `class First {
  method1() {}
}

class Second {
  method2() {}
}`,
			want: []Block{
				{Type: "class", Name: "First", StartLine: 0, EndLine: 2},
				{Type: "class", Name: "Second", StartLine: 4, EndLine: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := js.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestJavaScriptLanguage_DetectBlocks_Functions(t *testing.T) {
	js := JavaScriptLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name: "simple function",
			content: `function myFunc() {
  return 42;
}`,
			want: []Block{
				{Type: "function", Name: "myFunc", StartLine: 0, EndLine: 2},
			},
		},
		{
			name: "async function",
			content: `async function fetchData() {
  return await fetch('/api');
}`,
			want: []Block{
				{Type: "function", Name: "fetchData", StartLine: 0, EndLine: 2},
			},
		},
		{
			name: "multiple functions",
			content: `function first() {
  return 1;
}

function second() {
  return 2;
}`,
			want: []Block{
				{Type: "function", Name: "first", StartLine: 0, EndLine: 2},
				{Type: "function", Name: "second", StartLine: 4, EndLine: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := js.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestJavaScriptLanguage_DetectBlocks_ArrowFunctions(t *testing.T) {
	js := JavaScriptLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name:    "simple arrow function expression",
			content: `const add = (a, b) => a + b;`,
			want: []Block{
				{Type: "function", Name: "add", StartLine: 0, EndLine: 0},
			},
		},
		{
			name: "arrow function with block body",
			content: `const calculate = (x) => {
  const result = x * 2;
  return result;
};`,
			want: []Block{
				{Type: "function", Name: "calculate", StartLine: 0, EndLine: 3},
			},
		},
		{
			name: "exported arrow function",
			content: `export const handler = async (req) => {
  return { status: 200 };
};`,
			want: []Block{
				{Type: "function", Name: "handler", StartLine: 0, EndLine: 2},
			},
		},
		{
			name: "multiple arrow functions",
			content: `const first = () => 1;
const second = () => {
  return 2;
};`,
			want: []Block{
				{Type: "function", Name: "first", StartLine: 0, EndLine: 0},
				{Type: "function", Name: "second", StartLine: 1, EndLine: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := js.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestJavaScriptLanguage_DetectBlocks_NoTypeScriptConstructs(t *testing.T) {
	js := JavaScriptLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name: "should filter out interfaces",
			content: `interface User {
  name: string;
}

class MyClass {
  method() {}
}`,
			want: []Block{
				{Type: "class", Name: "MyClass", StartLine: 4, EndLine: 6},
			},
		},
		{
			name: "should filter out type aliases",
			content: `type ID = string;

function helper() {
  return 42;
}`,
			want: []Block{
				{Type: "function", Name: "helper", StartLine: 2, EndLine: 4},
			},
		},
		{
			name: "should keep only JavaScript constructs",
			content: `interface Config {
  port: number;
}

type Status = "active" | "inactive";

class Service {
  start() {}
}

function init() {
  return true;
}

const process = () => {
  return null;
};`,
			want: []Block{
				{Type: "class", Name: "Service", StartLine: 6, EndLine: 8},
				{Type: "function", Name: "init", StartLine: 10, EndLine: 12},
				{Type: "function", Name: "process", StartLine: 14, EndLine: 16},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := js.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestJavaScriptLanguage_DetectBlocks_Mixed(t *testing.T) {
	js := JavaScriptLanguage{}

	content := `import { foo } from 'bar';

class UserService {
  getUser(id) {
    return { name: "test" };
  }
}

function helper() {
  return 42;
}

const process = (data) => {
  return data;
};`

	got := js.DetectBlocks(content)

	// Should detect class, function, and arrow function (no interface or type)
	if len(got) != 3 {
		t.Errorf("DetectBlocks() found %d blocks, want 3", len(got))
	}

	// Check that we got the right types
	expectedTypes := []string{"class", "function", "function"}
	for i, block := range got {
		if block.Type != expectedTypes[i] {
			t.Errorf("Block %d: got type %s, want %s", i, block.Type, expectedTypes[i])
		}
	}
}

func TestJavaScriptLanguage_DetectBlocks_StringsAndComments(t *testing.T) {
	js := JavaScriptLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name: "braces in strings should not affect detection",
			content: `function test() {
  const str = "this { has } braces";
  return str;
}`,
			want: []Block{
				{Type: "function", Name: "test", StartLine: 0, EndLine: 3},
			},
		},
		{
			name: "braces in comments should not affect detection",
			content: `function test() {
  // This comment has { braces }
  /* And this one too { } */
  return 42;
}`,
			want: []Block{
				{Type: "function", Name: "test", StartLine: 0, EndLine: 4},
			},
		},
		{
			name: "template literals with braces",
			content: "function test() {\n  const str = `value: ${x}`;\n  return str;\n}",
			want: []Block{
				{Type: "function", Name: "test", StartLine: 0, EndLine: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := js.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestJavaScriptLanguage_DetectBlocks_EmptyContent(t *testing.T) {
	js := JavaScriptLanguage{}

	got := js.DetectBlocks("")
	if len(got) != 0 {
		t.Errorf("DetectBlocks(\"\") = %v, want empty slice", got)
	}
}

func TestJavaScriptLanguage_ImplementsInterface(t *testing.T) {
	// Compile-time check that JavaScriptLanguage implements Language
	var _ Language = JavaScriptLanguage{}
}
