package peer

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
)

/* Peer */

type Peer struct {
	outgoing bool
	conn     *Connection
	channels map[String]*Channel

	mtx     sync.Mutex
	quit    chan struct{}
	stopped uint32
}

func NewPeer(conn *Connection) *Peer {
	return &Peer{
		conn:    conn,
		quit:    make(chan struct{}),
		stopped: 0,
	}
}

func (p *Peer) Start(peerRecvQueues map[String]chan *InboundPacket) {
	log.Debugf("Starting %v", p)
	p.conn.Start(p.channels)
	for chName, _ := range p.channels {
		go p.recvHandler(chName, peerRecvQueues[chName])
		go p.sendHandler(chName)
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

func (p *Peer) Channel(chName String) *Channel {
	return p.channels[chName]
}

// If the channel's queue is full, just return false.
// Later the sendHandler will send the pkt to the underlying connection.
func (p *Peer) TrySend(pkt Packet) bool {
	channel := p.Channel(pkt.Channel)
	sendQueue := channel.SendQueue()

	// lock & defer
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if p.stopped == 1 {
		return false
	}
	select {
	case sendQueue <- pkt:
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

func (p *Peer) recvHandler(chName String, inboundPacketQueue chan<- *InboundPacket) {
	log.Tracef("%v recvHandler [%v]", p, chName)
	channel := p.channels[chName]
	recvQueue := channel.RecvQueue()

FOR_LOOP:
	for {
		select {
		case <-p.quit:
			break FOR_LOOP
		case pkt := <-recvQueue:
			// send to inboundPacketQueue
			inboundPacket := &InboundPacket{
				Peer:    p,
				Channel: channel,
				Time:    Time{time.Now()},
				Packet:  pkt,
			}
			select {
			case <-p.quit:
				break FOR_LOOP
			case inboundPacketQueue <- inboundPacket:
				continue
			}
		}
	}

	log.Tracef("%v recvHandler [%v] closed", p, chName)
	// cleanup
	// (none)
}

func (p *Peer) sendHandler(chName String) {
	log.Tracef("%v sendHandler [%v]", p, chName)
	chSendQueue := p.channels[chName].sendQueue
FOR_LOOP:
	for {
		select {
		case <-p.quit:
			break FOR_LOOP
		case pkt := <-chSendQueue:
			log.Tracef("Sending packet to peer chSendQueue")
			// blocks until the connection is Stop'd,
			// which happens when this peer is Stop'd.
			p.conn.Send(pkt)
		}
	}

	log.Tracef("%v sendHandler [%v] closed", p, chName)
	// cleanup
	// (none)
}

/*  Channel */

type Channel struct {
	name      String
	recvQueue chan Packet
	sendQueue chan Packet
	//stats           Stats
}

func NewChannel(name String, bufferSize int) *Channel {
	return &Channel{
		name:      name,
		recvQueue: make(chan Packet, bufferSize),
		sendQueue: make(chan Packet, bufferSize),
	}
}

func (c *Channel) Name() String {
	return c.name
}

func (c *Channel) RecvQueue() <-chan Packet {
	return c.recvQueue
}

func (c *Channel) SendQueue() chan<- Packet {
	return c.sendQueue
}
