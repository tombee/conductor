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
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestNewInputAnalyzer(t *testing.T) {
	inputs := []workflow.InputDefinition{
		{Name: "test", Type: "string"},
	}
	provided := map[string]interface{}{"test": "value"}

	ia := NewInputAnalyzer(inputs, provided)

	if ia == nil {
		t.Fatal("NewInputAnalyzer() returned nil")
	}

	if len(ia.workflowInputs) != 1 {
		t.Errorf("workflowInputs length = %d, want 1", len(ia.workflowInputs))
	}

	if len(ia.providedInputs) != 1 {
		t.Errorf("providedInputs length = %d, want 1", len(ia.providedInputs))
	}
}

func TestInputAnalyzer_FindMissingInputs_AllProvided(t *testing.T) {
	// Inputs without defaults are required
	inputs := []workflow.InputDefinition{
		{Name: "name", Type: "string"},
		{Name: "age", Type: "number"},
	}
	provided := map[string]interface{}{
		"name": "alice",
		"age":  30,
	}

	ia := NewInputAnalyzer(inputs, provided)
	missing := ia.FindMissingInputs()

	if len(missing) != 0 {
		t.Errorf("FindMissingInputs() returned %d items, want 0", len(missing))
	}
}

func TestInputAnalyzer_FindMissingInputs_RequiredMissing(t *testing.T) {
	// Inputs without defaults are required
	inputs := []workflow.InputDefinition{
		{Name: "name", Type: "string", Description: "User name"},
		{Name: "age", Type: "number", Description: "User age"},
		{Name: "email", Type: "string", Default: "test@example.com"}, // optional (has default)
	}
	provided := map[string]interface{}{}

	ia := NewInputAnalyzer(inputs, provided)
	missing := ia.FindMissingInputs()

	// name and age are missing (no defaults), email has default so not missing
	if len(missing) != 2 {
		t.Fatalf("FindMissingInputs() returned %d items, want 2", len(missing))
	}

	// Check first missing input
	if missing[0].Name != "name" {
		t.Errorf("missing[0].Name = %q, want 'name'", missing[0].Name)
	}
	if missing[0].Type != "string" {
		t.Errorf("missing[0].Type = %q, want 'string'", missing[0].Type)
	}
	if missing[0].Description != "User name" {
		t.Errorf("missing[0].Description = %q, want 'User name'", missing[0].Description)
	}

	// Check second missing input
	if missing[1].Name != "age" {
		t.Errorf("missing[1].Name = %q, want 'age'", missing[1].Name)
	}
	if missing[1].Type != "number" {
		t.Errorf("missing[1].Type = %q, want 'number'", missing[1].Type)
	}
}

func TestInputAnalyzer_FindMissingInputs_OptionalWithDefault(t *testing.T) {
	// Inputs with defaults are optional
	inputs := []workflow.InputDefinition{
		{
			Name:    "port",
			Type:    "number",
			Default: 8080,
		},
		{
			Name:    "host",
			Type:    "string",
			Default: "localhost",
		},
	}
	provided := map[string]interface{}{}

	ia := NewInputAnalyzer(inputs, provided)
	missing := ia.FindMissingInputs()

	// Inputs with defaults should not be in missing list
	if len(missing) != 0 {
		t.Errorf("FindMissingInputs() returned %d items, want 0 (inputs with defaults)", len(missing))
	}
}

func TestInputAnalyzer_FindMissingInputs_WithEnum(t *testing.T) {
	// Input without default is required
	inputs := []workflow.InputDefinition{
		{
			Name:        "env",
			Type:        "string",
			Description: "Environment",
			Enum:        []string{"dev", "staging", "prod"},
		},
	}
	provided := map[string]interface{}{}

	ia := NewInputAnalyzer(inputs, provided)
	missing := ia.FindMissingInputs()

	if len(missing) != 1 {
		t.Fatalf("FindMissingInputs() returned %d items, want 1", len(missing))
	}

	if len(missing[0].Enum) != 3 {
		t.Errorf("missing[0].Enum length = %d, want 3", len(missing[0].Enum))
	}

	expectedEnum := []string{"dev", "staging", "prod"}
	for i, want := range expectedEnum {
		if missing[0].Enum[i] != want {
			t.Errorf("missing[0].Enum[%d] = %q, want %q", i, missing[0].Enum[i], want)
		}
	}
}

func TestInputAnalyzer_ApplyDefaults_NoDefaults(t *testing.T) {
	inputs := []workflow.InputDefinition{
		{Name: "name", Type: "string"},
	}
	provided := map[string]interface{}{
		"name": "alice",
	}

	ia := NewInputAnalyzer(inputs, provided)
	result := ia.ApplyDefaults()

	if len(result) != 1 {
		t.Errorf("ApplyDefaults() returned %d items, want 1", len(result))
	}

	if result["name"] != "alice" {
		t.Errorf("result[name] = %v, want 'alice'", result["name"])
	}
}

func TestInputAnalyzer_ApplyDefaults_WithDefaults(t *testing.T) {
	inputs := []workflow.InputDefinition{
		{Name: "name", Type: "string"},
		{Name: "port", Type: "number", Default: 8080},
		{Name: "host", Type: "string", Default: "localhost"},
	}
	provided := map[string]interface{}{
		"name": "alice",
	}

	ia := NewInputAnalyzer(inputs, provided)
	result := ia.ApplyDefaults()

	if len(result) != 3 {
		t.Errorf("ApplyDefaults() returned %d items, want 3", len(result))
	}

	if result["name"] != "alice" {
		t.Errorf("result[name] = %v, want 'alice'", result["name"])
	}

	if result["port"] != 8080 {
		t.Errorf("result[port] = %v, want 8080", result["port"])
	}

	if result["host"] != "localhost" {
		t.Errorf("result[host] = %v, want 'localhost'", result["host"])
	}
}

func TestInputAnalyzer_ApplyDefaults_ProvidedOverridesDefault(t *testing.T) {
	inputs := []workflow.InputDefinition{
		{Name: "port", Type: "number", Default: 8080},
	}
	provided := map[string]interface{}{
		"port": 9000,
	}

	ia := NewInputAnalyzer(inputs, provided)
	result := ia.ApplyDefaults()

	if result["port"] != 9000 {
		t.Errorf("result[port] = %v, want 9000 (provided should override default)", result["port"])
	}
}

func TestInputAnalyzer_ApplyDefaults_EmptyProvided(t *testing.T) {
	inputs := []workflow.InputDefinition{
		{Name: "a", Type: "string", Default: "default_a"},
		{Name: "b", Type: "number", Default: 42},
		{Name: "c", Type: "boolean", Default: true},
	}
	provided := map[string]interface{}{}

	ia := NewInputAnalyzer(inputs, provided)
	result := ia.ApplyDefaults()

	if len(result) != 3 {
		t.Errorf("ApplyDefaults() returned %d items, want 3", len(result))
	}

	if result["a"] != "default_a" {
		t.Errorf("result[a] = %v, want 'default_a'", result["a"])
	}

	if result["b"] != 42 {
		t.Errorf("result[b] = %v, want 42", result["b"])
	}

	if result["c"] != true {
		t.Errorf("result[c] = %v, want true", result["c"])
	}
}

func TestInputAnalyzer_ApplyDefaults_NilDefault(t *testing.T) {
	inputs := []workflow.InputDefinition{
		{Name: "required", Type: "string", Default: nil},
	}
	provided := map[string]interface{}{}

	ia := NewInputAnalyzer(inputs, provided)
	result := ia.ApplyDefaults()

	// nil defaults should not be added
	if _, exists := result["required"]; exists {
		t.Error("ApplyDefaults() should not add nil defaults")
	}
}

func TestInputAnalyzer_ComplexScenario(t *testing.T) {
	// Inputs without defaults are required
	inputs := []workflow.InputDefinition{
		{Name: "required_no_default", Type: "string"},
		{Name: "optional_with_default", Type: "string", Default: "default"},
		{Name: "another_optional", Type: "number", Default: 100},
		{Name: "provided_required", Type: "string"},
		{Name: "provided_optional", Type: "boolean", Default: false},
	}
	provided := map[string]interface{}{
		"provided_required": "value",
		"provided_optional": true,
	}

	ia := NewInputAnalyzer(inputs, provided)

	// Test FindMissingInputs
	// Should only find: required_no_default (no default, not provided)
	missing := ia.FindMissingInputs()

	expectedMissingCount := 1 // only required_no_default
	if len(missing) != expectedMissingCount {
		t.Errorf("FindMissingInputs() returned %d items, want %d", len(missing), expectedMissingCount)
		for _, m := range missing {
			t.Logf("  missing: %s", m.Name)
		}
	}

	if len(missing) > 0 && missing[0].Name != "required_no_default" {
		t.Errorf("missing[0].Name = %q, want 'required_no_default'", missing[0].Name)
	}

	// Test ApplyDefaults
	result := ia.ApplyDefaults()
	// Should have: provided_required, provided_optional, optional_with_default, another_optional
	expectedCount := 4
	if len(result) != expectedCount {
		t.Errorf("ApplyDefaults() returned %d items, want %d", len(result), expectedCount)
		for k, v := range result {
			t.Logf("  result[%s] = %v", k, v)
		}
	}

	if result["provided_required"] != "value" {
		t.Errorf("result[provided_required] = %v, want 'value'", result["provided_required"])
	}

	if result["provided_optional"] != true {
		t.Errorf("result[provided_optional] = %v, want true", result["provided_optional"])
	}

	if result["optional_with_default"] != "default" {
		t.Errorf("result[optional_with_default] = %v, want 'default'", result["optional_with_default"])
	}

	if result["another_optional"] != 100 {
		t.Errorf("result[another_optional] = %v, want 100", result["another_optional"])
	}
}

func TestMissingInput(t *testing.T) {
	mi := MissingInput{
		Name:        "test",
		Type:        "string",
		Description: "test input",
		Enum:        []string{"a", "b"},
	}

	if mi.Name != "test" {
		t.Errorf("Name = %q, want 'test'", mi.Name)
	}

	if mi.Type != "string" {
		t.Errorf("Type = %q, want 'string'", mi.Type)
	}

	if mi.Description != "test input" {
		t.Errorf("Description = %q, want 'test input'", mi.Description)
	}

	if len(mi.Enum) != 2 {
		t.Errorf("Enum length = %d, want 2", len(mi.Enum))
	}
}
