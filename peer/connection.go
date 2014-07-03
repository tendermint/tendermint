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
	OUT_QUEUE_SIZE       = 50
	IDLE_TIMEOUT_MINUTES = 5
	PING_TIMEOUT_MINUTES = 2
)

/* Connnection */
type Connection struct {
	ioStats IOStats

	sendQueue     chan Packet // never closes
	conn          net.Conn
	bufWriter     *bufio.Writer
	bufReader     *bufio.Reader
	quit          chan struct{}
	stopped       uint32
	pingDebouncer *Debouncer
	pong          chan struct{}
}

var (
	PACKET_TYPE_PING = UInt8(0x00)
	PACKET_TYPE_PONG = UInt8(0x01)
	PACKET_TYPE_MSG  = UInt8(0x10)
)

func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		sendQueue:     make(chan Packet, OUT_QUEUE_SIZE),
		conn:          conn,
		bufWriter:     bufio.NewWriterSize(conn, 1024),
		bufReader:     bufio.NewReaderSize(conn, 1024),
		quit:          make(chan struct{}),
		pingDebouncer: NewDebouncer(PING_TIMEOUT_MINUTES * time.Minute),
		pong:          make(chan struct{}),
	}
}

// returns true if successfully queued,
// returns false if connection was closed.
// blocks.
func (c *Connection) Send(pkt Packet) bool {
	select {
	case c.sendQueue <- pkt:
		return true
	case <-c.quit:
		return false
	}
}

func (c *Connection) Start(channels map[String]*Channel) {
	log.Debugf("Starting %v", c)
	go c.sendHandler()
	go c.recvHandler(channels)
}

func (c *Connection) Stop() {
	if atomic.CompareAndSwapUint32(&c.stopped, 0, 1) {
		log.Debugf("Stopping %v", c)
		close(c.quit)
		c.conn.Close()
		c.pingDebouncer.Stop()
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

func (c *Connection) String() string {
	return fmt.Sprintf("Connection{%v}", c.conn.RemoteAddr())
}

func (c *Connection) flush() {
	// TODO flush? (turn off nagel, turn back on, etc)
}

func (c *Connection) sendHandler() {
	log.Tracef("%v sendHandler", c)

	// TODO: catch panics & stop connection.

FOR_LOOP:
	for {
		var err error
		select {
		case <-c.pingDebouncer.Ch:
			_, err = PACKET_TYPE_PING.WriteTo(c.bufWriter)
		case sendPkt := <-c.sendQueue:
			log.Tracef("Found pkt from sendQueue. Writing pkt to underlying connection")
			_, err = PACKET_TYPE_MSG.WriteTo(c.bufWriter)
			if err != nil {
				break
			}
			_, err = sendPkt.WriteTo(c.bufWriter)
		case <-c.pong:
			_, err = PACKET_TYPE_PONG.WriteTo(c.bufWriter)
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
		c.flush()
	}

	log.Tracef("%v sendHandler done", c)
	// cleanup
}

func (c *Connection) recvHandler(channels map[String]*Channel) {
	log.Tracef("%v recvHandler with %v channels", c, len(channels))

	// TODO: catch panics & stop connection.

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
		case PACKET_TYPE_PING:
			c.pong <- struct{}{}
		case PACKET_TYPE_PONG:
			// do nothing
		case PACKET_TYPE_MSG:
			pkt, err := ReadPacketSafe(c.bufReader)
			if err != nil {
				if atomic.LoadUint32(&c.stopped) != 1 {
					log.Infof("%v failed @ recvHandler", c)
					c.Stop()
				}
				break FOR_LOOP
			}
			channel := channels[pkt.Channel]
			if channel == nil {
				Panicf("Unknown channel %v", pkt.Channel)
			}
			channel.recvQueue <- pkt
		default:
			Panicf("Unknown message type %v", pktType)
		}

		c.pingDebouncer.Reset()
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
