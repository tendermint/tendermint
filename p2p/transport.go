package p2p

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/p2p/conn"
)

const (
	defaultProtocol Protocol = MConnProtocol
)

// Protocol identifies a transport protocol.
type Protocol string

// Transport is a connection-oriented mechanism for exchanging data with a peer.
type Transport interface {
	// Protocols returns the protocols the transport supports, which the
	// router uses to pick a transport for a PeerAddress.
	Protocols() []Protocol

	// Accept waits for the next inbound connection on a listening endpoint, or
	// returns io.EOF if the transport is closed.
	Accept(context.Context) (Connection, error)

	// Dial creates an outbound connection to an endpoint.
	Dial(context.Context, Endpoint) (Connection, error)

	// Endpoints lists endpoints the transport is listening on.
	Endpoints() []Endpoint

	// Close stops accepting new connections, but does not close active connections.
	Close() error

	// Stringer is used to display the transport, e.g. in logs.
	//
	// Without this, the logger may use reflection to access and display
	// internal fields -- these are written concurrently, which can trigger the
	// race detector or even cause a panic.
	fmt.Stringer
}

// Connection represents an established connection between two endpoints.
//
// FIXME: This is a temporary interface while we figure out whether we'll be
// adopting QUIC or not. If we do, this should be a byte-oriented multi-stream
// interface with one goroutine consuming each stream, and the MConnection
// transport either needs protocol changes or a shim. For details, see:
// https://github.com/tendermint/spec/pull/227
//
// FIXME: The interface is currently very broad in order to accommodate
// MConnection behavior that the rest of the P2P stack relies on. This should be
// removed once the P2P core is rewritten.
type Connection interface {
	// Handshake handshakes with the remote peer. It must be called immediately
	// after the connection is established, and returns the remote peer's node
	// info and public key. The caller is responsible for validation.
	//
	// FIXME: The handshaking should really be the Router's responsibility, but
	// that requires the connection interface to be byte-oriented rather than
	// message-oriented (see comment above).
	Handshake(context.Context, NodeInfo, crypto.PrivKey) (NodeInfo, crypto.PubKey, error)

	// ReceiveMessage returns the next message received on the connection,
	// blocking until one is available. io.EOF is returned when closed.
	ReceiveMessage() (chID byte, msg []byte, err error)

	// SendMessage sends a message on the connection.
	// FIXME: For compatibility with the current Peer, it returns an additional
	// boolean false if the message timed out waiting to be accepted into the
	// send buffer.
	SendMessage(chID byte, msg []byte) (bool, error)

	// TrySendMessage is a non-blocking version of SendMessage that returns
	// immediately if the message buffer is full. It returns true if the message
	// was accepted.
	//
	// FIXME: This is here for backwards-compatibility with the current Peer
	// code, and should be removed when possible.
	TrySendMessage(chID byte, msg []byte) (bool, error)

	// LocalEndpoint returns the local endpoint for the connection.
	LocalEndpoint() Endpoint

	// RemoteEndpoint returns the remote endpoint for the connection.
	RemoteEndpoint() Endpoint

	// Close closes the connection.
	Close() error

	// FlushClose flushes all pending sends and then closes the connection.
	//
	// FIXME: This only exists for backwards-compatibility with the current
	// MConnection implementation. There should really be a separate Flush()
	// method, but there is no easy way to synchronously flush pending data with
	// the current MConnection structure.
	FlushClose() error

	// Status returns the current connection status.
	// FIXME: Only here for compatibility with the current Peer code.
	Status() conn.ConnectionStatus
}

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

// PeerAddress converts the endpoint into a peer address for a given node ID.
func (e Endpoint) PeerAddress(nodeID NodeID) PeerAddress {
	address := PeerAddress{
		NodeID:   nodeID,
		Protocol: e.Protocol,
		Path:     e.Path,
	}
	if e.IP != nil {
		address.Hostname = e.IP.String()
		address.Port = e.Port
	}
	return address
}

// String formats an endpoint as a URL string.
func (e Endpoint) String() string {
	if e.IP == nil {
		return fmt.Sprintf("%s:%s", e.Protocol, e.Path)
	}
	s := fmt.Sprintf("%s://%s", e.Protocol, e.IP)
	if e.Port > 0 {
		s += strconv.Itoa(int(e.Port))
	}
	s += e.Path
	return s
}

// Validate validates an endpoint.
func (e Endpoint) Validate() error {
	switch {
	case e.Protocol == "":
		return errors.New("endpoint has no protocol")
	case e.Port > 0 && len(e.IP) == 0:
		return fmt.Errorf("endpoint has port %v but no IP", e.Port)
	case len(e.IP) == 0 && e.Path == "":
		return errors.New("endpoint has neither path nor IP")
	default:
		return nil
	}
}
