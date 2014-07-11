package p2p

import (
	"errors"
	"sync/atomic"

	. "github.com/tendermint/tendermint/common"
)

/*
All communication amongst peers are multiplexed by "channels".
(Not the same as Go "channels")

To send a message, encapsulate it into a "Packet" and send it to each peer.
You can find all connected and active peers by iterating over ".Peers().List()".
".Broadcast()" is provided for convenience, but by iterating over
the peers manually the caller can decide which subset receives a message.

Incoming messages are received by calling ".Receive()".
*/
type Switch struct {
	channels      []ChannelDescriptor
	pktRecvQueues map[string]chan *InboundPacket
	peers         *PeerSet
	quit          chan struct{}
	started       uint32
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
		peers:         NewPeerSet(),
		quit:          make(chan struct{}),
		stopped:       0,
	}

	return s
}

func (s *Switch) Start() {
	if atomic.CompareAndSwapUint32(&s.started, 0, 1) {
		log.Infof("Starting switch")
	}
}

func (s *Switch) Stop() {
	if atomic.CompareAndSwapUint32(&s.stopped, 0, 1) {
		log.Infof("Stopping switch")
		close(s.quit)
		// stop each peer.
		for _, peer := range s.peers.List() {
			peer.stop()
		}
		// empty tree.
		s.peers = NewPeerSet()
	}
}

func (s *Switch) AddPeerWithConnection(conn *Connection, outbound bool) (*Peer, error) {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return nil, ErrSwitchStopped
	}

	log.Infof("Adding peer with connection: %v, outbound: %v", conn, outbound)
	// Create channels for peer
	channels := map[string]*Channel{}
	for _, chDesc := range s.channels {
		channels[chDesc.Name] = newChannel(chDesc)
	}
	peer := newPeer(conn, channels)
	peer.outbound = outbound
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
	for _, peer := range s.peers.List() {
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

func (s *Switch) NumOutboundPeers() (count int) {
	peers := s.peers.List()
	for _, peer := range peers {
		if peer.outbound {
			count++
		}
	}
	return
}

func (s *Switch) Peers() ReadOnlyPeerSet {
	return s.peers
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
	s.peers.Remove(peer)
	peer.stop()
}

func (s *Switch) addPeer(peer *Peer) error {
	if s.stopped == 1 {
		return ErrSwitchStopped
	}
	if s.peers.Add(peer) {
		log.Tracef("Adding: %v", peer)
		return nil
	} else {
		// ignore duplicate peer
		log.Infof("Ignoring duplicate: %v", peer)
		return ErrSwitchDuplicatePeer
	}
}
