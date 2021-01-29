package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/net/netutil"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/protoio"
	"github.com/tendermint/tendermint/p2p/conn"
	p2pproto "github.com/tendermint/tendermint/proto/tendermint/p2p"
)

const (
	MConnProtocol Protocol = "mconn"
	TCPProtocol   Protocol = "tcp"
)

// MConnTransportOptions sets options for MConnTransport.
type MConnTransportOptions struct {
	// MaxAcceptedConnections is the maximum number of simultaneous accepted
	// (incoming) connections. Beyond this, new connections will block until
	// a slot is free. 0 means unlimited.
	//
	// FIXME: We may want to replace this with connection accounting in the
	// Router, since it will need to do e.g. rate limiting and such as well.
	// But it might also make sense to have per-transport limits.
	MaxAcceptedConnections uint32
}

// MConnTransport is a Transport implementation using the current multiplexed
// Tendermint protocol ("MConn").
type MConnTransport struct {
	logger       log.Logger
	nodeID       NodeID
	options      MConnTransportOptions
	mConnConfig  conn.MConnConfig
	channelDescs []*ChannelDescriptor
	closeCh      chan struct{}
	closeOnce    sync.Once

	listener net.Listener
}

// NewMConnTransport sets up a new MConnection transport. This uses the
// proprietary Tendermint MConnection protocol, which is implemented as
// conn.MConnection.
func NewMConnTransport(
	logger log.Logger,
	nodeID NodeID,
	mConnConfig conn.MConnConfig,
	channelDescs []*ChannelDescriptor,
	options MConnTransportOptions,
) *MConnTransport {
	return &MConnTransport{
		logger:       logger,
		nodeID:       nodeID,
		options:      options,
		mConnConfig:  mConnConfig,
		closeCh:      make(chan struct{}),
		channelDescs: channelDescs,
	}
}

// String implements Transport.
func (m *MConnTransport) String() string {
	return string(MConnProtocol)
}

// Protocols implements Transport. We support tcp for backwards-compatibility.
func (m *MConnTransport) Protocols() []Protocol {
	return []Protocol{MConnProtocol, TCPProtocol}
}

// Listen asynchronously listens for inbound connections on the given endpoint.
// It must be called exactly once before calling Accept(), and the caller must
// call Close() to shut down the listener.
func (m *MConnTransport) Listen(endpoint Endpoint) error {
	if m.listener != nil {
		return errors.New("transport is already listening")
	}
	endpoint, err := m.normalizeEndpoint(endpoint)
	if err != nil {
		return fmt.Errorf("invalid MConn listen endpoint %q: %w", endpoint, err)
	}

	m.listener, err = net.Listen("tcp", fmt.Sprintf("%v:%v", endpoint.IP, endpoint.Port))
	if err != nil {
		return err
	}
	if m.options.MaxAcceptedConnections > 0 {
		m.listener = netutil.LimitListener(m.listener, int(m.options.MaxAcceptedConnections))
	}
	return nil
}

// Accept implements Transport.
func (m *MConnTransport) Accept(ctx context.Context) (Connection, error) {
	if m.listener == nil {
		return nil, errors.New("transport is not listening")
	}

	if deadline, ok := ctx.Deadline(); ok {
		if tcpListener, ok := m.listener.(*net.TCPListener); ok {
			// FIXME: This probably needs to have a goroutine that overrides the
			// deadline on context cancellation as well.
			if err := tcpListener.SetDeadline(deadline); err != nil {
				return nil, err
			}
		}
	}

	tcpConn, err := m.listener.Accept()
	if err != nil {
		select {
		case <-m.closeCh:
			return nil, io.EOF
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return nil, err
		}
	}

	return newMConnConnection(m.logger, tcpConn, m.mConnConfig, m.channelDescs), nil
}

// Dial implements Transport.
func (m *MConnTransport) Dial(ctx context.Context, endpoint Endpoint) (Connection, error) {
	endpoint, err := m.normalizeEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	dialer := net.Dialer{}
	tcpConn, err := dialer.DialContext(ctx, "tcp",
		net.JoinHostPort(endpoint.IP.String(), fmt.Sprintf("%v", endpoint.Port)))
	if err != nil {
		return nil, err
	}

	return newMConnConnection(m.logger, tcpConn, m.mConnConfig, m.channelDescs), nil
}

// Endpoints implements Transport.
func (m *MConnTransport) Endpoints() []Endpoint {
	if m.listener == nil {
		return []Endpoint{}
	}
	endpoint := Endpoint{
		NodeID:   m.nodeID,
		Protocol: MConnProtocol,
	}
	if addr, ok := m.listener.Addr().(*net.TCPAddr); ok {
		endpoint.IP = addr.IP
		endpoint.Port = uint16(addr.Port)
	}
	return []Endpoint{endpoint}
}

// Close implements Transport.
func (m *MConnTransport) Close() error {
	var err error
	m.closeOnce.Do(func() {
		close(m.closeCh) // must be closed first, to handle error in Accept()
		if m.listener != nil {
			err = m.listener.Close()
		}
	})
	return err
}

// normalizeEndpoint normalizes and validates an endpoint.
func (m *MConnTransport) normalizeEndpoint(endpoint Endpoint) (Endpoint, error) {
	if err := endpoint.Validate(); err != nil {
		return Endpoint{}, err
	}
	if endpoint.Protocol != MConnProtocol && endpoint.Protocol != TCPProtocol {
		return Endpoint{}, fmt.Errorf("unsupported protocol %q", endpoint.Protocol)
	}
	if len(endpoint.IP) == 0 {
		return Endpoint{}, errors.New("endpoint must have an IP address")
	}
	if endpoint.Path != "" {
		return Endpoint{}, fmt.Errorf("endpoint cannot have path (got %q)", endpoint.Path)
	}
	if endpoint.Port == 0 {
		endpoint.Port = 26657
	}
	return endpoint, nil
}

// mConnConnection implements Connection for MConnTransport.
type mConnConnection struct {
	logger       log.Logger
	conn         net.Conn
	mConnConfig  conn.MConnConfig
	channelDescs []*ChannelDescriptor
	receiveCh    chan mConnMessage
	errorCh      chan error
	closeCh      chan struct{}
	closeOnce    sync.Once

	// These are set during Handshake()
	mconn    *conn.MConnection
	localID  NodeID
	remoteID NodeID
}

// mConnMessage passes MConnection messages through internal channels.
type mConnMessage struct {
	channelID byte
	payload   []byte
}

// newMConnConnection creates a new mConnConnection.
func newMConnConnection(
	logger log.Logger,
	conn net.Conn,
	mConnConfig conn.MConnConfig,
	channelDescs []*ChannelDescriptor,
) *mConnConnection {
	return &mConnConnection{
		logger:       logger,
		conn:         conn,
		mConnConfig:  mConnConfig,
		channelDescs: channelDescs,
		receiveCh:    make(chan mConnMessage),
		errorCh:      make(chan error),
		closeCh:      make(chan struct{}),
	}
}

// Handshake implements Transport.
//
// FIXME: Since the MConnection code panics, we need to recover it and turn it
// into an error. We should remove panics instead.
func (c *mConnConnection) Handshake(
	ctx context.Context,
	nodeInfo NodeInfo,
	privKey crypto.PrivKey,
) (peerInfo NodeInfo, peerKey crypto.PubKey, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from panic: %v", r)
		}
	}()

	peerInfo, peerKey, err = c.handshake(ctx, nodeInfo, privKey)
	return
}

// handshake is a helper for Handshake, simplifying error handling so we can
// keep panic recovery in Handshake. It sets c.mconn.
//
// FIXME: Move this into Handshake() when MConnection no longer panics.
func (c *mConnConnection) handshake(
	ctx context.Context,
	nodeInfo NodeInfo,
	privKey crypto.PrivKey,
) (NodeInfo, crypto.PubKey, error) {
	if c.mconn != nil {
		return NodeInfo{}, nil, errors.New("connection is already handshaked")
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetDeadline(deadline); err != nil {
			return NodeInfo{}, nil, err
		}
	}

	secretConn, err := conn.MakeSecretConnection(c.conn, privKey)
	if err != nil {
		return NodeInfo{}, nil, err
	}

	var pbPeerInfo p2pproto.NodeInfo
	errCh := make(chan error, 2)
	go func() {
		_, err := protoio.NewDelimitedWriter(secretConn).WriteMsg(nodeInfo.ToProto())
		errCh <- err
	}()
	go func() {
		_, err := protoio.NewDelimitedReader(secretConn, MaxNodeInfoSize()).ReadMsg(&pbPeerInfo)
		errCh <- err
	}()
	for i := 0; i < cap(errCh); i++ {
		if err = <-errCh; err != nil {
			return NodeInfo{}, nil, err
		}
	}
	peerInfo, err := NodeInfoFromProto(&pbPeerInfo)
	if err != nil {
		return NodeInfo{}, nil, err
	}

	if err = c.conn.SetDeadline(time.Time{}); err != nil {
		return NodeInfo{}, nil, err
	}

	c.localID = nodeInfo.NodeID
	c.remoteID = peerInfo.NodeID
	// FIXME: Uses NetAddress for backwards compatibility, should probably
	// just use Endpoint.String().
	c.logger = c.logger.With("peer", c.RemoteEndpoint().NetAddress().String())

	mconn := conn.NewMConnectionWithConfig(
		secretConn,
		c.channelDescs,
		c.onReceive,
		c.onError,
		c.mConnConfig,
	)
	mconn.SetLogger(c.logger)
	if err = mconn.Start(); err != nil {
		return NodeInfo{}, nil, err
	}
	c.mconn = mconn

	return peerInfo, secretConn.RemotePubKey(), nil
}

// onReceive is a callback for MConnection received messages.
func (c *mConnConnection) onReceive(channelID byte, payload []byte) {
	select {
	case c.receiveCh <- mConnMessage{channelID: channelID, payload: payload}:
	case <-c.closeCh:
	}
}

// onError is a callback for MConnection errors. The error is passed to errorCh,
// which is only consumed by ReceiveMessage() for parity with the old
// MConnection behavior.
func (c *mConnConnection) onError(e interface{}) {
	err, ok := e.(error)
	if !ok {
		err = fmt.Errorf("%v", err)
	}
	select {
	case c.errorCh <- err:
	case <-c.closeCh:
	}
}

// String displays connection information.
// FIXME: This is here for backwards compatibility with existing logging,
// it should probably just return RemoteEndpoint().String(), if anything.
func (c *mConnConnection) String() string {
	endpoint := c.RemoteEndpoint()
	return fmt.Sprintf("MConn{%v:%v}", endpoint.IP, endpoint.Port)
}

// SendMessage implements Connection.
func (c *mConnConnection) SendMessage(channelID byte, msg []byte) (bool, error) {
	// We don't check errorCh here, to preserve old MConnection behavior.
	select {
	case <-c.closeCh:
		return false, io.EOF
	default:
		return c.mconn.Send(channelID, msg), nil
	}
}

// TrySendMessage implements Connection.
func (c *mConnConnection) TrySendMessage(channelID byte, msg []byte) (bool, error) {
	// We don't check errorCh here, to preserve old MConnection behavior.
	select {
	case <-c.closeCh:
		return false, io.EOF
	default:
		return c.mconn.TrySend(channelID, msg), nil
	}
}

// ReceiveMessage implements Connection.
func (c *mConnConnection) ReceiveMessage() (byte, []byte, error) {
	select {
	case err := <-c.errorCh:
		return 0, nil, err
	case <-c.closeCh:
		return 0, nil, io.EOF
	case msg := <-c.receiveCh:
		return msg.channelID, msg.payload, nil
	}
}

// LocalEndpoint implements Connection.
func (c *mConnConnection) LocalEndpoint() Endpoint {
	endpoint := Endpoint{
		Protocol: MConnProtocol,
		NodeID:   c.localID,
	}
	if addr, ok := c.conn.LocalAddr().(*net.TCPAddr); ok {
		endpoint.IP = addr.IP
		endpoint.Port = uint16(addr.Port)
	}
	return endpoint
}

// RemoteEndpoint implements Connection.
func (c *mConnConnection) RemoteEndpoint() Endpoint {
	endpoint := Endpoint{
		Protocol: MConnProtocol,
		NodeID:   c.remoteID,
	}
	if addr, ok := c.conn.RemoteAddr().(*net.TCPAddr); ok {
		endpoint.IP = addr.IP
		endpoint.Port = uint16(addr.Port)
	}
	return endpoint
}

// Status implements Connection.
func (c *mConnConnection) Status() conn.ConnectionStatus {
	if c.mconn == nil {
		return conn.ConnectionStatus{}
	}
	return c.mconn.Status()
}

// Close implements Connection.
func (c *mConnConnection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		if c.mconn != nil {
			err = c.mconn.Stop()
		} else {
			err = c.conn.Close()
		}
		close(c.closeCh)
	})
	return err
}

// FlushClose implements Connection.
func (c *mConnConnection) FlushClose() error {
	var err error
	c.closeOnce.Do(func() {
		if c.mconn != nil {
			c.mconn.FlushStop()
		} else {
			err = c.conn.Close()
		}
		close(c.closeCh)
	})
	return err
}
