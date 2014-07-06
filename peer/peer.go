package peer

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
)

/* Peer */

type Peer struct {
	outgoing bool
	conn     *Connection
	channels map[String]*Channel
	quit     chan struct{}
	started  uint32
	stopped  uint32
}

func NewPeer(conn *Connection, channels map[String]*Channel) *Peer {
	return &Peer{
		conn:     conn,
		channels: channels,
		quit:     make(chan struct{}),
		stopped:  0,
	}
}

func (p *Peer) start(pktRecvQueues map[String]chan *InboundPacket, erroredPeers chan peerError) {
	log.Debugf("Starting %v", p)

	if atomic.CompareAndSwapUint32(&p.started, 0, 1) {
		// on connection error
		onError := func(r interface{}) {
			p.stop()
			erroredPeers <- peerError{p, r}
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
		log.Debugf("Stopping %v", p)
		close(p.quit)
		p.conn.Stop()
	}
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

// TrySend returns true if the packet was successfully queued.
// Returning true does not imply that the packet will be sent.
func (p *Peer) TrySend(pkt Packet) bool {
	channel := p.Channel(pkt.Channel)
	sendQueue := channel.sendQueue

	if atomic.LoadUint32(&p.stopped) == 1 {
		return false
	}

	select {
	case sendQueue <- pkt:
		return true
	default: // buffer full
		return false
	}
}

func (p *Peer) WriteTo(w io.Writer) (n int64, err error) {
	return p.RemoteAddress().WriteTo(w)
}

func (p *Peer) String() string {
	return fmt.Sprintf("Peer{%v-%v,o:%v}", p.LocalAddress(), p.RemoteAddress(), p.outgoing)
}

// sendHandler pulls from a channel and pushes to the connection.
// Each channel gets its own sendHandler goroutine;
// Golang's channel implementation handles the scheduling.
func (p *Peer) sendHandler(chName String) {
	log.Tracef("%v sendHandler [%v]", p, chName)
	channel := p.channels[chName]
	sendQueue := channel.sendQueue
FOR_LOOP:
	for {
		select {
		case <-p.quit:
			break FOR_LOOP
		case pkt := <-sendQueue:
			log.Tracef("Sending packet to peer sendQueue")
			// blocks until the connection is Stop'd,
			// which happens when this peer is Stop'd.
			p.conn.Send(pkt)
		}
	}

	log.Tracef("%v sendHandler [%v] closed", p, chName)
	// cleanup
	// (none)
}

// recvHandler pulls from a channel and pushes to the given pktRecvQueue.
// Each channel gets its own recvHandler goroutine.
// Many peers have goroutines that push to the same pktRecvQueue.
// Golang's channel implementation handles the scheduling.
func (p *Peer) recvHandler(chName String, pktRecvQueue chan<- *InboundPacket) {
	log.Tracef("%v recvHandler [%v]", p, chName)
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

	log.Tracef("%v recvHandler [%v] closed", p, chName)
	// cleanup
	// (none)
}

/* Channel */

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

/* Packet */

/*
Packet encapsulates a ByteSlice on a Channel.
*/
type Packet struct {
	Channel String
	Bytes   ByteSlice
	// Hash
}

func NewPacket(chName String, bytes ByteSlice) Packet {
	return Packet{
		Channel: chName,
		Bytes:   bytes,
	}
}

func (p Packet) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(&p.Channel, w, n, err)
	n, err = WriteOnto(&p.Bytes, w, n, err)
	return
}

func ReadPacketSafe(r io.Reader) (pkt Packet, err error) {
	chName, err := ReadStringSafe(r)
	if err != nil {
		return
	}
	// TODO: packet length sanity check.
	bytes, err := ReadByteSliceSafe(r)
	if err != nil {
		return
	}
	return NewPacket(chName, bytes), nil
}

/*
InboundPacket extends Packet with fields relevant to incoming packets.
*/
type InboundPacket struct {
	Peer *Peer
	Time Time
	Packet
}

/* Misc */

type peerError struct {
	peer *Peer
	err  interface{}
}
