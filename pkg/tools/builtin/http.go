package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/tracing"
	"github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/pkg/httpclient"
	"github.com/tombee/conductor/pkg/security"
	"github.com/tombee/conductor/pkg/tools"
)

// HTTPTool provides HTTP request capabilities.
type HTTPTool struct {
	// timeout sets the maximum request time
	timeout time.Duration

	// allowedHosts restricts which hosts can be accessed (normalized to lowercase)
	// If empty, all hosts are allowed
	allowedHosts []string

	// allowSubdomains enables subdomain matching for allowed hosts (default: false)
	allowSubdomains bool

	// blockPrivateIPs blocks requests to private IP ranges (default: true)
	blockPrivateIPs bool

	// requireHTTPS requires HTTPS scheme for all requests (default: false)
	requireHTTPS bool

	// logger for security audit logging (optional)
	logger *slog.Logger

	// client is the HTTP client
	client *http.Client

	// securityConfig provides enhanced security controls
	securityConfig *security.HTTPSecurityConfig

	// dnsCache caches DNS resolutions to prevent rebinding
	dnsCache *security.DNSCache
}

// NewHTTPTool creates a new HTTP tool with default settings.
// Secure defaults: allowSubdomains=false, blockPrivateIPs=true, requireHTTPS=false
func NewHTTPTool() *HTTPTool {
	secConfig := security.DefaultHTTPSecurityConfig()
	dnsCache := security.NewDNSCache(secConfig.DNSCacheTimeout)

	t := &HTTPTool{
		timeout:         30 * time.Second,
		allowedHosts:    []string{},
		allowSubdomains: false,
		blockPrivateIPs: true,
		requireHTTPS:    false,
		logger:          nil,
		securityConfig:  secConfig,
		dnsCache:        dnsCache,
	}
	t.client = t.createHTTPClient()
	return t
}

// WithTimeout sets the HTTP request timeout.
func (t *HTTPTool) WithTimeout(timeout time.Duration) *HTTPTool {
	t.timeout = timeout
	t.client = t.createHTTPClient()
	return t
}

// WithAllowedHosts restricts which hosts can be accessed.
// Hosts are normalized to lowercase for case-insensitive comparison.
// Empty hosts and duplicates are automatically filtered out.
func (t *HTTPTool) WithAllowedHosts(hosts []string) *HTTPTool {
	normalized := make([]string, 0, len(hosts))
	seen := make(map[string]bool)
	for _, host := range hosts {
		if host == "" {
			continue
		}
		lower := strings.ToLower(strings.TrimSpace(host))
		if !seen[lower] {
			normalized = append(normalized, lower)
			seen[lower] = true
		}
	}
	t.allowedHosts = normalized
	t.client = t.createHTTPClient()
	return t
}

// WithSubdomainMatching enables or disables subdomain matching for allowed hosts.
// When enabled, "example.com" will allow "api.example.com".
// Default: false (secure by default)
func (t *HTTPTool) WithSubdomainMatching(allow bool) *HTTPTool {
	t.allowSubdomains = allow
	t.client = t.createHTTPClient()
	return t
}

// WithBlockPrivateIPs enables or disables blocking of private IP addresses.
// When enabled, requests to RFC1918, loopback, and link-local addresses are blocked.
// Default: true (defense in depth)
func (t *HTTPTool) WithBlockPrivateIPs(block bool) *HTTPTool {
	t.blockPrivateIPs = block
	t.client = t.createHTTPClient()
	return t
}

// WithRequireHTTPS enables or disables HTTPS requirement.
// When enabled, only https:// URLs are allowed.
// Default: false (backward compatibility)
func (t *HTTPTool) WithRequireHTTPS(require bool) *HTTPTool {
	t.requireHTTPS = require
	t.client = t.createHTTPClient()
	return t
}

// WithLogger sets the logger for security audit logging.
// If nil, logging is disabled.
func (t *HTTPTool) WithLogger(logger *slog.Logger) *HTTPTool {
	t.logger = logger
	return t
}

// WithSecurityConfig sets the security configuration.
func (t *HTTPTool) WithSecurityConfig(config *security.HTTPSecurityConfig) *HTTPTool {
	t.securityConfig = config
	// Sync blockPrivateIPs from security config
	t.blockPrivateIPs = config.DenyPrivateIPs
	// Update DNS cache timeout if changed
	if config.DNSCacheTimeout > 0 {
		t.dnsCache = security.NewDNSCache(config.DNSCacheTimeout)
	}
	// Update client with secure transport
	if config != nil {
		// Create base client using shared httpclient package
		cfg := httpclient.DefaultConfig()
		cfg.Timeout = t.timeout
		cfg.UserAgent = "conductor-http-tool/1.0"

		baseClient, err := httpclient.New(cfg)
		if err != nil {
			// Fallback to basic client if creation fails
			baseClient = &http.Client{Timeout: t.timeout}
		}

		// Override transport with security-enhanced version
		baseClient.Transport = &http.Transport{
			DialContext: config.SecureDialContext(t.dnsCache),
		}

		// Set redirect validation
		baseClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= config.MaxRedirects {
				return fmt.Errorf("too many redirects")
			}
			// Validate redirect URL if configured
			if config.ValidateRedirects {
				if err := config.ValidateURL(req.URL.String()); err != nil {
					return fmt.Errorf("redirect blocked: %w", err)
				}
			}
			return nil
		}

		t.client = baseClient
	}
	return t
}

// Name returns the tool identifier.
func (t *HTTPTool) Name() string {
	return "http"
}

// Description returns a human-readable description.
func (t *HTTPTool) Description() string {
	return "Make HTTP requests to external APIs"
}

// Schema returns the tool's input/output schema.
func (t *HTTPTool) Schema() *tools.Schema {
	return &tools.Schema{
		Inputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"method": {
					Type:        "string",
					Description: "HTTP method (GET, POST, PUT, DELETE, etc.)",
					Default:     "GET",
				},
				"url": {
					Type:        "string",
					Description: "The URL to request",
					Format:      "uri",
				},
				"headers": {
					Type:        "object",
					Description: "HTTP headers to include (optional)",
				},
				"body": {
					Type:        "string",
					Description: "Request body (optional, for POST/PUT)",
				},
			},
			Required: []string{"url"},
		},
		Outputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"success": {
					Type:        "boolean",
					Description: "Whether the request succeeded (2xx status)",
				},
				"status_code": {
					Type:        "number",
					Description: "HTTP status code",
				},
				"headers": {
					Type:        "object",
					Description: "Response headers",
				},
				"body": {
					Type:        "string",
					Description: "Response body",
				},
				"error": {
					Type:        "string",
					Description: "Error message if request failed",
				},
			},
		},
	}
}

// Execute performs an HTTP request.
func (t *HTTPTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Extract URL
	url, ok := inputs["url"].(string)
	if !ok {
		return nil, &errors.ValidationError{
			Field:      "url",
			Message:    "url must be a string",
			Suggestion: "Provide a valid URL as a string",
		}
	}

	// Validate URL
	if err := t.validateURL(url); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	// Validate with security config
	if t.securityConfig != nil {
		if err := t.securityConfig.ValidateURL(url); err != nil {
			return nil, fmt.Errorf("security validation failed for URL %s: %w", url, err)
		}
	}

	// Extract method (default: GET)
	method := "GET"
	if methodRaw, ok := inputs["method"]; ok {
		method, ok = methodRaw.(string)
		if !ok {
			return nil, &errors.ValidationError{
				Field:      "method",
				Message:    "method must be a string",
				Suggestion: "Provide HTTP method as a string (GET, POST, PUT, DELETE, etc.)",
			}
		}
		method = strings.ToUpper(method)
	}

	// Validate method with security config
	if t.securityConfig != nil {
		if err := t.securityConfig.ValidateMethod(method); err != nil {
			return nil, fmt.Errorf("security validation failed for method %s: %w", method, err)
		}
	}

	// Extract body (optional)
	var body io.Reader
	if bodyRaw, ok := inputs["body"]; ok {
		bodyStr, ok := bodyRaw.(string)
		if !ok {
			return nil, &errors.ValidationError{
				Field:      "body",
				Message:    "body must be a string",
				Suggestion: "Provide request body as a string",
			}
		}
		body = bytes.NewBufferString(bodyStr)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	// Extract and set headers (optional)
	if headersRaw, ok := inputs["headers"]; ok {
		headers, ok := headersRaw.(map[string]interface{})
		if !ok {
			return nil, &errors.ValidationError{
				Field:      "headers",
				Message:    "headers must be an object",
				Suggestion: "Provide headers as a map of header names to values",
			}
		}
		for key, value := range headers {
			valueStr, ok := value.(string)
			if !ok {
				return nil, &errors.ValidationError{
					Field:      fmt.Sprintf("headers.%s", key),
					Message:    "header values must be strings",
					Suggestion: "Ensure all header values are strings",
				}
			}
			req.Header.Set(key, valueStr)
		}
	}

	// Set default Content-Type for POST/PUT if not specified
	if (method == "POST" || method == "PUT") && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Inject correlation ID for distributed tracing
	tracing.InjectIntoRequest(ctx, req)

	// Validate headers with security config
	if t.securityConfig != nil {
		if err := t.securityConfig.ValidateHeaders(req.Header); err != nil {
			return nil, fmt.Errorf("security validation failed: %w", err)
		}
	}

	// Execute request
	resp, err := t.client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"success":     false,
			"status_code": resp.StatusCode,
			"error":       fmt.Sprintf("failed to read response body: %v", err),
		}, nil
	}

	// Convert response headers to map
	headers := make(map[string]interface{})
	for key, values := range resp.Header {
		if len(values) == 1 {
			headers[key] = values[0]
		} else {
			headers[key] = values
		}
	}

	// Determine success (2xx status codes)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	return map[string]interface{}{
		"success":     success,
		"status_code": resp.StatusCode,
		"headers":     headers,
		"body":        string(respBody),
	}, nil
}

// validateURL checks if a URL is allowed using proper URL parsing and hostname matching.
// This prevents SSRF attacks via substring matching bypasses.
func (t *HTTPTool) validateURL(rawURL string) error {
	// Parse URL using standard library
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("URL parsing failed", "url", rawURL, "error", err)
		}
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Validate scheme (only http/https allowed)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		if t.logger != nil {
			t.logger.Warn("invalid URL scheme blocked", "scheme", parsedURL.Scheme, "url", rawURL)
		}
		return fmt.Errorf("invalid URL scheme: only http/https allowed")
	}

	// Check HTTPS requirement
	if t.requireHTTPS && parsedURL.Scheme != "https" {
		if t.logger != nil {
			t.logger.Warn("non-HTTPS URL blocked", "url", rawURL)
		}
		return fmt.Errorf("HTTPS required")
	}

	// Extract hostname without port, normalized to lowercase
	hostname := strings.ToLower(parsedURL.Hostname())
	if hostname == "" {
		if t.logger != nil {
			t.logger.Warn("empty hostname blocked", "url", rawURL)
		}
		return fmt.Errorf("invalid URL: empty hostname")
	}

	// Block private IPs (defense in depth)
	if t.blockPrivateIPs {
		if ip := net.ParseIP(hostname); ip != nil {
			if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				if t.logger != nil {
					t.logger.Warn("private IP address blocked", "ip", hostname, "url", rawURL)
				}
				return fmt.Errorf("requests to private IP addresses not allowed")
			}
		}
	}

	// Check against allowed hosts list
	if len(t.allowedHosts) > 0 {
		allowed := false
		for _, allowedHost := range t.allowedHosts {
			// Exact hostname match (already normalized to lowercase)
			if hostname == allowedHost {
				allowed = true
				break
			}
			// Subdomain matching (if enabled)
			if t.allowSubdomains && strings.HasSuffix(hostname, "."+allowedHost) {
				allowed = true
				break
			}
		}
		if !allowed {
			// Log for security audit, but don't expose allowed hosts list in error
			if t.logger != nil {
				t.logger.Warn("hostname not in allowed list", "hostname", hostname, "url", rawURL)
			}
			return fmt.Errorf("host not in allowed list")
		}
	}

	return nil
}

// createHTTPClient creates an HTTP client using the shared httpclient package
// for consistent retry/logging behavior, with redirect validation for SSRF protection.
func (t *HTTPTool) createHTTPClient() *http.Client {
	// Create base client with shared httpclient package
	cfg := httpclient.DefaultConfig()
	cfg.Timeout = t.timeout
	cfg.UserAgent = "conductor-http-tool/1.0"

	baseClient, err := httpclient.New(cfg)
	if err != nil {
		// Fallback to default client if httpclient creation fails
		// This should never happen with valid config, but provides safety
		baseClient = &http.Client{Timeout: t.timeout}
	}

	// Wrap with redirect validation for SSRF protection
	baseClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Limit redirect chain length (Go default is 10)
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		// Validate redirect target URL
		if err := t.validateURL(req.URL.String()); err != nil {
			if t.logger != nil {
				t.logger.Warn("redirect target blocked", "url", req.URL.String(), "error", err)
			}
			return fmt.Errorf("redirect target not allowed: %w", err)
		}
		return nil
	}

	return baseClient
}

// ParseJSON is a helper to parse JSON response bodies.
func ParseJSON(body string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}
