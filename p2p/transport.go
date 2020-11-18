package p2p

import (
	"context"
	"io"
	"net"

	"github.com/tendermint/tendermint/crypto"
)

// Transport is an arbitrary mechanism for exchanging bytes with a peer.
// FIXME Rename Transport when old Transport is removed.
type NewTransport interface {
	// Accept waits for the next inbound connection on a listening endpoint.
	Accept(context.Context) (Connection, error)

	// Dial creates an outbound connection to an endpoint.
	Dial(context.Context, Endpoint) (Connection, error)

	// Endpoints lists endpoints the transport is listening on. Any endpoint IP
	// addresses do not need to be normalized in any way (e.g. 0.0.0.0 is
	// valid), as they should be preprocessed before being advertised.
	Endpoints() []Endpoint
}

// Protocol identifies a transport protocol.
type Protocol string

// Endpoint represents a transport connection endpoint, either local or remote.
type Endpoint struct {
	// PeerID specifies the peer ID of the endpoint.
	//
	// FIXME This is here for backwards-compatibility with the existing MConn
	// protocol, we should consider moving this higher in the stack (i.e. to
	// the router).
	PeerID ID

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

// Connection represents an established connection between two endpoints.
type Connection interface {
	// Stream creates a new logically distinct IO stream within the connection.
	//
	// FIXME We use a stream ID for now, to allow the MConnTransport to get the
	// stream corresponding to a channel without protocol changes. We should
	// change this to use a channel handshake instead.
	Stream(id uint16) (Stream, error)

	// LocalEndpoint returns the local endpoint for the connection.
	LocalEndpoint() Endpoint

	// RemoteEndpoint returns the remote endpoint for the connection.
	RemoteEndpoint() Endpoint

	// PubKey returns the public key of the remote peer.
	PubKey() crypto.PubKey

	// Close closes the connection.
	Close() error
}

// Stream represents a single logical IO stream within a connection.
type Stream interface {
	io.Reader // Read([]byte) (int, error)
	io.Writer // Write([]byte) (int, error)
	io.Closer // Close() error
}
