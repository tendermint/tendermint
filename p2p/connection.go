package p2p

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
	minReadBufferSize  = 1024
	minWriteBufferSize = 1024
	flushThrottleMS    = 50
	outQueueSize       = 50
	idleTimeoutMinutes = 5
	pingTimeoutMinutes = 2
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
	channels        map[string]*Channel
	onError         func(interface{})
	started         uint32
	stopped         uint32
	errored         uint32
}

const (
	packetTypePing    = UInt8(0x00)
	packetTypePong    = UInt8(0x01)
	packetTypeMessage = UInt8(0x10)
)

func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		sendQueue:       make(chan Packet, outQueueSize),
		conn:            conn,
		bufReader:       bufio.NewReaderSize(conn, minReadBufferSize),
		bufWriter:       bufio.NewWriterSize(conn, minWriteBufferSize),
		flushThrottler:  NewThrottler(flushThrottleMS * time.Millisecond),
		quit:            make(chan struct{}),
		pingRepeatTimer: NewRepeatTimer(pingTimeoutMinutes * time.Minute),
		pong:            make(chan struct{}),
	}
}

// .Start() begins multiplexing packets to and from "channels".
// If an error occurs, the recovered reason is passed to "onError".
func (c *Connection) Start(channels map[string]*Channel, onError func(interface{})) {
	if atomic.CompareAndSwapUint32(&c.started, 0, 1) {
		log.Debug("Starting %v", c)
		c.channels = channels
		c.onError = onError
		go c.sendHandler()
		go c.recvHandler()
	}
}

func (c *Connection) Stop() {
	if atomic.CompareAndSwapUint32(&c.stopped, 0, 1) {
		log.Debug("Stopping %v", c)
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
			log.Warning("Connection flush failed: %v", err)
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
	log.Debug("%v sendHandler", c)
	defer c._recover()

FOR_LOOP:
	for {
		var err error
		select {
		case sendPkt := <-c.sendQueue:
			log.Debug("Found pkt from sendQueue. Writing pkt to underlying connection")
			_, err = packetTypeMessage.WriteTo(c.bufWriter)
			if err != nil {
				break
			}
			_, err = sendPkt.WriteTo(c.bufWriter)
			c.flushThrottler.Set()
		case <-c.flushThrottler.Ch:
			c.flush()
		case <-c.pingRepeatTimer.Ch:
			_, err = packetTypePing.WriteTo(c.bufWriter)
			log.Debug("Send [Ping] -> %v", c)
			c.flush()
		case <-c.pong:
			_, err = packetTypePong.WriteTo(c.bufWriter)
			log.Debug("Send [Pong] -> %v", c)
			c.flush()
		case <-c.quit:
			break FOR_LOOP
		}

		if atomic.LoadUint32(&c.stopped) == 1 {
			break FOR_LOOP
		}
		if err != nil {
			log.Info("%v failed @ sendHandler:\n%v", c, err)
			c.Stop()
			break FOR_LOOP
		}
	}

	log.Debug("%v sendHandler done", c)
	// cleanup
}

// recvHandler reads from .bufReader and pushes to the appropriate
// channel's recvQueue.
func (c *Connection) recvHandler() {
	log.Debug("%v recvHandler", c)
	defer c._recover()

FOR_LOOP:
	for {
		if true {
			// peeking into bufReader
			numBytes := c.bufReader.Buffered()
			bytes, err := c.bufReader.Peek(MinInt(numBytes, 100))
			log.Debug("recvHandler peeked: %X\nerr:%v", bytes, err)
		}
		pktType, err := ReadUInt8Safe(c.bufReader)
		if err != nil {
			if atomic.LoadUint32(&c.stopped) != 1 {
				log.Info("%v failed @ recvHandler", c)
				c.Stop()
			}
			break FOR_LOOP
		} else {
			log.Debug("Found pktType %v", pktType)
		}

		switch pktType {
		case packetTypePing:
			// TODO: keep track of these, make sure it isn't abused
			// as they cause flush()'s in the send buffer.
			c.pong <- struct{}{}
		case packetTypePong:
			// do nothing
			log.Debug("[%v] Received Pong", c)
		case packetTypeMessage:
			pkt, err := ReadPacketSafe(c.bufReader)
			if err != nil {
				if atomic.LoadUint32(&c.stopped) != 1 {
					log.Info("%v failed @ recvHandler", c)
					c.Stop()
				}
				break FOR_LOOP
			}
			channel := c.channels[string(pkt.Channel)]
			if channel == nil {
				Panicf("Unknown channel %v", pkt.Channel)
			}
			channel.recvQueue <- pkt
		default:
			Panicf("Unknown message type %v", pktType)
		}

		c.pingRepeatTimer.Reset()
	}

	log.Debug("%v recvHandler done", c)
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
