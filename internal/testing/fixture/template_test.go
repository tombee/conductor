// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fixture

import (
	"strings"
	"testing"
	"time"
)

func TestExpandTemplate_StringFunctions(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		want     string
	}{
		{
			name:     "upper function",
			template: `{{ upper "hello" }}`,
			data:     nil,
			want:     "HELLO",
		},
		{
			name:     "lower function",
			template: `{{ lower "WORLD" }}`,
			data:     nil,
			want:     "world",
		},
		{
			name:     "trim function",
			template: `{{ trim "  spaces  " }}`,
			data:     nil,
			want:     "spaces",
		},
		{
			name:     "replace function",
			template: `{{ replace "foo" "bar" "foo is foo" }}`,
			data:     nil,
			want:     "bar is bar",
		},
		{
			name:     "contains function",
			template: `{{ if contains "world" "hello world" }}yes{{ else }}no{{ end }}`,
			data:     nil,
			want:     "yes",
		},
		{
			name:     "join function",
			template: `{{ join "," .items }}`,
			data:     map[string]interface{}{"items": []string{"a", "b", "c"}},
			want:     "a,b,c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandTemplate(tt.template, tt.data)
			if err != nil {
				t.Fatalf("ExpandTemplate() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ExpandTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpandTemplate_MathFunctions(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		want     string
	}{
		{
			name:     "add function",
			template: `{{ add 2 3 }}`,
			data:     nil,
			want:     "5",
		},
		{
			name:     "sub function",
			template: `{{ sub 10 3 }}`,
			data:     nil,
			want:     "7",
		},
		{
			name:     "mul function",
			template: `{{ mul 4 5 }}`,
			data:     nil,
			want:     "20",
		},
		{
			name:     "div function",
			template: `{{ div 20 4 }}`,
			data:     nil,
			want:     "5",
		},
		{
			name:     "mod function",
			template: `{{ mod 10 3 }}`,
			data:     nil,
			want:     "1",
		},
		{
			name:     "max function",
			template: `{{ max 10 20 }}`,
			data:     nil,
			want:     "20",
		},
		{
			name:     "min function",
			template: `{{ min 10 20 }}`,
			data:     nil,
			want:     "10",
		},
		{
			name:     "round function",
			template: `{{ round 3.7 }}`,
			data:     nil,
			want:     "4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandTemplate(tt.template, tt.data)
			if err != nil {
				t.Fatalf("ExpandTemplate() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ExpandTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpandTemplate_DateFunctions(t *testing.T) {
	now := time.Now()
	data := map[string]interface{}{"now": now}

	tmpl := `{{ date "2006-01-02" .now }}`
	got, err := ExpandTemplate(tmpl, data)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}

	want := now.Format("2006-01-02")
	if got != want {
		t.Errorf("ExpandTemplate() = %q, want %q", got, want)
	}
}

func TestExpandTemplate_EncodingFunctions(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		want     string
	}{
		{
			name:     "b64enc function",
			template: `{{ b64enc "hello" }}`,
			data:     nil,
			want:     "aGVsbG8=",
		},
		{
			name:     "b64dec function",
			template: `{{ b64dec "aGVsbG8=" }}`,
			data:     nil,
			want:     "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandTemplate(tt.template, tt.data)
			if err != nil {
				t.Fatalf("ExpandTemplate() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ExpandTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpandTemplate_UUIDFunction(t *testing.T) {
	tmpl := `{{ uuidv4 }}`
	got, err := ExpandTemplate(tmpl, nil)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}

	// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	// where x is any hex digit and y is one of 8, 9, A, or B
	parts := strings.Split(got, "-")
	if len(parts) != 5 {
		t.Errorf("UUID format invalid, got %q", got)
	}
	if len(got) != 36 {
		t.Errorf("UUID length invalid, got %d, want 36", len(got))
	}
}

func TestExpandTemplate_DefaultFunction(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		want     string
	}{
		{
			name:     "default with nil value",
			template: `{{ default "fallback" .missing }}`,
			data:     map[string]interface{}{},
			want:     "fallback",
		},
		{
			name:     "default with present value",
			template: `{{ default "fallback" .present }}`,
			data:     map[string]interface{}{"present": "actual"},
			want:     "actual",
		},
		{
			name:     "default with empty string",
			template: `{{ default "fallback" .empty }}`,
			data:     map[string]interface{}{"empty": ""},
			want:     "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandTemplate(tt.template, tt.data)
			if err != nil {
				t.Fatalf("ExpandTemplate() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ExpandTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpandTemplate_WithData(t *testing.T) {
	tmpl := `User: {{ .name }}, ID: {{ uuidv4 }}, Created: {{ date "2006-01-02" now }}`
	data := map[string]interface{}{
		"name": "Alice",
	}

	got, err := ExpandTemplate(tmpl, data)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}

	if !strings.Contains(got, "User: Alice") {
		t.Errorf("Expected template to contain 'User: Alice', got %q", got)
	}
	if !strings.Contains(got, "ID: ") {
		t.Errorf("Expected template to contain 'ID: ', got %q", got)
	}
	if !strings.Contains(got, "Created: ") {
		t.Errorf("Expected template to contain 'Created: ', got %q", got)
	}
}

func TestExpandTemplate_InvalidTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "unclosed template tag",
			template: `{{ upper "test"`,
			data:     nil,
			wantErr:  true,
		},
		{
			name:     "undefined variable (returns empty)",
			template: `{{ .missing }}`,
			data:     map[string]interface{}{},
			wantErr:  false,
		},
		{
			name:     "invalid function",
			template: `{{ invalidFunc "test" }}`,
			data:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExpandTemplate(tt.template, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsForbiddenFunction(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"env", true},
		{"getenv", true},
		{"exec", true},
		{"shell", true},
		{"readFile", true},
		{"writeFile", true},
		{"readDir", true},
		{"glob", true},
		{"osBase", true},
		{"getHostByName", true},
		{"upper", false},
		{"lower", false},
		{"uuidv4", false},
		{"b64enc", false},
		{"add", false},
		{"now", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsForbiddenFunction(tt.name)
			if got != tt.want {
				t.Errorf("IsForbiddenFunction(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
