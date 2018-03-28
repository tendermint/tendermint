package p2p

import (
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"

	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	tmconn "github.com/tendermint/tendermint/p2p/conn"
)

// Peer is an interface representing a peer connected on a reactor.
type Peer interface {
	cmn.Service

	ID() ID             // peer's cryptographic ID
	IsOutbound() bool   // did we dial the peer
	IsPersistent() bool // do we redial this peer when we disconnect
	NodeInfo() NodeInfo // peer's info
	Status() tmconn.ConnectionStatus

	Send(byte, interface{}) bool
	TrySend(byte, interface{}) bool

	Set(string, interface{})
	Get(string) interface{}
}

//----------------------------------------------------------

// peerConn contains the raw connection and its config.
type peerConn struct {
	outbound   bool
	persistent bool
	config     *PeerConfig
	conn       net.Conn // source connection
}

// ID only exists for SecretConnection.
// NOTE: Will panic if conn is not *SecretConnection.
func (pc peerConn) ID() ID {
	return PubKeyToID(pc.conn.(*tmconn.SecretConnection).RemotePubKey())
}

// peer implements Peer.
//
// Before using a peer, you will need to perform a handshake on connection.
type peer struct {
	cmn.BaseService

	// raw peerConn and the multiplex connection
	peerConn
	mconn *tmconn.MConnection

	// peer's node info and the channel it knows about
	// channels = nodeInfo.Channels
	// cached to avoid copying nodeInfo in hasChannel
	nodeInfo NodeInfo
	channels []byte

	// User data
	Data *cmn.CMap
}

func newPeer(pc peerConn, nodeInfo NodeInfo,
	reactorsByCh map[byte]Reactor, chDescs []*tmconn.ChannelDescriptor,
	onPeerError func(Peer, interface{})) *peer {

	p := &peer{
		peerConn: pc,
		nodeInfo: nodeInfo,
		channels: nodeInfo.Channels,
		Data:     cmn.NewCMap(),
	}
	p.mconn = createMConnection(pc.conn, p, reactorsByCh, chDescs, onPeerError, pc.config.MConfig)
	p.BaseService = *cmn.NewBaseService(nil, "Peer", p)
	return p
}

// PeerConfig is a Peer configuration.
type PeerConfig struct {
	AuthEnc bool `mapstructure:"auth_enc"` // authenticated encryption

	// times are in seconds
	HandshakeTimeout time.Duration `mapstructure:"handshake_timeout"`
	DialTimeout      time.Duration `mapstructure:"dial_timeout"`

	MConfig *tmconn.MConnConfig `mapstructure:"connection"`

	Fuzz       bool            `mapstructure:"fuzz"` // fuzz connection (for testing)
	FuzzConfig *FuzzConnConfig `mapstructure:"fuzz_config"`
}

// DefaultPeerConfig returns the default config.
func DefaultPeerConfig() *PeerConfig {
	return &PeerConfig{
		AuthEnc:          true,
		HandshakeTimeout: 20, // * time.Second,
		DialTimeout:      3,  // * time.Second,
		MConfig:          tmconn.DefaultMConnConfig(),
		Fuzz:             false,
		FuzzConfig:       DefaultFuzzConnConfig(),
	}
}

func newOutboundPeerConn(addr *NetAddress, config *PeerConfig, persistent bool, ourNodePrivKey crypto.PrivKey) (peerConn, error) {
	var pc peerConn

	conn, err := dial(addr, config)
	if err != nil {
		return pc, errors.Wrap(err, "Error creating peer")
	}

	pc, err = newPeerConn(conn, config, true, persistent, ourNodePrivKey)
	if err != nil {
		if err2 := conn.Close(); err2 != nil {
			return pc, errors.Wrap(err, err2.Error())
		}
		return pc, err
	}

	// ensure dialed ID matches connection ID
	if config.AuthEnc && addr.ID != pc.ID() {
		if err2 := conn.Close(); err2 != nil {
			return pc, errors.Wrap(err, err2.Error())
		}
		return pc, ErrSwitchAuthenticationFailure{addr, pc.ID()}
	}
	return pc, nil
}

func newInboundPeerConn(conn net.Conn, config *PeerConfig, ourNodePrivKey crypto.PrivKey) (peerConn, error) {

	// TODO: issue PoW challenge

	return newPeerConn(conn, config, false, false, ourNodePrivKey)
}

func newPeerConn(rawConn net.Conn,
	config *PeerConfig, outbound, persistent bool,
	ourNodePrivKey crypto.PrivKey) (pc peerConn, err error) {

	conn := rawConn

	// Fuzz connection
	if config.Fuzz {
		// so we have time to do peer handshakes and get set up
		conn = FuzzConnAfterFromConfig(conn, 10*time.Second, config.FuzzConfig)
	}

	if config.AuthEnc {
		// Set deadline for secret handshake
		if err := conn.SetDeadline(time.Now().Add(config.HandshakeTimeout * time.Second)); err != nil {
			return pc, errors.Wrap(err, "Error setting deadline while encrypting connection")
		}

		// Encrypt connection
		conn, err = tmconn.MakeSecretConnection(conn, ourNodePrivKey)
		if err != nil {
			return pc, errors.Wrap(err, "Error creating peer")
		}
	}

	// Only the information we already have
	return peerConn{
		config:     config,
		outbound:   outbound,
		persistent: persistent,
		conn:       conn,
	}, nil
}

//---------------------------------------------------
// Implements cmn.Service

// SetLogger implements BaseService.
func (p *peer) SetLogger(l log.Logger) {
	p.Logger = l
	p.mconn.SetLogger(l)
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
	p.mconn.Stop() // stop everything and close the conn
}

//---------------------------------------------------
// Implements Peer

// ID returns the peer's ID - the hex encoded hash of its pubkey.
func (p *peer) ID() ID {
	return p.nodeInfo.ID()
}

// IsOutbound returns true if the connection is outbound, false otherwise.
func (p *peer) IsOutbound() bool {
	return p.peerConn.outbound
}

// IsPersistent returns true if the peer is persitent, false otherwise.
func (p *peer) IsPersistent() bool {
	return p.peerConn.persistent
}

// NodeInfo returns a copy of the peer's NodeInfo.
func (p *peer) NodeInfo() NodeInfo {
	return p.nodeInfo
}

// Status returns the peer's ConnectionStatus.
func (p *peer) Status() tmconn.ConnectionStatus {
	return p.mconn.Status()
}

// Send msg to the channel identified by chID byte. Returns false if the send
// queue is full after timeout, specified by MConnection.
func (p *peer) Send(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		// see Switch#Broadcast, where we fetch the list of peers and loop over
		// them - while we're looping, one peer may be removed and stopped.
		return false
	} else if !p.hasChannel(chID) {
		return false
	}
	return p.mconn.Send(chID, msg)
}

// TrySend msg to the channel identified by chID byte. Immediately returns
// false if the send queue is full.
func (p *peer) TrySend(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		return false
	} else if !p.hasChannel(chID) {
		return false
	}
	return p.mconn.TrySend(chID, msg)
}

// Get the data for a given key.
func (p *peer) Get(key string) interface{} {
	return p.Data.Get(key)
}

// Set sets the data for the given key.
func (p *peer) Set(key string, data interface{}) {
	p.Data.Set(key, data)
}

// hasChannel returns true if the peer reported
// knowing about the given chID.
func (p *peer) hasChannel(chID byte) bool {
	for _, ch := range p.channels {
		if ch == chID {
			return true
		}
	}
	// NOTE: probably will want to remove this
	// but could be helpful while the feature is new
	p.Logger.Debug("Unknown channel for peer", "channel", chID, "channels", p.channels)
	return false
}

//---------------------------------------------------
// methods used by the Switch

// CloseConn should be called by the Switch if the peer was created but never started.
func (pc *peerConn) CloseConn() {
	pc.conn.Close() // nolint: errcheck
}

// HandshakeTimeout performs the Tendermint P2P handshake between a given node and the peer
// by exchanging their NodeInfo. It sets the received nodeInfo on the peer.
// NOTE: blocking
func (pc *peerConn) HandshakeTimeout(ourNodeInfo NodeInfo, timeout time.Duration) (peerNodeInfo NodeInfo, err error) {
	// Set deadline for handshake so we don't block forever on conn.ReadFull
	if err := pc.conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return peerNodeInfo, errors.Wrap(err, "Error setting deadline")
	}

	var trs, _ = cmn.Parallel(
		func(_ int) (val interface{}, err error, abort bool) {
			var n int
			wire.WriteBinary(&ourNodeInfo, pc.conn, &n, &err)
			return
		},
		func(_ int) (val interface{}, err error, abort bool) {
			var n int
			wire.ReadBinary(&peerNodeInfo, pc.conn, MaxNodeInfoSize(), &n, &err)
			return
		},
	)
	if err := trs.FirstError(); err != nil {
		return peerNodeInfo, errors.Wrap(err, "Error during handshake")
	}

	// Remove deadline
	if err := pc.conn.SetDeadline(time.Time{}); err != nil {
		return peerNodeInfo, errors.Wrap(err, "Error removing deadline")
	}

	return peerNodeInfo, nil
}

// Addr returns peer's remote network address.
func (p *peer) Addr() net.Addr {
	return p.peerConn.conn.RemoteAddr()
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
		return fmt.Sprintf("Peer{%v %v out}", p.mconn, p.ID())
	}

	return fmt.Sprintf("Peer{%v %v in}", p.mconn, p.ID())
}

//------------------------------------------------------------------
// helper funcs

func dial(addr *NetAddress, config *PeerConfig) (net.Conn, error) {
	conn, err := addr.DialTimeout(config.DialTimeout * time.Second)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func createMConnection(conn net.Conn, p *peer, reactorsByCh map[byte]Reactor, chDescs []*tmconn.ChannelDescriptor,
	onPeerError func(Peer, interface{}), config *tmconn.MConnConfig) *tmconn.MConnection {

	onReceive := func(chID byte, msgBytes []byte) {
		reactor := reactorsByCh[chID]
		if reactor == nil {
			// Note that its ok to panic here as it's caught in the conn._recover,
			// which does onPeerError.
			panic(cmn.Fmt("Unknown channel %X", chID))
		}
		reactor.Receive(chID, p, msgBytes)
	}

	onError := func(r interface{}) {
		onPeerError(p, r)
	}

	return tmconn.NewMConnectionWithConfig(conn, chDescs, onReceive, onError, config)
}
