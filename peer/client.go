package peer

import (
	"errors"
	"sync"
	"sync/atomic"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
)

/*
A client is half of a p2p system.
It can reach out to the network and establish connections with other peers.
A client doesn't listen for incoming connections -- that's done by the server.

All communication amongst peers are multiplexed by "channels".
(Not the same as Go "channels")

To send a message, encapsulate it into a "Packet" and send it to each peer.
You can find all connected and active peers by iterating over ".Peers()".
".Broadcast()" is provided for convenience, but by iterating over
the peers manually the caller can decide which subset receives a message.

Incoming messages are received by calling ".Receive()".
*/
type Client struct {
	addrBook       *AddrBook
	targetNumPeers int
	makePeerFn     func(*Connection) *Peer
	self           *Peer
	pktRecvQueues  map[String]chan *InboundPacket
	peersMtx       sync.Mutex
	peers          merkle.Tree // addr -> *Peer
	quit           chan struct{}
	erroredPeers   chan peerError
	stopped        uint32
}

var (
	ErrClientStopped       = errors.New("Client already stopped")
	ErrClientDuplicatePeer = errors.New("Duplicate peer")
)

// "makePeerFn" is a factory method for generating new peers from new *Connections.
// "makePeerFn(nil)" must return a prototypical peer that represents the self "peer".
func NewClient(makePeerFn func(*Connection) *Peer) *Client {
	self := makePeerFn(nil)
	if self == nil {
		Panicf("makePeerFn(nil) must return a prototypical peer for self")
	}

	pktRecvQueues := make(map[String]chan *InboundPacket)
	for chName, _ := range self.channels {
		pktRecvQueues[chName] = make(chan *InboundPacket)
	}

	c := &Client{
		addrBook:       nil, // TODO
		targetNumPeers: 0,   // TODO
		makePeerFn:     makePeerFn,
		self:           self,
		pktRecvQueues:  pktRecvQueues,
		peers:          merkle.NewIAVLTree(nil),
		quit:           make(chan struct{}),
		erroredPeers:   make(chan peerError),
		stopped:        0,
	}

	// automatically start
	c.start()

	return c
}

func (c *Client) start() {
	// Handle peer disconnects & errors
	go c.peerErrorHandler()
}

func (c *Client) Stop() {
	log.Infof("Stopping client")
	// lock
	c.peersMtx.Lock()
	if atomic.CompareAndSwapUint32(&c.stopped, 0, 1) {
		close(c.quit)
		// stop each peer.
		for peerValue := range c.peers.Values() {
			peer := peerValue.(*Peer)
			peer.stop()
		}
		// empty tree.
		c.peers = merkle.NewIAVLTree(nil)
	}
	c.peersMtx.Unlock()
	// unlock
}

func (c *Client) AddPeerWithConnection(conn *Connection, outgoing bool) (*Peer, error) {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return nil, ErrClientStopped
	}

	log.Infof("Adding peer with connection: %v, outgoing: %v", conn, outgoing)
	peer := c.makePeerFn(conn)
	peer.outgoing = outgoing
	err := c.addPeer(peer)
	if err != nil {
		return nil, err
	}

	go peer.start(c.pktRecvQueues, c.erroredPeers)

	return peer, nil
}

func (c *Client) Broadcast(pkt Packet) (numSuccess, numFailure int) {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return
	}

	log.Tracef("Broadcast on [%v] len: %v", pkt.Channel, len(pkt.Bytes))
	for v := range c.peers.Values() {
		peer := v.(*Peer)
		success := peer.TrySend(pkt)
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
func (c *Client) Receive(chName String) *InboundPacket {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return nil
	}

	log.Tracef("Receive on [%v]", chName)
	q := c.pktRecvQueues[chName]
	if q == nil {
		Panicf("Expected pktRecvQueues[%f], found none", chName)
	}

	select {
	case <-c.quit:
		return nil
	case inPacket := <-q:
		return inPacket
	}
}

func (c *Client) Peers() merkle.Tree {
	// lock & defer
	c.peersMtx.Lock()
	defer c.peersMtx.Unlock()
	return c.peers.Copy()
	// unlock deferred
}

func (c *Client) StopPeer(peer *Peer) {
	// lock
	c.peersMtx.Lock()
	peerValue, _ := c.peers.Remove(peer.RemoteAddress())
	c.peersMtx.Unlock()
	// unlock

	peer_ := peerValue.(*Peer)
	if peer_ != nil {
		peer_.stop()
	}
}

func (c *Client) addPeer(peer *Peer) error {
	addr := peer.RemoteAddress()

	// lock & defer
	c.peersMtx.Lock()
	defer c.peersMtx.Unlock()
	if c.stopped == 1 {
		return ErrClientStopped
	}
	if !c.peers.Has(addr) {
		log.Tracef("Actually putting addr: %v, peer: %v", addr, peer)
		c.peers.Put(addr, peer)
		return nil
	} else {
		// ignore duplicate peer for addr.
		log.Infof("Ignoring duplicate peer for addr %v", addr)
		return ErrClientDuplicatePeer
	}
	// unlock deferred
}

func (c *Client) peerErrorHandler() {
	for {
		select {
		case <-c.quit:
			return
		case errPeer := <-c.erroredPeers:
			log.Infof("%v errored: %v", errPeer.peer, errPeer.err)
			// TODO: do more
			c.StopPeer(errPeer.peer)
			return
		}
	}
}
