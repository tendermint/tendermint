package p2p

import (
	"context"
	"errors"
	"fmt"
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

	// Close stops listening, but does not close active connections -- these
	// must be closed individually.
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

// NetAddress returns a NetAddress for the endpoint.
// FIXME This is temporary for compatibility with the old P2P stack.
func (e Endpoint) NetAddress() *NetAddress {
	return &NetAddress{
		ID:   e.PeerID,
		IP:   e.IP,
		Port: e.Port,
	}
}

// Connection represents an established connection between two endpoints.
//
// FIXME This is a temporary interface while we figure out whether we'll
// be adopting QUIC or not. If we do, this should be a byte-oriented
// multi-stream interface with one goroutine consuming each stream, and
// the MConnection transport either needs protocol changes or a shim.
// For details, see: https://github.com/tendermint/spec/pull/227
type Connection interface {
	// ReceiveMessage returns the next message received on the connection,
	// blocking until one is available. io.EOF is returned when closed.
	ReceiveMessage() (chID byte, msg []byte, err error)

	// SendMessage sends a message on the connection.
	SendMessage(chID byte, msg []byte) error

	// LocalEndpoint returns the local endpoint for the connection.
	LocalEndpoint() Endpoint

	// RemoteEndpoint returns the remote endpoint for the connection.
	RemoteEndpoint() Endpoint

	// PubKey returns the public key of the remote peer.
	PubKey() crypto.PubKey

	// NodeInfo returns the remote peer's node info.
	// FIXME We may want to do something else here.
	NodeInfo() DefaultNodeInfo

	// Close closes the connection.
	Close() error
}
