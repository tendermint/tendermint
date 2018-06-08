package p2p

import (
	"fmt"
	"net"
	"time"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p/conn"
)

type accept struct {
	conn net.Conn
	err  error
}

// TODO(xla): Remove once MConnection is refactored into mPer.
type peerConfig struct {
	chDescs      []*conn.ChannelDescriptor
	mConfig      conn.MConnConfig
	nodeInfo     NodeInfo
	nodeKey      NodeKey
	onPeerError  func(Peer, interface{})
	outbound     bool
	p2pConfig    config.P2PConfig
	persistent   bool
	reactorsByCh map[byte]Reactor
}

// PeerTransport proxies incoming and outgoing peer connections.
type PeerTransport interface {
	fmt.Stringer

	// Accept returns a newly connected Peer.
	Accept(peerConfig) (Peer, error)

	// Dial connects to a Peer.
	Dial(NetAddress, peerConfig) (Peer, error)

	ExternalAddress() NetAddress

	// lifecycle methods
	Close() error
	Listen(NetAddress) error
}

// MultiplexTransportOption sets an optional parameter on multiplexTransport.
type MultiplexTransportOption func(*multiplexTransport)

func MultiplexTransportMConfig(cfg conn.MConnConfig) MultiplexTransportOption {
	return func(mt *multiplexTransport) { mt.mConfig = cfg }
}

func MultiplexTransportNodeInfo(ni NodeInfo) MultiplexTransportOption {
	return func(mt *multiplexTransport) { mt.nodeInfo = ni }
}

// multiplexTransport accepts tcp connections and upgrades to multiplexted
// peers.
type multiplexTransport struct {
	listener net.Listener

	acceptc chan accept
	closec  <-chan struct{}
	listenc <-chan struct{}

	addr             NetAddress
	dialTimeout      time.Duration
	handshakeTimeout time.Duration
	nodeInfo         NodeInfo
	nodeKey          NodeKey

	// TODO(xla): Remove when MConnection is refactored into mPeer.
	mConfig conn.MConnConfig
}

var _ PeerTransport = (*multiplexTransport)(nil)

// NewMTransport returns network connected multiplexed peers.
func NewMTransport(
	addr NetAddress,
	nodeInfo NodeInfo,
	nodeKey NodeKey,
) *multiplexTransport {
	// TODO(xla): Move over proper external address discvoery.
	return &multiplexTransport{
		addr:     addr,
		acceptc:  make(chan accept),
		closec:   make(chan struct{}),
		mConfig:  conn.DefaultMConnConfig(),
		nodeInfo: nodeInfo,
		nodeKey:  nodeKey,
	}
}

func (mt *multiplexTransport) Accept(cfg peerConfig) (Peer, error) {
	a := <-mt.acceptc
	if a.err != nil {
		return nil, a.err
	}

	cfg.mConfig = mt.mConfig
	cfg.nodeInfo = mt.nodeInfo
	cfg.nodeKey = mt.nodeKey

	return upgrade(a.conn, mt.handshakeTimeout, cfg)
}

func (mt *multiplexTransport) Dial(
	addr NetAddress,
	cfg peerConfig,
) (Peer, error) {
	c, err := addr.DialTimeout(mt.dialTimeout)
	if err != nil {
		return nil, err
	}

	cfg.mConfig = mt.mConfig
	cfg.nodeInfo = mt.nodeInfo
	cfg.nodeKey = mt.nodeKey
	cfg.outbound = true

	return upgrade(c, mt.handshakeTimeout, cfg)
}

func (mt *multiplexTransport) ExternalAddress() NetAddress {
	return mt.addr
}

func (mt *multiplexTransport) Close() error {
	return fmt.Errorf("not implemented")
}

func (mt *multiplexTransport) Listen(addr NetAddress) error {
	ln, err := net.Listen("tcp", addr.DialString())
	if err != nil {
		return err
	}

	mt.listener = ln

	go mt.acceptPeers()

	return nil
}

func (mt *multiplexTransport) String() string {
	addr := mt.ExternalAddress()
	a := &addr
	return fmt.Sprintf("PeerTransport<%v>", a.String())
}

func (mt *multiplexTransport) acceptPeers() {
	for {
		c, err := mt.listener.Accept()
		if err != nil {
			// Close has been called so we silently exit.
			if _, ok := <-mt.closec; !ok {
				return
			}

			mt.acceptc <- accept{nil, err}
			return
		}

		mt.acceptc <- accept{c, err}
	}
}

func handshake(
	c net.Conn,
	timeout time.Duration,
	nodeInfo NodeInfo,
) (NodeInfo, error) {
	if err := c.SetDeadline(time.Now().Add(timeout)); err != nil {
		return NodeInfo{}, err
	}

	var (
		errc         = make(chan error, 2)
		peerNodeInfo NodeInfo
	)
	go func(errc chan<- error, c net.Conn) {
		_, err := cdc.MarshalBinaryWriter(c, nodeInfo)
		errc <- err
	}(errc, c)
	go func(errc chan<- error, c net.Conn) {
		_, err := cdc.UnmarshalBinaryReader(
			c,
			&peerNodeInfo,
			int64(MaxNodeInfoSize()),
		)
		errc <- err
	}(errc, c)

	for i := 0; i < cap(errc); i++ {
		err := <-errc
		if err != nil {
			return NodeInfo{}, err
		}
	}

	return peerNodeInfo, c.SetDeadline(time.Time{})
}

func secretConn(c net.Conn, timeout time.Duration, privKey crypto.PrivKey) (net.Conn, error) {
	if err := c.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	c, err := conn.MakeSecretConnection(c, privKey)
	if err != nil {
		return nil, err
	}

	return c, c.SetDeadline(time.Time{})
}

func upgrade(c net.Conn, timeout time.Duration, cfg peerConfig) (*peer, error) {
	c, err := secretConn(c, timeout, cfg.nodeKey.PrivKey)
	if err != nil {
		return nil, err
	}

	ni, err := handshake(c, timeout, cfg.nodeInfo)
	if err != nil {
		return nil, err
	}

	pc := peerConn{
		conn:       c,
		config:     &cfg.p2pConfig,
		outbound:   cfg.outbound,
		persistent: cfg.persistent,
	}

	return newPeer(
		pc,
		cfg.mConfig,
		ni,
		cfg.reactorsByCh,
		cfg.chDescs,
		cfg.onPeerError,
	), nil
}
