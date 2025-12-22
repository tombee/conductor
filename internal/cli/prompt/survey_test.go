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
	"strings"
	"testing"
)

func TestNewSurveyPrompter(t *testing.T) {
	sp := NewSurveyPrompter(true)
	if sp == nil {
		t.Fatal("NewSurveyPrompter() returned nil")
	}

	if !sp.IsInteractive() {
		t.Error("IsInteractive() should return true when created with true")
	}
}

func TestSurveyPrompter_IsInteractive(t *testing.T) {
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
			sp := NewSurveyPrompter(tt.interactive)
			if got := sp.IsInteractive(); got != tt.interactive {
				t.Errorf("IsInteractive() = %v, want %v", got, tt.interactive)
			}
		})
	}
}

func TestSurveyPrompter_NonInteractiveErrors(t *testing.T) {
	sp := NewSurveyPrompter(false)
	ctx := context.Background()

	t.Run("PromptString", func(t *testing.T) {
		_, err := sp.PromptString(ctx, "test", "desc", "")
		if err == nil {
			t.Error("PromptString() in non-interactive mode should return error")
		}
		if !strings.Contains(err.Error(), "non-interactive") {
			t.Errorf("error should mention non-interactive mode, got: %v", err)
		}
	})

	t.Run("PromptNumber", func(t *testing.T) {
		_, err := sp.PromptNumber(ctx, "test", "desc", 0)
		if err == nil {
			t.Error("PromptNumber() in non-interactive mode should return error")
		}
		if !strings.Contains(err.Error(), "non-interactive") {
			t.Errorf("error should mention non-interactive mode, got: %v", err)
		}
	})

	t.Run("PromptBool", func(t *testing.T) {
		_, err := sp.PromptBool(ctx, "test", "desc", false)
		if err == nil {
			t.Error("PromptBool() in non-interactive mode should return error")
		}
		if !strings.Contains(err.Error(), "non-interactive") {
			t.Errorf("error should mention non-interactive mode, got: %v", err)
		}
	})

	t.Run("PromptEnum", func(t *testing.T) {
		_, err := sp.PromptEnum(ctx, "test", "desc", []string{"a", "b"}, "")
		if err == nil {
			t.Error("PromptEnum() in non-interactive mode should return error")
		}
		if !strings.Contains(err.Error(), "non-interactive") {
			t.Errorf("error should mention non-interactive mode, got: %v", err)
		}
	})

	t.Run("PromptArray", func(t *testing.T) {
		_, err := sp.PromptArray(ctx, "test", "desc")
		if err == nil {
			t.Error("PromptArray() in non-interactive mode should return error")
		}
		if !strings.Contains(err.Error(), "non-interactive") {
			t.Errorf("error should mention non-interactive mode, got: %v", err)
		}
	})

	t.Run("PromptObject", func(t *testing.T) {
		_, err := sp.PromptObject(ctx, "test", "desc")
		if err == nil {
			t.Error("PromptObject() in non-interactive mode should return error")
		}
		if !strings.Contains(err.Error(), "non-interactive") {
			t.Errorf("error should mention non-interactive mode, got: %v", err)
		}
	})
}

func TestSurveyPrompter_PromptEnum_EmptyOptions(t *testing.T) {
	sp := NewSurveyPrompter(true)
	ctx := context.Background()

	_, err := sp.PromptEnum(ctx, "test", "desc", []string{}, "")
	if err == nil {
		t.Error("PromptEnum() with empty options should return error")
	}
	if !strings.Contains(err.Error(), "no options") {
		t.Errorf("error should mention no options, got: %v", err)
	}
}
