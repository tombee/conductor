package truncate

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkStripper benchmarks the string/comment stripper performance.
func BenchmarkStripper(b *testing.B) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "SmallFile",
			content: generateGoCode(100),
		},
		{
			name:    "MediumFile",
			content: generateGoCode(1000),
		},
		{
			name:    "LargeFile",
			content: generateGoCode(10000),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			stripper := NewStripper("//", "/*", "*/")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := stripper.Strip(tt.content)
				if err != nil {
					b.Fatalf("Strip failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkGoParser benchmarks the Go language parser.
func BenchmarkGoParser(b *testing.B) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "SmallFile",
			content: generateGoCode(100),
		},
		{
			name:    "MediumFile",
			content: generateGoCode(1000),
		},
		{
			name:    "LargeFile",
			content: generateGoCode(10000),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			parser := GoLanguage{}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = parser.DetectBlocks(tt.content)
			}
		})
	}
}

// BenchmarkTypeScriptParser benchmarks the TypeScript language parser.
func BenchmarkTypeScriptParser(b *testing.B) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "SmallFile",
			content: generateTypeScriptCode(100),
		},
		{
			name:    "MediumFile",
			content: generateTypeScriptCode(1000),
		},
		{
			name:    "LargeFile",
			content: generateTypeScriptCode(10000),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			parser := TypeScriptLanguage{}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = parser.DetectBlocks(tt.content)
			}
		})
	}
}

// BenchmarkPythonParser benchmarks the Python language parser.
func BenchmarkPythonParser(b *testing.B) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "SmallFile",
			content: generatePythonCode(100),
		},
		{
			name:    "MediumFile",
			content: generatePythonCode(1000),
		},
		{
			name:    "LargeFile",
			content: generatePythonCode(10000),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			parser := PythonLanguage{}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = parser.DetectBlocks(tt.content)
			}
		})
	}
}

// BenchmarkJavaScriptParser benchmarks the JavaScript language parser.
func BenchmarkJavaScriptParser(b *testing.B) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "SmallFile",
			content: generateJavaScriptCode(100),
		},
		{
			name:    "MediumFile",
			content: generateJavaScriptCode(1000),
		},
		{
			name:    "LargeFile",
			content: generateJavaScriptCode(10000),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			parser := JavaScriptLanguage{}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = parser.DetectBlocks(tt.content)
			}
		})
	}
}

// BenchmarkFallbackParser benchmarks the fallback line-based parser.
func BenchmarkFallbackParser(b *testing.B) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "SmallFile",
			content: generatePlainText(100),
		},
		{
			name:    "MediumFile",
			content: generatePlainText(1000),
		},
		{
			name:    "LargeFile",
			content: generatePlainText(10000),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			parser := FallbackLanguage{}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = parser.DetectBlocks(tt.content)
			}
		})
	}
}

// BenchmarkMemoryUsage measures memory allocation for large file processing.
func BenchmarkMemoryUsage(b *testing.B) {
	content := generateGoCode(10000)
	parser := GoLanguage{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = parser.DetectBlocks(content)
	}
}

// Helper functions to generate test content

func generateGoCode(lines int) string {
	var builder strings.Builder

	// Package and imports
	builder.WriteString("package main\n\n")
	builder.WriteString("import (\n")
	builder.WriteString("\t\"fmt\"\n")
	builder.WriteString("\t\"strings\"\n")
	builder.WriteString(")\n\n")

	// Generate functions to fill the target line count
	funcCount := (lines - 10) / 15 // Approximate 15 lines per function
	if funcCount < 1 {
		funcCount = 1
	}

	for i := 0; i < funcCount; i++ {
		builder.WriteString(fmt.Sprintf("// Function%d performs operation %d.\n", i, i))
		builder.WriteString(fmt.Sprintf("func Function%d(arg string) string {\n", i))
		builder.WriteString("\t// Comment inside function\n")
		builder.WriteString("\tif arg == \"\" {\n")
		builder.WriteString("\t\treturn \"default\"\n")
		builder.WriteString("\t}\n")
		builder.WriteString("\tresult := strings.ToUpper(arg)\n")
		builder.WriteString("\tfor i := 0; i < 10; i++ {\n")
		builder.WriteString("\t\tresult += fmt.Sprintf(\"_%d\", i)\n")
		builder.WriteString("\t}\n")
		builder.WriteString("\treturn result\n")
		builder.WriteString("}\n\n")
	}

	return builder.String()
}

func generateTypeScriptCode(lines int) string {
	var builder strings.Builder

	// Imports
	builder.WriteString("import { Component } from '@angular/core';\n")
	builder.WriteString("import { HttpClient } from '@angular/common/http';\n\n")

	// Generate classes/functions
	funcCount := (lines - 10) / 15
	if funcCount < 1 {
		funcCount = 1
	}

	for i := 0; i < funcCount; i++ {
		builder.WriteString(fmt.Sprintf("// Function%d documentation\n", i))
		builder.WriteString(fmt.Sprintf("function function%d(arg: string): string {\n", i))
		builder.WriteString("\t// Implementation\n")
		builder.WriteString("\tif (arg === '') {\n")
		builder.WriteString("\t\treturn 'default';\n")
		builder.WriteString("\t}\n")
		builder.WriteString("\tconst result = arg.toUpperCase();\n")
		builder.WriteString("\tfor (let i = 0; i < 10; i++) {\n")
		builder.WriteString("\t\tresult += `_${i}`;\n")
		builder.WriteString("\t}\n")
		builder.WriteString("\treturn result;\n")
		builder.WriteString("}\n\n")
	}

	return builder.String()
}

func generatePythonCode(lines int) string {
	var builder strings.Builder

	// Imports
	builder.WriteString("import os\n")
	builder.WriteString("import sys\n")
	builder.WriteString("from typing import Optional\n\n")

	// Generate functions
	funcCount := (lines - 10) / 12
	if funcCount < 1 {
		funcCount = 1
	}

	for i := 0; i < funcCount; i++ {
		builder.WriteString(fmt.Sprintf("def function_%d(arg: str) -> str:\n", i))
		builder.WriteString(fmt.Sprintf("    \"\"\"Function %d documentation.\"\"\"\n", i))
		builder.WriteString("    if not arg:\n")
		builder.WriteString("        return 'default'\n")
		builder.WriteString("    result = arg.upper()\n")
		builder.WriteString("    for i in range(10):\n")
		builder.WriteString("        result += f'_{i}'\n")
		builder.WriteString("    return result\n\n")
	}

	return builder.String()
}

func generateJavaScriptCode(lines int) string {
	var builder strings.Builder

	// Imports
	builder.WriteString("const fs = require('fs');\n")
	builder.WriteString("const path = require('path');\n\n")

	// Generate functions
	funcCount := (lines - 10) / 12
	if funcCount < 1 {
		funcCount = 1
	}

	for i := 0; i < funcCount; i++ {
		builder.WriteString(fmt.Sprintf("// Function%d implementation\n", i))
		builder.WriteString(fmt.Sprintf("function function%d(arg) {\n", i))
		builder.WriteString("\tif (arg === '') {\n")
		builder.WriteString("\t\treturn 'default';\n")
		builder.WriteString("\t}\n")
		builder.WriteString("\tlet result = arg.toUpperCase();\n")
		builder.WriteString("\tfor (let i = 0; i < 10; i++) {\n")
		builder.WriteString("\t\tresult += `_${i}`;\n")
		builder.WriteString("\t}\n")
		builder.WriteString("\treturn result;\n")
		builder.WriteString("}\n\n")
	}

	return builder.String()
}

func generatePlainText(lines int) string {
	var builder strings.Builder
	for i := 0; i < lines; i++ {
		builder.WriteString(fmt.Sprintf("Line %d of plain text content.\n", i+1))
	}
	return builder.String()
}
