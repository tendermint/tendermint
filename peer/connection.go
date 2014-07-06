package peer

import (
	"bufio"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

const (
	MinReadBufferSize  = 1024
	MinWriteBufferSize = 1024
	FlushThrottleMS    = 50
	OutQueueSize       = 50
	IdleTimeoutMinutes = 5
	PingTimeoutMinutes = 2
)

/*
A Connection wraps a network connection and handles buffering and multiplexing.
"Packets" are sent with ".Send(Packet)".
Packets received are sent to channels as commanded by the ".Start(...)" method.
*/
type Connection struct {
	ioStats IOStats

	sendQueue       chan Packet // never closes
	conn            net.Conn
	bufReader       *bufio.Reader
	bufWriter       *bufio.Writer
	flushThrottler  *Throttler
	quit            chan struct{}
	pingRepeatTimer *RepeatTimer
	pong            chan struct{}
	channels        map[String]*Channel
	onError         func(interface{})
	started         uint32
	stopped         uint32
	errored         uint32
}

var (
	PacketTypePing    = UInt8(0x00)
	PacketTypePong    = UInt8(0x01)
	PacketTypeMessage = UInt8(0x10)
)

func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		sendQueue:       make(chan Packet, OutQueueSize),
		conn:            conn,
		bufReader:       bufio.NewReaderSize(conn, MinReadBufferSize),
		bufWriter:       bufio.NewWriterSize(conn, MinWriteBufferSize),
		flushThrottler:  NewThrottler(FlushThrottleMS * time.Millisecond),
		quit:            make(chan struct{}),
		pingRepeatTimer: NewRepeatTimer(PingTimeoutMinutes * time.Minute),
		pong:            make(chan struct{}),
	}
}

// .Start() begins multiplexing packets to and from "channels".
// If an error occurs, the recovered reason is passed to "onError".
func (c *Connection) Start(channels map[String]*Channel, onError func(interface{})) {
	log.Debugf("Starting %v", c)
	if atomic.CompareAndSwapUint32(&c.started, 0, 1) {
		c.channels = channels
		c.onError = onError
		go c.sendHandler()
		go c.recvHandler()
	}
}

func (c *Connection) Stop() {
	if atomic.CompareAndSwapUint32(&c.stopped, 0, 1) {
		log.Debugf("Stopping %v", c)
		close(c.quit)
		c.conn.Close()
		c.flushThrottler.Stop()
		c.pingRepeatTimer.Stop()
		// We can't close pong safely here because
		// recvHandler may write to it after we've stopped.
		// Though it doesn't need to get closed at all,
		// we close it @ recvHandler.
		// close(c.pong)
	}
}

func (c *Connection) LocalAddress() *NetAddress {
	return NewNetAddress(c.conn.LocalAddr())
}

func (c *Connection) RemoteAddress() *NetAddress {
	return NewNetAddress(c.conn.RemoteAddr())
}

// Returns true if successfully queued,
// Returns false if connection was closed.
// Blocks.
func (c *Connection) Send(pkt Packet) bool {
	select {
	case c.sendQueue <- pkt:
		return true
	case <-c.quit:
		return false
	}
}

func (c *Connection) String() string {
	return fmt.Sprintf("Connection{%v}", c.conn.RemoteAddr())
}

func (c *Connection) flush() {
	// TODO: this is pretty naive.
	// We end up flushing when we don't have to (yet).
	// A better solution might require us implementing our own buffered writer.
	err := c.bufWriter.Flush()
	if err != nil {
		if atomic.LoadUint32(&c.stopped) != 1 {
			log.Warnf("Connection flush failed: %v", err)
		}
	}
}

// Catch panics, usually caused by remote disconnects.
func (c *Connection) _recover() {
	if r := recover(); r != nil {
		c.Stop()
		if atomic.CompareAndSwapUint32(&c.errored, 0, 1) {
			if c.onError != nil {
				c.onError(r)
			}
		}
	}
}

// sendHandler pulls from .sendQueue and writes to .bufWriter
func (c *Connection) sendHandler() {
	log.Tracef("%v sendHandler", c)
	defer c._recover()

FOR_LOOP:
	for {
		var err error
		select {
		case sendPkt := <-c.sendQueue:
			log.Tracef("Found pkt from sendQueue. Writing pkt to underlying connection")
			_, err = PacketTypeMessage.WriteTo(c.bufWriter)
			if err != nil {
				break
			}
			_, err = sendPkt.WriteTo(c.bufWriter)
			c.flushThrottler.Set()
		case <-c.flushThrottler.Ch:
			c.flush()
		case <-c.pingRepeatTimer.Ch:
			_, err = PacketTypePing.WriteTo(c.bufWriter)
			c.flush()
		case <-c.pong:
			_, err = PacketTypePong.WriteTo(c.bufWriter)
			c.flush()
		case <-c.quit:
			break FOR_LOOP
		}

		if atomic.LoadUint32(&c.stopped) == 1 {
			break FOR_LOOP
		}
		if err != nil {
			log.Infof("%v failed @ sendHandler:\n%v", c, err)
			c.Stop()
			break FOR_LOOP
		}
	}

	log.Tracef("%v sendHandler done", c)
	// cleanup
}

// recvHandler reads from .bufReader and pushes to the appropriate
// channel's recvQueue.
func (c *Connection) recvHandler() {
	log.Tracef("%v recvHandler", c)
	defer c._recover()

FOR_LOOP:
	for {
		pktType, err := ReadUInt8Safe(c.bufReader)
		if err != nil {
			if atomic.LoadUint32(&c.stopped) != 1 {
				log.Infof("%v failed @ recvHandler", c)
				c.Stop()
			}
			break FOR_LOOP
		} else {
			log.Tracef("Found pktType %v", pktType)
		}

		switch pktType {
		case PacketTypePing:
			// TODO: keep track of these, make sure it isn't abused
			// as they cause flush()'s in the send buffer.
			c.pong <- struct{}{}
		case PacketTypePong:
			// do nothing
		case PacketTypeMessage:
			pkt, err := ReadPacketSafe(c.bufReader)
			if err != nil {
				if atomic.LoadUint32(&c.stopped) != 1 {
					log.Infof("%v failed @ recvHandler", c)
					c.Stop()
				}
				break FOR_LOOP
			}
			channel := c.channels[pkt.Channel]
			if channel == nil {
				Panicf("Unknown channel %v", pkt.Channel)
			}
			channel.recvQueue <- pkt
		default:
			Panicf("Unknown message type %v", pktType)
		}

		c.pingRepeatTimer.Reset()
	}

	log.Tracef("%v recvHandler done", c)
	// cleanup
	close(c.pong)
	for _ = range c.pong {
		// drain
	}
}

/* IOStats */
type IOStats struct {
	TimeConnected Time
	LastSent      Time
	LastRecv      Time
	BytesRecv     UInt64
	BytesSent     UInt64
	PktsRecv      UInt64
	PktsSent      UInt64
}
