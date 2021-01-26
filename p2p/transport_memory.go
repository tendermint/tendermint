package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p/conn"
)

const (
	MemoryProtocol Protocol = "memory"
)

// MemoryNetwork is an in-memory "network" that uses Go channels to communicate
// between endpoints. Transport endpoints are created with CreateTransport. It
// is primarily used for testing.
type MemoryNetwork struct {
	logger log.Logger

	mtx        sync.RWMutex
	transports map[NodeID]*MemoryTransport
}

// NewMemoryNetwork creates a new in-memory network.
func NewMemoryNetwork(logger log.Logger) *MemoryNetwork {
	return &MemoryNetwork{
		logger:     logger,
		transports: map[NodeID]*MemoryTransport{},
	}
}

// CreateTransport creates a new memory transport and endpoint for the given
// NodeInfo and private key. Use GenerateTransport() to autogenerate a random
// key and node info.
//
// The transport immediately begins listening on the endpoint "memory:<id>", and
// can be accessed by other transports in the same memory network.
func (n *MemoryNetwork) CreateTransport(
	nodeInfo NodeInfo,
	privKey crypto.PrivKey,
) (*MemoryTransport, error) {
	nodeID := nodeInfo.NodeID
	if nodeID == "" {
		return nil, errors.New("no node ID")
	}
	t := newMemoryTransport(n, nodeInfo, privKey)

	n.mtx.Lock()
	defer n.mtx.Unlock()
	if _, ok := n.transports[nodeID]; ok {
		return nil, fmt.Errorf("transport with node ID %q already exists", nodeID)
	}
	n.transports[nodeID] = t
	return t, nil
}

// GenerateTransport generates a new transport endpoint by generating a random
// private key and node info. The endpoint address can be obtained via
// Transport.Endpoints().
func (n *MemoryNetwork) GenerateTransport() *MemoryTransport {
	privKey := ed25519.GenPrivKey()
	nodeID := NodeIDFromPubKey(privKey.PubKey())
	nodeInfo := NodeInfo{
		NodeID:     nodeID,
		ListenAddr: fmt.Sprintf("%v:%v", MemoryProtocol, nodeID),
	}
	t, err := n.CreateTransport(nodeInfo, privKey)
	if err != nil {
		// GenerateTransport is only used for testing, and the likelihood of
		// generating a duplicate node ID is very low, so we'll panic.
		panic(err)
	}
	return t
}

// GetTransport looks up a transport in the network, returning nil if not found.
func (n *MemoryNetwork) GetTransport(id NodeID) *MemoryTransport {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	return n.transports[id]
}

// RemoveTransport removes a transport from the network and closes it.
func (n *MemoryNetwork) RemoveTransport(id NodeID) error {
	n.mtx.Lock()
	t, ok := n.transports[id]
	delete(n.transports, id)
	n.mtx.Unlock()

	if ok {
		// Close may recursively call RemoveTransport() again, but this is safe
		// because we've already removed the transport from the map above.
		return t.Close()
	}
	return nil
}

// MemoryTransport is an in-memory transport that's primarily meant for testing.
// It communicates between endpoints using Go channels. To dial a different
// endpoint, both endpoints/transports must be in the same MemoryNetwork.
type MemoryTransport struct {
	network  *MemoryNetwork
	nodeInfo NodeInfo
	privKey  crypto.PrivKey
	logger   log.Logger

	acceptCh  chan *MemoryConnection
	closeCh   chan struct{}
	closeOnce sync.Once
}

// newMemoryTransport creates a new in-memory transport in the given network.
// Callers should use MemoryNetwork.CreateTransport() or GenerateTransport()
// to create transports, this is for internal use by MemoryNetwork.
func newMemoryTransport(
	network *MemoryNetwork,
	nodeInfo NodeInfo,
	privKey crypto.PrivKey,
) *MemoryTransport {
	return &MemoryTransport{
		network:  network,
		nodeInfo: nodeInfo,
		privKey:  privKey,
		logger: network.logger.With("local",
			fmt.Sprintf("%v:%v", MemoryProtocol, nodeInfo.NodeID)),

		acceptCh: make(chan *MemoryConnection),
		closeCh:  make(chan struct{}),
	}
}

// Accept implements Transport.
func (t *MemoryTransport) Accept(ctx context.Context) (Connection, error) {
	select {
	case conn := <-t.acceptCh:
		t.logger.Info("accepted connection from peer", "remote", conn.RemoteEndpoint())
		return conn, nil
	case <-t.closeCh:
		return nil, ErrTransportClosed{}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Dial implements Transport.
func (t *MemoryTransport) Dial(ctx context.Context, endpoint Endpoint) (Connection, error) {
	if endpoint.Protocol != MemoryProtocol {
		return nil, fmt.Errorf("invalid protocol %q", endpoint.Protocol)
	}
	if endpoint.PeerID == "" {
		return nil, errors.New("no peer ID")
	}
	t.logger.Info("dialing peer", "remote", endpoint)

	peerTransport := t.network.GetTransport(endpoint.PeerID)
	if peerTransport == nil {
		return nil, fmt.Errorf("unknown peer %q", endpoint.PeerID)
	}
	inCh := make(chan memoryMessage, 1)
	outCh := make(chan memoryMessage, 1)
	closeCh := make(chan struct{})
	closeOnce := sync.Once{}
	closer := func() bool {
		closed := false
		closeOnce.Do(func() {
			close(closeCh)
			closed = true
		})
		return closed
	}

	outConn := newMemoryConnection(t, peerTransport, inCh, outCh, closeCh, closer)
	inConn := newMemoryConnection(peerTransport, t, outCh, inCh, closeCh, closer)

	select {
	case peerTransport.acceptCh <- inConn:
		return outConn, nil
	case <-peerTransport.closeCh:
		return nil, ErrTransportClosed{}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// DialAccept is a convenience function that dials a peer MemoryTransport and
// returns both ends of the connection (A to B and B to A).
func (t *MemoryTransport) DialAccept(
	ctx context.Context,
	peer *MemoryTransport,
) (Connection, Connection, error) {
	endpoints := peer.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, fmt.Errorf("peer %q not listening on any endpoints", peer.nodeInfo.NodeID)
	}

	acceptCh := make(chan Connection, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := peer.Accept(ctx)
		errCh <- err
		acceptCh <- conn
	}()

	outConn, err := t.Dial(ctx, endpoints[0])
	if err != nil {
		return nil, nil, err
	}
	if err = <-errCh; err != nil {
		return nil, nil, err
	}
	inConn := <-acceptCh

	return outConn, inConn, nil
}

// Close implements Transport.
func (t *MemoryTransport) Close() error {
	err := t.network.RemoveTransport(t.nodeInfo.NodeID)
	t.closeOnce.Do(func() {
		close(t.closeCh)
	})
	t.logger.Info("stopped accepting connections")
	return err
}

// Endpoints implements Transport.
func (t *MemoryTransport) Endpoints() []Endpoint {
	select {
	case <-t.closeCh:
		return []Endpoint{}
	default:
		return []Endpoint{{
			Protocol: MemoryProtocol,
			PeerID:   t.nodeInfo.NodeID,
		}}
	}
}

// SetChannelDescriptors implements Transport.
func (t *MemoryTransport) SetChannelDescriptors(chDescs []*conn.ChannelDescriptor) {
}

// MemoryConnection is an in-memory connection between two transports (nodes).
type MemoryConnection struct {
	logger log.Logger
	local  *MemoryTransport
	remote *MemoryTransport

	receiveCh <-chan memoryMessage
	sendCh    chan<- memoryMessage
	closeCh   <-chan struct{}
	close     func() bool
}

// memoryMessage is used to pass messages internally in the connection.
type memoryMessage struct {
	channel byte
	message []byte
}

// newMemoryConnection creates a new MemoryConnection. It takes all channels
// (including the closeCh signal channel) on construction, such that they can be
// shared between both ends of the connection.
func newMemoryConnection(
	local *MemoryTransport,
	remote *MemoryTransport,
	receiveCh <-chan memoryMessage,
	sendCh chan<- memoryMessage,
	closeCh <-chan struct{},
	close func() bool,
) *MemoryConnection {
	c := &MemoryConnection{
		local:     local,
		remote:    remote,
		receiveCh: receiveCh,
		sendCh:    sendCh,
		closeCh:   closeCh,
		close:     close,
	}
	c.logger = c.local.logger.With("remote", c.RemoteEndpoint())
	return c
}

// ReceiveMessage implements Connection.
func (c *MemoryConnection) ReceiveMessage() (chID byte, msg []byte, err error) {
	// check close first, since channels are buffered
	select {
	case <-c.closeCh:
		return 0, nil, io.EOF
	default:
	}

	select {
	case msg := <-c.receiveCh:
		c.logger.Debug("received message", "channel", msg.channel, "message", msg.message)
		return msg.channel, msg.message, nil
	case <-c.closeCh:
		return 0, nil, io.EOF
	}
}

// SendMessage implements Connection.
func (c *MemoryConnection) SendMessage(chID byte, msg []byte) (bool, error) {
	// check close first, since channels are buffered
	select {
	case <-c.closeCh:
		return false, io.EOF
	default:
	}

	select {
	case c.sendCh <- memoryMessage{channel: chID, message: msg}:
		c.logger.Debug("sent message", "channel", chID, "message", msg)
		return true, nil
	case <-c.closeCh:
		return false, io.EOF
	}
}

// TrySendMessage implements Connection.
func (c *MemoryConnection) TrySendMessage(chID byte, msg []byte) (bool, error) {
	// check close first, since channels are buffered
	select {
	case <-c.closeCh:
		return false, io.EOF
	default:
	}

	select {
	case c.sendCh <- memoryMessage{channel: chID, message: msg}:
		c.logger.Debug("sent message", "channel", chID, "message", msg)
		return true, nil
	case <-c.closeCh:
		return false, io.EOF
	default:
		return false, nil
	}
}

// Close closes the connection.
func (c *MemoryConnection) Close() error {
	if c.close() {
		c.logger.Info("closed connection")
	}
	return nil
}

// FlushClose flushes all pending sends and then closes the connection.
func (c *MemoryConnection) FlushClose() error {
	return c.Close()
}

// LocalEndpoint returns the local endpoint for the connection.
func (c *MemoryConnection) LocalEndpoint() Endpoint {
	return Endpoint{
		PeerID:   c.local.nodeInfo.NodeID,
		Protocol: MemoryProtocol,
	}
}

// RemoteEndpoint returns the remote endpoint for the connection.
func (c *MemoryConnection) RemoteEndpoint() Endpoint {
	return Endpoint{
		PeerID:   c.remote.nodeInfo.NodeID,
		Protocol: MemoryProtocol,
	}
}

// PubKey returns the remote peer's public key.
func (c *MemoryConnection) PubKey() crypto.PubKey {
	return c.remote.privKey.PubKey()
}

// NodeInfo returns the remote peer's node info.
func (c *MemoryConnection) NodeInfo() NodeInfo {
	return c.remote.nodeInfo
}

// Status returns the current connection status.
func (c *MemoryConnection) Status() conn.ConnectionStatus {
	return conn.ConnectionStatus{}
}
