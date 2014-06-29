package peer

import (
    . "github.com/tendermint/tendermint/binary"
    "sync/atomic"
    "sync"
    "io"
    "time"
    "fmt"
)

/* Peer */

type Peer struct {
    outgoing        bool
    conn            *Connection
    channels        map[string]*Channel

    mtx             sync.Mutex
    quit            chan struct{}
    stopped         uint32
}

func NewPeer(conn *Connection) *Peer {
    return &Peer{
        conn:       conn,
        quit:       make(chan struct{}),
        stopped:    0,
    }
}

func (p *Peer) Start(peerInQueues map[string]chan *InboundMsg ) {
    log.Debugf("Starting %v", p)
    p.conn.Start()
    for chName, _ := range p.channels {
        go p.inHandler(chName, peerInQueues[chName])
        go p.outHandler(chName)
    }
}

func (p *Peer) Stop() {
    // lock
    p.mtx.Lock()
    if atomic.CompareAndSwapUint32(&p.stopped, 0, 1) {
        log.Debugf("Stopping %v", p)
        close(p.quit)
        p.conn.Stop()
    }
    p.mtx.Unlock()
    // unlock
}

func (p *Peer) LocalAddress() *NetAddress {
    return p.conn.LocalAddress()
}

func (p *Peer) RemoteAddress() *NetAddress {
    return p.conn.RemoteAddress()
}

func (p *Peer) Channel(chName string) *Channel {
    return p.channels[chName]
}

// Queue the msg for output.
// If the queue is full, just return false.
func (p *Peer) TryQueueOut(chName string, msg Msg) bool {
    channel := p.Channel(chName)
    outQueue := channel.OutQueue()

    // lock & defer
    p.mtx.Lock(); defer p.mtx.Unlock()
    if p.stopped == 1 { return false }
    select {
    case outQueue <- msg:
        return true
    default: // buffer full
        return false
    }
    // unlock deferred
}

func (p *Peer) WriteTo(w io.Writer) (n int64, err error) {
    return p.RemoteAddress().WriteTo(w)
}

func (p *Peer) String() string {
    return fmt.Sprintf("Peer{%v-%v,o:%v}", p.LocalAddress(), p.RemoteAddress(), p.outgoing)
}

func (p *Peer) inHandler(chName string, inboundMsgQueue chan<- *InboundMsg) {
    log.Tracef("Peer %v inHandler [%v]", p, chName)
    channel := p.channels[chName]
    inQueue := channel.InQueue()

    FOR_LOOP:
    for {
        select {
        case <-p.quit:
            break FOR_LOOP
        case msg := <-inQueue:
            // send to inboundMsgQueue
            inboundMsg := &InboundMsg{
                Peer:       p,
                Channel:    channel,
                Time:       Time{time.Now()},
                Msg:        msg,
            }
            select {
            case <-p.quit:
                break FOR_LOOP
            case inboundMsgQueue <- inboundMsg:
                continue
            }
        }
    }

    log.Tracef("Peer %v inHandler [%v] closed", p, chName)
    // cleanup
    // (none)
}

func (p *Peer) outHandler(chName string) {
    log.Tracef("Peer %v outHandler [%v]", p, chName)
    outQueue := p.channels[chName].outQueue
    FOR_LOOP:
    for {
        select {
        case <-p.quit:
            break FOR_LOOP
        case msg := <-outQueue:
            log.Tracef("Sending msg to peer outQueue")
            // blocks until the connection is Stop'd,
            // which happens when this peer is Stop'd.
            p.conn.QueueOut(msg.Bytes)
        }
    }

    log.Tracef("Peer %v outHandler [%v] closed", p, chName)
    // cleanup
    // (none)
}


/*  Channel */

type Channel struct {
    name            string
    inQueue         chan Msg
    outQueue        chan Msg
    //stats           Stats
}

func NewChannel(name string, bufferSize int) *Channel {
    return &Channel{
        name:       name,
        inQueue:    make(chan Msg, bufferSize),
        outQueue:   make(chan Msg, bufferSize),
    }
}

func (c *Channel) Name() string {
    return c.name
}

func (c *Channel) InQueue() <-chan Msg {
    return c.inQueue
}

func (c *Channel) OutQueue() chan<- Msg {
    return c.outQueue
}
