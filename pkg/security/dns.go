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

package security

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// DNSSecurityConfig defines security controls for DNS queries.
type DNSSecurityConfig struct {
	// AllowedDNSServers restricts to specific DNS servers
	// Empty list = use system default
	AllowedDNSServers []string `yaml:"allowed_dns_servers,omitempty" json:"allowed_dns_servers,omitempty"`

	// RebindingPrevention caches and validates DNS doesn't change during request
	RebindingPrevention bool `yaml:"rebinding_prevention" json:"rebinding_prevention"`

	// ExfiltrationLimits rate limits DNS queries
	ExfiltrationLimits DNSExfiltrationLimits `yaml:"exfiltration_limits,omitempty" json:"exfiltration_limits,omitempty"`

	// BlockDynamicDNS blocks known dynamic DNS providers
	BlockDynamicDNS bool `yaml:"block_dynamic_dns" json:"block_dynamic_dns"`

	// Allowlist contains domains that are exempt from dynamic DNS blocking (T11)
	Allowlist []string `yaml:"allowlist,omitempty" json:"allowlist,omitempty"`
}

// DNSExfiltrationLimits defines limits to prevent data exfiltration via DNS.
type DNSExfiltrationLimits struct {
	// MaxQueriesPerMinute rate limits unique subdomain queries
	MaxQueriesPerMinute int `yaml:"max_queries_per_minute" json:"max_queries_per_minute"`

	// MaxSubdomainDepth limits subdomain nesting (e.g., a.b.c.example.com = depth 3)
	MaxSubdomainDepth int `yaml:"max_subdomain_depth" json:"max_subdomain_depth"`

	// MaxLabelLength limits individual label length
	// Standard DNS limit is 63, but shorter may indicate encoding
	MaxLabelLength int `yaml:"max_label_length" json:"max_label_length"`
}

// DefaultDNSSecurityConfig returns a secure default configuration.
func DefaultDNSSecurityConfig() *DNSSecurityConfig {
	return &DNSSecurityConfig{
		AllowedDNSServers:   []string{},
		RebindingPrevention: true,
		ExfiltrationLimits: DNSExfiltrationLimits{
			MaxQueriesPerMinute: 100,
			MaxSubdomainDepth:   5,
			MaxLabelLength:      63,
		},
		BlockDynamicDNS: true,
	}
}

// DNSQueryMonitor tracks DNS queries to detect exfiltration attempts.
type DNSQueryMonitor struct {
	mu               sync.RWMutex
	config           DNSSecurityConfig
	queryHistory     map[string][]time.Time // domain -> query timestamps
	dynamicDNSSuffix []string
}

// NewDNSQueryMonitor creates a new DNS query monitor.
func NewDNSQueryMonitor(config DNSSecurityConfig) *DNSQueryMonitor {
	return &DNSQueryMonitor{
		config:       config,
		queryHistory: make(map[string][]time.Time),
		dynamicDNSSuffix: []string{
			".dyndns.org",
			".no-ip.com",
			".duckdns.org",
			".freedns.afraid.org",
			".ddns.net",
			".ngrok.io",
			".localhost.run",
			".tunnelto.dev",
		},
	}
}

// ValidateQuery validates a DNS query before execution.
func (m *DNSQueryMonitor) ValidateQuery(hostname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check dynamic DNS blocking
	if m.config.BlockDynamicDNS {
		if err := m.checkDynamicDNS(hostname); err != nil {
			return err
		}
	}

	// Check subdomain depth
	if m.config.ExfiltrationLimits.MaxSubdomainDepth > 0 {
		if err := m.checkSubdomainDepth(hostname); err != nil {
			return err
		}
	}

	// Check label length
	if m.config.ExfiltrationLimits.MaxLabelLength > 0 {
		if err := m.checkLabelLength(hostname); err != nil {
			return err
		}
	}

	// Check rate limiting
	if m.config.ExfiltrationLimits.MaxQueriesPerMinute > 0 {
		if err := m.checkRateLimit(hostname); err != nil {
			return err
		}
	}

	// Record query
	m.recordQuery(hostname)

	return nil
}

// checkDynamicDNS checks if hostname uses a dynamic DNS provider.
func (m *DNSQueryMonitor) checkDynamicDNS(hostname string) error {
	lowerHost := strings.ToLower(hostname)

	// T11: Check if hostname is in allowlist
	for _, allowed := range m.config.Allowlist {
		allowedLower := strings.ToLower(allowed)
		// Match exact domain or subdomain
		if lowerHost == allowedLower || strings.HasSuffix(lowerHost, "."+allowedLower) {
			return nil // Allowed, skip dynamic DNS check
		}
	}

	// Check against dynamic DNS providers
	for _, suffix := range m.dynamicDNSSuffix {
		if strings.HasSuffix(lowerHost, suffix) {
			return fmt.Errorf("dynamic DNS provider blocked: %s", hostname)
		}
	}
	return nil
}

// checkSubdomainDepth validates the subdomain nesting depth.
func (m *DNSQueryMonitor) checkSubdomainDepth(hostname string) error {
	// Count labels (parts separated by dots)
	labels := strings.Split(hostname, ".")

	// Remove empty labels
	validLabels := []string{}
	for _, label := range labels {
		if label != "" {
			validLabels = append(validLabels, label)
		}
	}

	// Depth is number of labels minus TLD and domain
	// e.g., "a.b.c.example.com" = 5 labels, typical depth would be 3
	// For simplicity, we count all labels
	depth := len(validLabels)

	if depth > m.config.ExfiltrationLimits.MaxSubdomainDepth {
		return fmt.Errorf("subdomain depth (%d) exceeds maximum (%d): %s",
			depth, m.config.ExfiltrationLimits.MaxSubdomainDepth, hostname)
	}

	return nil
}

// checkLabelLength validates individual label lengths.
func (m *DNSQueryMonitor) checkLabelLength(hostname string) error {
	labels := strings.Split(hostname, ".")

	for _, label := range labels {
		if len(label) > m.config.ExfiltrationLimits.MaxLabelLength {
			return fmt.Errorf("DNS label length (%d) exceeds maximum (%d): %s",
				len(label), m.config.ExfiltrationLimits.MaxLabelLength, label)
		}
	}

	return nil
}

// checkRateLimit enforces query rate limiting.
func (m *DNSQueryMonitor) checkRateLimit(hostname string) error {
	now := time.Now()
	cutoff := now.Add(-time.Minute)

	// Get query history for this domain
	history, exists := m.queryHistory[hostname]
	if !exists {
		return nil // First query is always allowed
	}

	// Count queries in last minute
	recentQueries := 0
	for _, timestamp := range history {
		if timestamp.After(cutoff) {
			recentQueries++
		}
	}

	if recentQueries >= m.config.ExfiltrationLimits.MaxQueriesPerMinute {
		return fmt.Errorf("DNS query rate limit exceeded for %s: %d queries/minute (max: %d)",
			hostname, recentQueries, m.config.ExfiltrationLimits.MaxQueriesPerMinute)
	}

	return nil
}

// recordQuery records a DNS query timestamp.
func (m *DNSQueryMonitor) recordQuery(hostname string) {
	now := time.Now()
	cutoff := now.Add(-time.Minute)

	// Get existing history
	history := m.queryHistory[hostname]

	// Prune old entries
	recentHistory := []time.Time{}
	for _, timestamp := range history {
		if timestamp.After(cutoff) {
			recentHistory = append(recentHistory, timestamp)
		}
	}

	// Add current query
	recentHistory = append(recentHistory, now)

	// Update history
	m.queryHistory[hostname] = recentHistory
}

// CleanupOldEntries removes query history older than the retention period.
func (m *DNSQueryMonitor) CleanupOldEntries() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute) // Keep 5 minutes of history

	for hostname, history := range m.queryHistory {
		recentHistory := []time.Time{}
		for _, timestamp := range history {
			if timestamp.After(cutoff) {
				recentHistory = append(recentHistory, timestamp)
			}
		}

		if len(recentHistory) == 0 {
			delete(m.queryHistory, hostname)
		} else {
			m.queryHistory[hostname] = recentHistory
		}
	}
}

// GetQueryStats returns statistics about DNS queries.
func (m *DNSQueryMonitor) GetQueryStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalDomains := len(m.queryHistory)
	totalQueries := 0
	for _, history := range m.queryHistory {
		totalQueries += len(history)
	}

	return map[string]interface{}{
		"total_domains": totalDomains,
		"total_queries": totalQueries,
		"monitoring":    true,
	}
}
