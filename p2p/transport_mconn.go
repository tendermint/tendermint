package p2p

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/p2p/conn"
	tmconn "github.com/tendermint/tendermint/p2p/conn"
	"golang.org/x/net/netutil"
)

// MConnProtocol is the MConn protocol identifier.
const MConnProtocol Protocol = "mconn"

// MConnTransportOption sets an option for MConnTransport.
type MConnTransportOption func(*MConnTransport)

// MConnTransportMaxIncomingConnections sets the maximum number of
// simultaneous incoming connections. Default: 0 (unlimited)
func MConnTransportMaxIncomingConnections(max int) MConnTransportOption {
	return func(mt *MConnTransport) { mt.maxIncomingConnections = max }
}

// MConnTransport is a Transport implementation using the current multiplexed
// Tendermint protocol ("MConn").
type MConnTransport struct {
	nodeKey NodeKey

	listener net.Listener
	chAccept chan *mConnConnection
	chError  chan error
	chClose  chan struct{}

	maxIncomingConnections int

	mconn *MultiplexTransport
	mtx   struct {
		sync.RWMutex
		peerConfig peerConfig
	}
}

var _ Transport = (*MConnTransport)(nil)

// NewMConnTransport sets up a new MConn transport.
func NewMConnTransport(
	nodeInfo NodeInfo,
	nodeKey NodeKey,
	mConfig conn.MConnConfig,
	opts ...MConnTransportOption,
) *MConnTransport {
	m := &MConnTransport{
		nodeKey:  nodeKey,
		chAccept: make(chan *mConnConnection),
		chError:  make(chan error),
		chClose:  make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *MConnTransport) setPeerConfig(peerConfig peerConfig) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.mtx.peerConfig = peerConfig
}

// normalizeEndpoint normalizes an endpoint, returning an error
// if invalid.
func (m *MConnTransport) normalizeEndpoint(endpoint *Endpoint) error {
	if endpoint == nil {
		return errors.New("nil endpoint")
	}
	if err := endpoint.Validate(); err != nil {
		return err
	}
	if endpoint.Protocol != MConnProtocol {
		return fmt.Errorf("unsupported protocol %q", endpoint.Protocol)
	}
	if len(endpoint.IP) == 0 {
		return errors.New("endpoint must have an IP address")
	}
	if endpoint.Path != "" {
		return fmt.Errorf("endpoint cannot have path (got %q)", endpoint.Path)
	}
	if endpoint.Port == 0 {
		endpoint.Port = 26657
	}
	return nil
}

// Listen listens for inbound connections on the given endpoint.
func (m *MConnTransport) Listen(endpoint Endpoint) error {
	if m.listener != nil {
		return fmt.Errorf("MConn transport is already listening")
	}
	err := m.normalizeEndpoint(&endpoint)
	if err != nil {
		return fmt.Errorf("invalid MConn endpoint %q: %w", endpoint, err)
	}

	m.listener, err = net.Listen("tcp", fmt.Sprintf("%v:%v", endpoint.IP, endpoint.Port))
	if err != nil {
		return err
	}
	if m.maxIncomingConnections > 0 {
		m.listener = netutil.LimitListener(m.listener, m.maxIncomingConnections)
	}

	// Spawn a goroutine to accept inbound connections asynchronously.
	go m.accept()

	return nil
}

// accept accepts inbound connections in a loop, and asynchronously
// handshakes with the peer to avoid head-of-line blocking.
// See: https://github.com/tendermint/tendermint/issues/204
func (m *MConnTransport) accept() {
	for {
		tcpConn, err := m.listener.Accept()
		if err != nil {
			select {
			case m.chError <- err:
			case <-m.chClose:
			}
			return
		}
		go func() {
			conn, err := m.acceptConn(tcpConn)
			if err != nil {
				_ = tcpConn.Close()
				select {
				case m.chError <- err:
				case <-m.chClose:
				}
			} else {
				select {
				case m.chAccept <- conn:
				case <-m.chClose:
					_ = conn.Close()
				}
			}
		}()
	}
}

// acceptConn accepts a connection and performs a handshake. It is called
// asynchronously by accept().
func (m *MConnTransport) acceptConn(tcpConn net.Conn) (conn *mConnConnection, err error) {
	// FIXME Since the MConnection code panics, we need to recover here
	// and turn it into an error.
	//
	// Be careful with err, since the recover function needs to be able to
	// write to the variable that is returned from the main function, otherwise
	// the error will be lost.
	defer func() {
		if r := recover(); r != nil {
			err = ErrRejected{
				conn:          tcpConn,
				err:           fmt.Errorf("recovered from panic: %v", r),
				isAuthFailure: true,
			}
		}
	}()

	// FIXME This needs to filter connections, see mt.filterConn().
	conn, err = newMConnConnection(tcpConn, m.nodeKey.PrivKey)
	return
}

// Accept implements Transport.
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

// Close implements Transport.
func (m *MConnTransport) Close() error {
	close(m.chClose)
	if m.listener != nil {
		return m.listener.Close()
	}
	return nil
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

// mConnConnection implements Connection for MConnTransport. It takes
// a base TCP connection as input, and upgrades it to MConn with a
// handshake.
type mConnConnection struct {
	secretConn *tmconn.SecretConnection
}

// newMConnConnection creates a new mConnConnection by handshaking
// with a peer.
func newMConnConnection(conn net.Conn, privKey crypto.PrivKey) (*mConnConnection, error) {
	// FIXME Needs to set deadlines.
	sconn, err := tmconn.MakeSecretConnection(conn, privKey)
	if err != nil {
		return nil, ErrRejected{
			conn:          conn,
			err:           fmt.Errorf("secret conn failed: %v", err),
			isAuthFailure: true,
		}
	}
	return &mConnConnection{
		secretConn: sconn,
	}, nil
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
