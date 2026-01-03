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

func TestNewInputCollector(t *testing.T) {
	mp := NewMockPrompter(true)
	ic := NewInputCollector(mp)

	if ic == nil {
		t.Fatal("NewInputCollector() returned nil")
	}

	if ic.prompter != mp {
		t.Error("InputCollector prompter not set correctly")
	}
}

func TestInputCollector_SetProgress(t *testing.T) {
	ic := NewInputCollector(NewMockPrompter(true))

	ic.SetProgress(3, 10)

	current, total := ic.GetProgress()
	if current != 3 {
		t.Errorf("GetProgress() current = %d, want 3", current)
	}
	if total != 10 {
		t.Errorf("GetProgress() total = %d, want 10", total)
	}
}

func TestInputCollector_FormatProgressPrefix(t *testing.T) {
	tests := []struct {
		name    string
		current int
		total   int
		want    string
	}{
		{
			name:    "no progress",
			current: 0,
			total:   0,
			want:    "",
		},
		{
			name:    "first of three",
			current: 1,
			total:   3,
			want:    "[Input 1 of 3] ",
		},
		{
			name:    "middle of five",
			current: 3,
			total:   5,
			want:    "[Input 3 of 5] ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := NewInputCollector(NewMockPrompter(true))
			ic.SetProgress(tt.current, tt.total)

			got := ic.FormatProgressPrefix()
			if got != tt.want {
				t.Errorf("FormatProgressPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInputCollector_CollectInput_String(t *testing.T) {
	mp := NewMockPrompter(true, "test value")
	ic := NewInputCollector(mp)

	config := PromptConfig{
		Name:        "username",
		Description: "Enter username",
		Type:        InputTypeString,
	}

	value, err := ic.CollectInput(context.Background(), config)
	if err != nil {
		t.Fatalf("CollectInput() error = %v", err)
	}

	str, ok := value.(string)
	if !ok {
		t.Fatalf("CollectInput() returned type %T, want string", value)
	}

	if str != "test value" {
		t.Errorf("CollectInput() = %q, want 'test value'", str)
	}
}

func TestInputCollector_CollectInput_Number(t *testing.T) {
	mp := NewMockPrompter(true, 42.5)
	ic := NewInputCollector(mp)

	config := PromptConfig{
		Name:        "age",
		Description: "Enter age",
		Type:        InputTypeNumber,
	}

	value, err := ic.CollectInput(context.Background(), config)
	if err != nil {
		t.Fatalf("CollectInput() error = %v", err)
	}

	num, ok := value.(float64)
	if !ok {
		t.Fatalf("CollectInput() returned type %T, want float64", value)
	}

	if num != 42.5 {
		t.Errorf("CollectInput() = %v, want 42.5", num)
	}
}

func TestInputCollector_CollectInput_Boolean(t *testing.T) {
	mp := NewMockPrompter(true, true)
	ic := NewInputCollector(mp)

	config := PromptConfig{
		Name:        "enabled",
		Description: "Enable feature?",
		Type:        InputTypeBoolean,
	}

	value, err := ic.CollectInput(context.Background(), config)
	if err != nil {
		t.Fatalf("CollectInput() error = %v", err)
	}

	b, ok := value.(bool)
	if !ok {
		t.Fatalf("CollectInput() returned type %T, want bool", value)
	}

	if !b {
		t.Errorf("CollectInput() = %v, want true", b)
	}
}

func TestInputCollector_CollectInput_Enum(t *testing.T) {
	mp := NewMockPrompter(true, "banana")
	ic := NewInputCollector(mp)

	config := PromptConfig{
		Name:        "fruit",
		Description: "Choose a fruit",
		Type:        InputTypeEnum,
		Options:     []string{"apple", "banana", "cherry"},
	}

	value, err := ic.CollectInput(context.Background(), config)
	if err != nil {
		t.Fatalf("CollectInput() error = %v", err)
	}

	str, ok := value.(string)
	if !ok {
		t.Fatalf("CollectInput() returned type %T, want string", value)
	}

	if str != "banana" {
		t.Errorf("CollectInput() = %q, want 'banana'", str)
	}
}

func TestInputCollector_CollectInput_Array(t *testing.T) {
	mp := NewMockPrompter(true, []interface{}{"a", "b", "c"})
	ic := NewInputCollector(mp)

	config := PromptConfig{
		Name:        "tags",
		Description: "Enter tags",
		Type:        InputTypeArray,
	}

	value, err := ic.CollectInput(context.Background(), config)
	if err != nil {
		t.Fatalf("CollectInput() error = %v", err)
	}

	arr, ok := value.([]interface{})
	if !ok {
		t.Fatalf("CollectInput() returned type %T, want []interface{}", value)
	}

	if len(arr) != 3 {
		t.Errorf("CollectInput() array length = %d, want 3", len(arr))
	}
}

func TestInputCollector_CollectInput_Object(t *testing.T) {
	mp := NewMockPrompter(true, map[string]interface{}{"key": "value"})
	ic := NewInputCollector(mp)

	config := PromptConfig{
		Name:        "metadata",
		Description: "Enter metadata",
		Type:        InputTypeObject,
	}

	value, err := ic.CollectInput(context.Background(), config)
	if err != nil {
		t.Fatalf("CollectInput() error = %v", err)
	}

	obj, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("CollectInput() returned type %T, want map[string]interface{}", value)
	}

	if obj["key"] != "value" {
		t.Errorf("CollectInput() object[key] = %v, want 'value'", obj["key"])
	}
}

func TestInputCollector_CollectInput_UnsupportedType(t *testing.T) {
	mp := NewMockPrompter(true)
	ic := NewInputCollector(mp)

	config := PromptConfig{
		Name:        "test",
		Description: "test",
		Type:        InputType("invalid"),
	}

	_, err := ic.CollectInput(context.Background(), config)
	if err == nil {
		t.Fatal("CollectInput() expected error for unsupported type")
	}

	if !strings.Contains(err.Error(), "unsupported input type") {
		t.Errorf("CollectInput() error = %v, want error containing 'unsupported input type'", err)
	}
}

func TestInputCollector_CollectInputs(t *testing.T) {
	mp := NewMockPrompter(true, "alice", 30.0, true)
	ic := NewInputCollector(mp)

	configs := []PromptConfig{
		{
			Name:        "name",
			Description: "Enter name",
			Type:        InputTypeString,
		},
		{
			Name:        "age",
			Description: "Enter age",
			Type:        InputTypeNumber,
		},
		{
			Name:        "active",
			Description: "Is active?",
			Type:        InputTypeBoolean,
		},
	}

	results, err := ic.CollectInputs(context.Background(), configs)
	if err != nil {
		t.Fatalf("CollectInputs() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("CollectInputs() returned %d results, want 3", len(results))
	}

	if results["name"] != "alice" {
		t.Errorf("results[name] = %v, want 'alice'", results["name"])
	}

	if results["age"] != 30.0 {
		t.Errorf("results[age] = %v, want 30.0", results["age"])
	}

	if results["active"] != true {
		t.Errorf("results[active] = %v, want true", results["active"])
	}
}

func TestInputCollector_CollectInputs_UpdatesProgress(t *testing.T) {
	mp := NewMockPrompter(true, "a", "b", "c")
	ic := NewInputCollector(mp)

	configs := []PromptConfig{
		{Name: "first", Type: InputTypeString},
		{Name: "second", Type: InputTypeString},
		{Name: "third", Type: InputTypeString},
	}

	_, err := ic.CollectInputs(context.Background(), configs)
	if err != nil {
		t.Fatalf("CollectInputs() error = %v", err)
	}

	current, total := ic.GetProgress()
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	// After completion, current should be 3 (last iteration was i=2, current=3)
	if current != 3 {
		t.Errorf("current = %d, want 3", current)
	}
}

func TestInputCollector_CollectInputs_EmptyConfigs(t *testing.T) {
	mp := NewMockPrompter(true)
	ic := NewInputCollector(mp)

	results, err := ic.CollectInputs(context.Background(), []PromptConfig{})
	if err != nil {
		t.Fatalf("CollectInputs() with empty configs error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("CollectInputs() returned %d results, want 0", len(results))
	}
}

func TestProgressTracker(t *testing.T) {
	pt := ProgressTracker{
		current: 5,
		total:   10,
	}

	if pt.current != 5 {
		t.Errorf("ProgressTracker.current = %d, want 5", pt.current)
	}

	if pt.total != 10 {
		t.Errorf("ProgressTracker.total = %d, want 10", pt.total)
	}
}

func TestValidationError(t *testing.T) {
	ve := &ValidationError{
		InputName: "test",
		InputType: "string",
		Reason:    "invalid value",
	}

	if ve.Error() != "invalid value" {
		t.Errorf("ValidationError.Error() = %q, want 'invalid value'", ve.Error())
	}
}

func TestPromptConfig(t *testing.T) {
	config := PromptConfig{
		Name:        "test",
		Description: "test input",
		Type:        InputTypeString,
		Options:     []string{"a", "b"},
	}

	if config.Name != "test" {
		t.Errorf("Name = %q, want 'test'", config.Name)
	}

	if config.Type != InputTypeString {
		t.Errorf("Type = %v, want InputTypeString", config.Type)
	}

	if len(config.Options) != 2 {
		t.Errorf("Options length = %d, want 2", len(config.Options))
	}
}

func TestInputTypeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value InputType
		want  string
	}{
		{"String", InputTypeString, "string"},
		{"Number", InputTypeNumber, "number"},
		{"Boolean", InputTypeBoolean, "boolean"},
		{"Array", InputTypeArray, "array"},
		{"Object", InputTypeObject, "object"},
		{"Enum", InputTypeEnum, "enum"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	if MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", MaxRetries)
	}

	if MaxInputSize != 65536 {
		t.Errorf("MaxInputSize = %d, want 65536", MaxInputSize)
	}

	if MaxNestedDepth != 10 {
		t.Errorf("MaxNestedDepth = %d, want 10", MaxNestedDepth)
	}
}
