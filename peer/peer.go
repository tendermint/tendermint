package peer

import (
    "atomic"
    "sync"
)

/* Peer */

type Peer struct {
    outgoing        bool
    conn            *Connection
    channels        map[String]*Channel

    mtx             sync.Mutex
    quit            chan struct{}
    stopped         uint32
}

func (p *Peer) Start(peerInQueues map[String]chan *InboundMsg ) {
    for chName, _ := range p.channels {
        go p.inHandler(chName, peerInQueues[chName])
        go p.outHandler(chName)
    }
}

func (p *Peer) Stop() {
    // lock
    p.mtx.Lock()
    if atomic.CompareAndSwapUint32(&p.stopped, 0, 1) {
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

func (p *Peer) Channel(chName String) *Channel {
    return p.channels[chName]
}

// If msg isn't already in the peer's filter, then
// queue the msg for output.
// If the queue is full, just return false.
func (p *Peer) TryQueueOut(chName String, msg Msg) bool {
    channel := p.Channel(chName)
    outQueue := channel.OutQueue()

    // just return if already in filter
    if channel.Filter().Has(msg) {
        return true
    }

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

func (p *Peer) inHandler(chName String, inboundMsgQueue chan<- *InboundMsg) {
    channel := p.channels[chName]
    inQueue := channel.InQueue()

    FOR_LOOP:
    for {
        select {
        case <-quit:
            break FOR_LOOP
        case msg := <-inQueue:
            // add to channel filter
            channel.Filter().Add(msg)
            // send to inboundMsgQueue
            inboundMsg := &InboundMsg{
                Peer:       p,
                Channel:    channel,
                Time:       Time(time.Now()),
                Msg:        msg,
            }
            select {
            case <-quit:
                break FOR_LOOP
            case inboundMsgQueue <- inboundMsg:
                continue
            }
        }
    }

    // cleanup
    // (none)
}

func (p *Peer) outHandler(chName String) {
    outQueue := p.channels[chName].OutQueue()
    FOR_LOOP:
    for {
        select {
        case <-quit:
            break FOR_LOOP
        case msg := <-outQueue:
            // blocks until the connection is Stop'd,
            // which happens when this peer is Stop'd.
            p.conn.QueueOut(msg.Bytes)
        }
    }

    // cleanup
    // (none)
}


/*  Channel */

type Channel struct {
    name            String

    mtx             sync.Mutex
    filter          Filter

    inQueue         chan Msg
    outQueue        chan Msg
    //stats           Stats
}

func NewChannel(name String, filter Filter, in, out chan Msg) *Channel {
    return &Channel{
        name:       name,
        filter:     filter,
        inQueue:    in,
        outQueue:   out,
    }
}

func (c *Channel) InQueue() <-chan Msg {
    return c.inQueue
}

func (c *Channel) OutQueue() chan<- Msg {
    return c.outQueue
}

func (c *Channel) Add(msg Msg) {
    c.Filter().Add(msg)
}

func (c *Channel) Has(msg Msg) bool {
    return c.Filter().Has(msg)
}

// TODO: maybe don't expose this
func (c *Channel) Filter() Filter {
    // lock & defer
    c.mtx.Lock(); defer c.mtx.Unlock()
    return c.filter
    // unlock deferred
}

// TODO: maybe don't expose this
func (c *Channel) UpdateFilter(filter Filter) {
    // lock
    c.mtx.Lock()
    c.filter = filter
    c.mtx.Unlock()
    // unlock
}
