package p2p

import (
	"fmt"
	"io"
	"net"
	"time"

	cmn "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
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

	authEnc    bool // authenticated encryption
	persistent bool
	config     cfg.Config

	*NodeInfo
	Key  string
	Data *cmn.CMap // User data.
}

func newPeer(addr *NetAddress, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), config cfg.Config, privKey crypto.PrivKeyEd25519) (*Peer, error) {
	conn, err := dial(addr, config)
	if err != nil {
		return nil, err
	}

	// outbound = true
	return newPeerFromExistingConn(conn, true, reactorsByCh, chDescs, onPeerError, config, privKey)
}

func newPeerFromExistingConn(conn net.Conn, outbound bool, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), config cfg.Config, privKey crypto.PrivKeyEd25519) (*Peer, error) {
	// Encrypt connection
	if config.GetBool(configKeyAuthEnc) {
		var err error
		conn, err = MakeSecretConnection(conn, privKey)
		if err != nil {
			return nil, err
		}
	}

	p := &Peer{
		outbound: outbound,
		authEnc:  config.GetBool(configKeyAuthEnc),
		conn:     conn,
		config:   config,
		Data:     cmn.NewCMap(),
	}

	p.mconn = createMConnection(conn, p, reactorsByCh, chDescs, onPeerError, config)

	p.BaseService = *cmn.NewBaseService(log, "Peer", p)

	return p, nil
}

// CloseConn should be used when the peer was created, but never started.
func (p *Peer) CloseConn() {
	p.conn.Close()
}

// MakePersistent marks the peer as persistent.
func (p *Peer) MakePersistent() {
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
			log.Notice("Peer handshake", "peerNodeInfo", peerNodeInfo)
		})
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}

	if p.authEnc {
		// Check that the professed PubKey matches the sconn's.
		if !peerNodeInfo.PubKey.Equals(p.PubKey()) {
			return fmt.Errorf("Ignoring connection with unmatching pubkey: %v vs %v",
				peerNodeInfo.PubKey, p.PubKey())
		}
	}

	// Remove deadline
	p.conn.SetDeadline(time.Time{})

	peerNodeInfo.RemoteAddr = p.RemoteAddr().String()

	p.NodeInfo = peerNodeInfo
	p.Key = peerNodeInfo.PubKey.KeyString()

	return nil
}

// RemoteAddr returns the remote network address.
func (p *Peer) RemoteAddr() net.Addr {
	return p.conn.RemoteAddr()
}

// PubKey returns the remote public key.
func (p *Peer) PubKey() crypto.PubKeyEd25519 {
	if p.authEnc {
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

func dial(addr *NetAddress, config cfg.Config) (net.Conn, error) {
	log.Info("Dialing address", "address", addr)
	conn, err := addr.DialTimeout(time.Duration(
		config.GetInt(configKeyDialTimeoutSeconds)) * time.Second)
	if err != nil {
		log.Info("Failed dialing address", "address", addr, "error", err)
		return nil, err
	}
	if config.GetBool(configFuzzEnable) {
		conn = FuzzConn(config, conn)
	}
	return conn, nil
}

func createMConnection(conn net.Conn, p *Peer, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{}), config cfg.Config) *MConnection {
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

	mconnConfig := &MConnectionConfig{
		SendRate: int64(config.GetInt(configKeySendRate)),
		RecvRate: int64(config.GetInt(configKeyRecvRate)),
	}

	return NewMConnectionWithConfig(conn, chDescs, onReceive, onError, mconnConfig)
}
