package operation

import (
	"os"
	"time"
)

// DefaultFieldInjector provides auto-population of common fields for observability connectors.
type DefaultFieldInjector struct {
	hostname  string
	timestamp int64
}

// NewDefaultFieldInjector creates a new default field injector.
func NewDefaultFieldInjector() *DefaultFieldInjector {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	return &DefaultFieldInjector{
		hostname:  hostname,
		timestamp: time.Now().Unix(),
	}
}

// InjectDefaults adds default fields to inputs if they are not already present.
// This is used for observability connectors that benefit from auto-populated
// timestamp and hostname fields.
func (d *DefaultFieldInjector) InjectDefaults(inputs map[string]interface{}, connectorName string) {
	// Only inject defaults for known observability connectors
	if !isObservabilityConnector(connectorName) {
		return
	}

	// Inject timestamp if not present
	// Different connectors use different field names and formats
	switch connectorName {
	case "datadog":
		// Datadog uses Unix timestamp in seconds
		if _, exists := inputs["timestamp"]; !exists {
			inputs["timestamp"] = d.timestamp
		}
		if _, exists := inputs["date_happened"]; !exists {
			inputs["date_happened"] = d.timestamp
		}
		if _, exists := inputs["hostname"]; !exists {
			inputs["hostname"] = d.hostname
		}
		if _, exists := inputs["host"]; !exists {
			inputs["host"] = d.hostname
		}

	case "splunk":
		// Splunk uses Unix timestamp in seconds
		if _, exists := inputs["time"]; !exists {
			inputs["time"] = d.timestamp
		}
		if _, exists := inputs["host"]; !exists {
			inputs["host"] = d.hostname
		}

	case "cloudwatch":
		// CloudWatch uses Unix timestamp in milliseconds
		if _, exists := inputs["timestamp"]; !exists {
			inputs["timestamp"] = d.timestamp * 1000
		}

	case "loki":
		// Loki uses Unix nanoseconds
		if _, exists := inputs["timestamp"]; !exists {
			// Convert to nanoseconds
			inputs["timestamp"] = d.timestamp * 1000000000
		}

	case "elasticsearch":
		// Elasticsearch uses ISO 8601 timestamp
		if doc, ok := inputs["document"].(map[string]interface{}); ok {
			if _, exists := doc["@timestamp"]; !exists {
				doc["@timestamp"] = time.Unix(d.timestamp, 0).UTC().Format(time.RFC3339)
			}
		}
	}
}

// isObservabilityConnector returns true if the connector is an observability platform.
func isObservabilityConnector(name string) bool {
	observabilityConnectors := map[string]bool{
		"datadog":       true,
		"splunk":        true,
		"cloudwatch":    true,
		"loki":          true,
		"elasticsearch": true,
	}
	return observabilityConnectors[name]
}
