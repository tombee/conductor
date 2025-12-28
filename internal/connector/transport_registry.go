package connector

import (
	"github.com/tombee/conductor/internal/connector/transport"
)

// NewDefaultTransportRegistry creates a transport registry with all built-in transports.
// Built-in transports include: http, aws_sigv4, oauth2.
func NewDefaultTransportRegistry() *TransportRegistry {
	return transport.NewDefaultRegistry()
}
