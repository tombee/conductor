package truncate

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzStripper tests the stripper against random inputs.
// Ensures no panics occur regardless of input.
func FuzzStripper(f *testing.F) {
	// Seed corpus with interesting test cases
	f.Add("func main() { /* comment */ }")
	f.Add("\"string with { bracket\"")
	f.Add("// comment with } bracket\n")
	f.Add("/* multi\nline\ncomment */")
	f.Add("`raw string with \" quote`")
	f.Add("\\\"escaped quote\\\"")
	f.Add(strings.Repeat("{", 100))
	f.Add(strings.Repeat("}", 100))
	f.Add("")
	f.Add("\n\n\n")

	f.Fuzz(func(t *testing.T, input string) {
		// Ensure input is valid UTF-8 to avoid noise
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		// Test Go comment syntax
		stripper := NewStripper("//", "/*", "*/")
		result, err := stripper.Strip(input)

		// Should either succeed or return known error
		if err != nil && err != ErrMaxNestingDepthExceeded {
			t.Errorf("unexpected error: %v", err)
		}

		// Result should be same length as input (spaces replace stripped content)
		if err == nil && len(result) != len(input) {
			t.Errorf("length mismatch: got %d, want %d", len(result), len(input))
		}

		// Test Python comment syntax
		stripperPy := NewStripper("#", `"""`, `"""`)
		resultPy, errPy := stripperPy.Strip(input)

		if errPy != nil && errPy != ErrMaxNestingDepthExceeded {
			t.Errorf("unexpected error (Python): %v", errPy)
		}

		if errPy == nil && len(resultPy) != len(input) {
			t.Errorf("length mismatch (Python): got %d, want %d", len(resultPy), len(input))
		}
	})
}

// FuzzGoParser tests the Go parser against random inputs.
// Ensures no panics occur regardless of input structure.
func FuzzGoParser(f *testing.F) {
	// Seed corpus
	f.Add("package main\nfunc main() {}")
	f.Add("type MyStruct struct { Field string }")
	f.Add("func (r *Receiver) Method() {}")
	f.Add("import \"fmt\"")
	f.Add("import (\n\"os\"\n\"io\"\n)")
	f.Add("// comment\n")
	f.Add("")
	f.Add("func unclosed() {")
	f.Add("func malformed(")
	f.Add(strings.Repeat("func f() {}\n", 100))

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		parser := GoLanguage{}

		// DetectBlocks should never panic
		blocks := parser.DetectBlocks(input)

		// Verify blocks are ordered and valid
		for i, block := range blocks {
			if block.StartLine < 0 {
				t.Errorf("block %d has negative StartLine: %d", i, block.StartLine)
			}
			if block.EndLine < block.StartLine {
				t.Errorf("block %d has EndLine < StartLine: %d < %d", i, block.EndLine, block.StartLine)
			}
		}

		// DetectImportEnd should never panic
		lines := strings.Split(input, "\n")
		importEnd := parser.DetectImportEnd(lines)

		if importEnd < 0 {
			t.Errorf("DetectImportEnd returned negative value: %d", importEnd)
		}
		if importEnd > len(lines) {
			t.Errorf("DetectImportEnd returned value > line count: %d > %d", importEnd, len(lines))
		}
	})
}

// FuzzTypeScriptParser tests the TypeScript parser against random inputs.
func FuzzTypeScriptParser(f *testing.F) {
	// Seed corpus
	f.Add("import { Component } from '@angular/core';")
	f.Add("export class MyClass {}")
	f.Add("function myFunc() {}")
	f.Add("const x = () => {}")
	f.Add("interface MyInterface { field: string }")
	f.Add("")
	f.Add("class unclosed {")
	f.Add(strings.Repeat("function f() {}\n", 100))

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		parser := TypeScriptLanguage{}

		// Should never panic
		blocks := parser.DetectBlocks(input)

		for i, block := range blocks {
			if block.StartLine < 0 {
				t.Errorf("block %d has negative StartLine: %d", i, block.StartLine)
			}
			if block.EndLine < block.StartLine {
				t.Errorf("block %d has EndLine < StartLine: %d < %d", i, block.EndLine, block.StartLine)
			}
		}

		lines := strings.Split(input, "\n")
		importEnd := parser.DetectImportEnd(lines)

		if importEnd < 0 || importEnd > len(lines) {
			t.Errorf("DetectImportEnd out of bounds: %d (line count: %d)", importEnd, len(lines))
		}
	})
}

// FuzzPythonParser tests the Python parser against random inputs.
func FuzzPythonParser(f *testing.F) {
	// Seed corpus
	f.Add("import os")
	f.Add("from typing import Optional")
	f.Add("def my_func():\n    pass")
	f.Add("class MyClass:\n    def method(self):\n        pass")
	f.Add("@decorator\ndef func():\n    pass")
	f.Add("")
	f.Add("def unclosed():")
	f.Add(strings.Repeat("def f():\n    pass\n", 100))

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		parser := PythonLanguage{}

		// Should never panic
		blocks := parser.DetectBlocks(input)

		for i, block := range blocks {
			if block.StartLine < 0 {
				t.Errorf("block %d has negative StartLine: %d", i, block.StartLine)
			}
			if block.EndLine < block.StartLine {
				t.Errorf("block %d has EndLine < StartLine: %d < %d", i, block.EndLine, block.StartLine)
			}
		}

		lines := strings.Split(input, "\n")
		importEnd := parser.DetectImportEnd(lines)

		if importEnd < 0 || importEnd > len(lines) {
			t.Errorf("DetectImportEnd out of bounds: %d (line count: %d)", importEnd, len(lines))
		}
	})
}

// FuzzJavaScriptParser tests the JavaScript parser against random inputs.
func FuzzJavaScriptParser(f *testing.F) {
	// Seed corpus
	f.Add("const fs = require('fs');")
	f.Add("export function myFunc() {}")
	f.Add("class MyClass {}")
	f.Add("const arrow = () => {}")
	f.Add("")
	f.Add("function unclosed() {")
	f.Add(strings.Repeat("function f() {}\n", 100))

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		parser := JavaScriptLanguage{}

		// Should never panic
		blocks := parser.DetectBlocks(input)

		for i, block := range blocks {
			if block.StartLine < 0 {
				t.Errorf("block %d has negative StartLine: %d", i, block.StartLine)
			}
			if block.EndLine < block.StartLine {
				t.Errorf("block %d has EndLine < StartLine: %d < %d", i, block.EndLine, block.StartLine)
			}
		}

		lines := strings.Split(input, "\n")
		importEnd := parser.DetectImportEnd(lines)

		if importEnd < 0 || importEnd > len(lines) {
			t.Errorf("DetectImportEnd out of bounds: %d (line count: %d)", importEnd, len(lines))
		}
	})
}

// FuzzFallbackParser tests the fallback parser against random inputs.
func FuzzFallbackParser(f *testing.F) {
	// Seed corpus
	f.Add("plain text")
	f.Add("")
	f.Add("\n\n\n")
	f.Add(strings.Repeat("line\n", 1000))
	f.Add("random { } [ ] ( ) content")

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		parser := FallbackLanguage{}

		// Should never panic
		blocks := parser.DetectBlocks(input)

		// Fallback returns entire content as single block (or empty for empty input)
		if input == "" {
			if len(blocks) != 0 {
				t.Errorf("FallbackLanguage should return empty blocks for empty input, got %d", len(blocks))
			}
		} else {
			if len(blocks) != 1 {
				t.Errorf("FallbackLanguage should return 1 block for non-empty input, got %d", len(blocks))
			}
			if len(blocks) == 1 {
				if blocks[0].Type != "block" {
					t.Errorf("expected block type 'block', got %q", blocks[0].Type)
				}
				if blocks[0].StartLine != 0 {
					t.Errorf("expected StartLine 0, got %d", blocks[0].StartLine)
				}
			}
		}

		lines := strings.Split(input, "\n")
		importEnd := parser.DetectImportEnd(lines)

		// Fallback should always return 0
		if importEnd != 0 {
			t.Errorf("FallbackLanguage DetectImportEnd should return 0, got %d", importEnd)
		}
	})
}

// FuzzBracketCounting tests bracket counting with malformed inputs.
func FuzzBracketCounting(f *testing.F) {
	// Seed corpus with bracket-heavy inputs
	f.Add("{{{}}}")
	f.Add("}{")
	f.Add(strings.Repeat("{", 500))
	f.Add(strings.Repeat("}", 500))
	f.Add("func f() { if true { for { }}}")
	f.Add("{ /* { */ }")
	f.Add("{ \"string with {\" }")

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		// Wrap in a function to test bracket counting
		content := "package test\nfunc test() {\n" + input + "\n}\n"

		parser := GoLanguage{}

		// Should not panic even with unbalanced brackets
		blocks := parser.DetectBlocks(content)

		// Should detect at least the wrapping function
		if len(blocks) == 0 {
			t.Error("expected at least one block for wrapped function")
		}
	})
}
