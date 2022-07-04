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
	"time"

	"golang.org/x/net/netutil"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	"github.com/tendermint/tendermint/internal/p2p/conn"
	"github.com/tendermint/tendermint/libs/log"
	p2pproto "github.com/tendermint/tendermint/proto/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
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

	closeOnce sync.Once
	doneCh    chan struct{}
	listener  net.Listener
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
		doneCh:       make(chan struct{}),
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

// Endpoint implements Transport.
func (m *MConnTransport) Endpoint() (*Endpoint, error) {
	if m.listener == nil {
		return nil, errors.New("listenter not defined")
	}
	select {
	case <-m.doneCh:
		return nil, errors.New("transport closed")
	default:
	}

	endpoint := &Endpoint{
		Protocol: MConnProtocol,
	}
	if addr, ok := m.listener.Addr().(*net.TCPAddr); ok {
		endpoint.IP = addr.IP
		endpoint.Port = uint16(addr.Port)
	}
	return endpoint, nil
}

// Listen asynchronously listens for inbound connections on the given endpoint.
// It must be called exactly once before calling Accept(), and the caller must
// call Close() to shut down the listener.
//
// FIXME: Listen currently only supports listening on a single endpoint, it
// might be useful to support listening on multiple addresses (e.g. IPv4 and
// IPv6, or a private and public address) via multiple Listen() calls.
func (m *MConnTransport) Listen(endpoint *Endpoint) error {
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
func (m *MConnTransport) Accept(ctx context.Context) (Connection, error) {
	if m.listener == nil {
		return nil, errors.New("transport is not listening")
	}

	conCh := make(chan net.Conn)
	errCh := make(chan error)
	go func() {
		tcpConn, err := m.listener.Accept()
		if err != nil {
			select {
			case errCh <- err:
			case <-ctx.Done():
			}
		}
		select {
		case conCh <- tcpConn:
		case <-ctx.Done():
		}
	}()

	select {
	case <-ctx.Done():
		m.listener.Close()
		return nil, io.EOF
	case <-m.doneCh:
		m.listener.Close()
		return nil, io.EOF
	case err := <-errCh:
		return nil, err
	case tcpConn := <-conCh:
		return newMConnConnection(m.logger, tcpConn, m.mConnConfig, m.channelDescs), nil
	}

}

// Dial implements Transport.
func (m *MConnTransport) Dial(ctx context.Context, endpoint *Endpoint) (Connection, error) {
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
		close(m.doneCh)
		if m.listener != nil {
			err = m.listener.Close()
		}
	})
	return err
}

// SetChannels sets the channel descriptors to be used when
// establishing a connection.
//
// FIXME: To be removed when the legacy p2p stack is removed. Channel
// descriptors should be managed by the router. The underlying transport and
// connections should be agnostic to everything but the channel ID's which are
// initialized in the handshake.
func (m *MConnTransport) AddChannelDescriptors(channelDesc []*ChannelDescriptor) {
	m.channelDescs = append(m.channelDescs, channelDesc...)
}

// validateEndpoint validates an endpoint.
func (m *MConnTransport) validateEndpoint(endpoint *Endpoint) error {
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
	doneCh       chan struct{}
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
		doneCh:       make(chan struct{}),
	}
}

// Handshake implements Connection.
func (c *mConnConnection) Handshake(
	ctx context.Context,
	timeout time.Duration,
	nodeInfo types.NodeInfo,
	privKey crypto.PrivKey,
) (types.NodeInfo, crypto.PubKey, error) {
	var (
		mconn    *conn.MConnection
		peerInfo types.NodeInfo
		peerKey  crypto.PubKey
		errCh    = make(chan error, 1)
	)
	handshakeCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		handshakeCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
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
		mconn, peerInfo, peerKey, err = c.handshake(handshakeCtx, nodeInfo, privKey)

		select {
		case errCh <- err:
		case <-handshakeCtx.Done():
		}

	}()

	select {
	case <-handshakeCtx.Done():
		_ = c.Close()
		return types.NodeInfo{}, nil, handshakeCtx.Err()

	case err := <-errCh:
		if err != nil {
			return types.NodeInfo{}, nil, err
		}
		c.mconn = mconn
		// Start must not use the handshakeCtx. The handshakeCtx may have a
		// timeout set that is intended to terminate only the handshake procedure.
		// The context passed to Start controls the entire lifecycle of the
		// mconn.
		if err = c.mconn.Start(ctx); err != nil {
			return types.NodeInfo{}, nil, err
		}
		return peerInfo, peerKey, nil
	}
}

// handshake is a helper for Handshake, simplifying error handling so we can
// keep context handling and panic recovery in Handshake. It returns an
// unstarted but handshaked MConnection, to avoid concurrent field writes.
func (c *mConnConnection) handshake(
	ctx context.Context,
	nodeInfo types.NodeInfo,
	privKey crypto.PrivKey,
) (*conn.MConnection, types.NodeInfo, crypto.PubKey, error) {
	if c.mconn != nil {
		return nil, types.NodeInfo{}, nil, errors.New("connection is already handshaked")
	}

	secretConn, err := conn.MakeSecretConnection(c.conn, privKey)
	if err != nil {
		return nil, types.NodeInfo{}, nil, err
	}

	wg := &sync.WaitGroup{}
	var pbPeerInfo p2pproto.NodeInfo
	errCh := make(chan error, 2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := protoio.NewDelimitedWriter(secretConn).WriteMsg(nodeInfo.ToProto())
		select {
		case errCh <- err:
		case <-ctx.Done():
		}

	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := protoio.NewDelimitedReader(secretConn, types.MaxNodeInfoSize()).ReadMsg(&pbPeerInfo)
		select {
		case errCh <- err:
		case <-ctx.Done():
		}
	}()

	wg.Wait()

	if err, ok := <-errCh; ok && err != nil {
		return nil, types.NodeInfo{}, nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, types.NodeInfo{}, nil, err
	}

	peerInfo, err := types.NodeInfoFromProto(&pbPeerInfo)
	if err != nil {
		return nil, types.NodeInfo{}, nil, err
	}

	mconn := conn.NewMConnection(
		c.logger.With("peer", c.RemoteEndpoint().NodeAddress(peerInfo.NodeID)),
		secretConn,
		c.channelDescs,
		c.onReceive,
		c.onError,
		c.mConnConfig,
	)

	return mconn, peerInfo, secretConn.RemotePubKey(), nil
}

// onReceive is a callback for MConnection received messages.
func (c *mConnConnection) onReceive(ctx context.Context, chID ChannelID, payload []byte) {
	select {
	case c.receiveCh <- mConnMessage{channelID: chID, payload: payload}:
	case <-ctx.Done():
	}
}

// onError is a callback for MConnection errors. The error is passed via errorCh
// to ReceiveMessage (but not SendMessage, for legacy P2P stack behavior).
func (c *mConnConnection) onError(ctx context.Context, e interface{}) {
	err, ok := e.(error)
	if !ok {
		err = fmt.Errorf("%v", err)
	}
	// We have to close the connection here, since MConnection will have stopped
	// the service on any errors.
	_ = c.Close()
	select {
	case c.errorCh <- err:
	case <-ctx.Done():
	}
}

// String displays connection information.
func (c *mConnConnection) String() string {
	return c.RemoteEndpoint().String()
}

// SendMessage implements Connection.
func (c *mConnConnection) SendMessage(ctx context.Context, chID ChannelID, msg []byte) error {
	if chID > math.MaxUint8 {
		return fmt.Errorf("MConnection only supports 1-byte channel IDs (got %v)", chID)
	}
	select {
	case err := <-c.errorCh:
		return err
	case <-ctx.Done():
		return io.EOF
	default:
		if ok := c.mconn.Send(chID, msg); !ok {
			return errors.New("sending message timed out")
		}

		return nil
	}
}

// ReceiveMessage implements Connection.
func (c *mConnConnection) ReceiveMessage(ctx context.Context) (ChannelID, []byte, error) {
	select {
	case err := <-c.errorCh:
		return 0, nil, err
	case <-c.doneCh:
		return 0, nil, io.EOF
	case <-ctx.Done():
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

// Close implements Connection.
func (c *mConnConnection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		defer close(c.doneCh)

		if c.mconn != nil && c.mconn.IsRunning() {
			c.mconn.Stop()
		} else {
			err = c.conn.Close()
		}
	})
	return err
}
