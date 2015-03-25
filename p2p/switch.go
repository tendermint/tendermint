package p2p

import (
	"errors"
	"fmt"
	"net"
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
	network      string
	reactors     map[string]Reactor
	chDescs      []*ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *CMap
	listeners    *CMap // listenerName -> chan interface{}
}

var (
	ErrSwitchDuplicatePeer = errors.New("Duplicate peer")
)

const (
	peerDialTimeoutSeconds = 3
)

func NewSwitch() *Switch {

	sw := &Switch{
		network:      "",
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      NewCMap(),
		listeners:    NewCMap(),
	}

	return sw
}

// Not goroutine safe.
func (sw *Switch) SetNetwork(network string) {
	sw.network = network
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

func (sw *Switch) Reactor(name string) Reactor {
	return sw.reactors[name]
}

// Convenience function
func (sw *Switch) StartReactors() {
	for _, reactor := range sw.reactors {
		reactor.Start(sw)
	}
}

// Convenience function
func (sw *Switch) StopReactors() {
	// Stop all reactors.
	for _, reactor := range sw.reactors {
		reactor.Stop()
	}
}

// Convenience function
func (sw *Switch) StopPeers() {
	// Stop each peer.
	for _, peer := range sw.peers.List() {
		peer.stop()
	}
	sw.peers = NewPeerSet()
}

// Convenience function
func (sw *Switch) Stop() {
	sw.StopPeers()
	sw.StopReactors()
}

// Not goroutine safe to modify.
func (sw *Switch) Reactors() map[string]Reactor {
	return sw.reactors
}

func (sw *Switch) AddPeerWithConnection(conn net.Conn, outbound bool) (*Peer, error) {
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
	msg := &pexHandshakeMessage{Network: sw.network}
	peer.Send(PexChannel, msg)

	return peer, nil
}

func (sw *Switch) DialPeerWithAddress(addr *NetAddress) (*Peer, error) {
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
	for _, reactor := range sw.reactors {
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
