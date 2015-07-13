package p2p

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

type Reactor interface {
	Start(sw *Switch)
	Stop()
	GetChannels() []*ChannelDescriptor
	AddPeer(peer *Peer)
	RemovePeer(peer *Peer, reason interface{})
	Receive(chId byte, peer *Peer, msgBytes []byte)
}

//--------------------------------------

type BaseReactor struct{}

func (_ BaseReactor) Start(sw *Switch)                               {}
func (_ BaseReactor) Stop()                                          {}
func (_ BaseReactor) GetChannels() []*ChannelDescriptor              { return nil }
func (_ BaseReactor) AddPeer(peer *Peer)                             {}
func (_ BaseReactor) RemovePeer(peer *Peer, reason interface{})      {}
func (_ BaseReactor) Receive(chId byte, peer *Peer, msgBytes []byte) {}

//-----------------------------------------------------------------------------

/*
The `Switch` handles peer connections and exposes an API to receive incoming messages
on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
incoming messages are received on the reactor.
*/
type Switch struct {
	listeners    []Listener
	reactors     map[string]Reactor
	chDescs      []*ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *CMap
	running      uint32
	nodeInfo     *types.NodeInfo // our node info
}

var (
	ErrSwitchDuplicatePeer      = errors.New("Duplicate peer")
	ErrSwitchMaxPeersPerIPRange = errors.New("IP range has too many peers")
)

const (
	peerDialTimeoutSeconds = 3
)

func NewSwitch() *Switch {
	sw := &Switch{
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      NewCMap(),
		running:      0,
		nodeInfo:     nil,
	}
	return sw
}

// Not goroutine safe.
func (sw *Switch) AddReactor(name string, reactor Reactor) Reactor {
	// Validate the reactor.
	// No two reactors can share the same channel.
	reactorChannels := reactor.GetChannels()
	for _, chDesc := range reactorChannels {
		chId := chDesc.Id
		if sw.reactorsByCh[chId] != nil {
			panic(fmt.Sprintf("Channel %X has multiple reactors %v & %v", chId, sw.reactorsByCh[chId], reactor))
		}
		sw.chDescs = append(sw.chDescs, chDesc)
		sw.reactorsByCh[chId] = reactor
	}
	sw.reactors[name] = reactor
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
func (sw *Switch) SetNodeInfo(nodeInfo *types.NodeInfo) {
	sw.nodeInfo = nodeInfo
}

// Not goroutine safe.
func (sw *Switch) NodeInfo() *types.NodeInfo {
	return sw.nodeInfo
}

func (sw *Switch) Start() {
	if atomic.CompareAndSwapUint32(&sw.running, 0, 1) {
		// Start reactors
		for _, reactor := range sw.reactors {
			reactor.Start(sw)
		}
		// Start peers
		for _, peer := range sw.peers.List() {
			sw.startInitPeer(peer)
		}
		// Start listeners
		for _, listener := range sw.listeners {
			go sw.listenerRoutine(listener)
		}
	}
}

func (sw *Switch) Stop() {
	if atomic.CompareAndSwapUint32(&sw.running, 1, 0) {
		// Stop listeners
		for _, listener := range sw.listeners {
			listener.Stop()
		}
		sw.listeners = nil
		// Stop peers
		for _, peer := range sw.peers.List() {
			peer.stop()
		}
		sw.peers = NewPeerSet()
		// Stop reactors
		for _, reactor := range sw.reactors {
			reactor.Stop()
		}
	}
}

// NOTE: This performs a blocking handshake before the peer is added.
// CONTRACT: Iff error is returned, peer is nil, and conn is immediately closed.
func (sw *Switch) AddPeerWithConnection(conn net.Conn, outbound bool) (*Peer, error) {
	// First, perform handshake
	peerNodeInfo, err := peerHandshake(conn, sw.nodeInfo)
	if err != nil {
		conn.Close()
		return nil, err
	}
	// check version, chain id
	if err := sw.nodeInfo.CompatibleWith(peerNodeInfo); err != nil {
		conn.Close()
		return nil, err
	}
	// avoid self
	if peerNodeInfo.UUID == sw.nodeInfo.UUID {
		conn.Close()
		return nil, fmt.Errorf("Ignoring connection from self")
	}

	// the peerNodeInfo is not verified,
	// so we overwrite the IP with that from the conn
	// and if we dialed out, the port too
	// everything else we just have to trust
	ip, port, _ := net.SplitHostPort(conn.RemoteAddr().String())
	peerNodeInfo.Host = ip
	if outbound {
		porti, _ := strconv.Atoi(port)
		peerNodeInfo.P2PPort = uint16(porti)
	}
	peer := newPeer(conn, peerNodeInfo, outbound, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError)

	// Add the peer to .peers
	// ignore if duplicate or if we already have too many for that IP range
	if err := sw.peers.Add(peer); err != nil {
		log.Info("Ignoring peer", "error", err, "peer", peer)
		peer.stop() // will also close conn
		return nil, err
	}

	if atomic.LoadUint32(&sw.running) == 1 {
		sw.startInitPeer(peer)
	}

	log.Info("Added peer", "peer", peer)
	return peer, nil
}

func (sw *Switch) startInitPeer(peer *Peer) {
	peer.start()               // spawn send/recv routines
	sw.addPeerToReactors(peer) // run AddPeer on each reactor
}

func (sw *Switch) DialPeerWithAddress(addr *NetAddress) (*Peer, error) {
	log.Debug("Dialing address", "address", addr)
	sw.dialing.Set(addr.IP.String(), addr)
	conn, err := addr.DialTimeout(peerDialTimeoutSeconds * time.Second)
	sw.dialing.Delete(addr.IP.String())
	if err != nil {
		log.Debug("Failed dialing address", "address", addr, "error", err)
		return nil, err
	}
	peer, err := sw.AddPeerWithConnection(conn, true)
	if err != nil {
		log.Debug("Failed adding peer", "address", addr, "conn", conn, "error", err)
		return nil, err
	}
	log.Info("Dialed and added peer", "address", addr, "peer", peer)
	return peer, nil
}

func (sw *Switch) IsDialing(addr *NetAddress) bool {
	return sw.dialing.Has(addr.IP.String())
}

// Broadcast runs a go routine for each attempted send, which will block
// trying to send for defaultSendTimeoutSeconds. Returns a channel
// which receives success values for each attempted send (false if times out)
func (sw *Switch) Broadcast(chId byte, msg interface{}) chan bool {
	successChan := make(chan bool, len(sw.peers.List()))
	log.Debug("Broadcast", "channel", chId, "msg", msg)
	for _, peer := range sw.peers.List() {
		go func(peer *Peer) {
			success := peer.Send(chId, msg)
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
	log.Info("Stopping peer for error", "peer", peer, "error", reason)
	sw.peers.Remove(peer)
	peer.stop()
	sw.removePeerFromReactors(peer, reason)
}

// Disconnect from a peer gracefully.
// TODO: handle graceful disconnects.
func (sw *Switch) StopPeerGracefully(peer *Peer) {
	log.Info("Stopping peer gracefully")
	sw.peers.Remove(peer)
	peer.stop()
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
		// note we might exceed the maxNumPeers in order to
		// achieve minNumOutboundPeers
		if sw.peers.Size() >= maxNumPeers {
			log.Debug("Ignoring inbound connection: already have enough peers", "conn", inConn, "numPeers", sw.peers.Size(), "max", maxNumPeers)
			continue
		}

		// Ignore connections from IP ranges for which we have too many
		if sw.peers.HasMaxForIPRange(inConn) {
			log.Debug("Ignoring inbound connection: already have enough peers for that IP range", "address", inConn.RemoteAddr().String())
			continue
		}

		// New inbound connection!
		_, err := sw.AddPeerWithConnection(inConn, false)
		if err != nil {
			log.Info("Ignoring inbound connection: error on AddPeerWithConnection", "conn", inConn, "error", err)
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
