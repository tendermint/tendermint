package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/protoio"
	"github.com/tendermint/tendermint/p2p/conn"
	p2pproto "github.com/tendermint/tendermint/proto/tendermint/p2p"

	"golang.org/x/net/netutil"
)

const (
	defaultDialTimeout      = time.Second
	defaultFilterTimeout    = 5 * time.Second
	defaultHandshakeTimeout = 3 * time.Second
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

// MConnTransportFilterTimeout sets the timeout for filter callbacks.
func MConnTransportFilterTimeout(timeout time.Duration) MConnTransportOption {
	return func(mt *MConnTransport) { mt.filterTimeout = timeout }
}

// MConnTransportConnFilters sets connection filters.
func MConnTransportConnFilters(filters ...ConnFilterFunc) MConnTransportOption {
	return func(mt *MConnTransport) { mt.connFilters = filters }
}

// ConnFilterFunc is a callback for connection filtering. If it returns an
// error, the connection is rejected. The set of existing connections is passed
// along with the new connection and all resolved IPs.
type ConnFilterFunc func(ConnSet, net.Conn, []net.IP) error

// ConnDuplicateIPFilter resolves and keeps all ips for an incoming connection
// and refuses new ones if they come from a known ip.
var ConnDuplicateIPFilter ConnFilterFunc = func(cs ConnSet, c net.Conn, ips []net.IP) error {
	for _, ip := range ips {
		if cs.HasIP(ip) {
			return ErrRejected{
				conn:        c,
				err:         fmt.Errorf("ip<%v> already connected", ip),
				isDuplicate: true,
			}
		}
	}
	return nil
}

// MConnTransport is a Transport implementation using the current multiplexed
// Tendermint protocol ("MConn"). It inherits lots of code and logic from the
// previous implementation for parity with the current P2P stack (such as
// connection filtering, peer verification, and panic handling), this should be
// moved out of the transport once the rest of the P2P stack is rewritten.
type MConnTransport struct {
	privKey      crypto.PrivKey
	nodeInfo     DefaultNodeInfo
	channelDescs []*ChannelDescriptor
	mConnConfig  conn.MConnConfig

	maxIncomingConnections int
	dialTimeout            time.Duration
	handshakeTimeout       time.Duration
	filterTimeout          time.Duration

	logger      log.Logger
	listener    net.Listener
	chAccept    chan *mConnConnection
	chError     chan error
	chClose     chan struct{}
	chCloseOnce sync.Once

	// FIXME This is a vestige from the old transport, and should be managed
	// by the router once we rewrite the P2P core.
	conns       ConnSet
	connFilters []ConnFilterFunc
}

// NewMConnTransport sets up a new MConn transport.
func NewMConnTransport(
	logger log.Logger,
	nodeInfo NodeInfo, // FIXME should use DefaultNodeInfo, left for code compatibility
	privKey crypto.PrivKey,
	mConnConfig conn.MConnConfig,
	opts ...MConnTransportOption,
) *MConnTransport {
	m := &MConnTransport{
		privKey:      privKey,
		nodeInfo:     nodeInfo.(DefaultNodeInfo),
		mConnConfig:  mConnConfig,
		channelDescs: []*ChannelDescriptor{}, // FIXME Set by switch, for code compatibility

		dialTimeout:      defaultDialTimeout,
		handshakeTimeout: defaultHandshakeTimeout,
		filterTimeout:    defaultFilterTimeout,

		logger:   logger,
		chAccept: make(chan *mConnConnection),
		chError:  make(chan error),
		chClose:  make(chan struct{}),

		conns:       NewConnSet(),
		connFilters: []ConnFilterFunc{},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// SetChannelDescriptors implements Transport.
func (m *MConnTransport) SetChannelDescriptors(chDescs []*conn.ChannelDescriptor) {
	m.channelDescs = chDescs
}

// Listen asynchronously listens for inbound connections on the given endpoint.
// It must be called exactly once before calling Accept(), and the caller must
// call Close() to shut down the listener.
func (m *MConnTransport) Listen(endpoint Endpoint) error {
	if m.listener != nil {
		return errors.New("MConn transport is already listening")
	}
	if len(m.channelDescs) == 0 {
		return errors.New("no MConn channel descriptors")
	}
	err := m.normalizeEndpoint(&endpoint)
	if err != nil {
		return fmt.Errorf("invalid MConn listen endpoint %q: %w", endpoint, err)
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

// accept accepts inbound connections in a loop, and asynchronously handshakes
// with the peer to avoid head-of-line blocking. Established connections are
// passed to Accept() via the channel m.chAccept.
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
			err := m.filterTCPConn(tcpConn)
			if err != nil {
				_ = tcpConn.Close()
				select {
				case m.chError <- err:
				case <-m.chClose:
				}
			}
			conn, err := newMConnConnection(m, tcpConn, "")
			if err != nil {
				m.conns.Remove(tcpConn)
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

// Accept implements Transport.
func (m *MConnTransport) Accept(ctx context.Context) (Connection, error) {
	select {
	case conn := <-m.chAccept:
		return conn, nil
	case err := <-m.chError:
		return nil, err
	case <-m.chClose:
		return nil, ErrTransportClosed{}
	case <-ctx.Done():
		return nil, nil
	}
}

// Dial implements Transport.
func (m *MConnTransport) Dial(ctx context.Context, endpoint Endpoint) (Connection, error) {
	err := m.normalizeEndpoint(&endpoint)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, m.dialTimeout)
	defer cancel()
	dialer := net.Dialer{}
	tcpConn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%v:%v", endpoint.IP, endpoint.Port))
	if err != nil {
		return nil, err
	}

	err = m.filterTCPConn(tcpConn)
	if err != nil {
		return nil, err
	}

	conn, err := newMConnConnection(m, tcpConn, endpoint.PeerID)
	if err != nil {
		m.conns.Remove(tcpConn)
		return nil, err
	}

	return conn, nil
}

// Endpoints implements Transport.
func (m *MConnTransport) Endpoints() []Endpoint {
	if m.listener == nil {
		return []Endpoint{}
	}
	addr := m.listener.Addr().(*net.TCPAddr)
	return []Endpoint{{
		Protocol: MConnProtocol,
		PeerID:   m.nodeInfo.ID(),
		IP:       addr.IP,
		Port:     uint16(addr.Port),
	}}
}

// Close implements Transport.
func (m *MConnTransport) Close() error {
	m.chCloseOnce.Do(func() { close(m.chClose) })
	if m.listener != nil {
		return m.listener.Close()
	}
	return nil
}

// filterTCPConn filters a TCP connection, rejecting it if this function errors.
func (m *MConnTransport) filterTCPConn(tcpConn net.Conn) error {

	if m.conns.Has(tcpConn) {
		return ErrRejected{conn: tcpConn, isDuplicate: true}
	}

	host, _, err := net.SplitHostPort(tcpConn.RemoteAddr().String())
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("connection address has invalid IP address %q", host)
	}

	// Apply filter callbacks.
	chErr := make(chan error, len(m.connFilters))
	for _, connFilter := range m.connFilters {
		go func(connFilter ConnFilterFunc) {
			chErr <- connFilter(m.conns, tcpConn, []net.IP{ip})
		}(connFilter)
	}

	for i := 0; i < cap(chErr); i++ {
		select {
		case err := <-chErr:
			if err != nil {
				return ErrRejected{conn: tcpConn, err: err, isFiltered: true}
			}
		case <-time.After(m.filterTimeout):
			return ErrFilterTimeout{}
		}

	}

	// FIXME Doesn't really make sense to set this here, but we preserve the
	// behavior from the previous P2P transport implementation. This should
	// be moved to the router.
	m.conns.Set(tcpConn, []net.IP{ip})
	return nil
}

// normalizeEndpoint normalizes and validates an endpoint.
func (m *MConnTransport) normalizeEndpoint(endpoint *Endpoint) error {
	if endpoint == nil {
		return errors.New("nil endpoint")
	}
	if err := endpoint.Validate(); err != nil {
		return err
	}
	if endpoint.Protocol == "" {
		endpoint.Protocol = MConnProtocol
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

// mConnConnection implements Connection for MConnTransport. It takes a base TCP
// connection as input, and upgrades it to MConn with a handshake.
type mConnConnection struct {
	logger      log.Logger
	transport   *MConnTransport
	secretConn  *conn.SecretConnection
	mConn       *conn.MConnection
	nodeInfo    DefaultNodeInfo
	chReceive   chan mConnMessage
	chError     chan error
	chClose     chan struct{}
	chCloseOnce sync.Once
}

// newMConnConnection creates a new mConnConnection by handshaking
// with a peer.
func newMConnConnection(
	transport *MConnTransport,
	tcpConn net.Conn,
	expectPeerID ID,
) (c *mConnConnection, err error) {
	// FIXME Since the MConnection code panics, we need to recover here
	// and turn it into an error. Be careful not to alias err, so we can
	// update it from within this function. We should remove panics instead.
	defer func() {
		if r := recover(); r != nil {
			err = ErrRejected{
				conn:          tcpConn,
				err:           fmt.Errorf("recovered from panic: %v", r),
				isAuthFailure: true,
			}
		}
	}()

	err = tcpConn.SetDeadline(time.Now().Add(transport.handshakeTimeout))
	if err != nil {
		err = ErrRejected{
			conn:          tcpConn,
			err:           fmt.Errorf("secret conn failed: %v", err),
			isAuthFailure: true,
		}
		return
	}

	c = &mConnConnection{
		transport: transport,
		chReceive: make(chan mConnMessage),
		chError:   make(chan error),
		chClose:   make(chan struct{}),
	}
	c.secretConn, err = conn.MakeSecretConnection(tcpConn, transport.privKey)
	if err != nil {
		err = ErrRejected{
			conn:          tcpConn,
			err:           fmt.Errorf("secret conn failed: %v", err),
			isAuthFailure: true,
		}
		return
	}
	c.nodeInfo, err = c.handshake()
	if err != nil {
		err = ErrRejected{
			conn:          tcpConn,
			err:           fmt.Errorf("handshake failed: %v", err),
			isAuthFailure: true,
		}
		return
	}
	err = c.nodeInfo.Validate()
	if err != nil {
		err = ErrRejected{
			conn:              tcpConn,
			err:               err,
			isNodeInfoInvalid: true,
		}
		return
	}

	// FIXME All of the ID verification code below should be moved to the
	// router, or whatever ends up managing peer lifecycles.

	// For outgoing conns, ensure connection key matches dialed key.
	if expectPeerID != "" {
		peerID := PubKeyToID(c.PubKey())
		if expectPeerID != peerID {
			err = ErrRejected{
				conn: tcpConn,
				id:   peerID,
				err: fmt.Errorf(
					"conn.ID (%v) dialed ID (%v) mismatch",
					peerID,
					expectPeerID,
				),
				isAuthFailure: true,
			}
			return
		}
	}

	// Reject self.
	if transport.nodeInfo.ID() == c.nodeInfo.ID() {
		err = ErrRejected{
			addr:   *NewNetAddress(c.nodeInfo.ID(), c.secretConn.RemoteAddr()),
			conn:   tcpConn,
			id:     c.nodeInfo.ID(),
			isSelf: true,
		}
		return
	}

	err = transport.nodeInfo.CompatibleWith(c.nodeInfo)
	if err != nil {
		err = ErrRejected{
			conn:           tcpConn,
			err:            err,
			id:             c.nodeInfo.ID(),
			isIncompatible: true,
		}
		return
	}

	err = tcpConn.SetDeadline(time.Time{})
	if err != nil {
		err = ErrRejected{
			conn:          tcpConn,
			err:           fmt.Errorf("secret conn failed: %v", err),
			isAuthFailure: true,
		}
		return
	}

	// Set up the MConnection wrapper
	c.mConn = conn.NewMConnectionWithConfig(
		c.secretConn,
		transport.channelDescs,
		c.onReceive,
		c.onError,
		transport.mConnConfig,
	)
	c.logger = transport.logger.With("peer", c.RemoteEndpoint().String())
	c.mConn.SetLogger(c.logger)
	err = c.mConn.Start()
	return c, err
}

// handshake performs an MConn handshake, returning the peer's node info.
func (c *mConnConnection) handshake() (DefaultNodeInfo, error) {
	var pbNodeInfo p2pproto.DefaultNodeInfo
	chErr := make(chan error, 2)
	go func() {
		_, err := protoio.NewDelimitedWriter(c.secretConn).WriteMsg(c.transport.nodeInfo.ToProto())
		chErr <- err
	}()
	go func() {
		chErr <- protoio.NewDelimitedReader(c.secretConn, MaxNodeInfoSize()).ReadMsg(&pbNodeInfo)
	}()
	for i := 0; i < cap(chErr); i++ {
		if err := <-chErr; err != nil {
			return DefaultNodeInfo{}, err
		}
	}

	return DefaultNodeInfoFromProto(&pbNodeInfo)
}

// onReceive is a callback for MConnection received messages.
func (c *mConnConnection) onReceive(channelID byte, payload []byte) {
	select {
	case c.chReceive <- mConnMessage{channelID: channelID, payload: payload}:
	case <-c.chClose:
	}
}

// onError is a callback for MConnection errors. The error is passed to
// chError, which is only consumed by ReceiveMessage() for parity with
// the old MConnection behavior.
func (c *mConnConnection) onError(e interface{}) {
	err, ok := e.(error)
	if !ok {
		err = fmt.Errorf("%v", err)
	}
	select {
	case c.chError <- err:
	case <-c.chClose:
	}
}

// String displays connection information.
// FIXME This is here for backwards compatibility with existing code,
// it should probably just return RemoteEndpoint().String(), if anything.
func (c *mConnConnection) String() string {
	endpoint := c.RemoteEndpoint()
	return fmt.Sprintf("MConn{%v:%v}", endpoint.IP, endpoint.Port)
}

// SendMessage implements Connection.
func (c *mConnConnection) SendMessage(channelID byte, msg []byte) (bool, error) {
	// We don't check chError here, to preserve old MConnection behavior.
	select {
	case <-c.chClose:
		return false, io.EOF
	default:
		return c.mConn.Send(channelID, msg), nil
	}
}

// TrySendMessage implements Connection.
func (c *mConnConnection) TrySendMessage(channelID byte, msg []byte) (bool, error) {
	// We don't check chError here, to preserve old MConnection behavior.
	select {
	case <-c.chClose:
		return false, io.EOF
	default:
		return c.mConn.TrySend(channelID, msg), nil
	}
}

// ReceiveMessage implements Connection.
func (c *mConnConnection) ReceiveMessage() (byte, []byte, error) {
	select {
	case err := <-c.chError:
		return 0, nil, err
	case <-c.chClose:
		return 0, nil, io.EOF
	case msg := <-c.chReceive:
		return msg.channelID, msg.payload, nil
	}
}

// NodeInfo implements Connection.
func (c *mConnConnection) NodeInfo() DefaultNodeInfo {
	return c.nodeInfo
}

// PubKey implements Connection.
func (c *mConnConnection) PubKey() crypto.PubKey {
	return c.secretConn.RemotePubKey()
}

// LocalEndpoint implements Connection.
func (c *mConnConnection) LocalEndpoint() Endpoint {
	addr := c.secretConn.LocalAddr().(*net.TCPAddr)
	return Endpoint{
		Protocol: MConnProtocol,
		PeerID:   c.transport.nodeInfo.ID(),
		IP:       addr.IP,
		Port:     uint16(addr.Port),
	}
}

// RemoteEndpoint implements Connection.
func (c *mConnConnection) RemoteEndpoint() Endpoint {
	addr := c.secretConn.RemoteAddr().(*net.TCPAddr)
	return Endpoint{
		Protocol: MConnProtocol,
		PeerID:   c.nodeInfo.ID(),
		IP:       addr.IP,
		Port:     uint16(addr.Port),
	}
}

// Status implements Connection.
func (c *mConnConnection) Status() conn.ConnectionStatus {
	return c.mConn.Status()
}

// Close implements Connection.
func (c *mConnConnection) Close() error {
	return c.close(false)
}

// FlushClose implements Connection.
func (c *mConnConnection) FlushClose() error {
	return c.close(true)
}

// close closes the connection, flushing pending data if requested.
func (c *mConnConnection) close(flush bool) error {
	c.transport.conns.RemoveAddr(c.secretConn.RemoteAddr())
	var err error
	c.chCloseOnce.Do(func() {
		close(c.chClose)
		if flush {
			c.mConn.FlushStop()
		} else {
			err = c.mConn.Stop()
		}
	})
	return err
}

// mConnMessage is used to pass received MConnection messages
// through internal channels.
type mConnMessage struct {
	channelID byte
	payload   []byte
}
