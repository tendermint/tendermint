package p2p

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
)

/* Peer */

type Peer struct {
	outbound bool
	conn     *Connection
	channels map[string]*Channel
	quit     chan struct{}
	started  uint32
	stopped  uint32
}

func newPeer(conn *Connection, channels map[string]*Channel) *Peer {
	return &Peer{
		conn:     conn,
		channels: channels,
		quit:     make(chan struct{}),
		stopped:  0,
	}
}

func (p *Peer) start(pktRecvQueues map[string]chan *InboundPacket, onPeerError func(*Peer, interface{})) {
	log.Debug("Starting %v", p)

	if atomic.CompareAndSwapUint32(&p.started, 0, 1) {
		// on connection error
		onError := func(r interface{}) {
			p.stop()
			onPeerError(p, r)
		}
		p.conn.Start(p.channels, onError)
		for chName, _ := range p.channels {
			chInQueue := pktRecvQueues[chName]
			go p.recvHandler(chName, chInQueue)
			go p.sendHandler(chName)
		}
	}
}

func (p *Peer) stop() {
	if atomic.CompareAndSwapUint32(&p.stopped, 0, 1) {
		log.Debug("Stopping %v", p)
		close(p.quit)
		p.conn.Stop()
	}
}

func (p *Peer) IsOutbound() bool {
	return p.outbound
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

// TrySend returns true if the packet was successfully queued.
// Returning true does not imply that the packet will be sent.
func (p *Peer) TrySend(pkt Packet) bool {
	channel := p.Channel(string(pkt.Channel))
	sendQueue := channel.sendQueue

	if atomic.LoadUint32(&p.stopped) == 1 {
		return false
	}

	select {
	case sendQueue <- pkt:
		log.Debug("SEND %v: %v", p, pkt)
		return true
	default: // buffer full
		log.Debug("FAIL SEND %v: %v", p, pkt)
		return false
	}
}

func (p *Peer) Send(pkt Packet) bool {
	channel := p.Channel(string(pkt.Channel))
	sendQueue := channel.sendQueue

	if atomic.LoadUint32(&p.stopped) == 1 {
		return false
	}

	sendQueue <- pkt
	log.Debug("SEND %v: %v", p, pkt)
	return true
}

func (p *Peer) WriteTo(w io.Writer) (n int64, err error) {
	return p.RemoteAddress().WriteTo(w)
}

func (p *Peer) String() string {
	if p.outbound {
		return fmt.Sprintf("P(->%v)", p.conn)
	} else {
		return fmt.Sprintf("P(%v->)", p.conn)
	}
}

// sendHandler pulls from a channel and pushes to the connection.
// Each channel gets its own sendHandler goroutine;
// Golang's channel implementation handles the scheduling.
func (p *Peer) sendHandler(chName string) {
	// log.Debug("%v sendHandler [%v]", p, chName)
	channel := p.channels[chName]
	sendQueue := channel.sendQueue
FOR_LOOP:
	for {
		select {
		case <-p.quit:
			break FOR_LOOP
		case pkt := <-sendQueue:
			// blocks until the connection is Stop'd,
			// which happens when this peer is Stop'd.
			p.conn.Send(pkt)
		}
	}

	// log.Debug("%v sendHandler [%v] closed", p, chName)
	// Cleanup
}

// recvHandler pulls from a channel and pushes to the given pktRecvQueue.
// Each channel gets its own recvHandler goroutine.
// Many peers have goroutines that push to the same pktRecvQueue.
// Golang's channel implementation handles the scheduling.
func (p *Peer) recvHandler(chName string, pktRecvQueue chan<- *InboundPacket) {
	// log.Debug("%v recvHandler [%v]", p, chName)
	channel := p.channels[chName]
	recvQueue := channel.recvQueue

FOR_LOOP:
	for {
		select {
		case <-p.quit:
			break FOR_LOOP
		case pkt := <-recvQueue:
			// send to pktRecvQueue
			inboundPacket := &InboundPacket{
				Peer:   p,
				Time:   Time{time.Now()},
				Packet: pkt,
			}
			select {
			case <-p.quit:
				break FOR_LOOP
			case pktRecvQueue <- inboundPacket:
				continue
			}
		}
	}

	// log.Debug("%v recvHandler [%v] closed", p, chName)
	// Cleanup
}

//-----------------------------------------------------------------------------

/* ChannelDescriptor */

type ChannelDescriptor struct {
	Name           string
	SendBufferSize int
	RecvBufferSize int
}

/* Channel */

type Channel struct {
	name      string
	recvQueue chan Packet
	sendQueue chan Packet
	//stats           Stats
}

func newChannel(desc ChannelDescriptor) *Channel {
	return &Channel{
		name:      desc.Name,
		recvQueue: make(chan Packet, desc.RecvBufferSize),
		sendQueue: make(chan Packet, desc.SendBufferSize),
	}
}

func (c *Channel) Name() string {
	return c.name
}

func (c *Channel) RecvQueue() <-chan Packet {
	return c.recvQueue
}

func (c *Channel) SendQueue() chan<- Packet {
	return c.sendQueue
}
