package p2p

import (
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/wire"
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

// peer implements Peer.
//
// Before using a peer, you will need to perform a handshake on connection.
type peer struct {
	cmn.BaseService

	outbound bool

	conn  net.Conn            // source connection
	mconn *tmconn.MConnection // multiplex connection

	persistent bool
	config     *PeerConfig

	nodeInfo NodeInfo  // peer's node info
	channels []byte    // channels the peer knows about
	Data     *cmn.CMap // User data.
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

func newOutboundPeer(addr *NetAddress, reactorsByCh map[byte]Reactor, chDescs []*tmconn.ChannelDescriptor,
	onPeerError func(Peer, interface{}), ourNodePrivKey crypto.PrivKeyEd25519, config *PeerConfig, persistent bool) (*peer, error) {

	conn, err := dial(addr, config)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating peer")
	}

	peer, err := newPeerFromConnAndConfig(conn, true, reactorsByCh, chDescs, onPeerError, ourNodePrivKey, config)
	if err != nil {
		if err := conn.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}
	peer.persistent = persistent

	return peer, nil
}

func newInboundPeer(conn net.Conn, reactorsByCh map[byte]Reactor, chDescs []*tmconn.ChannelDescriptor,
	onPeerError func(Peer, interface{}), ourNodePrivKey crypto.PrivKeyEd25519, config *PeerConfig) (*peer, error) {

	// TODO: issue PoW challenge

	return newPeerFromConnAndConfig(conn, false, reactorsByCh, chDescs, onPeerError, ourNodePrivKey, config)
}

func newPeerFromConnAndConfig(rawConn net.Conn, outbound bool, reactorsByCh map[byte]Reactor, chDescs []*tmconn.ChannelDescriptor,
	onPeerError func(Peer, interface{}), ourNodePrivKey crypto.PrivKeyEd25519, config *PeerConfig) (*peer, error) {

	conn := rawConn

	// Fuzz connection
	if config.Fuzz {
		// so we have time to do peer handshakes and get set up
		conn = FuzzConnAfterFromConfig(conn, 10*time.Second, config.FuzzConfig)
	}

	// Encrypt connection
	if config.AuthEnc {
		if err := conn.SetDeadline(time.Now().Add(config.HandshakeTimeout * time.Second)); err != nil {
			return nil, errors.Wrap(err, "Error setting deadline while encrypting connection")
		}

		var err error
		conn, err = tmconn.MakeSecretConnection(conn, ourNodePrivKey)
		if err != nil {
			return nil, errors.Wrap(err, "Error creating peer")
		}
	}

	// NodeInfo is set after Handshake
	p := &peer{
		outbound: outbound,
		conn:     conn,
		config:   config,
		Data:     cmn.NewCMap(),
	}

	p.mconn = createMConnection(conn, p, reactorsByCh, chDescs, onPeerError, config.MConfig)

	p.BaseService = *cmn.NewBaseService(nil, "Peer", p)

	return p, nil
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
	return PubKeyToID(p.PubKey())
}

// IsOutbound returns true if the connection is outbound, false otherwise.
func (p *peer) IsOutbound() bool {
	return p.outbound
}

// IsPersistent returns true if the peer is persitent, false otherwise.
func (p *peer) IsPersistent() bool {
	return p.persistent
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
func (p *peer) CloseConn() {
	p.conn.Close() // nolint: errcheck
}

// HandshakeTimeout performs the Tendermint P2P handshake between a given node and the peer
// by exchanging their NodeInfo. It sets the received nodeInfo on the peer.
// NOTE: blocking
func (p *peer) HandshakeTimeout(ourNodeInfo NodeInfo, timeout time.Duration) error {
	// Set deadline for handshake so we don't block forever on conn.ReadFull
	if err := p.conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return errors.Wrap(err, "Error setting deadline")
	}

	var peerNodeInfo NodeInfo
	var err1 error
	var err2 error
	var n int
	cmn.Parallel(
		func() {
			var bz []byte
			bz, err1 = wire.MarshalBinary(ourNodeInfo)
			if err1 == nil {
				_, err1 = p.conn.Write(bz)
			}
		},
		func() {
			bz := make([]byte, maxNodeInfoSize)
			n, err2 = p.conn.Read(bz)
			if err2 == nil {
				err2 = wire.UnmarshalBinary(bz[:n], &peerNodeInfo) // maxNodeInfoSize
			}
		},
	)
	if err1 != nil {
		return errors.Wrap(err1, "Error during handshake/write")
	}
	if err2 != nil {
		return errors.Wrap(err2, "Error during handshake/read")
	}

	// Remove deadline
	if err := p.conn.SetDeadline(time.Time{}); err != nil {
		return errors.Wrap(err, "Error removing deadline")
	}

	p.setNodeInfo(peerNodeInfo)
	return nil
}

func (p *peer) setNodeInfo(nodeInfo NodeInfo) {
	p.nodeInfo = nodeInfo
	// cache the channels so we dont copy nodeInfo
	// every time we check hasChannel
	p.channels = nodeInfo.Channels
}

// Addr returns peer's remote network address.
func (p *peer) Addr() net.Addr {
	return p.conn.RemoteAddr()
}

// PubKey returns peer's public key.
func (p *peer) PubKey() crypto.PubKey {
	if p.nodeInfo.PubKey != nil {
		return p.nodeInfo.PubKey
	} else if p.config.AuthEnc {
		return p.conn.(*tmconn.SecretConnection).RemotePubKey()
	}
	panic("Attempt to get peer's PubKey before calling Handshake")
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
			cmn.PanicSanity(cmn.Fmt("Unknown channel %X", chID))
		}
		reactor.Receive(chID, p, msgBytes)
	}

	onError := func(r interface{}) {
		onPeerError(p, r)
	}

	return tmconn.NewMConnectionWithConfig(conn, chDescs, onReceive, onError, config)
}
