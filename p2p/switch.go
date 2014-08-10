package p2p

import (
	"errors"
	"net"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

/*
All communication amongst peers are multiplexed by "channels".
(Not the same as Go "channels")

To send a message, serialize it into a ByteSlice and send it to each peer.
For best performance, re-use the same immutable ByteSlice to each peer.
You can also use a TypedBytes{} struct for convenience.
You can find all connected and active peers by iterating over ".Peers().List()".
".Broadcast()" is provided for convenience, but by iterating over
the peers manually the caller can decide which subset receives a message.

Inbound messages are received by calling ".Receive()".
The receiver is responsible for decoding the message bytes, which may be preceded
by a single type byte if a TypedBytes{} was used.
*/
type Switch struct {
	chDescs    []*ChannelDescriptor
	recvQueues map[byte]chan InboundBytes
	peers      *PeerSet
	dialing    *CMap
	listeners  *CMap // listenerName -> chan interface{}
	quit       chan struct{}
	started    uint32
	stopped    uint32
}

var (
	ErrSwitchStopped       = errors.New("Switch already stopped")
	ErrSwitchDuplicatePeer = errors.New("Duplicate peer")
)

const (
	peerDialTimeoutSeconds = 30
)

func NewSwitch(chDescs []*ChannelDescriptor) *Switch {
	s := &Switch{
		chDescs:    chDescs,
		recvQueues: make(map[byte]chan InboundBytes),
		peers:      NewPeerSet(),
		dialing:    NewCMap(),
		listeners:  NewCMap(),
		quit:       make(chan struct{}),
		stopped:    0,
	}

	// Create global recvQueues, one per channel.
	for _, chDesc := range chDescs {
		recvQueue := make(chan InboundBytes, chDesc.RecvQueueCapacity)
		chDesc.recvQueue = recvQueue
		s.recvQueues[chDesc.Id] = recvQueue
	}

	return s
}

func (s *Switch) Start() {
	if atomic.CompareAndSwapUint32(&s.started, 0, 1) {
		log.Info("Starting switch")
	}
}

func (s *Switch) Stop() {
	if atomic.CompareAndSwapUint32(&s.stopped, 0, 1) {
		log.Info("Stopping switch")
		close(s.quit)
		// stop each peer.
		for _, peer := range s.peers.List() {
			peer.stop()
		}
		// empty tree.
		s.peers = NewPeerSet()
	}
}

func (s *Switch) AddPeerWithConnection(conn net.Conn, outbound bool) (*Peer, error) {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return nil, ErrSwitchStopped
	}

	peer := newPeer(conn, outbound, s.chDescs, s.StopPeerForError)

	// Add the peer to .peers
	if s.peers.Add(peer) {
		log.Info("+ %v", peer)
	} else {
		log.Info("Ignoring duplicate: %v", peer)
		return nil, ErrSwitchDuplicatePeer
	}

	// Start the peer
	go peer.start()

	// Notify listeners.
	s.emit(SwitchEventNewPeer{Peer: peer})

	return peer, nil
}

func (s *Switch) DialPeerWithAddress(addr *NetAddress) (*Peer, error) {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return nil, ErrSwitchStopped
	}

	log.Info("Dialing peer @ %v", addr)
	s.dialing.Set(addr.String(), addr)
	conn, err := addr.DialTimeout(peerDialTimeoutSeconds * time.Second)
	s.dialing.Delete(addr.String())
	if err != nil {
		return nil, err
	}
	peer, err := s.AddPeerWithConnection(conn, true)
	if err != nil {
		return nil, err
	}
	return peer, nil
}

func (s *Switch) IsDialing(addr *NetAddress) bool {
	return s.dialing.Has(addr.String())
}

func (s *Switch) Broadcast(chId byte, msg Binary) (numSuccess, numFailure int) {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return
	}

	log.Debug("Broadcast on [%X]", chId, msg)
	for _, peer := range s.peers.List() {
		success := peer.TrySend(chId, msg)
		log.Debug("Broadcast for peer %v success: %v", peer, success)
		if success {
			numSuccess += 1
		} else {
			numFailure += 1
		}
	}
	return

}

// The events are of type SwitchEvent* defined below.
// Switch does not close these listeners.
func (s *Switch) AddEventListener(name string, listener chan<- interface{}) {
	s.listeners.Set(name, listener)
}

func (s *Switch) RemoveEventListener(name string) {
	s.listeners.Delete(name)
}

/*
Receive blocks on a channel until a message is found.
*/
func (s *Switch) Receive(chId byte) (InboundBytes, bool) {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return InboundBytes{}, false
	}

	q := s.recvQueues[chId]
	if q == nil {
		Panicf("Expected recvQueues[%X], found none", chId)
	}

	select {
	case <-s.quit:
		return InboundBytes{}, false
	case inBytes := <-q:
		log.Debug("RECV %v", inBytes)
		return inBytes, true
	}
}

// Returns the count of outbound/inbound and outbound-dialing peers.
func (s *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := s.peers.List()
	for _, peer := range peers {
		if peer.outbound {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = s.dialing.Size()
	return
}

func (s *Switch) Peers() IPeerSet {
	return s.peers
}

// Disconnect from a peer due to external error.
// TODO: make record depending on reason.
func (s *Switch) StopPeerForError(peer *Peer, reason interface{}) {
	log.Info("- %v !! reason: %v", peer, reason)
	s.peers.Remove(peer)
	peer.stop()

	// Notify listeners
	s.emit(SwitchEventDonePeer{Peer: peer, Error: reason})
}

// Disconnect from a peer gracefully.
// TODO: handle graceful disconnects.
func (s *Switch) StopPeerGracefully(peer *Peer) {
	log.Info("- %v", peer)
	s.peers.Remove(peer)
	peer.stop()

	// Notify listeners
	s.emit(SwitchEventDonePeer{Peer: peer})
}

func (s *Switch) emit(event interface{}) {
	for _, ch_i := range s.listeners.Values() {
		ch := ch_i.(chan<- interface{})
		ch <- event
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
