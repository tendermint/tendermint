package p2p

import (
	"errors"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/common"
)

/*
All communication amongst peers are multiplexed by "channels".
(Not the same as Go "channels")

To send a message, encapsulate it into a "Packet" and send it to each peer.
You can find all connected and active peers by iterating over ".Peers().List()".
".Broadcast()" is provided for convenience, but by iterating over
the peers manually the caller can decide which subset receives a message.

Inbound messages are received by calling ".Receive()".
*/
type Switch struct {
	channels      []ChannelDescriptor
	pktRecvQueues map[string]chan *InboundPacket
	peers         *PeerSet
	dialing       *CMap
	quit          chan struct{}
	started       uint32
	stopped       uint32
}

var (
	ErrSwitchStopped       = errors.New("Switch already stopped")
	ErrSwitchDuplicatePeer = errors.New("Duplicate peer")
)

const (
	peerDialTimeoutSeconds = 30
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
		dialing:       NewCMap(),
		quit:          make(chan struct{}),
		stopped:       0,
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

func (s *Switch) AddPeerWithConnection(conn *Connection, outbound bool) (*Peer, error) {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return nil, ErrSwitchStopped
	}

	log.Info("Adding peer with connection: %v, outbound: %v", conn, outbound)
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

func (s *Switch) Broadcast(pkt Packet) (numSuccess, numFailure int) {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return
	}

	log.Debug("Broadcast on [%v] len: %v", pkt.Channel, len(pkt.Bytes))
	for _, peer := range s.peers.List() {
		success := peer.TrySend(pkt)
		log.Debug("Broadcast for peer %v success: %v", peer, success)
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

	q := s.pktRecvQueues[chName]
	if q == nil {
		Panicf("Expected pktRecvQueues[%f], found none", chName)
	}

	select {
	case <-s.quit:
		return nil
	case inPacket := <-q:
		log.Debug("Received packet on [%v]", chName)
		return inPacket
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
	log.Info("%v errored: %v", peer, reason)
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
		log.Debug("Adding: %v", peer)
		return nil
	} else {
		// ignore duplicate peer
		log.Info("Ignoring duplicate: %v", peer)
		return ErrSwitchDuplicatePeer
	}
}
