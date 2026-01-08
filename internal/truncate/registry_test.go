package truncate

import (
	"sync"
	"testing"
)

// mockLanguage is a test implementation of the Language interface
type mockLanguage struct {
	name string
}

func (m mockLanguage) DetectImportEnd(lines []string) int {
	return 0
}

func (m mockLanguage) DetectBlocks(content string) []Block {
	return []Block{}
}

func (m mockLanguage) CommentSyntax() (string, string, string) {
	return "//", "/*", "*/"
}

func TestRegisterLanguage(t *testing.T) {
	// Save and restore registry for test isolation
	registryMu.Lock()
	saved := registry
	registry = make(map[string]Language)
	registryMu.Unlock()

	defer func() {
		registryMu.Lock()
		registry = saved
		registryMu.Unlock()
	}()

	tests := []struct {
		name     string
		language string
		parser   Language
	}{
		{
			name:     "register go",
			language: "go",
			parser:   mockLanguage{name: "go"},
		},
		{
			name:     "register python",
			language: "python",
			parser:   mockLanguage{name: "python"},
		},
		{
			name:     "register with uppercase",
			language: "TYPESCRIPT",
			parser:   mockLanguage{name: "typescript"},
		},
		{
			name:     "register with spaces",
			language: "  javascript  ",
			parser:   mockLanguage{name: "javascript"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterLanguage(tt.language, tt.parser)

			// Verify it was registered
			got := GetLanguage(tt.language)
			if got == nil {
				t.Errorf("GetLanguage(%q) = nil, want non-nil", tt.language)
			}
		})
	}
}

func TestGetLanguage(t *testing.T) {
	// Save and restore registry for test isolation
	registryMu.Lock()
	saved := registry
	registry = make(map[string]Language)
	registryMu.Unlock()

	defer func() {
		registryMu.Lock()
		registry = saved
		registryMu.Unlock()
	}()

	goParser := mockLanguage{name: "go"}
	pythonParser := mockLanguage{name: "python"}

	RegisterLanguage("go", goParser)
	RegisterLanguage("python", pythonParser)

	tests := []struct {
		name     string
		language string
		wantNil  bool
	}{
		{
			name:     "get registered language",
			language: "go",
			wantNil:  false,
		},
		{
			name:     "get with uppercase",
			language: "GO",
			wantNil:  false,
		},
		{
			name:     "get with mixed case",
			language: "Go",
			wantNil:  false,
		},
		{
			name:     "get with spaces",
			language: "  go  ",
			wantNil:  false,
		},
		{
			name:     "get unregistered language",
			language: "rust",
			wantNil:  true,
		},
		{
			name:     "get empty string",
			language: "",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLanguage(tt.language)
			if tt.wantNil && got != nil {
				t.Errorf("GetLanguage(%q) = %v, want nil", tt.language, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("GetLanguage(%q) = nil, want non-nil", tt.language)
			}
		})
	}
}

func TestGetLanguage_CaseInsensitive(t *testing.T) {
	// Save and restore registry for test isolation
	registryMu.Lock()
	saved := registry
	registry = make(map[string]Language)
	registryMu.Unlock()

	defer func() {
		registryMu.Lock()
		registry = saved
		registryMu.Unlock()
	}()

	parser := mockLanguage{name: "test"}
	RegisterLanguage("TypeScript", parser)

	// All these should retrieve the same parser
	variations := []string{"typescript", "TYPESCRIPT", "TypeScript", "tYpEsCrIpT"}

	for _, variation := range variations {
		t.Run(variation, func(t *testing.T) {
			got := GetLanguage(variation)
			if got == nil {
				t.Errorf("GetLanguage(%q) = nil, want non-nil", variation)
			}
		})
	}
}

func TestRegisterLanguage_Replacement(t *testing.T) {
	// Save and restore registry for test isolation
	registryMu.Lock()
	saved := registry
	registry = make(map[string]Language)
	registryMu.Unlock()

	defer func() {
		registryMu.Lock()
		registry = saved
		registryMu.Unlock()
	}()

	parser1 := mockLanguage{name: "first"}
	parser2 := mockLanguage{name: "second"}

	RegisterLanguage("go", parser1)
	RegisterLanguage("go", parser2)

	// Should get the second parser (replacement)
	got := GetLanguage("go")
	if got == nil {
		t.Fatal("GetLanguage(\"go\") = nil, want non-nil")
	}

	// Verify it's the second parser by checking the name field
	if mock, ok := got.(mockLanguage); ok {
		if mock.name != "second" {
			t.Errorf("GetLanguage(\"go\") returned parser with name %q, want %q", mock.name, "second")
		}
	}
}

func TestRegistry_ThreadSafety(t *testing.T) {
	// Save and restore registry for test isolation
	registryMu.Lock()
	saved := registry
	registry = make(map[string]Language)
	registryMu.Unlock()

	defer func() {
		registryMu.Lock()
		registry = saved
		registryMu.Unlock()
	}()

	// Run concurrent registrations and lookups
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Concurrent registrations
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				parser := mockLanguage{name: "concurrent"}
				RegisterLanguage("go", parser)
			}
		}(i)
	}

	// Concurrent lookups
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = GetLanguage("go")
			}
		}(i)
	}

	wg.Wait()

	// Verify registry is still functional
	got := GetLanguage("go")
	if got == nil {
		t.Error("GetLanguage(\"go\") = nil after concurrent operations, want non-nil")
	}
}

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "go", want: "go"},
		{input: "Go", want: "go"},
		{input: "GO", want: "go"},
		{input: "TypeScript", want: "typescript"},
		{input: "PYTHON", want: "python"},
		{input: "  javascript  ", want: "javascript"},
		{input: "\tgo\t", want: "go"},
		{input: "", want: ""},
		{input: "   ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeLanguage(tt.input)
			if got != tt.want {
				t.Errorf("normalizeLanguage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
