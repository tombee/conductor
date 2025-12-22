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

package prompt

import (
	"context"
	"testing"
)

func TestMockPrompter_PromptString(t *testing.T) {
	tests := []struct {
		name      string
		responses []interface{}
		want      string
		wantErr   bool
	}{
		{
			name:      "returns string response",
			responses: []interface{}{"test"},
			want:      "test",
		},
		{
			name:      "returns default when no responses",
			responses: []interface{}{},
			want:      "default",
		},
		{
			name:      "errors on wrong type",
			responses: []interface{}{42},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := NewMockPrompter(true, tt.responses...)
			got, err := mp.PromptString(context.Background(), "test", "desc", "default")
			if (err != nil) != tt.wantErr {
				t.Errorf("PromptString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("PromptString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMockPrompter_PromptNumber(t *testing.T) {
	tests := []struct {
		name      string
		responses []interface{}
		want      float64
		wantErr   bool
	}{
		{
			name:      "returns float64 response",
			responses: []interface{}{3.14},
			want:      3.14,
		},
		{
			name:      "returns int as float64",
			responses: []interface{}{42},
			want:      42.0,
		},
		{
			name:      "returns default when no responses",
			responses: []interface{}{},
			want:      99.9,
		},
		{
			name:      "errors on wrong type",
			responses: []interface{}{"not a number"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := NewMockPrompter(true, tt.responses...)
			got, err := mp.PromptNumber(context.Background(), "test", "desc", 99.9)
			if (err != nil) != tt.wantErr {
				t.Errorf("PromptNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("PromptNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMockPrompter_PromptBool(t *testing.T) {
	tests := []struct {
		name      string
		responses []interface{}
		want      bool
		wantErr   bool
	}{
		{
			name:      "returns true",
			responses: []interface{}{true},
			want:      true,
		},
		{
			name:      "returns false",
			responses: []interface{}{false},
			want:      false,
		},
		{
			name:      "returns default when no responses",
			responses: []interface{}{},
			want:      true,
		},
		{
			name:      "errors on wrong type",
			responses: []interface{}{"yes"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := NewMockPrompter(true, tt.responses...)
			got, err := mp.PromptBool(context.Background(), "test", "desc", true)
			if (err != nil) != tt.wantErr {
				t.Errorf("PromptBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("PromptBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMockPrompter_PromptEnum(t *testing.T) {
	options := []string{"apple", "banana", "cherry"}

	tests := []struct {
		name      string
		responses []interface{}
		want      string
		wantErr   bool
	}{
		{
			name:      "returns string response",
			responses: []interface{}{"banana"},
			want:      "banana",
		},
		{
			name:      "returns default when no responses",
			responses: []interface{}{},
			want:      "apple",
		},
		{
			name:      "errors on wrong type",
			responses: []interface{}{2},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := NewMockPrompter(true, tt.responses...)
			got, err := mp.PromptEnum(context.Background(), "test", "desc", options, "apple")
			if (err != nil) != tt.wantErr {
				t.Errorf("PromptEnum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("PromptEnum() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMockPrompter_PromptArray(t *testing.T) {
	tests := []struct {
		name      string
		responses []interface{}
		want      []interface{}
		wantErr   bool
	}{
		{
			name:      "returns array response",
			responses: []interface{}{[]interface{}{"a", "b", "c"}},
			want:      []interface{}{"a", "b", "c"},
		},
		{
			name:      "errors when no responses",
			responses: []interface{}{},
			wantErr:   true,
		},
		{
			name:      "errors on wrong type",
			responses: []interface{}{"not an array"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := NewMockPrompter(true, tt.responses...)
			got, err := mp.PromptArray(context.Background(), "test", "desc")
			if (err != nil) != tt.wantErr {
				t.Errorf("PromptArray() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("PromptArray() length = %d, want %d", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("PromptArray()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestMockPrompter_PromptObject(t *testing.T) {
	tests := []struct {
		name      string
		responses []interface{}
		wantErr   bool
		check     func(map[string]interface{}) bool
	}{
		{
			name: "returns object response",
			responses: []interface{}{
				map[string]interface{}{"key": "value"},
			},
			check: func(obj map[string]interface{}) bool {
				return obj["key"] == "value"
			},
		},
		{
			name:      "errors when no responses",
			responses: []interface{}{},
			wantErr:   true,
		},
		{
			name:      "errors on wrong type",
			responses: []interface{}{"not an object"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := NewMockPrompter(true, tt.responses...)
			got, err := mp.PromptObject(context.Background(), "test", "desc")
			if (err != nil) != tt.wantErr {
				t.Errorf("PromptObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				if !tt.check(got) {
					t.Errorf("PromptObject() result failed validation check")
				}
			}
		})
	}
}

func TestMockPrompter_IsInteractive(t *testing.T) {
	tests := []struct {
		name        string
		interactive bool
	}{
		{
			name:        "interactive mode",
			interactive: true,
		},
		{
			name:        "non-interactive mode",
			interactive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := NewMockPrompter(tt.interactive)
			if got := mp.IsInteractive(); got != tt.interactive {
				t.Errorf("IsInteractive() = %v, want %v", got, tt.interactive)
			}
		})
	}
}

func TestMockPrompter_CallLog(t *testing.T) {
	mp := NewMockPrompter(true, "test", 42.0, true, "banana", []interface{}{"a"}, map[string]interface{}{"k": "v"})

	ctx := context.Background()
	mp.PromptString(ctx, "str", "desc", "")
	mp.PromptNumber(ctx, "num", "desc", 0)
	mp.PromptBool(ctx, "bool", "desc", false)
	mp.PromptEnum(ctx, "enum", "desc", []string{"apple", "banana"}, "")
	mp.PromptArray(ctx, "arr", "desc")
	mp.PromptObject(ctx, "obj", "desc")

	log := mp.GetCallLog()
	expected := []string{
		"PromptString(str)",
		"PromptNumber(num)",
		"PromptBool(bool)",
		"PromptEnum(enum)",
		"PromptArray(arr)",
		"PromptObject(obj)",
	}

	if len(log) != len(expected) {
		t.Errorf("CallLog length = %d, want %d", len(log), len(expected))
		return
	}

	for i, want := range expected {
		if log[i] != want {
			t.Errorf("CallLog[%d] = %q, want %q", i, log[i], want)
		}
	}
}

func TestMockPrompter_Reset(t *testing.T) {
	mp := NewMockPrompter(true, "first", "second")

	// Use first response
	val1, _ := mp.PromptString(context.Background(), "test1", "desc", "")
	if val1 != "first" {
		t.Errorf("First call got %q, want 'first'", val1)
	}

	// Reset and use first response again
	mp.Reset()
	val2, _ := mp.PromptString(context.Background(), "test2", "desc", "")
	if val2 != "first" {
		t.Errorf("After reset got %q, want 'first'", val2)
	}

	// Check call log was cleared
	log := mp.GetCallLog()
	if len(log) != 1 {
		t.Errorf("After reset, CallLog length = %d, want 1", len(log))
	}
}

func TestMockPrompter_MultipleResponses(t *testing.T) {
	mp := NewMockPrompter(true, "first", "second", "third")

	ctx := context.Background()

	val1, _ := mp.PromptString(ctx, "test1", "desc", "")
	if val1 != "first" {
		t.Errorf("First call got %q, want 'first'", val1)
	}

	val2, _ := mp.PromptString(ctx, "test2", "desc", "")
	if val2 != "second" {
		t.Errorf("Second call got %q, want 'second'", val2)
	}

	val3, _ := mp.PromptString(ctx, "test3", "desc", "")
	if val3 != "third" {
		t.Errorf("Third call got %q, want 'third'", val3)
	}

	// Fourth call should return default
	val4, _ := mp.PromptString(ctx, "test4", "desc", "default")
	if val4 != "default" {
		t.Errorf("Fourth call got %q, want 'default'", val4)
	}
}
