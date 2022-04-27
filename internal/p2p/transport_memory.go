package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

const (
	MemoryProtocol Protocol = "memory"
)

// MemoryNetwork is an in-memory "network" that uses buffered Go channels to
// communicate between endpoints. It is primarily meant for testing.
//
// Network endpoints are allocated via CreateTransport(), which takes a node ID,
// and the endpoint is then immediately accessible via the URL "memory:<nodeID>".
type MemoryNetwork struct {
	logger log.Logger

	mtx        sync.RWMutex
	transports map[types.NodeID]*MemoryTransport
	bufferSize int
}

// NewMemoryNetwork creates a new in-memory network.
func NewMemoryNetwork(logger log.Logger, bufferSize int) *MemoryNetwork {
	return &MemoryNetwork{
		bufferSize: bufferSize,
		logger:     logger,
		transports: map[types.NodeID]*MemoryTransport{},
	}
}

// CreateTransport creates a new memory transport endpoint with the given node
// ID and immediately begins listening on the address "memory:<id>". It panics
// if the node ID is already in use (which is fine, since this is for tests).
func (n *MemoryNetwork) CreateTransport(nodeID types.NodeID) *MemoryTransport {
	t := newMemoryTransport(n, nodeID)

	n.mtx.Lock()
	defer n.mtx.Unlock()
	if _, ok := n.transports[nodeID]; ok {
		panic(fmt.Sprintf("memory transport with node ID %q already exists", nodeID))
	}
	n.transports[nodeID] = t
	return t
}

// GetTransport looks up a transport in the network, returning nil if not found.
func (n *MemoryNetwork) GetTransport(id types.NodeID) *MemoryTransport {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	return n.transports[id]
}

// RemoveTransport removes a transport from the network and closes it.
func (n *MemoryNetwork) RemoveTransport(id types.NodeID) {
	n.mtx.Lock()
	t, ok := n.transports[id]
	delete(n.transports, id)
	n.mtx.Unlock()

	if ok {
		// Close may recursively call RemoveTransport() again, but this is safe
		// because we've already removed the transport from the map above.
		if err := t.Close(); err != nil {
			n.logger.Error("failed to close memory transport", "id", id, "err", err)
		}
	}
}

// Size returns the number of transports in the network.
func (n *MemoryNetwork) Size() int {
	return len(n.transports)
}

// MemoryTransport is an in-memory transport that uses buffered Go channels to
// communicate between endpoints. It is primarily meant for testing.
//
// New transports are allocated with MemoryNetwork.CreateTransport(). To contact
// a different endpoint, both transports must be in the same MemoryNetwork.
type MemoryTransport struct {
	logger     log.Logger
	network    *MemoryNetwork
	nodeID     types.NodeID
	bufferSize int

	acceptCh chan *MemoryConnection
	closeCh  chan struct{}
	closeFn  func()
}

// newMemoryTransport creates a new MemoryTransport. This is for internal use by
// MemoryNetwork, use MemoryNetwork.CreateTransport() instead.
func newMemoryTransport(network *MemoryNetwork, nodeID types.NodeID) *MemoryTransport {
	once := &sync.Once{}
	closeCh := make(chan struct{})
	return &MemoryTransport{
		logger:     network.logger.With("local", nodeID),
		network:    network,
		nodeID:     nodeID,
		bufferSize: network.bufferSize,
		acceptCh:   make(chan *MemoryConnection),
		closeCh:    closeCh,
		closeFn:    func() { once.Do(func() { close(closeCh) }) },
	}
}

// String implements Transport.
func (t *MemoryTransport) String() string {
	return string(MemoryProtocol)
}

func (*MemoryTransport) Listen(*Endpoint) error { return nil }

func (t *MemoryTransport) AddChannelDescriptors([]*ChannelDescriptor) {}

// Protocols implements Transport.
func (t *MemoryTransport) Protocols() []Protocol {
	return []Protocol{MemoryProtocol}
}

// Endpoints implements Transport.
func (t *MemoryTransport) Endpoint() (*Endpoint, error) {
	if n := t.network.GetTransport(t.nodeID); n == nil {
		return nil, errors.New("node not defined")
	}

	return &Endpoint{
		Protocol: MemoryProtocol,
		Path:     string(t.nodeID),
		// An arbitrary IP and port is used in order for the pex
		// reactor to be able to send addresses to one another.
		IP:   net.IPv4zero,
		Port: 0,
	}, nil
}

// Accept implements Transport.
func (t *MemoryTransport) Accept(ctx context.Context) (Connection, error) {
	select {
	case <-t.closeCh:
		return nil, io.EOF
	case conn := <-t.acceptCh:
		t.logger.Info("accepted connection", "remote", conn.RemoteEndpoint().Path)
		return conn, nil
	case <-ctx.Done():
		return nil, io.EOF
	}
}

// Dial implements Transport.
func (t *MemoryTransport) Dial(ctx context.Context, endpoint *Endpoint) (Connection, error) {
	if endpoint.Protocol != MemoryProtocol {
		return nil, fmt.Errorf("invalid protocol %q", endpoint.Protocol)
	}
	if endpoint.Path == "" {
		return nil, errors.New("no path")
	}
	if err := endpoint.Validate(); err != nil {
		return nil, err
	}

	nodeID, err := types.NewNodeID(endpoint.Path)
	if err != nil {
		return nil, err
	}

	t.logger.Info("dialing peer", "remote", nodeID)
	peer := t.network.GetTransport(nodeID)
	if peer == nil {
		return nil, fmt.Errorf("unknown peer %q", nodeID)
	}

	inCh := make(chan memoryMessage, t.bufferSize)
	outCh := make(chan memoryMessage, t.bufferSize)

	once := &sync.Once{}
	closeCh := make(chan struct{})
	closeFn := func() { once.Do(func() { close(closeCh) }) }

	outConn := newMemoryConnection(t.logger, t.nodeID, peer.nodeID, inCh, outCh)
	outConn.closeCh = closeCh
	outConn.closeFn = closeFn
	inConn := newMemoryConnection(peer.logger, peer.nodeID, t.nodeID, outCh, inCh)
	inConn.closeCh = closeCh
	inConn.closeFn = closeFn

	select {
	case peer.acceptCh <- inConn:
		return outConn, nil
	case <-ctx.Done():
		return nil, io.EOF
	}
}

// Close implements Transport.
func (t *MemoryTransport) Close() error {
	t.network.RemoveTransport(t.nodeID)
	t.closeFn()
	return nil
}

// MemoryConnection is an in-memory connection between two transport endpoints.
type MemoryConnection struct {
	logger   log.Logger
	localID  types.NodeID
	remoteID types.NodeID

	receiveCh <-chan memoryMessage
	sendCh    chan<- memoryMessage

	closeFn func()
	closeCh <-chan struct{}
}

// memoryMessage is passed internally, containing either a message or handshake.
type memoryMessage struct {
	channelID ChannelID
	message   []byte

	// For handshakes.
	nodeInfo *types.NodeInfo
	pubKey   crypto.PubKey
}

// newMemoryConnection creates a new MemoryConnection.
func newMemoryConnection(
	logger log.Logger,
	localID types.NodeID,
	remoteID types.NodeID,
	receiveCh <-chan memoryMessage,
	sendCh chan<- memoryMessage,
) *MemoryConnection {
	return &MemoryConnection{
		logger:    logger.With("remote", remoteID),
		localID:   localID,
		remoteID:  remoteID,
		receiveCh: receiveCh,
		sendCh:    sendCh,
	}
}

// String implements Connection.
func (c *MemoryConnection) String() string {
	return c.RemoteEndpoint().String()
}

// LocalEndpoint implements Connection.
func (c *MemoryConnection) LocalEndpoint() Endpoint {
	return Endpoint{
		Protocol: MemoryProtocol,
		Path:     string(c.localID),
	}
}

// RemoteEndpoint implements Connection.
func (c *MemoryConnection) RemoteEndpoint() Endpoint {
	return Endpoint{
		Protocol: MemoryProtocol,
		Path:     string(c.remoteID),
	}
}

// Handshake implements Connection.
func (c *MemoryConnection) Handshake(
	ctx context.Context,
	nodeInfo types.NodeInfo,
	privKey crypto.PrivKey,
) (types.NodeInfo, crypto.PubKey, error) {
	select {
	case c.sendCh <- memoryMessage{nodeInfo: &nodeInfo, pubKey: privKey.PubKey()}:
		c.logger.Debug("sent handshake", "nodeInfo", nodeInfo)
	case <-c.closeCh:
		return types.NodeInfo{}, nil, io.EOF
	case <-ctx.Done():
		return types.NodeInfo{}, nil, ctx.Err()
	}

	select {
	case msg := <-c.receiveCh:
		if msg.nodeInfo == nil {
			return types.NodeInfo{}, nil, errors.New("no NodeInfo in handshake")
		}
		c.logger.Debug("received handshake", "peerInfo", msg.nodeInfo)
		return *msg.nodeInfo, msg.pubKey, nil
	case <-c.closeCh:
		return types.NodeInfo{}, nil, io.EOF
	case <-ctx.Done():
		return types.NodeInfo{}, nil, ctx.Err()
	}
}

// ReceiveMessage implements Connection.
func (c *MemoryConnection) ReceiveMessage(ctx context.Context) (ChannelID, []byte, error) {
	// Check close first, since channels are buffered. Otherwise, below select
	// may non-deterministically return non-error even when closed.
	select {
	case <-c.closeCh:
		return 0, nil, io.EOF
	case <-ctx.Done():
		return 0, nil, io.EOF
	default:
	}

	select {
	case msg := <-c.receiveCh:
		c.logger.Debug("received message", "chID", msg.channelID, "msg", msg.message)
		return msg.channelID, msg.message, nil
	case <-ctx.Done():
		return 0, nil, io.EOF
	case <-c.closeCh:
		return 0, nil, io.EOF
	}
}

// SendMessage implements Connection.
func (c *MemoryConnection) SendMessage(ctx context.Context, chID ChannelID, msg []byte) error {
	// Check close first, since channels are buffered. Otherwise, below select
	// may non-deterministically return non-error even when closed.
	select {
	case <-c.closeCh:
		return io.EOF
	case <-ctx.Done():
		return io.EOF
	default:
	}

	select {
	case c.sendCh <- memoryMessage{channelID: chID, message: msg}:
		c.logger.Debug("sent message", "chID", chID, "msg", msg)
		return nil
	case <-ctx.Done():
		return io.EOF
	case <-c.closeCh:
		return io.EOF
	}
}

// Close implements Connection.
func (c *MemoryConnection) Close() error { c.closeFn(); return nil }
