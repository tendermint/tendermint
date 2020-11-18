package p2p

import (
	"context"
	"net"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/p2p/conn"
)

const MConnProtocol Protocol = "mconn"

// MConnTransport is a NewTransport implementation using the current multiplexed
// Tendermint protocol.
type MConnTransport struct {
	mconn      *MultiplexTransport
	peerConfig peerConfig
}

var _ NewTransport = (*MConnTransport)(nil)

func NewMConnTransport(
	nodeInfo NodeInfo,
	nodeKey NodeKey,
	mConfig conn.MConnConfig,
	peerConfig peerConfig,
) *MConnTransport {
	return &MConnTransport{
		mconn:      NewMultiplexTransport(nodeInfo, nodeKey, mConfig),
		peerConfig: peerConfig,
	}
}

func (m *MConnTransport) Listen(endpoint Endpoint) error {
	return m.mconn.Listen(NetAddress{
		ID:   m.mconn.nodeInfo.ID(),
		IP:   endpoint.IP,
		Port: endpoint.Port,
	})
}

func (m *MConnTransport) Close() error {
	return m.mconn.Close()
}

func (m *MConnTransport) Accept(ctx context.Context) (Connection, error) {
	p, err := m.mconn.Accept(m.peerConfig)
	if err != nil {
		return nil, err
	}
	return &mConnConnection{peer: p.(*peer)}, nil
}

func (m *MConnTransport) Dial(ctx context.Context, endpoint Endpoint) (Connection, error) {
	return nil, nil
}

func (m *MConnTransport) Endpoints() []Endpoint {
	if !m.mconn.netAddr.HasID() {
		return []Endpoint{}
	}
	return []Endpoint{{
		Protocol: MConnProtocol,
		IP:       m.mconn.netAddr.IP, // FIXME copy
		Port:     m.mconn.netAddr.Port,
	}}
}

// mConnConnection implements Connection for MConnTransport.
type mConnConnection struct {
	peer *peer
}

var _ Connection = (*mConnConnection)(nil)

func (c *mConnConnection) PubKey() crypto.PubKey {
	return c.peer.conn.(*conn.SecretConnection).RemotePubKey()
}

func (c *mConnConnection) LocalEndpoint() Endpoint {
	addr := c.peer.conn.LocalAddr().(*net.TCPAddr)
	return Endpoint{
		Protocol: MConnProtocol,
		IP:       addr.IP,
		Port:     uint16(addr.Port),
	}
}

func (c *mConnConnection) RemoteEndpoint() Endpoint {
	addr := c.peer.conn.RemoteAddr().(*net.TCPAddr)
	return Endpoint{
		Protocol: MConnProtocol,
		IP:       addr.IP,
		Port:     uint16(addr.Port),
	}
}

func (c *mConnConnection) Stream(id uint16) (Stream, error) {
	return &mConnStream{}, nil
}

func (c *mConnConnection) Close() error {
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
