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

package forms

import (
	"strings"
	"testing"
)

func TestFooter_Render(t *testing.T) {
	tests := []struct {
		name         string
		context      FooterContext
		wantContains []string
	}{
		{
			name:    "selection context",
			context: FooterContextSelection,
			wantContains: []string{
				"Enter: Select",
				"Up/Down: Navigate",
				"Esc: Back",
				"?: Help",
			},
		},
		{
			name:    "input context",
			context: FooterContextInput,
			wantContains: []string{
				"Enter: Submit",
				"Esc: Cancel",
				"Tab: Next field",
			},
		},
		{
			name:    "confirm context",
			context: FooterContextConfirm,
			wantContains: []string{
				"Enter: Confirm",
				"Esc: Cancel",
			},
		},
		{
			name:         "unknown context returns empty",
			context:      FooterContext("unknown"),
			wantContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Footer{
				Context: tt.context,
			}

			got := f.Render()

			if len(tt.wantContains) == 0 {
				if got != "" {
					t.Errorf("Render() = %q, want empty string", got)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("Render() missing %q in output", want)
				}
			}

			// Check separator is present
			if !strings.Contains(got, "|") {
				t.Errorf("Render() missing separator '|'")
			}
		})
	}
}

func TestFooter_getShortcuts(t *testing.T) {
	tests := []struct {
		name    string
		context FooterContext
		want    int // number of shortcuts
	}{
		{
			name:    "selection has 4 shortcuts",
			context: FooterContextSelection,
			want:    4,
		},
		{
			name:    "input has 3 shortcuts",
			context: FooterContextInput,
			want:    3,
		},
		{
			name:    "confirm has 2 shortcuts",
			context: FooterContextConfirm,
			want:    2,
		},
		{
			name:    "unknown context has 0 shortcuts",
			context: FooterContext("invalid"),
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Footer{
				Context: tt.context,
			}

			got := f.getShortcuts()

			if len(got) != tt.want {
				t.Errorf("getShortcuts() returned %d shortcuts, want %d", len(got), tt.want)
			}
		})
	}
}

func TestRenderWithCustomShortcuts(t *testing.T) {
	tests := []struct {
		name         string
		shortcuts    []string
		wantContains []string
		wantEmpty    bool
	}{
		{
			name: "custom shortcuts",
			shortcuts: []string{
				"Ctrl+C: Exit",
				"F1: Help",
			},
			wantContains: []string{
				"Ctrl+C: Exit",
				"F1: Help",
				"|",
			},
			wantEmpty: false,
		},
		{
			name:      "empty shortcuts",
			shortcuts: []string{},
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderWithCustomShortcuts(tt.shortcuts)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("RenderWithCustomShortcuts() = %q, want empty string", got)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("RenderWithCustomShortcuts() missing %q in output", want)
				}
			}
		})
	}
}
