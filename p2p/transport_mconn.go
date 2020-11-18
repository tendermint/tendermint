package p2p

import (
	"context"
	"net"
	"sync"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/p2p/conn"
)

const MConnProtocol Protocol = "mconn"

// MConnTransport is a NewTransport implementation using the current multiplexed
// Tendermint protocol.
type MConnTransport struct {
	mconn *MultiplexTransport
	mtx   struct {
		sync.RWMutex
		peerConfig peerConfig
	}
}

var _ Transport = (*MConnTransport)(nil)

func NewMConnTransport(
	nodeInfo NodeInfo,
	nodeKey NodeKey,
	mConfig conn.MConnConfig,
	opts ...MultiplexTransportOption,
) *MConnTransport {
	mt := NewMultiplexTransport(nodeInfo, nodeKey, mConfig)
	for _, opt := range opts {
		opt(mt)
	}
	return &MConnTransport{mconn: mt}
}

func (m *MConnTransport) setPeerConfig(peerConfig peerConfig) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.mtx.peerConfig = peerConfig
}

// takes NetAddress for compatibility with rest of code
func (m *MConnTransport) Listen(addr NetAddress) error {
	return m.mconn.Listen(addr)
}

func (m *MConnTransport) Close() error {
	return m.mconn.Close()
}

func (m *MConnTransport) Accept(ctx context.Context) (Connection, error) {
	m.mtx.RLock()
	peerConfig := m.mtx.peerConfig
	m.mtx.RUnlock()
	p, err := m.mconn.Accept(peerConfig)
	if err != nil {
		return nil, err
	}
	return &mConnConnection{
		transport: m,
		peer:      p.(*peer),
	}, nil
}

func (m *MConnTransport) Dial(ctx context.Context, endpoint Endpoint) (Connection, error) {
	m.mtx.RLock()
	peerConfig := m.mtx.peerConfig
	m.mtx.RUnlock()
	p, err := m.mconn.Dial(NetAddress{
		ID:   endpoint.PeerID,
		IP:   endpoint.IP,
		Port: endpoint.Port,
	}, peerConfig)
	if err != nil {
		return nil, err
	}
	return &mConnConnection{
		transport: m,
		peer:      p.(*peer),
	}, nil
}

func (m *MConnTransport) Endpoints() []Endpoint {
	if !m.mconn.netAddr.HasID() {
		return []Endpoint{}
	}
	return []Endpoint{{
		Protocol: MConnProtocol,
		PeerID:   m.mconn.nodeInfo.ID(),
		IP:       m.mconn.netAddr.IP, // FIXME copy
		Port:     m.mconn.netAddr.Port,
	}}
}

// mConnConnection implements Connection for MConnTransport.
type mConnConnection struct {
	peer      *peer
	transport *MConnTransport
}

var _ Connection = (*mConnConnection)(nil)

func (c *mConnConnection) PubKey() crypto.PubKey {
	return c.peer.conn.(*conn.SecretConnection).RemotePubKey()
}

func (c *mConnConnection) LocalEndpoint() Endpoint {
	addr := c.peer.conn.LocalAddr().(*net.TCPAddr)
	return Endpoint{
		Protocol: MConnProtocol,
		PeerID:   c.transport.mconn.nodeInfo.ID(),
		IP:       addr.IP,
		Port:     uint16(addr.Port),
	}
}

func (c *mConnConnection) RemoteEndpoint() Endpoint {
	addr := c.peer.conn.RemoteAddr().(*net.TCPAddr)
	return Endpoint{
		Protocol: MConnProtocol,
		PeerID:   c.peer.ID(),
		IP:       addr.IP,
		Port:     uint16(addr.Port),
	}
}

func (c *mConnConnection) Stream(id uint16) (Stream, error) {
	return &mConnStream{}, nil
}

func (c *mConnConnection) Close() error {
	c.transport.mconn.Cleanup(c.peer)
	return c.peer.Stop()
}

// mConnStream implements Stream for MConnTransport.
type mConnStream struct {
}

func (s *mConnStream) Close() error {
	panic("not implemented")
}

func (s *mConnStream) Write(bz []byte) (int, error) {
	panic("not implemented")
}

func (s *mConnStream) Read(target []byte) (int, error) {
	panic("not implemented")
}
