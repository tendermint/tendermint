package p2p

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	crypto "github.com/libp2p/go-libp2p-crypto"
	inet "github.com/libp2p/go-libp2p-net"
	lpeer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

// Peer is an interface representing a peer connected on a reactor.
type Peer interface {
	cmn.Service

	Key() string
	PeerID() lpeer.ID
	IsOutbound() bool
	IsPersistent() bool
	NodeInfo() *NodeInfo
	Status() ConnectionStatus

	Send(byte, interface{}) bool
	TrySend(byte, interface{}) bool

	Set(string, interface{})
	Get(string) interface{}

	RemoteMultiaddr() ma.Multiaddr
	LocalMultiaddr() ma.Multiaddr
}

// Peer could be marked as persistent, in which case you can use
// Redial function to reconnect. Note that inbound peers can't be
// made persistent. They should be made persistent on the other end.
//
// Before using a peer, you will need to perform a handshake on connection.
type peer struct {
	cmn.BaseService

	outbound bool

	conn  inet.Stream  // source connection
	mconn *MConnection // multiplex connection

	persistent bool
	config     *PeerConfig

	nodeInfo *NodeInfo
	key      string
	peerId   lpeer.ID
	pubKey   crypto.PubKey
	Data     *cmn.CMap // User data.
}

// PeerConfig is a Peer configuration.
type PeerConfig struct {
	// times are in seconds
	HandshakeTimeout time.Duration `mapstructure:"handshake_timeout"`
	DialTimeout      time.Duration `mapstructure:"dial_timeout"`

	MConfig *MConnConfig `mapstructure:"connection"`

	Fuzz       bool            `mapstructure:"fuzz"` // fuzz connection (for testing)
	FuzzConfig *FuzzConnConfig `mapstructure:"fuzz_config"`
}

// DefaultPeerConfig returns the default config.
func DefaultPeerConfig() *PeerConfig {
	return &PeerConfig{
		HandshakeTimeout: 20, // * time.Second,
		DialTimeout:      3,  // * time.Second,
		MConfig:          DefaultMConnConfig(),
		Fuzz:             false,
		FuzzConfig:       DefaultFuzzConnConfig(),
	}
}

func newOutboundPeer(conn inet.Stream, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor,
	onPeerError func(Peer, interface{}), ourNodePrivKey crypto.PrivKey, config *PeerConfig) (*peer, error) {

	peer, err := newPeerFromConnAndConfig(conn, true, reactorsByCh, chDescs, onPeerError, ourNodePrivKey, config)
	if err != nil {
		if err := conn.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}
	return peer, nil
}

func newInboundPeer(conn inet.Stream, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor,
	onPeerError func(Peer, interface{}), ourNodePrivKey crypto.PrivKey, config *PeerConfig) (*peer, error) {

	return newPeerFromConnAndConfig(conn, false, reactorsByCh, chDescs, onPeerError, ourNodePrivKey, config)
}

func newPeerFromConnAndConfig(rawConn inet.Stream, outbound bool, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor,
	onPeerError func(Peer, interface{}), ourNodePrivKey crypto.PrivKey, config *PeerConfig) (*peer, error) {
	conn := rawConn

	// Fuzz connection
	if config.Fuzz {
		// so we have time to do peer handshakes and get set up
		conn = FuzzConnAfterFromConfig(conn, 10*time.Second, config.FuzzConfig)
	}

	// Key and NodeInfo are set after Handshake
	p := &peer{
		outbound: outbound,
		conn:     conn,
		config:   config,
		pubKey:   rawConn.Conn().RemotePublicKey(),
		peerId:   rawConn.Conn().RemotePeer(),
		Data:     cmn.NewCMap(),
	}

	p.mconn = createMConnection(conn, p, reactorsByCh, chDescs, onPeerError, config.MConfig)

	p.BaseService = *cmn.NewBaseService(nil, "Peer", p)

	return p, nil
}

func (p *peer) SetLogger(l log.Logger) {
	p.Logger = l
	p.mconn.SetLogger(l)
}

func (p *peer) RemoteMultiaddr() ma.Multiaddr {
	return p.conn.Conn().RemoteMultiaddr()
}

func (p *peer) LocalMultiaddr() ma.Multiaddr {
	return p.conn.Conn().RemoteMultiaddr()
}

// CloseConn should be used when the peer was created, but never started.
func (p *peer) CloseConn() {
	p.conn.Close() // nolint: errcheck
}

// makePersistent marks the peer as persistent.
func (p *peer) makePersistent() {
	if !p.outbound {
		panic("inbound peers can't be made persistent")
	}

	p.persistent = true
}

// IsPersistent returns true if the peer is persitent, false otherwise.
func (p *peer) IsPersistent() bool {
	return p.persistent
}

// HandshakeTimeout performs a handshake between a given node and the peer.
// NOTE: blocking
func (p *peer) HandshakeTimeout(ourNodeInfo *NodeInfo, timeout time.Duration) error {
	// Set deadline for handshake so we don't block forever on conn.ReadFull
	if err := p.conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return errors.Wrap(err, "Error setting deadline")
	}

	var peerNodeInfo = new(NodeInfo)
	var err1 error
	var err2 error
	cmn.Parallel(
		func() {
			var n int
			wire.WriteBinary(ourNodeInfo, p.conn, &n, &err1)
		},
		func() {
			var n int
			wire.ReadBinary(peerNodeInfo, p.conn, maxNodeInfoSize, &n, &err2)
			p.Logger.Info("Peer handshake", "peerNodeInfo", peerNodeInfo)
		})
	if err1 != nil {
		return errors.Wrap(err1, "Error during handshake/write")
	}
	if err2 != nil {
		return errors.Wrap(err2, "Error during handshake/read")
	}

	// Check that the professed PubKey matches the sconn's.
	peerPubKey, err := peerNodeInfo.ParsePublicKey()
	if err != nil {
		return errors.Wrap(err, "Error parsing peer public key")
	}

	peerId, err := lpeer.IDFromPublicKey(peerPubKey)
	if err != nil {
		return errors.Wrap(err, "Error generating peer id from public key")
	}
	peerIdPretty := peerId.Pretty()

	peerExpectedIdPretty := p.peerId.Pretty()
	if peerIdPretty != peerExpectedIdPretty {
		return fmt.Errorf("Ignoring connection with unmatching peer id: %v vs %v",
			peerIdPretty, peerExpectedIdPretty)
	}

	if !peerPubKey.Equals(p.PubKey()) {
		return fmt.Errorf("Ignoring connection with unmatching pubkey: %v vs %v",
			peerId.Pretty(), peerExpectedIdPretty)
	}

	// Remove deadline
	if err := p.conn.SetDeadline(time.Time{}); err != nil {
		return errors.Wrap(err, "Error removing deadline")
	}

	peerNodeInfo.RemoteAddr = p.Addr().String()

	p.nodeInfo = peerNodeInfo
	p.key = peerId.Pretty()
	p.peerId = peerId

	return nil
}

// Addr returns peer's multiaddr.
func (p *peer) Addr() ma.Multiaddr {
	return p.conn.Conn().LocalMultiaddr()
}

// PubKey returns peer's public key.
func (p *peer) PubKey() crypto.PubKey {
	return p.pubKey
}

// OnStart implements BaseService.
func (p *peer) OnStart() error {
	if err := p.BaseService.OnStart(); err != nil {
		return err
	}
	err := p.mconn.Start()
	return err
}

// OnStop implements BaseService.
func (p *peer) OnStop() {
	p.BaseService.OnStop()
	p.mconn.Stop()
}

// Connection returns underlying MConnection.
func (p *peer) Connection() *MConnection {
	return p.mconn
}

// IsOutbound returns true if the connection is outbound, false otherwise.
func (p *peer) IsOutbound() bool {
	return p.outbound
}

// Send msg to the channel identified by chID byte. Returns false if the send
// queue is full after timeout, specified by MConnection.
func (p *peer) Send(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		// see Switch#Broadcast, where we fetch the list of peers and loop over
		// them - while we're looping, one peer may be removed and stopped.
		return false
	}
	return p.mconn.Send(chID, msg)
}

// TrySend msg to the channel identified by chID byte. Immediately returns
// false if the send queue is full.
func (p *peer) TrySend(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.TrySend(chID, msg)
}

// CanSend returns true if the send queue is not full, false otherwise.
func (p *peer) CanSend(chID byte) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.CanSend(chID)
}

// String representation.
func (p *peer) String() string {
	if p.outbound {
		return fmt.Sprintf("Peer{%v %v out}", p.mconn, p.Key())
	}

	return fmt.Sprintf("Peer{%v %v in}", p.mconn, p.Key())
}

// Equals reports whenever 2 peers are actually represent the same node.
func (p *peer) Equals(other Peer) bool {
	return p.Key() == other.Key()
}

// Get the data for a given key.
func (p *peer) Get(key string) interface{} {
	return p.Data.Get(key)
}

// Set sets the data for the given key.
func (p *peer) Set(key string, data interface{}) {
	p.Data.Set(key, data)
}

// Key returns the peer's id key.
func (p *peer) Key() string {
	return p.nodeInfo.ListenAddr // XXX: should probably be PubKey.KeyString()
}

// PeerID returns the peer's ID.
func (p *peer) PeerID() lpeer.ID {
	return p.peerId
}

// NodeInfo returns a copy of the peer's NodeInfo.
func (p *peer) NodeInfo() *NodeInfo {
	if p.nodeInfo == nil {
		return nil
	}
	n := *p.nodeInfo // copy
	return &n
}

// Status returns the peer's ConnectionStatus.
func (p *peer) Status() ConnectionStatus {
	return p.mconn.Status()
}

func createMConnection(conn inet.Stream, p *peer, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor,
	onPeerError func(Peer, interface{}), config *MConnConfig) *MConnection {

	onReceive := func(chID byte, msgBytes []byte) {
		reactor := reactorsByCh[chID]
		if reactor == nil {
			cmn.PanicSanity(cmn.Fmt("Unknown channel %X", chID))
		}
		reactor.Receive(chID, p, msgBytes)
	}

	onError := func(r interface{}) {
		onPeerError(p, r)
	}

	return NewMConnectionWithConfig(conn, chDescs, onReceive, onError, config)
}
