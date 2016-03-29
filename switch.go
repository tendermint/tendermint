package p2p

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/log15"
)

type Reactor interface {
	Service // Start, Stop

	SetSwitch(*Switch)
	GetChannels() []*ChannelDescriptor
	AddPeer(peer *Peer)
	RemovePeer(peer *Peer, reason interface{})
	Receive(chID byte, peer *Peer, msgBytes []byte)
}

//--------------------------------------

type BaseReactor struct {
	QuitService // Provides Start, Stop, .Quit
	Switch      *Switch
}

func NewBaseReactor(log log15.Logger, name string, impl Reactor) *BaseReactor {
	return &BaseReactor{
		QuitService: *NewQuitService(log, name, impl),
		Switch:      nil,
	}
}

func (br *BaseReactor) SetSwitch(sw *Switch) {
	br.Switch = sw
}
func (_ *BaseReactor) GetChannels() []*ChannelDescriptor              { return nil }
func (_ *BaseReactor) AddPeer(peer *Peer)                             {}
func (_ *BaseReactor) RemovePeer(peer *Peer, reason interface{})      {}
func (_ *BaseReactor) Receive(chID byte, peer *Peer, msgBytes []byte) {}

//-----------------------------------------------------------------------------

/*
The `Switch` handles peer connections and exposes an API to receive incoming messages
on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
incoming messages are received on the reactor.
*/
type Switch struct {
	BaseService

	listeners    []Listener
	reactors     map[string]Reactor
	chDescs      []*ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *CMap
	nodeInfo     *NodeInfo             // our node info
	nodePrivKey  crypto.PrivKeyEd25519 // our node privkey
}

var (
	ErrSwitchDuplicatePeer      = errors.New("Duplicate peer")
	ErrSwitchMaxPeersPerIPRange = errors.New("IP range has too many peers")
)

// config keys
const (
	dialTimeoutKey      = "p2p_dial_timeout_seconds"
	handshakeTimeoutKey = "p2p_handshake_timeout_seconds"
	maxNumPeersKey      = "p2p_max_num_peers"
	authEncKey          = "p2p_authenticated_encryption"
)

func NewSwitch() *Switch {
	sw := &Switch{
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      NewCMap(),
		nodeInfo:     nil,
	}
	sw.BaseService = *NewBaseService(log, "P2P Switch", sw)
	return sw
}

// Not goroutine safe.
func (sw *Switch) AddReactor(name string, reactor Reactor) Reactor {
	// Validate the reactor.
	// No two reactors can share the same channel.
	reactorChannels := reactor.GetChannels()
	for _, chDesc := range reactorChannels {
		chID := chDesc.ID
		if sw.reactorsByCh[chID] != nil {
			PanicSanity(fmt.Sprintf("Channel %X has multiple reactors %v & %v", chID, sw.reactorsByCh[chID], reactor))
		}
		sw.chDescs = append(sw.chDescs, chDesc)
		sw.reactorsByCh[chID] = reactor
	}
	sw.reactors[name] = reactor
	reactor.SetSwitch(sw)
	return reactor
}

// Not goroutine safe.
func (sw *Switch) Reactors() map[string]Reactor {
	return sw.reactors
}

// Not goroutine safe.
func (sw *Switch) Reactor(name string) Reactor {
	return sw.reactors[name]
}

// Not goroutine safe.
func (sw *Switch) AddListener(l Listener) {
	sw.listeners = append(sw.listeners, l)
}

// Not goroutine safe.
func (sw *Switch) Listeners() []Listener {
	return sw.listeners
}

// Not goroutine safe.
func (sw *Switch) IsListening() bool {
	return len(sw.listeners) > 0
}

// Not goroutine safe.
func (sw *Switch) SetNodeInfo(nodeInfo *NodeInfo) {
	sw.nodeInfo = nodeInfo
}

// Not goroutine safe.
func (sw *Switch) NodeInfo() *NodeInfo {
	return sw.nodeInfo
}

// Not goroutine safe.
// NOTE: Overwrites sw.nodeInfo.PubKey
func (sw *Switch) SetNodePrivKey(nodePrivKey crypto.PrivKeyEd25519) {
	sw.nodePrivKey = nodePrivKey
	if sw.nodeInfo != nil {
		sw.nodeInfo.PubKey = nodePrivKey.PubKey().(crypto.PubKeyEd25519)
	}
}

// Switch.Start() starts all the reactors, peers, and listeners.
func (sw *Switch) OnStart() error {
	sw.BaseService.OnStart()
	// Start reactors
	for _, reactor := range sw.reactors {
		_, err := reactor.Start()
		if err != nil {
			return err
		}
	}
	// Start peers
	for _, peer := range sw.peers.List() {
		sw.startInitPeer(peer)
	}
	// Start listeners
	for _, listener := range sw.listeners {
		go sw.listenerRoutine(listener)
	}
	return nil
}

func (sw *Switch) OnStop() {
	sw.BaseService.OnStop()
	// Stop listeners
	for _, listener := range sw.listeners {
		listener.Stop()
	}
	sw.listeners = nil
	// Stop peers
	for _, peer := range sw.peers.List() {
		peer.Stop()
	}
	sw.peers = NewPeerSet()
	// Stop reactors
	for _, reactor := range sw.reactors {
		reactor.Stop()
	}
}

// NOTE: This performs a blocking handshake before the peer is added.
// CONTRACT: Iff error is returned, peer is nil, and conn is immediately closed.
func (sw *Switch) AddPeerWithConnection(conn net.Conn, outbound bool) (*Peer, error) {
	// Set deadline for handshake so we don't block forever on conn.ReadFull
	conn.SetDeadline(time.Now().Add(time.Duration(config.GetInt(handshakeTimeoutKey)) * time.Second))

	// First, encrypt the connection.
	var sconn net.Conn = conn
	if config.GetBool(authEncKey) {
		var err error
		sconn, err = MakeSecretConnection(conn, sw.nodePrivKey)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}
	// Then, perform node handshake
	peerNodeInfo, err := peerHandshake(sconn, sw.nodeInfo)
	if err != nil {
		sconn.Close()
		return nil, err
	}
	if config.GetBool("p2p_authenticated_encryption") {
		// Check that the professed PubKey matches the sconn's.
		if !peerNodeInfo.PubKey.Equals(sconn.(*SecretConnection).RemotePubKey()) {
			sconn.Close()
			return nil, fmt.Errorf("Ignoring connection with unmatching pubkey: %v vs %v",
				peerNodeInfo.PubKey, sconn.(*SecretConnection).RemotePubKey())
		}
	}
	// Avoid self
	if peerNodeInfo.PubKey.Equals(sw.nodeInfo.PubKey) {
		sconn.Close()
		return nil, fmt.Errorf("Ignoring connection from self")
	}
	// Check version, chain id
	if err := sw.nodeInfo.CompatibleWith(peerNodeInfo); err != nil {
		sconn.Close()
		return nil, err
	}

	peer := newPeer(sconn, peerNodeInfo, outbound, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError)

	// Add the peer to .peers
	// ignore if duplicate or if we already have too many for that IP range
	if err := sw.peers.Add(peer); err != nil {
		log.Notice("Ignoring peer", "error", err, "peer", peer)
		peer.Stop()
		return nil, err
	}

	// remove deadline and start peer
	conn.SetDeadline(time.Time{})
	if sw.IsRunning() {
		sw.startInitPeer(peer)
	}

	log.Notice("Added peer", "peer", peer)
	return peer, nil
}

func (sw *Switch) startInitPeer(peer *Peer) {
	peer.Start()               // spawn send/recv routines
	sw.addPeerToReactors(peer) // run AddPeer on each reactor
}

// Dial a list of seeds in random order
// Spawns a go routine for each dial
func (sw *Switch) DialSeeds(seeds []string) {
	// permute the list, dial them in random order.
	perm := rand.Perm(len(seeds))
	for i := 0; i < len(perm); i++ {
		go func(i int) {
			time.Sleep(time.Duration(rand.Int63n(3000)) * time.Millisecond)
			j := perm[i]
			addr := NewNetAddressString(seeds[j])

			sw.dialSeed(addr)
		}(i)
	}
}

func (sw *Switch) dialSeed(addr *NetAddress) {
	peer, err := sw.DialPeerWithAddress(addr)
	if err != nil {
		log.Error("Error dialing seed", "error", err)
		return
	} else {
		log.Notice("Connected to seed", "peer", peer)
	}
}

func (sw *Switch) DialPeerWithAddress(addr *NetAddress) (*Peer, error) {
	log.Info("Dialing address", "address", addr)
	sw.dialing.Set(addr.IP.String(), addr)
	conn, err := addr.DialTimeout(time.Duration(config.GetInt(dialTimeoutKey)) * time.Second)
	sw.dialing.Delete(addr.IP.String())
	if err != nil {
		log.Info("Failed dialing address", "address", addr, "error", err)
		return nil, err
	}
	peer, err := sw.AddPeerWithConnection(conn, true)
	if err != nil {
		log.Info("Failed adding peer", "address", addr, "conn", conn, "error", err)
		return nil, err
	}
	log.Notice("Dialed and added peer", "address", addr, "peer", peer)
	return peer, nil
}

func (sw *Switch) IsDialing(addr *NetAddress) bool {
	return sw.dialing.Has(addr.IP.String())
}

// Broadcast runs a go routine for each attempted send, which will block
// trying to send for defaultSendTimeoutSeconds. Returns a channel
// which receives success values for each attempted send (false if times out)
// NOTE: Broadcast uses goroutines, so order of broadcast may not be preserved.
func (sw *Switch) Broadcast(chID byte, msg interface{}) chan bool {
	successChan := make(chan bool, len(sw.peers.List()))
	log.Debug("Broadcast", "channel", chID, "msg", msg)
	for _, peer := range sw.peers.List() {
		go func(peer *Peer) {
			success := peer.Send(chID, msg)
			successChan <- success
		}(peer)
	}
	return successChan
}

// Returns the count of outbound/inbound and outbound-dialing peers.
func (sw *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := sw.peers.List()
	for _, peer := range peers {
		if peer.outbound {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = sw.dialing.Size()
	return
}

func (sw *Switch) Peers() IPeerSet {
	return sw.peers
}

// Disconnect from a peer due to external error.
// TODO: make record depending on reason.
func (sw *Switch) StopPeerForError(peer *Peer, reason interface{}) {
	log.Notice("Stopping peer for error", "peer", peer, "error", reason)
	sw.peers.Remove(peer)
	peer.Stop()
	sw.removePeerFromReactors(peer, reason)
}

// Disconnect from a peer gracefully.
// TODO: handle graceful disconnects.
func (sw *Switch) StopPeerGracefully(peer *Peer) {
	log.Notice("Stopping peer gracefully")
	sw.peers.Remove(peer)
	peer.Stop()
	sw.removePeerFromReactors(peer, nil)
}

func (sw *Switch) addPeerToReactors(peer *Peer) {
	for _, reactor := range sw.reactors {
		reactor.AddPeer(peer)
	}
}

func (sw *Switch) removePeerFromReactors(peer *Peer, reason interface{}) {
	for _, reactor := range sw.reactors {
		reactor.RemovePeer(peer, reason)
	}
}

func (sw *Switch) listenerRoutine(l Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}

		// ignore connection if we already have enough
		maxPeers := config.GetInt(maxNumPeersKey)
		if maxPeers <= sw.peers.Size() {
			log.Info("Ignoring inbound connection: already have enough peers", "address", inConn.RemoteAddr().String(), "numPeers", sw.peers.Size(), "max", maxPeers)
			continue
		}

		// New inbound connection!
		_, err := sw.AddPeerWithConnection(inConn, false)
		if err != nil {
			log.Notice("Ignoring inbound connection: error on AddPeerWithConnection", "address", inConn.RemoteAddr().String(), "error", err)
			continue
		}

		// NOTE: We don't yet have the listening port of the
		// remote (if they have a listener at all).
		// The peerHandshake will handle that
	}

	// cleanup
}

//-----------------------------------------------------------------------------

type SwitchEventNewPeer struct {
	Peer *Peer
}

type SwitchEventDonePeer struct {
	Peer  *Peer
	Error interface{}
}
