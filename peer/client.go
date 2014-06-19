package peer

import (
    . "github.com/tendermint/tendermint/binary"
    "github.com/tendermint/tendermint/merkle"
    "sync"
    "io"
)

/* Client */
type Client struct {
    listener        *Listener
    addrBook        AddrBook
    strategies      map[String]*FilterStrategy
    targetNumPeers  int

    peersMtx        sync.Mutex
    peers           merkle.Tree // addr -> *Peer

    filtersMtx      sync.Mutex
    filters         merkle.Tree // channelName -> Filter (objects that I know of)
}

func NewClient(protocol string, laddr string) *Client {
    // XXX set the handler
    listener := NewListener(protocol, laddr, nil)
    c := &Client{
        listener:   listener,
        peers:      merkle.NewIAVLTree(nil),
        filters:    merkle.NewIAVLTree(nil),
    }
    return c
}

func (c *Client) Start() (<-chan *IncomingMsg) {
    return nil
}

func (c *Client) Stop() {
    c.listener.Close()
}

func (c *Client) LocalAddress() *NetAddress {
    return c.listener.LocalAddress()
}

func (c *Client) ConnectTo(addr *NetAddress) (*Peer, error) {

    conn, err := addr.Dial()
    if err != nil { return nil, err }
    peer := NewPeer(conn)

    // lock
    c.peersMtx.Lock()
    c.peers.Put(addr, peer)
    c.peersMtx.Unlock()
    // unlock

    return peer, nil
}

func (c *Client) Broadcast(channel String, msg Binary) {
    for v := range c.peersCopy().Values() {
        peer, ok := v.(*Peer)
        if !ok { panic("Expected peer but got something else") }
        peer.Queue(channel, msg)
    }
}

// Updates the client's filter for a channel & broadcasts it.
func (c *Client) UpdateFilter(channel String, filter Filter) {
    c.filtersMtx.Lock()
    c.filters.Put(channel, filter)
    c.filtersMtx.Unlock()

    c.Broadcast("", &NewFilterMsg{
        Channel:    channel,
        Filter:     filter,
    })
}

func (c *Client) peersCopy() merkle.Tree {
    c.peersMtx.Lock(); defer c.peersMtx.Unlock()
    return c.peers.Copy()
}


/* Channel */
type Channel struct {
    Name            String
    Filter          Filter
    //Stats           Stats
}


/* Peer */
type Peer struct {
    Conn            *Connection
    Channels        map[String]*Channel
}

func NewPeer(conn *Connection) *Peer {
    return &Peer{
        Conn:       conn,
        Channels:   nil,
    }
}

// Must be quick and nonblocking.
func (p *Peer) Queue(channel String, msg Binary) {}

func (p *Peer) WriteTo(w io.Writer) (n int64, err error) {
    return 0, nil // TODO
}


/* IncomingMsg */
type IncomingMsg struct {
    SPeer           *Peer
    SChan           *Channel

    Time            Time

    Msg             Binary
}


/*  Filter

    A Filter could be a bloom filter for lossy filtering, or could be a lossless filter.
    Either way, it's used to keep track of what a peer knows of.
*/
type Filter interface {
    Binary
    Add(ByteSlice)
    Has(ByteSlice) bool
}

/* FilterStrategy

    Defines how filters are generated per peer, and whether they need to get refreshed occasionally.
*/
type FilterStrategy interface {
    LoadFilter(ByteSlice) Filter
}

/* NewFilterMsg */
type NewFilterMsg struct {
    Channel         String
    Filter          Filter
}

func (m *NewFilterMsg) WriteTo(w io.Writer) (int64, error) {
    return 0, nil // TODO
}
