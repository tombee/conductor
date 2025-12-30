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

package record

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestNewRecorder(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		FixturesDir: tmpDir,
	}

	recorder, err := NewRecorder(cfg)
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	// Check that .recorded directory was created
	recordedDir := filepath.Join(tmpDir, ".recorded")
	if _, err := os.Stat(recordedDir); os.IsNotExist(err) {
		t.Errorf("Expected .recorded directory to be created")
	}

	// Check permissions
	info, err := os.Stat(recordedDir)
	if err != nil {
		t.Fatalf("Failed to stat .recorded directory: %v", err)
	}

	perm := info.Mode().Perm()
	expected := os.FileMode(0750)
	if perm != expected {
		t.Errorf("Expected directory permissions %v, got %v", expected, perm)
	}

	if recorder.redactor == nil {
		t.Errorf("Expected default redactor to be created")
	}
}

func TestRecordLLM(t *testing.T) {
	tmpDir := t.TempDir()

	recorder, err := NewRecorder(Config{
		FixturesDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	// Record an LLM response with a sensitive API key
	response := "Here is the response with sk-1234567890abcdefghij"
	err = recorder.RecordLLM("test_step", response, "claude-sonnet-4", 1500*time.Millisecond, 100, 50)
	if err != nil {
		t.Fatalf("RecordLLM() error = %v", err)
	}

	// Read the fixture file
	filePath := filepath.Join(tmpDir, ".recorded", "test_step.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read fixture file: %v", err)
	}

	// Check file permissions (NFR14)
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat fixture file: %v", err)
	}
	perm := info.Mode().Perm()
	expected := os.FileMode(0600)
	if perm != expected {
		t.Errorf("Expected file permissions %v, got %v", expected, perm)
	}

	// Parse the fixture
	var fixture LLMResponse
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("Failed to unmarshal fixture: %v", err)
	}

	// Verify redaction
	if fixture.Response == response {
		t.Errorf("Expected API key to be redacted, got original response")
	}
	if fixture.Response != "Here is the response with [REDACTED-OPENAI-KEY]" {
		t.Errorf("Expected redacted response, got: %s", fixture.Response)
	}

	// Verify metadata
	if fixture.Metadata.Model != "claude-sonnet-4" {
		t.Errorf("Expected model 'claude-sonnet-4', got: %s", fixture.Metadata.Model)
	}
	if fixture.Metadata.DurationMs != 1500 {
		t.Errorf("Expected duration 1500ms, got: %d", fixture.Metadata.DurationMs)
	}
	if fixture.Metadata.PromptTokens != 100 {
		t.Errorf("Expected 100 prompt tokens, got: %d", fixture.Metadata.PromptTokens)
	}
	if fixture.Metadata.CompletionTokens != 50 {
		t.Errorf("Expected 50 completion tokens, got: %d", fixture.Metadata.CompletionTokens)
	}

	// Verify comment
	if fixture.Comment == "" {
		t.Errorf("Expected comment to be set")
	}
}

func TestRecordHTTP(t *testing.T) {
	tmpDir := t.TempDir()

	recorder, err := NewRecorder(Config{
		FixturesDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	// Record an HTTP response with sensitive data
	reqHeaders := map[string]string{
		"Authorization": "Bearer secret-token",
		"Content-Type":  "application/json",
	}
	reqBody := map[string]interface{}{
		"api_key": "sk-1234567890abcdefghij",
		"data":    "test",
	}

	respHeaders := map[string]string{
		"Content-Type": "application/json",
	}
	respBody := map[string]interface{}{
		"result": "success",
		"token":  "ghp_1234567890abcdefghijklmnopqrstuvwx",
	}

	err = recorder.RecordHTTP("http_step", "POST", "https://api.example.com/endpoint",
		reqHeaders, reqBody, 200, respHeaders, respBody)
	if err != nil {
		t.Fatalf("RecordHTTP() error = %v", err)
	}

	// Read and parse the fixture
	filePath := filepath.Join(tmpDir, ".recorded", "http_step.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read fixture file: %v", err)
	}

	var fixture HTTPResponse
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("Failed to unmarshal fixture: %v", err)
	}

	// Verify request header redaction
	if fixture.Request.Headers["Authorization"] != "[REDACTED]" {
		t.Errorf("Expected Authorization header to be redacted, got: %s", fixture.Request.Headers["Authorization"])
	}
	if fixture.Request.Headers["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type to be preserved, got: %s", fixture.Request.Headers["Content-Type"])
	}

	// Verify request body redaction
	reqBodyMap := fixture.Request.Body.(map[string]interface{})
	if reqBodyMap["api_key"] != "[REDACTED]" {
		t.Errorf("Expected api_key to be redacted, got: %v", reqBodyMap["api_key"])
	}
	if reqBodyMap["data"] != "test" {
		t.Errorf("Expected data to be preserved, got: %v", reqBodyMap["data"])
	}

	// Verify response body redaction
	respBodyMap := fixture.Response.Body.(map[string]interface{})
	if respBodyMap["result"] != "success" {
		t.Errorf("Expected result to be preserved, got: %v", respBodyMap["result"])
	}
	// The GitHub token should be redacted by pattern matching
	if respBodyMap["token"] == "ghp_1234567890abcdefghijklmnopqrstuvwx" {
		t.Errorf("Expected token to be redacted, got original value")
	}
}

func TestRecordIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	recorder, err := NewRecorder(Config{
		FixturesDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	// Record an integration response with sensitive data
	params := map[string]interface{}{
		"repo":  "test/repo",
		"token": "ghp_1234567890abcdefghijklmnopqrstuvwx",
	}

	response := map[string]interface{}{
		"number":   42,
		"html_url": "https://github.com/test/repo/issues/42",
	}

	err = recorder.RecordIntegration("create_issue", "create_issue", params, response)
	if err != nil {
		t.Fatalf("RecordIntegration() error = %v", err)
	}

	// Read and parse the fixture
	filePath := filepath.Join(tmpDir, ".recorded", "create_issue.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read fixture file: %v", err)
	}

	var fixture IntegrationResponse
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("Failed to unmarshal fixture: %v", err)
	}

	// Verify param redaction
	if fixture.Request.Params["repo"] != "test/repo" {
		t.Errorf("Expected repo to be preserved, got: %v", fixture.Request.Params["repo"])
	}
	if fixture.Request.Params["token"] == "ghp_1234567890abcdefghijklmnopqrstuvwx" {
		t.Errorf("Expected token to be redacted, got original value")
	}

	// Verify response
	respMap := fixture.Response.(map[string]interface{})
	if respMap["number"] != 42 {
		t.Errorf("Expected number to be preserved, got: %v", respMap["number"])
	}
}

func TestRecorder_NestedRedaction(t *testing.T) {
	tmpDir := t.TempDir()

	recorder, err := NewRecorder(Config{
		FixturesDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	// Test nested structures
	body := map[string]interface{}{
		"user": map[string]interface{}{
			"name":     "alice",
			"password": "secret123",
		},
		"items": []interface{}{
			map[string]interface{}{
				"id":    1,
				"token": "ghp_1234567890abcdefghijklmnopqrstuvwx",
			},
			map[string]interface{}{
				"id":    2,
				"token": "sk-1234567890abcdefghij",
			},
		},
	}

	err = recorder.RecordHTTP("nested_step", "POST", "https://api.example.com",
		nil, body, 200, nil, nil)
	if err != nil {
		t.Fatalf("RecordHTTP() error = %v", err)
	}

	// Read and verify
	filePath := filepath.Join(tmpDir, ".recorded", "nested_step.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read fixture file: %v", err)
	}

	var fixture HTTPResponse
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("Failed to unmarshal fixture: %v", err)
	}

	bodyMap := fixture.Request.Body.(map[string]interface{})
	userMap := bodyMap["user"].(map[string]interface{})
	if userMap["password"] != "[REDACTED]" {
		t.Errorf("Expected nested password to be redacted, got: %v", userMap["password"])
	}

	items := bodyMap["items"].([]interface{})
	item1 := items[0].(map[string]interface{})
	if item1["token"] == "ghp_1234567890abcdefghijklmnopqrstuvwx" {
		t.Errorf("Expected token in array to be redacted")
	}

	item2 := items[1].(map[string]interface{})
	if item2["token"] == "sk-1234567890abcdefghij" {
		t.Errorf("Expected API key in array to be redacted")
	}
}
