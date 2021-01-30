package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
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

// CreateTransport creates a new memory transport and endpoint with the given
// node ID. It immediately begins listening on the endpoint "memory:<id>", and
// can be accessed by other transports in the same memory network.
func (n *MemoryNetwork) CreateTransport(nodeID NodeID) (*MemoryTransport, error) {
	t := newMemoryTransport(n, nodeID)

	n.mtx.Lock()
	defer n.mtx.Unlock()
	if _, ok := n.transports[nodeID]; ok {
		return nil, fmt.Errorf("transport with node ID %q already exists", nodeID)
	}
	n.transports[nodeID] = t
	return t, nil
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
	network *MemoryNetwork
	nodeID  NodeID
	logger  log.Logger

	acceptCh  chan *MemoryConnection
	closeCh   chan struct{}
	closeOnce sync.Once
}

// newMemoryTransport creates a new in-memory transport in the given network.
// Callers should use MemoryNetwork.CreateTransport() or GenerateTransport()
// to create transports, this is for internal use by MemoryNetwork.
func newMemoryTransport(network *MemoryNetwork, nodeID NodeID) *MemoryTransport {
	return &MemoryTransport{
		network: network,
		nodeID:  nodeID,
		logger:  network.logger.With("local", fmt.Sprintf("%v:%v", MemoryProtocol, nodeID)),

		acceptCh: make(chan *MemoryConnection),
		closeCh:  make(chan struct{}),
	}
}

// String displays the transport.
func (t *MemoryTransport) String() string {
	return string(MemoryProtocol)
}

// Protocols implements Transport.
func (t *MemoryTransport) Protocols() []Protocol {
	return []Protocol{MemoryProtocol}
}

// Accept implements Transport.
func (t *MemoryTransport) Accept(ctx context.Context) (Connection, error) {
	select {
	case conn := <-t.acceptCh:
		t.logger.Info("accepted connection from peer", "remote", conn.RemoteEndpoint())
		return conn, nil
	case <-t.closeCh:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Dial implements Transport.
func (t *MemoryTransport) Dial(ctx context.Context, endpoint Endpoint) (Connection, error) {
	if endpoint.Protocol != MemoryProtocol {
		return nil, fmt.Errorf("invalid protocol %q", endpoint.Protocol)
	}
	if endpoint.Path == "" {
		return nil, errors.New("no path")
	}
	nodeID, err := NewNodeID(endpoint.Path)
	if err != nil {
		return nil, err
	}
	t.logger.Info("dialing peer", "remote", endpoint)

	peerTransport := t.network.GetTransport(nodeID)
	if peerTransport == nil {
		return nil, fmt.Errorf("unknown peer %q", nodeID)
	}
	inCh := make(chan memoryMessage, 1)
	outCh := make(chan memoryMessage, 1)
	closer := tmsync.NewCloser()

	outConn := newMemoryConnection(t, peerTransport, inCh, outCh, closer)
	inConn := newMemoryConnection(peerTransport, t, outCh, inCh, closer)

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
		return nil, nil, fmt.Errorf("peer %q not listening on any endpoints", peer.nodeID)
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
	err := t.network.RemoveTransport(t.nodeID)
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
			Path:     string(t.nodeID),
		}}
	}
}

// MemoryConnection is an in-memory connection between two transports (nodes).
type MemoryConnection struct {
	logger log.Logger
	local  *MemoryTransport
	remote *MemoryTransport

	receiveCh <-chan memoryMessage
	sendCh    chan<- memoryMessage
	closer    *tmsync.Closer
}

// memoryMessage is used to pass messages internally in the connection.
// For handshakes, nodeInfo and pubKey are set instead of channel and message.
type memoryMessage struct {
	channelID ChannelID
	message   []byte

	// For handshakes.
	nodeInfo NodeInfo
	pubKey   crypto.PubKey
}

// newMemoryConnection creates a new MemoryConnection. It takes all channels
// (including the closeCh signal channel) on construction, such that they can be
// shared between both ends of the connection.
func newMemoryConnection(
	local *MemoryTransport,
	remote *MemoryTransport,
	receiveCh <-chan memoryMessage,
	sendCh chan<- memoryMessage,
	closer *tmsync.Closer,
) *MemoryConnection {
	c := &MemoryConnection{
		local:     local,
		remote:    remote,
		receiveCh: receiveCh,
		sendCh:    sendCh,
		closer:    closer,
	}
	c.logger = c.local.logger.With("remote", c.RemoteEndpoint())
	return c
}

// Handshake implements Connection.
func (c *MemoryConnection) Handshake(
	ctx context.Context,
	nodeInfo NodeInfo,
	privKey crypto.PrivKey,
) (NodeInfo, crypto.PubKey, error) {
	select {
	case c.sendCh <- memoryMessage{nodeInfo: nodeInfo, pubKey: privKey.PubKey()}:
	case <-ctx.Done():
		return NodeInfo{}, nil, ctx.Err()
	case <-c.closer.Done():
		return NodeInfo{}, nil, io.EOF
	}

	select {
	case msg := <-c.receiveCh:
		c.logger.Debug("handshake complete")
		return msg.nodeInfo, msg.pubKey, nil
	case <-ctx.Done():
		return NodeInfo{}, nil, ctx.Err()
	case <-c.closer.Done():
		return NodeInfo{}, nil, io.EOF
	}
}

// ReceiveMessage implements Connection.
func (c *MemoryConnection) ReceiveMessage() (ChannelID, []byte, error) {
	// check close first, since channels are buffered
	select {
	case <-c.closer.Done():
		return 0, nil, io.EOF
	default:
	}

	select {
	case msg := <-c.receiveCh:
		c.logger.Debug("received message", "channel", msg.channelID, "message", msg.message)
		return msg.channelID, msg.message, nil
	case <-c.closer.Done():
		return 0, nil, io.EOF
	}
}

// SendMessage implements Connection.
func (c *MemoryConnection) SendMessage(chID ChannelID, msg []byte) (bool, error) {
	// check close first, since channels are buffered
	select {
	case <-c.closer.Done():
		return false, io.EOF
	default:
	}

	select {
	case c.sendCh <- memoryMessage{channelID: chID, message: msg}:
		c.logger.Debug("sent message", "channel", chID, "message", msg)
		return true, nil
	case <-c.closer.Done():
		return false, io.EOF
	}
}

// TrySendMessage implements Connection.
func (c *MemoryConnection) TrySendMessage(chID ChannelID, msg []byte) (bool, error) {
	// check close first, since channels are buffered
	select {
	case <-c.closer.Done():
		return false, io.EOF
	default:
	}

	select {
	case c.sendCh <- memoryMessage{channelID: chID, message: msg}:
		c.logger.Debug("sent message", "channel", chID, "message", msg)
		return true, nil
	case <-c.closer.Done():
		return false, io.EOF
	default:
		return false, nil
	}
}

// Close closes the connection.
func (c *MemoryConnection) Close() error {
	c.closer.Close()
	c.logger.Info("closed connection")
	return nil
}

// FlushClose flushes all pending sends and then closes the connection.
func (c *MemoryConnection) FlushClose() error {
	return c.Close()
}

// LocalEndpoint returns the local endpoint for the connection.
func (c *MemoryConnection) LocalEndpoint() Endpoint {
	return Endpoint{
		Protocol: MemoryProtocol,
		Path:     string(c.local.nodeID),
	}
}

// RemoteEndpoint returns the remote endpoint for the connection.
func (c *MemoryConnection) RemoteEndpoint() Endpoint {
	return Endpoint{
		Protocol: MemoryProtocol,
		Path:     string(c.remote.nodeID),
	}
}

// Status returns the current connection status.
func (c *MemoryConnection) Status() conn.ConnectionStatus {
	return conn.ConnectionStatus{}
}
