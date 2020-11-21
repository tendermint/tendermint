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
	"github.com/tendermint/tendermint/libs/protoio"
	tmconn "github.com/tendermint/tendermint/p2p/conn"
	p2pproto "github.com/tendermint/tendermint/proto/tendermint/p2p"
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
	nodeKey      NodeKey
	nodeInfo     DefaultNodeInfo
	channelDescs []*ChannelDescriptor
	mConnConfig  tmconn.MConnConfig

	listener net.Listener
	chAccept chan *mConnConnection
	chError  chan error
	chClose  chan struct{}

	maxIncomingConnections int
	handshakeTimeout       time.Duration
	dialTimeout            time.Duration
}

// NewMConnTransport sets up a new MConn transport.
func NewMConnTransport(
	nodeInfo NodeInfo,
	nodeKey NodeKey,
	mConnConfig tmconn.MConnConfig,
	opts ...MConnTransportOption,
) *MConnTransport {
	m := &MConnTransport{
		nodeInfo:         nodeInfo.(DefaultNodeInfo),
		nodeKey:          nodeKey,
		handshakeTimeout: defaultHandshakeTimeout,
		dialTimeout:      defaultDialTimeout,
		mConnConfig:      mConnConfig,

		chAccept: make(chan *mConnConnection),
		chError:  make(chan error),
		chClose:  make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// SetChannelDescriptors is used to set MConn channel descriptors. It exists for
// the switch to configure this separately from transport construction (which is
// done by the node). It must be called before Listen().
func (m *MConnTransport) SetChannelDescriptors(channelDescs []*ChannelDescriptor) {
	m.channelDescs = channelDescs
}

// Listen listens for inbound connections on the given endpoint.
func (m *MConnTransport) Listen(endpoint Endpoint) error {
	if m.listener != nil {
		return fmt.Errorf("MConn transport is already listening")
	}
	// FIXME Maybe check this, needs code restructuring
	/*if len(m.channelDescs) == 0 {
		return fmt.Errorf("No MConn channel descriptors")
	}*/
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

// accept accepts inbound connections in a loop, and asynchronously handshakes
// with the peer to avoid head-of-line blocking.
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
			conn, err := newMConnConnection(m, tcpConn)
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

	// FIXME Filter connections

	return newMConnConnection(m, tcpConn)
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
	close(m.chClose)
	if m.listener != nil {
		return m.listener.Close()
	}
	return nil
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

// mConnConnection implements Connection for MConnTransport. It takes a base TCP
// connection as input, and upgrades it to MConn with a handshake.
type mConnConnection struct {
	transport  *MConnTransport
	secretConn *tmconn.SecretConnection
	mConn      *tmconn.MConnection
	streams    map[byte]*mConnStream
	nodeInfo   DefaultNodeInfo
}

// newMConnConnection creates a new mConnConnection by handshaking
// with a peer.
func newMConnConnection(
	transport *MConnTransport,
	tcpConn net.Conn,
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
		streams:   make(map[byte]*mConnStream), // FIXME capacity
	}
	conn.secretConn, err = tmconn.MakeSecretConnection(tcpConn, transport.nodeKey.PrivKey)
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

	// FIXME Check node IDs

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
	//err = conn.mConn.Start()
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
	return c.secretConn.Close()
}

// mConnStream implements Stream for MConnTransport.
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
		return 0, errors.New(fmt.Sprintf("MConn send failed for stream 0x%x", s.id))
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
