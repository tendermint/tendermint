package peer

import (
    . "github.com/tendermint/tendermint/binary"
    "github.com/tendermint/tendermint/merkle"
    "atomic"
    "sync"
    "io"
    "errors"
)

/*  Client

    A client is half of a p2p system.
    It can reach out to the network and establish connections with servers.
    A client doesn't listen for incoming connections -- that's done by the server.

    newPeerCb is a factory method for generating new peers from new *Connections.
    newPeerCb(nil) must return a prototypical peer that represents the self "peer".

    XXX what about peer disconnects?
*/
type Client struct {
    addrBook        AddrBook
    targetNumPeers  int
    newPeerCb       func(*Connection) *Peer
    self            *Peer
    inQueues        map[String]chan *InboundMsg

    mtx             sync.Mutex
    peers           merkle.Tree // addr -> *Peer
    quit            chan struct{}
    stopped         uint32
}

var (
    CLIENT_STOPPED_ERROR =          errors.New("Client already stopped")
    CLIENT_DUPLICATE_PEER_ERROR =   errors.New("Duplicate peer")
)

func NewClient(newPeerCb func(*Connect) *Peer) *Client {
    self := newPeerCb(nil)
    if self == nil {
        Panicf("newPeerCb(nil) must return a prototypical peer for self")
    }

    inQueues := make(map[String]chan *InboundMsg)
    for chName, channel := peer.channels {
        inQueues[chName] = make(chan *InboundMsg)
    }

    c := &Client{
        newPeerCb:  newPeerCb,
        peers:      merkle.NewIAVLTree(nil),
        self:       self,
        inQueues:   inQueues,
    }
    return c
}

func (c *Client) Stop() {
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

    peer := c.newPeerCb(conn)
    peer.outgoing = outgoing
    err := c.addPeer(peer)
    if err != nil { return nil, err }

    go peer.Start(c.inQueues)

    return peer, nil
}

func (c *Client) Broadcast(chName String, msg Msg) {
    if atomic.LoadUint32(&c.stopped) == 1 { return }

    for v := range c.peersCopy().Values() {
        peer := v.(*Peer)
        success := peer.TryQueueOut(chName , msg)
        if !success {
            // TODO: notify the peer
        }
    }
}

func (c *Client) PopMessage(chName String) *InboundMsg {
    if atomic.LoadUint32(&c.stopped) == 1 { return nil }

    channel := c.Channel(chName)
    q := c.inQueues[chName]
    if q == nil { Panicf("Expected inQueues[%f], found none", chName) }

    for {
        select {
        case <-quit:
            return nil
        case msg := <-q:
            // skip if known.
            if channel.Filter().Has(msg) {
                continue
            }
            return msg
        }
    }
}

// Updates self's filter for a channel & broadcasts it.
// TODO: maybe don't expose this
func (c *Client) UpdateFilter(chName String, filter Filter) {
    if atomic.LoadUint32(&c.stopped) == 1 { return }

    c.self.Channel(chName).UpdateFilter(filter)

    c.Broadcast("", &NewFilterMsg{
        Channel:    chName,
        Filter:     filter,
    })
}

func (c *Client) StopPeer(peer *Peer) {
    // lock
    c.mtx.Lock()
    p, _ := c.peers.Remove(peer.RemoteAddress())
    c.mtx.Unlock()
    // unlock

    if p != nil {
        p.Stop()
    }
}

func (c *Client) addPeer(peer *Peer) error {
    addr := peer.RemoteAddress()

    // lock & defer
    c.mtx.Lock(); defer c.mtx.Unlock()
    if c.stopped == 1 { return CLIENT_STOPPED_ERROR }
    if !c.peers.Has(addr) {
        c.peers.Put(addr, peer)
        return nil
    } else {
        // ignore duplicate peer for addr.
        log.Infof("Ignoring duplicate peer for addr %v", addr)
        return CLIENT_DUPLICATE_PEER_ERROR
    }
    // unlock deferred
}

func (c *Client) peersCopy() merkle.Tree {
    // lock & defer
    c.mtx.Lock(); defer c.mtx.Unlock()
    return c.peers.Copy()
    // unlock deferred
}
