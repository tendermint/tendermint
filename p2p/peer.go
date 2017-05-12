package p2p

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pkg/errors"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
)

// Peer could be marked as persistent, in which case you can use
// Redial function to reconnect. Note that inbound peers can't be
// made persistent. They should be made persistent on the other end.
//
// Before using a peer, you will need to perform a handshake on connection.
type Peer struct {
	cmn.BaseService

	outbound bool

	conn  net.Conn     // source connection
	mconn *MConnection // multiplex connection

	persistent bool
	config     *PeerConfig

	*NodeInfo
	Key  string
	Data *cmn.CMap // User data.
}

// PeerConfig is a Peer configuration.
type PeerConfig struct {
	AuthEnc bool `mapstructure:"auth_enc"` // authenticated encryption

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
		AuthEnc:          true,
		HandshakeTimeout: 20, // * time.Second,
		DialTimeout:      3,  // * time.Second,
		MConfig:          DefaultMConnConfig(),
		Fuzz:             false,
		FuzzConfig:       DefaultFuzzConnConfig(),
	}
}

func newOutboundPeer(addr *NetAddress, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), ourNodePrivKey crypto.PrivKeyEd25519) (*Peer, error) {
	return newOutboundPeerWithConfig(addr, reactorsByCh, chDescs, onPeerError, ourNodePrivKey, DefaultPeerConfig())
}

func newOutboundPeerWithConfig(addr *NetAddress, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), ourNodePrivKey crypto.PrivKeyEd25519, config *PeerConfig) (*Peer, error) {
	conn, err := dial(addr, config)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating peer")
	}

	peer, err := newPeerFromConnAndConfig(conn, true, reactorsByCh, chDescs, onPeerError, ourNodePrivKey, config)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return peer, nil
}

func newInboundPeer(conn net.Conn, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), ourNodePrivKey crypto.PrivKeyEd25519) (*Peer, error) {
	return newInboundPeerWithConfig(conn, reactorsByCh, chDescs, onPeerError, ourNodePrivKey, DefaultPeerConfig())
}

func newInboundPeerWithConfig(conn net.Conn, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), ourNodePrivKey crypto.PrivKeyEd25519, config *PeerConfig) (*Peer, error) {
	return newPeerFromConnAndConfig(conn, false, reactorsByCh, chDescs, onPeerError, ourNodePrivKey, config)
}

func newPeerFromConnAndConfig(rawConn net.Conn, outbound bool, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), ourNodePrivKey crypto.PrivKeyEd25519, config *PeerConfig) (*Peer, error) {
	conn := rawConn

	// Fuzz connection
	if config.Fuzz {
		// so we have time to do peer handshakes and get set up
		conn = FuzzConnAfterFromConfig(conn, 10*time.Second, config.FuzzConfig)
	}

	// Encrypt connection
	if config.AuthEnc {
		conn.SetDeadline(time.Now().Add(config.HandshakeTimeout * time.Second))

		var err error
		conn, err = MakeSecretConnection(conn, ourNodePrivKey)
		if err != nil {
			return nil, errors.Wrap(err, "Error creating peer")
		}
	}

	// Key and NodeInfo are set after Handshake
	p := &Peer{
		outbound: outbound,
		conn:     conn,
		config:   config,
		Data:     cmn.NewCMap(),
	}

	p.mconn = createMConnection(conn, p, reactorsByCh, chDescs, onPeerError, config.MConfig)

	p.BaseService = *cmn.NewBaseService(nil, "Peer", p)

	return p, nil
}

// CloseConn should be used when the peer was created, but never started.
func (p *Peer) CloseConn() {
	p.conn.Close()
}

// makePersistent marks the peer as persistent.
func (p *Peer) makePersistent() {
	if !p.outbound {
		panic("inbound peers can't be made persistent")
	}

	p.persistent = true
}

// IsPersistent returns true if the peer is persitent, false otherwise.
func (p *Peer) IsPersistent() bool {
	return p.persistent
}

// HandshakeTimeout performs a handshake between a given node and the peer.
// NOTE: blocking
func (p *Peer) HandshakeTimeout(ourNodeInfo *NodeInfo, timeout time.Duration) error {
	// Set deadline for handshake so we don't block forever on conn.ReadFull
	p.conn.SetDeadline(time.Now().Add(timeout))

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

	if p.config.AuthEnc {
		// Check that the professed PubKey matches the sconn's.
		if !peerNodeInfo.PubKey.Equals(p.PubKey().Wrap()) {
			return fmt.Errorf("Ignoring connection with unmatching pubkey: %v vs %v",
				peerNodeInfo.PubKey, p.PubKey())
		}
	}

	// Remove deadline
	p.conn.SetDeadline(time.Time{})

	peerNodeInfo.RemoteAddr = p.Addr().String()

	p.NodeInfo = peerNodeInfo
	p.Key = peerNodeInfo.PubKey.KeyString()

	return nil
}

// Addr returns peer's remote network address.
func (p *Peer) Addr() net.Addr {
	return p.conn.RemoteAddr()
}

// PubKey returns peer's public key.
func (p *Peer) PubKey() crypto.PubKeyEd25519 {
	if p.config.AuthEnc {
		return p.conn.(*SecretConnection).RemotePubKey()
	}
	if p.NodeInfo == nil {
		panic("Attempt to get peer's PubKey before calling Handshake")
	}
	return p.PubKey()
}

// OnStart implements BaseService.
func (p *Peer) OnStart() error {
	p.BaseService.OnStart()
	_, err := p.mconn.Start()
	return err
}

// OnStop implements BaseService.
func (p *Peer) OnStop() {
	p.BaseService.OnStop()
	p.mconn.Stop()
}

// Connection returns underlying MConnection.
func (p *Peer) Connection() *MConnection {
	return p.mconn
}

// IsOutbound returns true if the connection is outbound, false otherwise.
func (p *Peer) IsOutbound() bool {
	return p.outbound
}

// Send msg to the channel identified by chID byte. Returns false if the send
// queue is full after timeout, specified by MConnection.
func (p *Peer) Send(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		// see Switch#Broadcast, where we fetch the list of peers and loop over
		// them - while we're looping, one peer may be removed and stopped.
		return false
	}
	return p.mconn.Send(chID, msg)
}

// TrySend msg to the channel identified by chID byte. Immediately returns
// false if the send queue is full.
func (p *Peer) TrySend(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.TrySend(chID, msg)
}

// CanSend returns true if the send queue is not full, false otherwise.
func (p *Peer) CanSend(chID byte) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.CanSend(chID)
}

// WriteTo writes the peer's public key to w.
func (p *Peer) WriteTo(w io.Writer) (n int64, err error) {
	var n_ int
	wire.WriteString(p.Key, w, &n_, &err)
	n += int64(n_)
	return
}

// String representation.
func (p *Peer) String() string {
	if p.outbound {
		return fmt.Sprintf("Peer{%v %v out}", p.mconn, p.Key[:12])
	}

	return fmt.Sprintf("Peer{%v %v in}", p.mconn, p.Key[:12])
}

// Equals reports whenever 2 peers are actually represent the same node.
func (p *Peer) Equals(other *Peer) bool {
	return p.Key == other.Key
}

// Get the data for a given key.
func (p *Peer) Get(key string) interface{} {
	return p.Data.Get(key)
}

func dial(addr *NetAddress, config *PeerConfig) (net.Conn, error) {
	conn, err := addr.DialTimeout(config.DialTimeout * time.Second)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func createMConnection(conn net.Conn, p *Peer, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), config *MConnConfig) *MConnection {
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
