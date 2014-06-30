package peer

import (
    . "github.com/tendermint/tendermint/common"
    . "github.com/tendermint/tendermint/binary"
    "github.com/tendermint/tendermint/merkle"
    "sync/atomic"
    "sync"
    "errors"
)

/*  Client

    A client is half of a p2p system.
    It can reach out to the network and establish connections with servers.
    A client doesn't listen for incoming connections -- that's done by the server.

    makePeerFn is a factory method for generating new peers from new *Connections.
    makePeerFn(nil) must return a prototypical peer that represents the self "peer".

    XXX what about peer disconnects?
*/
type Client struct {
    addrBook        *AddrBook
    targetNumPeers  int
    makePeerFn      func(*Connection) *Peer
    self            *Peer
    recvQueues      map[String]chan *InboundPacket

    mtx             sync.Mutex
    peers           merkle.Tree // addr -> *Peer
    quit            chan struct{}
    stopped         uint32
}

var (
    CLIENT_STOPPED_ERROR =          errors.New("Client already stopped")
    CLIENT_DUPLICATE_PEER_ERROR =   errors.New("Duplicate peer")
)

func NewClient(makePeerFn func(*Connection) *Peer) *Client {
    self := makePeerFn(nil)
    if self == nil {
        Panicf("makePeerFn(nil) must return a prototypical peer for self")
    }

    recvQueues := make(map[String]chan *InboundPacket)
    for chName, _ := range self.channels {
        recvQueues[chName] = make(chan *InboundPacket)
    }

    c := &Client{
        addrBook:       nil, // TODO
        targetNumPeers: 0, // TODO
        makePeerFn:     makePeerFn,
        self:           self,
        recvQueues:     recvQueues,

        peers:          merkle.NewIAVLTree(nil),
        quit:           make(chan struct{}),
        stopped:        0,
    }
    return c
}

func (c *Client) Stop() {
    log.Infof("Stopping client")
    // lock
    c.mtx.Lock()
    if atomic.CompareAndSwapUint32(&c.stopped, 0, 1) {
        close(c.quit)
        // stop each peer.
        for peerValue := range c.peers.Values() {
            peer := peerValue.(*Peer)
            peer.Stop()
        }
        // empty tree.
        c.peers = merkle.NewIAVLTree(nil)
    }
    c.mtx.Unlock()
    // unlock
}

func (c *Client) AddPeerWithConnection(conn *Connection, outgoing bool) (*Peer, error) {
    if atomic.LoadUint32(&c.stopped) == 1 { return nil, CLIENT_STOPPED_ERROR }

    log.Infof("Adding peer with connection: %v, outgoing: %v", conn, outgoing)
    peer := c.makePeerFn(conn)
    peer.outgoing = outgoing
    err := c.addPeer(peer)
    if err != nil { return nil, err }

    go peer.Start(c.recvQueues)

    return peer, nil
}

func (c *Client) Broadcast(pkt Packet) {
    if atomic.LoadUint32(&c.stopped) == 1 { return }

    log.Tracef("Broadcast on [%v] len: %v", pkt.Channel, len(pkt.Bytes))
    for v := range c.Peers().Values() {
        peer := v.(*Peer)
        success := peer.TrySend(pkt)
        log.Tracef("Broadcast for peer %v success: %v", peer, success)
        if !success {
            // TODO: notify the peer
        }
    }

}

// blocks until a message is popped.
func (c *Client) Receive(chName String) *InboundPacket {
    if atomic.LoadUint32(&c.stopped) == 1 { return nil }

    log.Tracef("Receive on [%v]", chName)
    q := c.recvQueues[chName]
    if q == nil { Panicf("Expected recvQueues[%f], found none", chName) }

    for {
        select {
        case <-c.quit:
            return nil
        case inPacket := <-q:
            return inPacket
        }
    }
}

func (c *Client) Peers() merkle.Tree {
    // lock & defer
    c.mtx.Lock(); defer c.mtx.Unlock()
    return c.peers.Copy()
    // unlock deferred
}

func (c *Client) StopPeer(peer *Peer) {
    // lock
    c.mtx.Lock()
    peerValue, _ := c.peers.Remove(peer.RemoteAddress())
    c.mtx.Unlock()
    // unlock

    peer_ := peerValue.(*Peer)
    if peer_ != nil {
        peer_.Stop()
    }
}

func (c *Client) addPeer(peer *Peer) error {
    addr := peer.RemoteAddress()

    // lock & defer
    c.mtx.Lock(); defer c.mtx.Unlock()
    if c.stopped == 1 { return CLIENT_STOPPED_ERROR }
    if !c.peers.Has(addr) {
        log.Tracef("Actually putting addr: %v, peer: %v", addr, peer)
        c.peers.Put(addr, peer)
        return nil
    } else {
        // ignore duplicate peer for addr.
        log.Infof("Ignoring duplicate peer for addr %v", addr)
        return CLIENT_DUPLICATE_PEER_ERROR
    }
    // unlock deferred
}
