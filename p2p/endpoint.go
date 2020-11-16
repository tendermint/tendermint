package p2p

import (
	"net"
)

// Protocol identifies a transport protocol.
type Protocol string

// Endpoint represents a transport connection endpoint, either local or remote.
type Endpoint struct {
	// Protocol specifies the transport protocol, used by the router to pick a
	// transport for an endpoint.
	Protocol Protocol

	// Path is an optional, arbitrary transport-specific path or identifier.
	Path string

	// IP is an IP address (v4 or v6) to connect to. If set, this defines the
	// endpoint as a networked endpoint.
	IP net.IP

	// Port is a network port (either TCP or UDP). If not set, a default port
	// may be used depending on the protocol.
	Port uint16
}
