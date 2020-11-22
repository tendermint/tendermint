package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/protoio"
	tmconn "github.com/tendermint/tendermint/p2p/conn"
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

// MConnTransportOption sets an option for mConnTransport.
type MConnTransportOption func(*mConnTransport)

// MConnTransportMaxIncomingConnections sets the maximum number of
// simultaneous incoming connections. Default: 0 (unlimited)
func MConnTransportMaxIncomingConnections(max int) MConnTransportOption {
	return func(mt *mConnTransport) { mt.maxIncomingConnections = max }
}

// MConnTransportFilterTimeout sets the timeout for filter callbacks.
func MConnTransportFilterTimeout(timeout time.Duration) MConnTransportOption {
	return func(mt *mConnTransport) { mt.filterTimeout = timeout }
}

// MConnTransportConnFilters sets connection filters.
func MConnTransportConnFilters(filters ...ConnFilterFunc) MConnTransportOption {
	return func(mt *mConnTransport) { mt.connFilters = filters }
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

// mConnTransport is a Transport implementation using the current multiplexed
// Tendermint protocol ("MConn").
type mConnTransport struct {
	privKey      crypto.PrivKey
	nodeInfo     DefaultNodeInfo
	channelDescs []*ChannelDescriptor
	mConnConfig  tmconn.MConnConfig

	maxIncomingConnections int
	dialTimeout            time.Duration
	handshakeTimeout       time.Duration
	filterTimeout          time.Duration

	logger   log.Logger
	listener net.Listener
	chAccept chan *mConnConnection
	chError  chan error
	chClose  chan struct{}

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
	mConnConfig tmconn.MConnConfig,
	opts ...MConnTransportOption,
) Transport {
	m := &mConnTransport{
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

// Listen asynchronously listens for inbound connections on the given endpoint.
// It must be called exactly once before calling Accept(), and the caller must
// call Close() to shut down the listener.
func (m *mConnTransport) Listen(endpoint Endpoint) error {
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
func (m *mConnTransport) accept() {
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
func (m *mConnTransport) Accept(ctx context.Context) (Connection, error) {
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
func (m *mConnTransport) Dial(ctx context.Context, endpoint Endpoint) (Connection, error) {
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

	return newMConnConnection(m, tcpConn, endpoint.PeerID)
}

// Endpoints implements Transport.
func (m *mConnTransport) Endpoints() []Endpoint {
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
func (m *mConnTransport) Close() error {
	if _, ok := <-m.chClose; !ok {
		close(m.chClose)
	}
	if m.listener != nil {
		return m.listener.Close()
	}
	return nil
}

// filterTCPConn filters a TCP connection, rejecting it if this function errors.
func (m *mConnTransport) filterTCPConn(tcpConn net.Conn) error {

	// Reject if connection is already present.
	if m.conns.Has(tcpConn) {
		return ErrRejected{conn: tcpConn, isDuplicate: true}
	}

	// Resolve IPs for incoming connections.
	host, _, err := net.SplitHostPort(tcpConn.RemoteAddr().String())
	if err != nil {
		return err
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return err
	}
	ips := []net.IP{}
	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}

	// Apply filter callbacks.
	chErr := make(chan error, len(m.connFilters))
	for _, connFilter := range m.connFilters {
		go func(connFilter ConnFilterFunc) {
			chErr <- connFilter(m.conns, tcpConn, ips)
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

	// FIXME Doesn't really make sense to set this here, but this is where
	// we have access to ips. Clean this up when we move connection tracking
	// to the router.
	m.conns.Set(tcpConn, ips)
	return nil
}

// normalizeEndpoint normalizes and validates an endpoint.
func (m *mConnTransport) normalizeEndpoint(endpoint *Endpoint) error {
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

// mConnConnection implements Connection for mConnTransport. It takes a base TCP
// connection as input, and upgrades it to MConn with a handshake.
type mConnConnection struct {
	transport  *mConnTransport
	secretConn *tmconn.SecretConnection
	mConn      *tmconn.MConnection
	streams    map[byte]*mConnStream
	nodeInfo   DefaultNodeInfo
}

// newMConnConnection creates a new mConnConnection by handshaking
// with a peer.
func newMConnConnection(
	transport *mConnTransport,
	tcpConn net.Conn,
	expectPeerID ID,
) (conn *mConnConnection, err error) {
	// FIXME Since the MConnection code panics, we need to recover here
	// and turn it into an error. Be careful not to alias err, so we can
	// update it from within this function.
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

	conn = &mConnConnection{
		transport: transport,
		streams:   make(map[byte]*mConnStream, len(transport.channelDescs)),
	}
	conn.secretConn, err = tmconn.MakeSecretConnection(tcpConn, transport.privKey)
	if err != nil {
		err = ErrRejected{
			conn:          tcpConn,
			err:           fmt.Errorf("secret conn failed: %v", err),
			isAuthFailure: true,
		}
		return
	}
	conn.nodeInfo, err = conn.handshake()
	if err != nil {
		err = ErrRejected{
			conn:          tcpConn,
			err:           fmt.Errorf("handshake failed: %v", err),
			isAuthFailure: true,
		}
		return
	}
	err = conn.nodeInfo.Validate()
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
		peerID := PubKeyToID(conn.PubKey())
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
	if transport.nodeInfo.ID() == conn.nodeInfo.ID() {
		err = ErrRejected{
			addr:   *NewNetAddress(conn.nodeInfo.ID(), conn.secretConn.RemoteAddr()),
			conn:   tcpConn,
			id:     conn.nodeInfo.ID(),
			isSelf: true,
		}
		return
	}

	err = transport.nodeInfo.CompatibleWith(conn.nodeInfo)
	if err != nil {
		err = ErrRejected{
			conn:           tcpConn,
			err:            err,
			id:             conn.nodeInfo.ID(),
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

	for _, chDesc := range transport.channelDescs {
		conn.streams[chDesc.ID], err = newMConnStream(conn, uint16(chDesc.ID))
		if err != nil {
			err = ErrRejected{
				conn:          tcpConn,
				err:           fmt.Errorf("mconn failed: %v", err),
				isAuthFailure: true,
			}
			return
		}
	}
	conn.mConn = tmconn.NewMConnectionWithConfig(
		conn.secretConn,
		transport.channelDescs,
		func(chID byte, bz []byte) {
			fmt.Printf("recv bytes on channel 0x%x\n", chID)
			if stream, ok := conn.streams[chID]; ok {
				bzCopy := make([]byte, len(bz))
				copy(bzCopy, bz)
				stream.chReceive <- bzCopy
				fmt.Printf("queued bytes for channel 0x%x\n", chID)
			} else {
				fmt.Printf("unknown stream 0x%x\n", chID)
			}
		},
		func(err interface{}) {
			fmt.Printf("connection error: %v\n", err)
			_ = conn.Close()
		},
		transport.mConnConfig,
	)
	conn.mConn.SetLogger(transport.logger.With("peer", conn.RemoteEndpoint().String()))
	err = conn.mConn.Start()

	return
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

// Stream implements Connection.
func (c *mConnConnection) Stream(id uint16) (Stream, error) {
	// FIXME Check byte
	return c.streams[byte(id)], nil
}

// Close implements Connection.
func (c *mConnConnection) Close() error {
	c.transport.conns.RemoveAddr(c.secretConn.RemoteAddr())
	err := c.mConn.Stop()
	if e := c.secretConn.Close(); e != nil && err == nil {
		err = e
	}
	return err
}

// mConnStream implements Stream for mConnTransport.
type mConnStream struct {
	connection *mConnConnection
	id         byte
	chReceive  chan []byte
}

// newMConnStream represents a stream in a connection. It corresponds
// to a MConnection Channel.
func newMConnStream(connection *mConnConnection, id uint16) (*mConnStream, error) {
	if id > math.MaxUint8 {
		return nil, fmt.Errorf("MConn only supports channel IDs up to 255")
	}
	return &mConnStream{
		connection: connection,
		id:         byte(id),
		chReceive:  make(chan []byte),
	}, nil
}

// Close implements Stream.
func (s *mConnStream) Close() error {
	return nil
}

// Write implements Stream.
func (s *mConnStream) Write(bz []byte) (int, error) {
	if !s.connection.mConn.Send(s.id, bz) {
		return 0, fmt.Errorf("MConn send failed for stream 0x%x", s.id)
	}
	return len(bz), nil
}

// Read implements Stream.
func (s *mConnStream) Read(target []byte) (int, error) {
	select {
	case bz := <-s.chReceive:
		copy(target, bz)
		return len(bz), nil
	case <-s.connection.transport.chClose:
		return 0, io.EOF
	}
}
