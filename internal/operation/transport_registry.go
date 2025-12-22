package operation

import (
	"github.com/tombee/conductor/internal/operation/transport"
)

// NewDefaultTransportRegistry creates a transport registry with all built-in transports.
// Built-in transports include: http, aws_sigv4, oauth2.
func NewDefaultTransportRegistry() *TransportRegistry {
	return transport.NewDefaultRegistry()
}
