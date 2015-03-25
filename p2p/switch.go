package p2p

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/common"
)

type Reactor interface {
	Start(sw *Switch)
	Stop()
	GetChannels() []*ChannelDescriptor
	AddPeer(peer *Peer)
	RemovePeer(peer *Peer, reason interface{})
	Receive(chId byte, peer *Peer, msgBytes []byte)
}

//-----------------------------------------------------------------------------

/*
The `Switch` handles peer connections and exposes an API to receive incoming messages
on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
incoming messages are received on the reactor.
*/
type Switch struct {
	chainId      string
	reactors     map[string]Reactor
	chDescs      []*ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *CMap
	listeners    *CMap  // listenerName -> chan interface{}
	running      uint32 // atomic
}

var (
	ErrSwitchDuplicatePeer = errors.New("Duplicate peer")
	ErrSwitchStopped       = errors.New("Switch stopped")
)

const (
	peerDialTimeoutSeconds = 3
)

func NewSwitch() *Switch {

	sw := &Switch{
		chainId:      "",
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      NewCMap(),
		listeners:    NewCMap(),
		running:      0,
	}

	return sw
}

func (sw *Switch) SetChainId(hash []byte, network string) {
	sw.chainId = hex.EncodeToString(hash) + "-" + network
}

func (sw *Switch) AddReactor(name string, reactor Reactor) {
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
	time.Sleep(1 * time.Second)
}

func (sw *Switch) StartReactor(name string) {
	atomic.StoreUint32(&sw.running, 1)
	sw.reactors[name].Start(sw)
}

// Convenience function
func (sw *Switch) StartAll() {
	atomic.StoreUint32(&sw.running, 1)
	for _, reactor := range sw.reactors {
		reactor.Start(sw)
	}
}

func (sw *Switch) StopReactor(name string) {
	sw.reactors[name].Stop()
}

// Convenience function
// Not goroutine safe
func (sw *Switch) StopAll() {
	atomic.StoreUint32(&sw.running, 0)
	// Stop each peer.
	for _, peer := range sw.peers.List() {
		peer.stop()
	}
	sw.peers = NewPeerSet()
	// Stop all reactors.
	for _, reactor := range sw.reactors {
		reactor.Stop()
	}
}

// Not goroutine safe
func (sw *Switch) Reactors() map[string]Reactor {
	return sw.reactors
}

func (sw *Switch) AddPeerWithConnection(conn net.Conn, outbound bool) (*Peer, error) {
	if atomic.LoadUint32(&sw.running) == 0 {
		return nil, ErrSwitchStopped
	}

	peer := newPeer(conn, outbound, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError)

	// Add the peer to .peers
	if sw.peers.Add(peer) {
		log.Info("Added peer", "peer", peer)
	} else {
		log.Info("Ignoring duplicate peer", "peer", peer)
		return nil, ErrSwitchDuplicatePeer
	}

	// Notify listeners.
	sw.doAddPeer(peer)

	// Start the peer
	go peer.start()

	// Send handshake
	msg := &pexHandshakeMessage{ChainId: sw.chainId}
	peer.Send(PexChannel, msg)

	return peer, nil
}

func (sw *Switch) DialPeerWithAddress(addr *NetAddress) (*Peer, error) {
	if atomic.LoadUint32(&sw.running) == 0 {
		return nil, ErrSwitchStopped
	}

	log.Debug("Dialing address", "address", addr)
	sw.dialing.Set(addr.String(), addr)
	conn, err := addr.DialTimeout(peerDialTimeoutSeconds * time.Second)
	sw.dialing.Delete(addr.String())
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
	return sw.dialing.Has(addr.String())
}

// Broadcast runs a go routine for each attempted send, which will block
// trying to send for defaultSendTimeoutSeconds. Returns a channel
// which receives success values for each attempted send (false if times out)
func (sw *Switch) Broadcast(chId byte, msg interface{}) chan bool {
	if atomic.LoadUint32(&sw.running) == 0 {
		return nil
	}
	successChan := make(chan bool, len(sw.peers.List()))
	log.Debug("Broadcast", "channel", chId, "msg", msg)
	for _, peer := range sw.peers.List() {
		go func() {
			success := peer.Send(chId, msg)
			successChan <- success
		}()
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

	// Notify listeners
	sw.doRemovePeer(peer, reason)
}

// Disconnect from a peer gracefully.
// TODO: handle graceful disconnects.
func (sw *Switch) StopPeerGracefully(peer *Peer) {
	log.Info("Stopping peer gracefully")
	sw.peers.Remove(peer)
	peer.stop()

	// Notify listeners
	sw.doRemovePeer(peer, nil)
}

func (sw *Switch) IsListening() bool {
	return sw.listeners.Size() > 0
}

func (sw *Switch) doAddPeer(peer *Peer) {
	for name, reactor := range sw.reactors {
		reactor.AddPeer(peer)
	}
}

func (sw *Switch) doRemovePeer(peer *Peer, reason interface{}) {
	for _, reactor := range sw.reactors {
		reactor.RemovePeer(peer, reason)
	}
}

//-----------------------------------------------------------------------------

type SwitchEventNewPeer struct {
	Peer *Peer
}

type SwitchEventDonePeer struct {
	Peer  *Peer
	Error interface{}
}
