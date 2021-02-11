package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"sync"

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
	mConnConfig conn.MConnConfig,
	channelDescs []*ChannelDescriptor,
	options MConnTransportOptions,
) *MConnTransport {
	return &MConnTransport{
		logger:       logger,
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

// Endpoints implements Transport.
func (m *MConnTransport) Endpoints() []Endpoint {
	if m.listener == nil {
		return []Endpoint{}
	}
	select {
	case <-m.closeCh:
		return []Endpoint{}
	default:
	}
	endpoint := Endpoint{
		Protocol: MConnProtocol,
	}
	if addr, ok := m.listener.Addr().(*net.TCPAddr); ok {
		endpoint.IP = addr.IP
		endpoint.Port = uint16(addr.Port)
	}
	return []Endpoint{endpoint}
}

// Listen asynchronously listens for inbound connections on the given endpoint.
// It must be called exactly once before calling Accept(), and the caller must
// call Close() to shut down the listener.
//
// FIXME: Listen currently only supports listening on a single endpoint, it
// might be useful to support listening on multiple addresses (e.g. IPv4 and
// IPv6, or a private and public address) via multiple Listen() calls.
func (m *MConnTransport) Listen(endpoint Endpoint) error {
	if m.listener != nil {
		return errors.New("transport is already listening")
	}
	if err := m.validateEndpoint(endpoint); err != nil {
		return err
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(
		endpoint.IP.String(), strconv.Itoa(int(endpoint.Port))))
	if err != nil {
		return err
	}
	if m.options.MaxAcceptedConnections > 0 {
		// FIXME: This will establish the inbound connection but simply hang it
		// until another connection is released. It would probably be better to
		// return an error to the remote peer or close the connection. This is
		// also a DoS vector since the connection will take up kernel resources.
		// This was just carried over from the legacy P2P stack.
		listener = netutil.LimitListener(listener, int(m.options.MaxAcceptedConnections))
	}
	m.listener = listener

	return nil
}

// Accept implements Transport.
func (m *MConnTransport) Accept() (Connection, error) {
	if m.listener == nil {
		return nil, errors.New("transport is not listening")
	}

	tcpConn, err := m.listener.Accept()
	if err != nil {
		select {
		case <-m.closeCh:
			return nil, io.EOF
		default:
			return nil, err
		}
	}

	return newMConnConnection(m.logger, tcpConn, m.mConnConfig, m.channelDescs), nil
}

// Dial implements Transport.
func (m *MConnTransport) Dial(ctx context.Context, endpoint Endpoint) (Connection, error) {
	if err := m.validateEndpoint(endpoint); err != nil {
		return nil, err
	}
	if endpoint.Port == 0 {
		endpoint.Port = 26657
	}

	dialer := net.Dialer{}
	tcpConn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(
		endpoint.IP.String(), strconv.Itoa(int(endpoint.Port))))
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return nil, err
		}
	}

	return newMConnConnection(m.logger, tcpConn, m.mConnConfig, m.channelDescs), nil
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

// validateEndpoint validates an endpoint.
func (m *MConnTransport) validateEndpoint(endpoint Endpoint) error {
	if err := endpoint.Validate(); err != nil {
		return err
	}
	if endpoint.Protocol != MConnProtocol && endpoint.Protocol != TCPProtocol {
		return fmt.Errorf("unsupported protocol %q", endpoint.Protocol)
	}
	if len(endpoint.IP) == 0 {
		return errors.New("endpoint has no IP address")
	}
	if endpoint.Path != "" {
		return fmt.Errorf("endpoints with path not supported (got %q)", endpoint.Path)
	}
	return nil
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

	mconn *conn.MConnection // set during Handshake()
}

// mConnMessage passes MConnection messages through internal channels.
type mConnMessage struct {
	channelID ChannelID
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
		errorCh:      make(chan error, 1), // buffered to avoid onError leak
		closeCh:      make(chan struct{}),
	}
}

// Handshake implements Connection.
func (c *mConnConnection) Handshake(
	ctx context.Context,
	nodeInfo NodeInfo,
	privKey crypto.PrivKey,
) (NodeInfo, crypto.PubKey, error) {
	var (
		mconn    *conn.MConnection
		peerInfo NodeInfo
		peerKey  crypto.PubKey
		errCh    = make(chan error, 1)
	)
	// To handle context cancellation, we need to do the handshake in a
	// goroutine and abort the blocking network calls by closing the connection
	// when the context is canceled.
	go func() {
		// FIXME: Since the MConnection code panics, we need to recover it and turn it
		// into an error. We should remove panics instead.
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("recovered from panic: %v", r)
			}
		}()
		var err error
		mconn, peerInfo, peerKey, err = c.handshake(ctx, nodeInfo, privKey)
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		_ = c.Close()
		return NodeInfo{}, nil, ctx.Err()

	case err := <-errCh:
		if err != nil {
			return NodeInfo{}, nil, err
		}
		c.mconn = mconn
		c.logger = mconn.Logger
		if err = c.mconn.Start(); err != nil {
			return NodeInfo{}, nil, err
		}
		return peerInfo, peerKey, nil
	}
}

// handshake is a helper for Handshake, simplifying error handling so we can
// keep context handling and panic recovery in Handshake. It returns an
// unstarted but handshaked MConnection, to avoid concurrent field writes.
func (c *mConnConnection) handshake(
	ctx context.Context,
	nodeInfo NodeInfo,
	privKey crypto.PrivKey,
) (*conn.MConnection, NodeInfo, crypto.PubKey, error) {
	if c.mconn != nil {
		return nil, NodeInfo{}, nil, errors.New("connection is already handshaked")
	}

	secretConn, err := conn.MakeSecretConnection(c.conn, privKey)
	if err != nil {
		return nil, NodeInfo{}, nil, err
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
			return nil, NodeInfo{}, nil, err
		}
	}
	peerInfo, err := NodeInfoFromProto(&pbPeerInfo)
	if err != nil {
		return nil, NodeInfo{}, nil, err
	}

	mconn := conn.NewMConnectionWithConfig(
		secretConn,
		c.channelDescs,
		c.onReceive,
		c.onError,
		c.mConnConfig,
	)
	mconn.SetLogger(c.logger.With("peer", c.RemoteEndpoint().NodeAddress(peerInfo.NodeID)))

	return mconn, peerInfo, secretConn.RemotePubKey(), nil
}

// onReceive is a callback for MConnection received messages.
func (c *mConnConnection) onReceive(chID byte, payload []byte) {
	select {
	case c.receiveCh <- mConnMessage{channelID: ChannelID(chID), payload: payload}:
	case <-c.closeCh:
	}
}

// onError is a callback for MConnection errors. The error is passed via errorCh
// to ReceiveMessage (but not SendMessage, for legacy P2P stack behavior).
func (c *mConnConnection) onError(e interface{}) {
	err, ok := e.(error)
	if !ok {
		err = fmt.Errorf("%v", err)
	}
	// We have to close the connection here, since MConnection will have stopped
	// the service on any errors.
	_ = c.Close()
	select {
	case c.errorCh <- err:
	case <-c.closeCh:
	}
}

// String displays connection information.
func (c *mConnConnection) String() string {
	return c.RemoteEndpoint().String()
}

// SendMessage implements Connection.
func (c *mConnConnection) SendMessage(chID ChannelID, msg []byte) (bool, error) {
	if chID > math.MaxUint8 {
		return false, fmt.Errorf("MConnection only supports 1-byte channel IDs (got %v)", chID)
	}
	select {
	case err := <-c.errorCh:
		return false, err
	case <-c.closeCh:
		return false, io.EOF
	default:
		return c.mconn.Send(byte(chID), msg), nil
	}
}

// TrySendMessage implements Connection.
func (c *mConnConnection) TrySendMessage(chID ChannelID, msg []byte) (bool, error) {
	if chID > math.MaxUint8 {
		return false, fmt.Errorf("MConnection only supports 1-byte channel IDs (got %v)", chID)
	}
	select {
	case err := <-c.errorCh:
		return false, err
	case <-c.closeCh:
		return false, io.EOF
	default:
		return c.mconn.TrySend(byte(chID), msg), nil
	}
}

// ReceiveMessage implements Connection.
func (c *mConnConnection) ReceiveMessage() (ChannelID, []byte, error) {
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
		if c.mconn != nil && c.mconn.IsRunning() {
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
		if c.mconn != nil && c.mconn.IsRunning() {
			c.mconn.FlushStop()
		} else {
			err = c.conn.Close()
		}
		close(c.closeCh)
	})
	return err
}
