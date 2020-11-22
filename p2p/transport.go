package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"

	"github.com/tendermint/tendermint/crypto"
)

// Transport is an arbitrary mechanism for exchanging bytes with a peer.
type Transport interface {
	// Listen begins listening on the given endpoint.
	// FIXME This is mostly here for compatibility with existing code, we
	// should consider removing it when we rewrite the P2P code.
	Listen(Endpoint) error

	// Accept waits for the next inbound connection on a listening endpoint.
	Accept(context.Context) (Connection, error)

	// Dial creates an outbound connection to an endpoint.
	Dial(context.Context, Endpoint) (Connection, error)

	// Endpoints lists endpoints the transport is listening on. Any endpoint IP
	// addresses do not need to be normalized in any way (e.g. 0.0.0.0 is
	// valid), as they should be preprocessed before being advertised.
	Endpoints() []Endpoint

	// Close stops listening and closes all active connections.
	Close() error
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

// String formats an endpoint as a URL string.
func (e Endpoint) String() string {
	u := url.URL{Scheme: string(e.Protocol)}
	if e.PeerID != "" {
		u.User = url.User(string(e.PeerID))
	}
	if len(e.IP) > 0 {
		u.Host = e.IP.String()
		if e.Port > 0 {
			u.Host += fmt.Sprintf(":%v", e.Port)
		}
	} else if e.Path != "" {
		u.Opaque = e.Path
	}
	return u.String()
}

// Validate validates an endpoint.
func (e Endpoint) Validate() error {
	switch {
	case e.PeerID == "":
		return errors.New("endpoint has no peer ID")
	case e.Protocol == "":
		return errors.New("endpoint has no protocol")
	case len(e.IP) == 0 && len(e.Path) == 0:
		return errors.New("endpoint must have either IP or path")
	case e.Port > 0 && len(e.IP) == 0:
		return fmt.Errorf("endpoint has port %v but no IP", e.Port)
	default:
		return nil
	}
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
