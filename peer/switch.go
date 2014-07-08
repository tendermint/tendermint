package peer

import (
	"errors"
	"sync"
	"sync/atomic"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
)

/*
All communication amongst peers are multiplexed by "channels".
(Not the same as Go "channels")

To send a message, encapsulate it into a "Packet" and send it to each peer.
You can find all connected and active peers by iterating over ".Peers()".
".Broadcast()" is provided for convenience, but by iterating over
the peers manually the caller can decide which subset receives a message.

Incoming messages are received by calling ".Receive()".
*/
type Switch struct {
	channels      []ChannelDescriptor
	pktRecvQueues map[string]chan *InboundPacket
	peersMtx      sync.Mutex
	peers         merkle.Tree // addr -> *Peer
	quit          chan struct{}
	stopped       uint32
}

var (
	ErrSwitchStopped       = errors.New("Switch already stopped")
	ErrSwitchDuplicatePeer = errors.New("Duplicate peer")
)

func NewSwitch(channels []ChannelDescriptor) *Switch {
	// make pktRecvQueues...
	pktRecvQueues := make(map[string]chan *InboundPacket)
	for _, chDesc := range channels {
		pktRecvQueues[chDesc.Name] = make(chan *InboundPacket)
	}

	s := &Switch{
		channels:      channels,
		pktRecvQueues: pktRecvQueues,
		peers:         merkle.NewIAVLTree(nil),
		quit:          make(chan struct{}),
		stopped:       0,
	}

	// automatically start
	s.start()

	return s
}

func (s *Switch) start() {
	// Handle PEX messages
	// TODO: hmm
	// go peerExchangeHandler(c)
}

func (s *Switch) Stop() {
	log.Infof("Stopping switch")
	// lock
	s.peersMtx.Lock()
	if atomic.CompareAndSwapUint32(&s.stopped, 0, 1) {
		close(s.quit)
		// stop each peer.
		for peerValue := range s.peers.Values() {
			peer := peerValue.(*Peer)
			peer.stop()
		}
		// empty tree.
		s.peers = merkle.NewIAVLTree(nil)
	}
	s.peersMtx.Unlock()
	// unlock
}

func (s *Switch) AddPeerWithConnection(conn *Connection, outgoing bool) (*Peer, error) {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return nil, ErrSwitchStopped
	}

	log.Infof("Adding peer with connection: %v, outgoing: %v", conn, outgoing)
	// Create channels for peer
	channels := map[string]*Channel{}
	for _, chDesc := range s.channels {
		channels[chDesc.Name] = newChannel(chDesc)
	}
	peer := newPeer(conn, channels)
	peer.outgoing = outgoing
	err := s.addPeer(peer)
	if err != nil {
		return nil, err
	}

	go peer.start(s.pktRecvQueues, s.StopPeerForError)

	return peer, nil
}

func (s *Switch) Broadcast(pkt Packet) (numSuccess, numFailure int) {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return
	}

	log.Tracef("Broadcast on [%v] len: %v", pkt.Channel, len(pkt.Bytes))
	for v := range s.peers.Values() {
		peer := v.(*Peer)
		success := peer.TryQueue(pkt)
		log.Tracef("Broadcast for peer %v success: %v", peer, success)
		if success {
			numSuccess += 1
		} else {
			numFailure += 1
		}
	}
	return

}

/*
Receive blocks on a channel until a message is found.
*/
func (s *Switch) Receive(chName string) *InboundPacket {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return nil
	}

	log.Tracef("Receive on [%v]", chName)
	q := s.pktRecvQueues[chName]
	if q == nil {
		Panicf("Expected pktRecvQueues[%f], found none", chName)
	}

	select {
	case <-s.quit:
		return nil
	case inPacket := <-q:
		return inPacket
	}
}

func (s *Switch) Peers() merkle.Tree {
	// lock & defer
	s.peersMtx.Lock()
	defer s.peersMtx.Unlock()
	return s.peers.Copy()
	// unlock deferred
}

// Disconnect from a peer due to external error.
// TODO: make record depending on reason.
func (s *Switch) StopPeerForError(peer *Peer, reason interface{}) {
	log.Infof("%v errored: %v", peer, reason)
	s.StopPeer(peer, false)
}

// Disconnect from a peer.
// If graceful is true, last message sent is a disconnect message.
// TODO: handle graceful disconnects.
func (s *Switch) StopPeer(peer *Peer, graceful bool) {
	// lock
	s.peersMtx.Lock()
	peerValue, _ := s.peers.Remove(peer.RemoteAddress())
	s.peersMtx.Unlock()
	// unlock

	peer_ := peerValue.(*Peer)
	if peer_ != nil {
		peer_.stop()
	}
}

func (s *Switch) addPeer(peer *Peer) error {
	addr := peer.RemoteAddress()

	// lock & defer
	s.peersMtx.Lock()
	defer s.peersMtx.Unlock()
	if s.stopped == 1 {
		return ErrSwitchStopped
	}
	if !s.peers.Has(addr) {
		log.Tracef("Actually putting addr: %v, peer: %v", addr, peer)
		s.peers.Put(addr, peer)
		return nil
	} else {
		// ignore duplicate peer for addr.
		log.Infof("Ignoring duplicate peer for addr %v", addr)
		return ErrSwitchDuplicatePeer
	}
	// unlock deferred
}
