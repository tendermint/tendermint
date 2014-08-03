package p2p

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net"
	"sync/atomic"
	"time"

	flow "code.google.com/p/mxk/go1/flowcontrol"
	"github.com/op/go-logging"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

const (
	numBatchPackets    = 10
	minReadBufferSize  = 1024
	minWriteBufferSize = 1024
	flushThrottleMS    = 50
	idleTimeoutMinutes = 5
	updateStatsSeconds = 2
	pingTimeoutMinutes = 2
	defaultSendRate    = 51200 // 5Kb/s
	defaultRecvRate    = 51200 // 5Kb/s
)

/*
A MConnection wraps a network connection and handles buffering and multiplexing.
ByteSlices are sent with ".Send(channelId, bytes)".
Inbound ByteSlices are pushed to the designated chan<- InboundBytes.
*/
type MConnection struct {
	conn         net.Conn
	bufReader    *bufio.Reader
	bufWriter    *bufio.Writer
	sendMonitor  *flow.Monitor
	recvMonitor  *flow.Monitor
	sendRate     int64
	recvRate     int64
	flushTimer   *ThrottleTimer // flush writes as necessary but throttled.
	send         chan struct{}
	quit         chan struct{}
	pingTimer    *RepeatTimer // send pings periodically
	pong         chan struct{}
	chStatsTimer *RepeatTimer // update channel stats periodically
	channels     []*Channel
	channelsIdx  map[byte]*Channel
	onError      func(interface{})
	started      uint32
	stopped      uint32
	errored      uint32

	Peer          *Peer // hacky optimization, gets set by Peer
	LocalAddress  *NetAddress
	RemoteAddress *NetAddress
}

func NewMConnection(conn net.Conn, chDescs []*ChannelDescriptor, onError func(interface{})) *MConnection {

	mconn := &MConnection{
		conn:          conn,
		bufReader:     bufio.NewReaderSize(conn, minReadBufferSize),
		bufWriter:     bufio.NewWriterSize(conn, minWriteBufferSize),
		sendMonitor:   flow.New(0, 0),
		recvMonitor:   flow.New(0, 0),
		sendRate:      defaultSendRate,
		recvRate:      defaultRecvRate,
		flushTimer:    NewThrottleTimer(flushThrottleMS * time.Millisecond),
		send:          make(chan struct{}, 1),
		quit:          make(chan struct{}),
		pingTimer:     NewRepeatTimer(pingTimeoutMinutes * time.Minute),
		pong:          make(chan struct{}),
		chStatsTimer:  NewRepeatTimer(updateStatsSeconds * time.Second),
		onError:       onError,
		LocalAddress:  NewNetAddress(conn.LocalAddr()),
		RemoteAddress: NewNetAddress(conn.RemoteAddr()),
	}

	// Create channels
	var channelsIdx = map[byte]*Channel{}
	var channels = []*Channel{}

	for _, desc := range chDescs {
		channel := newChannel(mconn, desc)
		channelsIdx[channel.id] = channel
		channels = append(channels, channel)
	}
	mconn.channels = channels
	mconn.channelsIdx = channelsIdx

	return mconn
}

// .Start() begins multiplexing packets to and from "channels".
func (c *MConnection) Start() {
	if atomic.CompareAndSwapUint32(&c.started, 0, 1) {
		log.Debug("Starting %v", c)
		go c.sendHandler()
		go c.recvHandler()
	}
}

func (c *MConnection) Stop() {
	if atomic.CompareAndSwapUint32(&c.stopped, 0, 1) {
		log.Debug("Stopping %v", c)
		close(c.quit)
		c.conn.Close()
		c.flushTimer.Stop()
		c.chStatsTimer.Stop()
		c.pingTimer.Stop()
		// We can't close pong safely here because
		// recvHandler may write to it after we've stopped.
		// Though it doesn't need to get closed at all,
		// we close it @ recvHandler.
		// close(c.pong)
	}
}

func (c *MConnection) String() string {
	return fmt.Sprintf("/%v/", c.conn.RemoteAddr())
}

func (c *MConnection) flush() {
	err := c.bufWriter.Flush()
	if err != nil {
		if atomic.LoadUint32(&c.stopped) != 1 {
			log.Warning("MConnection flush failed: %v", err)
		}
	}
}

// Catch panics, usually caused by remote disconnects.
func (c *MConnection) _recover() {
	if r := recover(); r != nil {
		c.stopForError(r)
	}
}

func (c *MConnection) stopForError(r interface{}) {
	c.Stop()
	if atomic.CompareAndSwapUint32(&c.errored, 0, 1) {
		if c.onError != nil {
			c.onError(r)
		}
	}
}

// Queues a message to be sent to channel.
func (c *MConnection) Send(chId byte, bytes ByteSlice) bool {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return false
	}

	// Send message to channel.
	channel, ok := c.channelsIdx[chId]
	if !ok {
		log.Error("Cannot send bytes, unknown channel %X", chId)
		return false
	}

	channel.sendBytes(bytes)

	// Wake up sendHandler if necessary
	select {
	case c.send <- struct{}{}:
	default:
	}

	return true
}

// Queues a message to be sent to channel.
// Nonblocking, returns true if successful.
func (c *MConnection) TrySend(chId byte, bytes ByteSlice) bool {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return false
	}

	// Send message to channel.
	channel, ok := c.channelsIdx[chId]
	if !ok {
		log.Error("Cannot send bytes, unknown channel %X", chId)
		return false
	}

	ok = channel.trySendBytes(bytes)
	if ok {
		// Wake up sendHandler if necessary
		select {
		case c.send <- struct{}{}:
		default:
		}
	}

	return ok
}

func (c *MConnection) CanSend(chId byte) bool {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return false
	}

	channel, ok := c.channelsIdx[chId]
	if !ok {
		log.Error("Unknown channel %X", chId)
		return 0
	}
	return channel.canSend()
}

// sendHandler polls for packets to send from channels.
func (c *MConnection) sendHandler() {
	defer c._recover()

FOR_LOOP:
	for {
		var err error
		select {
		case <-c.flushTimer.Ch:
			// NOTE: flushTimer.Set() must be called every time
			// something is written to .bufWriter.
			c.flush()
		case <-c.chStatsTimer.Ch:
			for _, channel := range c.channels {
				channel.updateStats()
			}
		case <-c.pingTimer.Ch:
			var n int64
			n, err = packetTypePing.WriteTo(c.bufWriter)
			c.sendMonitor.Update(int(n))
			c.flush()
		case <-c.pong:
			var n int64
			n, err = packetTypePong.WriteTo(c.bufWriter)
			c.sendMonitor.Update(int(n))
			c.flush()
		case <-c.quit:
			break FOR_LOOP
		case <-c.send:
			// Send some packets
			eof := c.sendSomePackets()
			if !eof {
				// Keep sendHandler awake.
				select {
				case c.send <- struct{}{}:
				default:
				}
			}
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

	// Cleanup
}

// Returns true if messages from channels were exhausted.
// Blocks in accordance to .sendMonitor throttling.
func (c *MConnection) sendSomePackets() bool {
	// Block until .sendMonitor says we can write.
	// Once we're ready we send more than we asked for,
	// but amortized it should even out.
	c.sendMonitor.Limit(maxPacketSize, atomic.LoadInt64(&c.sendRate), true)

	// Now send some packets.
	for i := 0; i < numBatchPackets; i++ {
		if c.sendPacket() {
			return true
		}
	}
	return false
}

// Returns true if messages from channels were exhausted.
func (c *MConnection) sendPacket() bool {
	// Choose a channel to create a packet from.
	// The chosen channel will be the one whose recentlySent/priority is the least.
	var leastRatio float32 = math.MaxFloat32
	var leastChannel *Channel
	for _, channel := range c.channels {
		// If nothing to send, skip this channel
		if !channel.sendPending() {
			continue
		}
		// Get ratio, and keep track of lowest ratio.
		ratio := float32(channel.recentlySent) / float32(channel.priority)
		if ratio < leastRatio {
			leastRatio = ratio
			leastChannel = channel
		}
	}

	// Nothing to send?
	if leastChannel == nil {
		return true
	} else {
		log.Debug("Found a packet to send")
	}

	// Make & send a packet from this channel
	n, err := leastChannel.writePacketTo(c.bufWriter)
	if err != nil {
		log.Warning("Failed to write packet. Error: %v", err)
		c.stopForError(err)
		return true
	}
	c.sendMonitor.Update(int(n))
	c.flushTimer.Set()
	return false
}

// recvHandler reads packets and reconstructs the message using the channels' "recving" buffer.
// After a whole message has been assembled, it's pushed to the Channel's recvQueue.
// Blocks depending on how the connection is throttled.
func (c *MConnection) recvHandler() {
	defer c._recover()

FOR_LOOP:
	for {
		// Block until .recvMonitor says we can read.
		c.recvMonitor.Limit(maxPacketSize, atomic.LoadInt64(&c.recvRate), true)

		// Read packet type
		pktType, n, err := ReadUInt8Safe(c.bufReader)
		c.recvMonitor.Update(int(n))
		if err != nil {
			if atomic.LoadUint32(&c.stopped) != 1 {
				log.Info("%v failed @ recvHandler with err: %v", c, err)
				c.Stop()
			}
			break FOR_LOOP
		}

		// Peek into bufReader for debugging
		if log.IsEnabledFor(logging.DEBUG) {
			numBytes := c.bufReader.Buffered()
			bytes, err := c.bufReader.Peek(MinInt(numBytes, 100))
			if err != nil {
				log.Debug("recvHandler packet type %X, peeked: %X", pktType, bytes)
			}
		}

		// Read more depending on packet type.
		switch pktType {
		case packetTypePing:
			// TODO: prevent abuse, as they cause flush()'s.
			c.pong <- struct{}{}
		case packetTypePong:
			// do nothing
		case packetTypeMessage:
			pkt, n, err := readPacketSafe(c.bufReader)
			c.recvMonitor.Update(int(n))
			if err != nil {
				if atomic.LoadUint32(&c.stopped) != 1 {
					log.Info("%v failed @ recvHandler", c)
					c.Stop()
				}
				break FOR_LOOP
			}
			channel := c.channels[pkt.ChannelId]
			if channel == nil {
				Panicf("Unknown channel %v", pkt.ChannelId)
			}
			channel.recvPacket(pkt)
		default:
			Panicf("Unknown message type %v", pktType)
		}

		// TODO: shouldn't this go in the sendHandler?
		// Better to send a packet when *we* haven't sent anything for a while.
		c.pingTimer.Reset()
	}

	// Cleanup
	close(c.pong)
	for _ = range c.pong {
		// Drain
	}
}

//-----------------------------------------------------------------------------

type ChannelDescriptor struct {
	Id                byte
	SendQueueCapacity int // One per MConnection.
	RecvQueueCapacity int // Global for this channel.
	RecvBufferSize    int
	DefaultPriority   uint

	// TODO: kinda hacky.
	// This is created by the switch, one per channel.
	recvQueue chan InboundBytes
}

// TODO: lowercase.
// NOTE: not goroutine-safe.
type Channel struct {
	conn          *MConnection
	desc          *ChannelDescriptor
	id            byte
	recvQueue     chan InboundBytes
	sendQueue     chan ByteSlice
	sendQueueSize uint32
	recving       ByteSlice
	sending       ByteSlice
	priority      uint
	recentlySent  int64 // exponential moving average
}

func newChannel(conn *MConnection, desc *ChannelDescriptor) *Channel {
	if desc.DefaultPriority <= 0 {
		panic("Channel default priority must be a postive integer")
	}
	return &Channel{
		conn:      conn,
		desc:      desc,
		id:        desc.Id,
		recvQueue: desc.recvQueue,
		sendQueue: make(chan ByteSlice, desc.SendQueueCapacity),
		recving:   make([]byte, 0, desc.RecvBufferSize),
		priority:  desc.DefaultPriority,
	}
}

// Queues message to send to this channel.
// Goroutine-safe
func (ch *Channel) sendBytes(bytes ByteSlice) {
	ch.sendQueue <- bytes
	atomic.AddUint32(&ch.sendQueueSize, 1)
}

// Queues message to send to this channel.
// Nonblocking, returns true if successful.
// Goroutine-safe
func (ch *Channel) trySendBytes(bytes ByteSlice) bool {
	select {
	case ch.sendQueue <- bytes:
		atomic.AddUint32(&ch.sendQueueSize, 1)
		return true
	default:
		return false
	}
}

// Goroutine-safe
func (ch *Channel) sendQueueSize() (size int) {
	return int(atomic.LoadUint32(&ch.sendQueueSize))
}

// Goroutine-safe
// Use only as a heuristic.
func (ch *Channel) canSend() bool {
	return ch.sendQueueSize() < ch.desc.SendQueueCapacity
}

// Returns true if any packets are pending to be sent.
// Call before calling nextPacket()
// Goroutine-safe
func (ch *Channel) sendPending() bool {
	if len(ch.sending) == 0 {
		if len(ch.sendQueue) == 0 {
			return false
		}
		ch.sending = <-ch.sendQueue
	}
	return true
}

// Creates a new packet to send.
// Not goroutine-safe
func (ch *Channel) nextPacket() packet {
	packet := packet{}
	packet.ChannelId = Byte(ch.id)
	packet.Bytes = ch.sending[:MinInt(maxPacketSize, len(ch.sending))]
	if len(ch.sending) <= maxPacketSize {
		packet.EOF = Byte(0x01)
		ch.sending = nil
		atomic.AddUint32(&ch.sendQueueSize, ^uint32(0)) // decrement sendQueueSize
	} else {
		packet.EOF = Byte(0x00)
		ch.sending = ch.sending[MinInt(maxPacketSize, len(ch.sending)):]
	}
	return packet
}

// Writes next packet to w.
// Not goroutine-safe
func (ch *Channel) writePacketTo(w io.Writer) (n int64, err error) {
	packet := ch.nextPacket()
	n, err = WriteTo(packetTypeMessage, w, n, err)
	n, err = WriteTo(packet, w, n, err)
	if err != nil {
		ch.recentlySent += n
	}
	return
}

// Handles incoming packets.
// Not goroutine-safe
func (ch *Channel) recvPacket(pkt packet) {
	ch.recving = append(ch.recving, pkt.Bytes...)
	if pkt.EOF == Byte(0x01) {
		ch.recvQueue <- InboundBytes{ch.conn, ch.recving}
		ch.recving = make([]byte, 0, ch.desc.RecvBufferSize)
	}
}

// Call this periodically to update stats for throttling purposes.
// Not goroutine-safe
func (ch *Channel) updateStats() {
	// Exponential decay of stats.
	// TODO: optimize.
	ch.recentlySent = int64(float64(ch.recentlySent) * 0.5)
}

//-----------------------------------------------------------------------------

const (
	maxPacketSize     = 1024
	packetTypePing    = UInt8(0x00)
	packetTypePong    = UInt8(0x01)
	packetTypeMessage = UInt8(0x10)
)

// Messages in channels are chopped into smaller packets for multiplexing.
type packet struct {
	ChannelId Byte
	EOF       Byte // 1 means message ends here.
	Bytes     ByteSlice
}

func (p packet) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(p.ChannelId, w, n, err)
	n, err = WriteTo(p.EOF, w, n, err)
	n, err = WriteTo(p.Bytes, w, n, err)
	return
}

func (p packet) String() string {
	return fmt.Sprintf("%v:%X", p.ChannelId, p.Bytes)
}

func readPacketSafe(r io.Reader) (pkt packet, n int64, err error) {
	chId, n_, err := ReadByteSafe(r)
	n += n_
	if err != nil {
		return
	}
	eof, n_, err := ReadByteSafe(r)
	n += n_
	if err != nil {
		return
	}
	// TODO: packet length sanity check.
	bytes, n_, err := ReadByteSliceSafe(r)
	n += n_
	if err != nil {
		return
	}
	return packet{chId, eof, bytes}, n, nil
}

//-----------------------------------------------------------------------------

type InboundBytes struct {
	MConn *MConnection
	Bytes ByteSlice
}

//-----------------------------------------------------------------------------

// Convenience struct for writing typed messages.
// Reading requires a custom decoder that switches on the first type byte of a ByteSlice.
type TypedMessage struct {
	Type Byte
	Msg  Binary
}

func (tm TypedMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(tm.Type, w, n, err)
	n, err = WriteTo(tm.Msg, w, n, err)
	return
}

func (tm TypedMessage) String() string {
	return fmt.Sprintf("<%X:%v>", tm.Type, tm.Msg)
}

func (tm TypedMessage) Bytes() ByteSlice {
	return BinaryBytes(tm)
}
