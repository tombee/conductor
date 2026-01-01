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

package shared

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/tombee/conductor/pkg/httpclient"
)

// BuildAPIURL constructs a full API URL with query parameters
func BuildAPIURL(path string, params map[string]string) string {
	// Get controller URL from environment or use default
	baseURL := os.Getenv("CONDUCTOR_CONTROLLER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Build URL with parameters
	u, err := url.Parse(baseURL + path)
	if err != nil {
		return baseURL + path
	}

	if len(params) > 0 {
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	return u.String()
}

// MakeAPIRequest makes an HTTP request to the controller API
func MakeAPIRequest(method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add authentication if available
	if token := os.Getenv("CONDUCTOR_API_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Create HTTP client using shared httpclient package
	cfg := httpclient.DefaultConfig()
	cfg.UserAgent = "conductor-cli/1.0"

	client, err := httpclient.New(cfg)
	if err != nil {
		// Fallback to basic client
		client = &http.Client{}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
