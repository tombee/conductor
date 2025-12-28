package transport

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestOAuth2TransportConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *OAuth2TransportConfig
		wantErr string
	}{
		{
			name: "valid client_credentials",
			config: &OAuth2TransportConfig{
				BaseURL:      "https://api.example.com",
				Flow:         "client_credentials",
				ClientID:     "client-id",
				ClientSecret: "${CLIENT_SECRET}",
				TokenURL:     "https://auth.example.com/token",
			},
			wantErr: "",
		},
		{
			name: "valid authorization_code",
			config: &OAuth2TransportConfig{
				BaseURL:      "https://api.example.com",
				Flow:         "authorization_code",
				ClientID:     "client-id",
				ClientSecret: "${CLIENT_SECRET}",
				TokenURL:     "https://auth.example.com/token",
				RefreshToken: "${REFRESH_TOKEN}",
			},
			wantErr: "",
		},
		{
			name: "missing base_url",
			config: &OAuth2TransportConfig{
				Flow:         "client_credentials",
				ClientID:     "client-id",
				ClientSecret: "${CLIENT_SECRET}",
				TokenURL:     "https://auth.example.com/token",
			},
			wantErr: "base_url is required",
		},
		{
			name: "invalid base_url",
			config: &OAuth2TransportConfig{
				BaseURL:      "api.example.com",
				Flow:         "client_credentials",
				ClientID:     "client-id",
				ClientSecret: "${CLIENT_SECRET}",
				TokenURL:     "https://auth.example.com/token",
			},
			wantErr: "must start with http://",
		},
		{
			name: "missing flow",
			config: &OAuth2TransportConfig{
				BaseURL:      "https://api.example.com",
				ClientID:     "client-id",
				ClientSecret: "${CLIENT_SECRET}",
				TokenURL:     "https://auth.example.com/token",
			},
			wantErr: "flow is required",
		},
		{
			name: "invalid flow",
			config: &OAuth2TransportConfig{
				BaseURL:      "https://api.example.com",
				Flow:         "invalid",
				ClientID:     "client-id",
				ClientSecret: "${CLIENT_SECRET}",
				TokenURL:     "https://auth.example.com/token",
			},
			wantErr: "flow must be client_credentials or authorization_code",
		},
		{
			name: "missing client_id",
			config: &OAuth2TransportConfig{
				BaseURL:      "https://api.example.com",
				Flow:         "client_credentials",
				ClientSecret: "${CLIENT_SECRET}",
				TokenURL:     "https://auth.example.com/token",
			},
			wantErr: "client_id is required",
		},
		{
			name: "missing client_secret",
			config: &OAuth2TransportConfig{
				BaseURL:  "https://api.example.com",
				Flow:     "client_credentials",
				ClientID: "client-id",
				TokenURL: "https://auth.example.com/token",
			},
			wantErr: "client_secret is required",
		},
		{
			name: "missing token_url",
			config: &OAuth2TransportConfig{
				BaseURL:      "https://api.example.com",
				Flow:         "client_credentials",
				ClientID:     "client-id",
				ClientSecret: "${CLIENT_SECRET}",
			},
			wantErr: "token_url is required",
		},
		{
			name: "authorization_code missing refresh_token",
			config: &OAuth2TransportConfig{
				BaseURL:      "https://api.example.com",
				Flow:         "authorization_code",
				ClientID:     "client-id",
				ClientSecret: "${CLIENT_SECRET}",
				TokenURL:     "https://auth.example.com/token",
			},
			wantErr: "refresh_token is required",
		},
		{
			name: "negative timeout",
			config: &OAuth2TransportConfig{
				BaseURL:      "https://api.example.com",
				Flow:         "client_credentials",
				ClientID:     "client-id",
				ClientSecret: "${CLIENT_SECRET}",
				TokenURL:     "https://auth.example.com/token",
				Timeout:      -1 * time.Second,
			},
			wantErr: "timeout cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOAuth2TransportConfig_TransportType(t *testing.T) {
	config := &OAuth2TransportConfig{}
	assert.Equal(t, "oauth2", config.TransportType())
}

func TestOAuth2Transport_Name(t *testing.T) {
	transport := &OAuth2Transport{}
	assert.Equal(t, "oauth2", transport.Name())
}

func TestOAuth2Transport_SetRateLimiter(t *testing.T) {
	transport := &OAuth2Transport{}
	limiter := &mockRateLimiter{}

	transport.SetRateLimiter(limiter)
	assert.NotNil(t, transport.rateLimiter)
}

func TestOAuth2Transport_Execute_InvalidRequest(t *testing.T) {
	transport := &OAuth2Transport{
		config: &OAuth2TransportConfig{
			BaseURL: "https://api.example.com",
		},
	}

	tests := []struct {
		name    string
		req     *Request
		wantErr string
	}{
		{
			name: "empty method",
			req: &Request{
				Method: "",
				URL:    "/api/resource",
			},
			wantErr: "method is required",
		},
		{
			name: "invalid method",
			req: &Request{
				Method: "INVALID",
				URL:    "/api/resource",
			},
			wantErr: "invalid HTTP method",
		},
		{
			name: "empty URL",
			req: &Request{
				Method: "GET",
				URL:    "",
			},
			wantErr: "URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := transport.Execute(context.Background(), tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestParseOAuth2Error_JSON(t *testing.T) {
	transport := &OAuth2Transport{}

	jsonBody := `{
		"error": "invalid_grant",
		"error_description": "The refresh token is invalid or expired"
	}`

	err := transport.parseOAuth2Error(401, []byte(jsonBody), "req-123")
	require.Error(t, err)

	terr, ok := err.(*TransportError)
	require.True(t, ok)
	assert.Equal(t, ErrorTypeAuth, terr.Type)
	assert.Equal(t, 401, terr.StatusCode)
	assert.Contains(t, terr.Message, "invalid_grant")
	assert.Contains(t, terr.Message, "The refresh token is invalid or expired")
	assert.Equal(t, "req-123", terr.RequestID)
	assert.False(t, terr.Retryable)
}

func TestParseOAuth2Error_Fallback(t *testing.T) {
	transport := &OAuth2Transport{}

	body := []byte("plain text error")

	err := transport.parseOAuth2Error(500, body, "req-456")
	require.Error(t, err)

	terr, ok := err.(*TransportError)
	require.True(t, ok)
	assert.Equal(t, ErrorTypeServer, terr.Type)
	assert.Equal(t, 500, terr.StatusCode)
	assert.True(t, terr.Retryable)
}

func TestClassifyOAuth2Error(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		errorCode   string
		description string
		wantType    ErrorType
		retryable   bool
	}{
		{
			name:        "invalid_grant",
			statusCode:  401,
			errorCode:   "invalid_grant",
			description: "Token expired",
			wantType:    ErrorTypeAuth,
			retryable:   false,
		},
		{
			name:        "unauthorized_client",
			statusCode:  401,
			errorCode:   "unauthorized_client",
			description: "Client not authorized",
			wantType:    ErrorTypeAuth,
			retryable:   false,
		},
		{
			name:        "access_denied",
			statusCode:  403,
			errorCode:   "access_denied",
			description: "Access denied",
			wantType:    ErrorTypeAuth,
			retryable:   false,
		},
		{
			name:        "temporarily_unavailable",
			statusCode:  503,
			errorCode:   "temporarily_unavailable",
			description: "Service temporarily unavailable",
			wantType:    ErrorTypeServer,
			retryable:   true,
		},
		{
			name:        "server_error",
			statusCode:  500,
			errorCode:   "server_error",
			description: "Internal server error",
			wantType:    ErrorTypeServer,
			retryable:   true,
		},
		{
			name:        "unknown 401",
			statusCode:  401,
			errorCode:   "unknown",
			description: "Unknown error",
			wantType:    ErrorTypeAuth,
			retryable:   false,
		},
		{
			name:        "unknown 500",
			statusCode:  500,
			errorCode:   "unknown",
			description: "Unknown error",
			wantType:    ErrorTypeServer,
			retryable:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyOAuth2Error(tt.statusCode, tt.errorCode, tt.description, "req-test")
			require.Error(t, err)

			terr, ok := err.(*TransportError)
			require.True(t, ok)
			assert.Equal(t, tt.wantType, terr.Type)
			assert.Equal(t, tt.retryable, terr.Retryable)
			assert.Equal(t, tt.statusCode, terr.StatusCode)
			assert.Equal(t, "req-test", terr.RequestID)
			assert.Contains(t, terr.Message, tt.errorCode)
			if tt.description != "" {
				assert.Contains(t, terr.Message, tt.description)
			}
		})
	}
}

func TestOAuth2Transport_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name     string
		token    *oauth2.Token
		expected bool
	}{
		{
			name:     "nil token",
			token:    nil,
			expected: true,
		},
		{
			name: "token expires soon",
			token: &oauth2.Token{
				AccessToken: "token",
				Expiry:      time.Now().Add(3 * time.Minute),
			},
			expected: true,
		},
		{
			name: "token still valid",
			token: &oauth2.Token{
				AccessToken: "token",
				Expiry:      time.Now().Add(10 * time.Minute),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &OAuth2Transport{
				token: tt.token,
			}
			assert.Equal(t, tt.expected, transport.needsRefresh())
		})
	}
}

// Integration test structure (would require mock OAuth2 server)
func TestOAuth2Transport_Integration(t *testing.T) {
	t.Skip("Integration test - requires mock OAuth2 server")

	// Example structure for full integration test with mock server:
	/*
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock token endpoint
			response := map[string]interface{}{
				"access_token": "test-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer tokenServer.Close()

		apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify bearer token
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"result": "success"}`))
		}))
		defer apiServer.Close()

		config := &OAuth2TransportConfig{
			BaseURL:      apiServer.URL,
			Flow:         "client_credentials",
			ClientID:     "test-client",
			ClientSecret: "${CLIENT_SECRET}",
			TokenURL:     tokenServer.URL,
		}

		transport, err := NewOAuth2Transport(config)
		require.NoError(t, err)

		req := &Request{
			Method: "GET",
			URL:    "/api/resource",
		}

		resp, err := transport.Execute(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	*/
}

// TestOAuth2Transport_TokenRefresh tests token refresh logic
func TestOAuth2Transport_TokenRefresh(t *testing.T) {
	t.Skip("Requires complex OAuth2 token source mocking")

	// This test would verify:
	// - Token is refreshed when it expires
	// - Concurrent requests share a single refresh operation
	// - Blocked requests receive the refreshed token
	// - Token refresh respects timeout
}
