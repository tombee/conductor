package operation

import (
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/operation/transport"
	"github.com/tombee/conductor/pkg/workflow"
)

// toHTTPTransportConfig converts a IntegrationDefinition to HTTPTransportConfig.
func toHTTPTransportConfig(def *workflow.IntegrationDefinition) *transport.HTTPTransportConfig {
	config := &transport.HTTPTransportConfig{
		BaseURL: def.BaseURL,
		Timeout: 30 * time.Second, // Default timeout
		Headers: def.Headers,
	}

	// Include auth if present - converted to transport.AuthConfig
	if def.Auth != nil {
		config.Auth = toAuthConfig(def.Auth)
	}

	return config
}

// toAuthConfig converts workflow AuthDefinition to transport AuthConfig.
func toAuthConfig(auth *workflow.AuthDefinition) *transport.AuthConfig {
	if auth == nil {
		return nil
	}

	authConfig := &transport.AuthConfig{
		Type: auth.Type,
	}

	switch auth.Type {
	case "bearer", "":
		authConfig.Token = auth.Token
	case "basic":
		authConfig.Username = auth.Username
		authConfig.Password = auth.Password
	case "api_key":
		authConfig.HeaderName = auth.Header
		authConfig.HeaderValue = auth.Value
	}

	return authConfig
}

// toAWSTransportConfig converts a IntegrationDefinition to AWSTransportConfig.
func toAWSTransportConfig(def *workflow.IntegrationDefinition) (*transport.AWSTransportConfig, error) {
	if def.AWS == nil {
		return nil, fmt.Errorf("aws configuration required for aws_sigv4 transport")
	}

	config := &transport.AWSTransportConfig{
		Service: def.AWS.Service,
		Region:  def.AWS.Region,
		BaseURL: def.BaseURL,
		Timeout: 30 * time.Second,
	}

	return config, nil
}

// toOAuth2TransportConfig converts a IntegrationDefinition to OAuth2TransportConfig.
func toOAuth2TransportConfig(def *workflow.IntegrationDefinition) (*transport.OAuth2TransportConfig, error) {
	if def.OAuth2 == nil {
		return nil, fmt.Errorf("oauth2 configuration required for oauth2 transport")
	}

	config := &transport.OAuth2TransportConfig{
		ClientID:     def.OAuth2.ClientID,
		ClientSecret: def.OAuth2.ClientSecret,
		TokenURL:     def.OAuth2.TokenURL,
		Scopes:       def.OAuth2.Scopes,
		BaseURL:      def.BaseURL,
		Timeout:      30 * time.Second,
	}

	// Set flow type, default to client_credentials
	if def.OAuth2.Flow != "" {
		config.Flow = def.OAuth2.Flow
	} else {
		config.Flow = "client_credentials"
	}

	// Add refresh token if provided (for authorization_code flow)
	if def.OAuth2.RefreshToken != "" {
		config.RefreshToken = def.OAuth2.RefreshToken
	}

	return config, nil
}
