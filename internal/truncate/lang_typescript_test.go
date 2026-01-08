package truncate

import (
	"testing"
)

func TestTypeScriptLanguage_CommentSyntax(t *testing.T) {
	ts := TypeScriptLanguage{}

	single, multiOpen, multiClose := ts.CommentSyntax()

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

func TestTypeScriptLanguage_DetectImportEnd(t *testing.T) {
	ts := TypeScriptLanguage{}

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
			name: "imports with comments",
			lines: []string{
				"// This is a comment",
				"import { foo } from 'bar';",
				"/* Multi-line",
				" * comment */",
				"import { baz } from 'qux';",
				"",
				"const x = 1;",
			},
			want: 5,
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
		{
			name: "mixed import and export",
			lines: []string{
				"import { foo } from 'bar';",
				"export { baz } from 'qux';",
				"",
				"const x = 1;",
			},
			want: 2,
		},
		{
			name: "import without space",
			lines: []string{
				"import{foo}from'bar';",
				"",
				"const x = 1;",
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ts.DetectImportEnd(tt.lines)
			if got != tt.want {
				t.Errorf("DetectImportEnd() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTypeScriptLanguage_DetectBlocks_Classes(t *testing.T) {
	ts := TypeScriptLanguage{}

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
		{
			name: "class with nested braces",
			content: `class MyClass {
  method() {
    if (true) {
      return { key: "value" };
    }
  }
}`,
			want: []Block{
				{Type: "class", Name: "MyClass", StartLine: 0, EndLine: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ts.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestTypeScriptLanguage_DetectBlocks_Functions(t *testing.T) {
	ts := TypeScriptLanguage{}

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
		{
			name: "function with nested braces",
			content: `function complex() {
  if (condition) {
    const obj = { key: "value" };
    return obj;
  }
  return null;
}`,
			want: []Block{
				{Type: "function", Name: "complex", StartLine: 0, EndLine: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ts.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestTypeScriptLanguage_DetectBlocks_ArrowFunctions(t *testing.T) {
	ts := TypeScriptLanguage{}

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
		{
			name: "arrow function with nested braces",
			content: `const process = (data) => {
  const obj = { key: "value" };
  if (data) {
    return obj;
  }
  return null;
};`,
			want: []Block{
				{Type: "function", Name: "process", StartLine: 0, EndLine: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ts.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestTypeScriptLanguage_DetectBlocks_Interfaces(t *testing.T) {
	ts := TypeScriptLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name: "simple interface",
			content: `interface User {
  name: string;
  age: number;
}`,
			want: []Block{
				{Type: "interface", Name: "User", StartLine: 0, EndLine: 3},
			},
		},
		{
			name: "exported interface",
			content: `export interface Config {
  port: number;
  host: string;
}`,
			want: []Block{
				{Type: "interface", Name: "Config", StartLine: 0, EndLine: 3},
			},
		},
		{
			name: "multiple interfaces",
			content: `interface First {
  id: number;
}

interface Second {
  name: string;
}`,
			want: []Block{
				{Type: "interface", Name: "First", StartLine: 0, EndLine: 2},
				{Type: "interface", Name: "Second", StartLine: 4, EndLine: 6},
			},
		},
		{
			name: "interface with nested object",
			content: `interface Complex {
  data: {
    nested: {
      value: string;
    };
  };
}`,
			want: []Block{
				{Type: "interface", Name: "Complex", StartLine: 0, EndLine: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ts.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestTypeScriptLanguage_DetectBlocks_Types(t *testing.T) {
	ts := TypeScriptLanguage{}

	tests := []struct {
		name    string
		content string
		want    []Block
	}{
		{
			name:    "simple type alias",
			content: `type ID = string | number;`,
			want: []Block{
				{Type: "type", Name: "ID", StartLine: 0, EndLine: 0},
			},
		},
		{
			name: "object type",
			content: `type User = {
  name: string;
  age: number;
};`,
			want: []Block{
				{Type: "type", Name: "User", StartLine: 0, EndLine: 3},
			},
		},
		{
			name: "exported type",
			content: `export type Status = "active" | "inactive";`,
			want: []Block{
				{Type: "type", Name: "Status", StartLine: 0, EndLine: 0},
			},
		},
		{
			name: "multiple types",
			content: `type First = string;

type Second = {
  value: number;
};`,
			want: []Block{
				{Type: "type", Name: "First", StartLine: 0, EndLine: 0},
				{Type: "type", Name: "Second", StartLine: 2, EndLine: 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ts.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestTypeScriptLanguage_DetectBlocks_Mixed(t *testing.T) {
	ts := TypeScriptLanguage{}

	content := `import { foo } from 'bar';

interface User {
  name: string;
}

type ID = string;

class UserService {
  getUser(id: ID): User {
    return { name: "test" };
  }
}

function helper() {
  return 42;
}

const process = (data) => {
  return data;
};`

	got := ts.DetectBlocks(content)

	// Should detect interface, type, class, function, and arrow function
	if len(got) != 5 {
		t.Errorf("DetectBlocks() found %d blocks, want 5", len(got))
	}

	// Check that we got the right types
	expectedTypes := map[string]bool{
		"interface": true,
		"type":      true,
		"class":     true,
		"function":  true, // both regular and arrow
	}

	for _, block := range got {
		if !expectedTypes[block.Type] {
			t.Errorf("Unexpected block type: %s", block.Type)
		}
	}
}

func TestTypeScriptLanguage_DetectBlocks_StringsAndComments(t *testing.T) {
	ts := TypeScriptLanguage{}

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
			got := ts.DetectBlocks(tt.content)
			if !compareBlocks(got, tt.want) {
				t.Errorf("DetectBlocks() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestTypeScriptLanguage_DetectBlocks_EmptyContent(t *testing.T) {
	ts := TypeScriptLanguage{}

	got := ts.DetectBlocks("")
	if len(got) != 0 {
		t.Errorf("DetectBlocks(\"\") = %v, want empty slice", got)
	}
}

func TestTypeScriptLanguage_ImplementsInterface(t *testing.T) {
	// Compile-time check that TypeScriptLanguage implements Language
	var _ Language = TypeScriptLanguage{}
}

// compareBlocks compares two slices of blocks for equality.
func compareBlocks(got, want []Block) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i].Type != want[i].Type ||
			got[i].Name != want[i].Name ||
			got[i].StartLine != want[i].StartLine ||
			got[i].EndLine != want[i].EndLine {
			return false
		}
	}

	return true
}
