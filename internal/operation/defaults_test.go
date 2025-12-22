package operation

import (
	"testing"
	"time"
)

func TestDefaultFieldInjector_InjectDefaults(t *testing.T) {
	injector := NewDefaultFieldInjector()

	tests := []struct {
		name            string
		integrationName string
		inputs          map[string]interface{}
		wantFields      map[string]bool // Fields that should exist after injection
		wantNotFields   map[string]bool // Fields that should NOT be overridden
	}{
		{
			name:            "datadog log with no defaults",
			integrationName: "datadog",
			inputs: map[string]interface{}{
				"message": "test log",
			},
			wantFields: map[string]bool{
				"timestamp": true,
				"hostname":  true,
			},
		},
		{
			name:            "datadog log with existing timestamp",
			integrationName: "datadog",
			inputs: map[string]interface{}{
				"message":   "test log",
				"timestamp": int64(1234567890),
			},
			wantNotFields: map[string]bool{
				"timestamp": true, // Should not override
			},
		},
		{
			name:            "splunk event with no defaults",
			integrationName: "splunk",
			inputs: map[string]interface{}{
				"event": "test event",
			},
			wantFields: map[string]bool{
				"time": true,
				"host": true,
			},
		},
		{
			name:            "cloudwatch log with no defaults",
			integrationName: "cloudwatch",
			inputs: map[string]interface{}{
				"message": "test log",
			},
			wantFields: map[string]bool{
				"timestamp": true,
			},
		},
		{
			name:            "loki push with no defaults",
			integrationName: "loki",
			inputs: map[string]interface{}{
				"line": "test log",
			},
			wantFields: map[string]bool{
				"timestamp": true,
			},
		},
		{
			name:            "elasticsearch index with document",
			integrationName: "elasticsearch",
			inputs: map[string]interface{}{
				"document": map[string]interface{}{
					"message": "test doc",
				},
			},
			wantFields: map[string]bool{
				"@timestamp": true, // Should be in document
			},
		},
		{
			name:            "non-observability integration should not inject",
			integrationName: "slack",
			inputs: map[string]interface{}{
				"message": "test",
			},
			wantNotFields: map[string]bool{
				"timestamp": true,
				"hostname":  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of inputs to avoid mutation between tests
			inputsCopy := make(map[string]interface{})
			for k, v := range tt.inputs {
				inputsCopy[k] = v
			}

			// Inject defaults
			injector.InjectDefaults(inputsCopy, tt.integrationName)

			// Check wanted fields exist
			for field := range tt.wantFields {
				// For elasticsearch, check inside document
				if tt.integrationName == "elasticsearch" && field == "@timestamp" {
					doc, ok := inputsCopy["document"].(map[string]interface{})
					if !ok {
						t.Errorf("document field is not a map")
						continue
					}
					if _, exists := doc[field]; !exists {
						t.Errorf("expected field %q to be injected in document, but it wasn't", field)
					}
				} else {
					if _, exists := inputsCopy[field]; !exists {
						t.Errorf("expected field %q to be injected, but it wasn't", field)
					}
				}
			}

			// Check unwanted fields don't exist or weren't overridden
			for field := range tt.wantNotFields {
				if originalValue, existed := tt.inputs[field]; existed {
					// Field existed in original, check it wasn't overridden
					if currentValue := inputsCopy[field]; currentValue != originalValue {
						t.Errorf("field %q was overridden (original: %v, current: %v)", field, originalValue, currentValue)
					}
				} else {
					// Field didn't exist, check it wasn't added
					if _, exists := inputsCopy[field]; exists {
						t.Errorf("field %q should not have been injected", field)
					}
				}
			}
		})
	}
}

func TestDefaultFieldInjector_DatadogEventDefaults(t *testing.T) {
	injector := NewDefaultFieldInjector()

	inputs := map[string]interface{}{
		"title": "Test Event",
		"text":  "Event description",
	}

	injector.InjectDefaults(inputs, "datadog")

	// Should inject date_happened for events
	if _, exists := inputs["date_happened"]; !exists {
		t.Error("expected date_happened to be injected for Datadog event")
	}

	// Should inject host
	if _, exists := inputs["host"]; !exists {
		t.Error("expected host to be injected for Datadog event")
	}
}

func TestDefaultFieldInjector_CloudWatchMilliseconds(t *testing.T) {
	injector := NewDefaultFieldInjector()

	inputs := map[string]interface{}{
		"message": "Test log",
	}

	injector.InjectDefaults(inputs, "cloudwatch")

	timestamp, ok := inputs["timestamp"].(int64)
	if !ok {
		t.Fatal("timestamp not set or not int64")
	}

	// CloudWatch timestamp should be in milliseconds
	// Check it's reasonable (within the last day and not more than 1s in the future)
	now := time.Now().Unix() * 1000
	if timestamp < now-86400000 || timestamp > now+1000 {
		t.Errorf("CloudWatch timestamp %d is not in milliseconds or is out of reasonable range", timestamp)
	}
}

func TestDefaultFieldInjector_LokiNanoseconds(t *testing.T) {
	injector := NewDefaultFieldInjector()

	inputs := map[string]interface{}{
		"line": "Test log",
	}

	injector.InjectDefaults(inputs, "loki")

	timestamp, ok := inputs["timestamp"].(int64)
	if !ok {
		t.Fatal("timestamp not set or not int64")
	}

	// Loki timestamp should be in nanoseconds
	// Check it's reasonable (within the last day)
	now := time.Now().UnixNano()
	if timestamp < now-86400000000000 || timestamp > now+1000000000 {
		t.Errorf("Loki timestamp %d is not in nanoseconds or is out of reasonable range", timestamp)
	}
}

func TestIsObservabilityIntegration(t *testing.T) {
	tests := []struct {
		name        string
		integration string
		want        bool
	}{
		{"datadog is observability", "datadog", true},
		{"splunk is observability", "splunk", true},
		{"cloudwatch is observability", "cloudwatch", true},
		{"loki is observability", "loki", true},
		{"elasticsearch is observability", "elasticsearch", true},
		{"slack is not observability", "slack", false},
		{"github is not observability", "github", false},
		{"jira is not observability", "jira", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isObservabilityIntegration(tt.integration); got != tt.want {
				t.Errorf("isObservabilityIntegration(%q) = %v, want %v", tt.integration, got, tt.want)
			}
		})
	}
}
