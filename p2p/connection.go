package p2p

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net"
	"runtime/debug"
	"sync/atomic"
	"time"

	flow "github.com/tendermint/tendermint/Godeps/_workspace/src/code.google.com/p/mxk/go1/flowcontrol"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/wire" //"github.com/tendermint/log15"
)

const (
	numBatchMsgPackets        = 10
	minReadBufferSize         = 1024
	minWriteBufferSize        = 1024
	flushThrottleMS           = 50
	idleTimeoutMinutes        = 5
	updateStatsSeconds        = 2
	pingTimeoutSeconds        = 40
	defaultSendRate           = 51200 // 5Kb/s
	defaultRecvRate           = 51200 // 5Kb/s
	defaultSendQueueCapacity  = 1
	defaultRecvBufferCapacity = 4096
	defaultSendTimeoutSeconds = 10
)

type receiveCbFunc func(chId byte, msgBytes []byte)
type errorCbFunc func(interface{})

/*
Each peer has one `MConnection` (multiplex connection) instance.

__multiplex__ *noun* a system or signal involving simultaneous transmission of
several messages along a single channel of communication.

Each `MConnection` handles message transmission on multiple abstract communication
`Channel`s.  Each channel has a globally unique byte id.
The byte id and the relative priorities of each `Channel` are configured upon
initialization of the connection.

There are two methods for sending messages:
	func (m MConnection) Send(chId byte, msg interface{}) bool {}
	func (m MConnection) TrySend(chId byte, msg interface{}) bool {}

`Send(chId, msg)` is a blocking call that waits until `msg` is successfully queued
for the channel with the given id byte `chId`, or until the request times out.
The message `msg` is serialized using the `tendermint/wire` submodule's
`WriteBinary()` reflection routine.

`TrySend(chId, msg)` is a nonblocking call that returns false if the channel's
queue is full.

Inbound message bytes are handled with an onReceive callback function.
*/
type MConnection struct {
	BaseService

	conn        net.Conn
	bufReader   *bufio.Reader
	bufWriter   *bufio.Writer
	sendMonitor *flow.Monitor
	recvMonitor *flow.Monitor
	sendRate    int64
	recvRate    int64
	send        chan struct{}
	pong        chan struct{}
	channels    []*Channel
	channelsIdx map[byte]*Channel
	onReceive   receiveCbFunc
	onError     errorCbFunc
	errored     uint32

	quit         chan struct{}
	flushTimer   *ThrottleTimer // flush writes as necessary but throttled.
	pingTimer    *RepeatTimer   // send pings periodically
	chStatsTimer *RepeatTimer   // update channel stats periodically

	LocalAddress  *NetAddress
	RemoteAddress *NetAddress
}

func NewMConnection(conn net.Conn, chDescs []*ChannelDescriptor, onReceive receiveCbFunc, onError errorCbFunc) *MConnection {

	mconn := &MConnection{
		conn:        conn,
		bufReader:   bufio.NewReaderSize(conn, minReadBufferSize),
		bufWriter:   bufio.NewWriterSize(conn, minWriteBufferSize),
		sendMonitor: flow.New(0, 0),
		recvMonitor: flow.New(0, 0),
		sendRate:    defaultSendRate,
		recvRate:    defaultRecvRate,
		send:        make(chan struct{}, 1),
		pong:        make(chan struct{}),
		onReceive:   onReceive,
		onError:     onError,

		// Initialized in Start()
		quit:         nil,
		flushTimer:   nil,
		pingTimer:    nil,
		chStatsTimer: nil,

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

	mconn.BaseService = *NewBaseService(log, "MConnection", mconn)

	return mconn
}

func (c *MConnection) OnStart() error {
	c.BaseService.OnStart()
	c.quit = make(chan struct{})
	c.flushTimer = NewThrottleTimer("flush", flushThrottleMS*time.Millisecond)
	c.pingTimer = NewRepeatTimer("ping", pingTimeoutSeconds*time.Second)
	c.chStatsTimer = NewRepeatTimer("chStats", updateStatsSeconds*time.Second)
	go c.sendRoutine()
	go c.recvRoutine()
	return nil
}

func (c *MConnection) OnStop() {
	c.BaseService.OnStop()
	c.flushTimer.Stop()
	c.pingTimer.Stop()
	c.chStatsTimer.Stop()
	if c.quit != nil {
		close(c.quit)
	}
	c.conn.Close()
	// We can't close pong safely here because
	// recvRoutine may write to it after we've stopped.
	// Though it doesn't need to get closed at all,
	// we close it @ recvRoutine.
	// close(c.pong)
}

func (c *MConnection) String() string {
	return fmt.Sprintf("MConn{%v}", c.conn.RemoteAddr())
}

func (c *MConnection) flush() {
	log.Debug("Flush", "conn", c)
	err := c.bufWriter.Flush()
	if err != nil {
		log.Warn("MConnection flush failed", "error", err)
	}
}

// Catch panics, usually caused by remote disconnects.
func (c *MConnection) _recover() {
	if r := recover(); r != nil {
		stack := debug.Stack()
		err := StackError{r, stack}
		c.stopForError(err)
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
func (c *MConnection) Send(chId byte, msg interface{}) bool {
	if !c.IsRunning() {
		return false
	}

	log.Info("Send", "channel", chId, "conn", c, "msg", msg) //, "bytes", wire.BinaryBytes(msg))

	// Send message to channel.
	channel, ok := c.channelsIdx[chId]
	if !ok {
		log.Error(Fmt("Cannot send bytes, unknown channel %X", chId))
		return false
	}

	success := channel.sendBytes(wire.BinaryBytes(msg))
	if success {
		// Wake up sendRoutine if necessary
		select {
		case c.send <- struct{}{}:
		default:
		}
	} else {
		log.Warn("Send failed", "channel", chId, "conn", c, "msg", msg)
	}
	return success
}

// Queues a message to be sent to channel.
// Nonblocking, returns true if successful.
func (c *MConnection) TrySend(chId byte, msg interface{}) bool {
	if !c.IsRunning() {
		return false
	}

	log.Info("TrySend", "channel", chId, "conn", c, "msg", msg)

	// Send message to channel.
	channel, ok := c.channelsIdx[chId]
	if !ok {
		log.Error(Fmt("Cannot send bytes, unknown channel %X", chId))
		return false
	}

	ok = channel.trySendBytes(wire.BinaryBytes(msg))
	if ok {
		// Wake up sendRoutine if necessary
		select {
		case c.send <- struct{}{}:
		default:
		}
	}

	return ok
}

func (c *MConnection) CanSend(chId byte) bool {
	if !c.IsRunning() {
		return false
	}

	channel, ok := c.channelsIdx[chId]
	if !ok {
		log.Error(Fmt("Unknown channel %X", chId))
		return false
	}
	return channel.canSend()
}

// sendRoutine polls for packets to send from channels.
func (c *MConnection) sendRoutine() {
	defer c._recover()

FOR_LOOP:
	for {
		var n int64
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
			log.Info("Send Ping")
			wire.WriteByte(packetTypePing, c.bufWriter, &n, &err)
			c.sendMonitor.Update(int(n))
			c.flush()
		case <-c.pong:
			log.Info("Send Pong")
			wire.WriteByte(packetTypePong, c.bufWriter, &n, &err)
			c.sendMonitor.Update(int(n))
			c.flush()
		case <-c.quit:
			break FOR_LOOP
		case <-c.send:
			// Send some msgPackets
			eof := c.sendSomeMsgPackets()
			if !eof {
				// Keep sendRoutine awake.
				select {
				case c.send <- struct{}{}:
				default:
				}
			}
		}

		if !c.IsRunning() {
			break FOR_LOOP
		}
		if err != nil {
			log.Warn("Connection failed @ sendRoutine", "conn", c, "error", err)
			c.stopForError(err)
			break FOR_LOOP
		}
	}

	// Cleanup
}

// Returns true if messages from channels were exhausted.
// Blocks in accordance to .sendMonitor throttling.
func (c *MConnection) sendSomeMsgPackets() bool {
	// Block until .sendMonitor says we can write.
	// Once we're ready we send more than we asked for,
	// but amortized it should even out.
	c.sendMonitor.Limit(maxMsgPacketSize, atomic.LoadInt64(&c.sendRate), true)

	// Now send some msgPackets.
	for i := 0; i < numBatchMsgPackets; i++ {
		if c.sendMsgPacket() {
			return true
		}
	}
	return false
}

// Returns true if messages from channels were exhausted.
func (c *MConnection) sendMsgPacket() bool {
	// Choose a channel to create a msgPacket from.
	// The chosen channel will be the one whose recentlySent/priority is the least.
	var leastRatio float32 = math.MaxFloat32
	var leastChannel *Channel
	for _, channel := range c.channels {
		// If nothing to send, skip this channel
		if !channel.isSendPending() {
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
		// log.Info("Found a msgPacket to send")
	}

	// Make & send a msgPacket from this channel
	n, err := leastChannel.writeMsgPacketTo(c.bufWriter)
	if err != nil {
		log.Warn("Failed to write msgPacket", "error", err)
		c.stopForError(err)
		return true
	}
	c.sendMonitor.Update(int(n))
	c.flushTimer.Set()
	return false
}

// recvRoutine reads msgPackets and reconstructs the message using the channels' "recving" buffer.
// After a whole message has been assembled, it's pushed to onReceive().
// Blocks depending on how the connection is throttled.
func (c *MConnection) recvRoutine() {
	defer c._recover()

FOR_LOOP:
	for {
		// Block until .recvMonitor says we can read.
		c.recvMonitor.Limit(maxMsgPacketSize, atomic.LoadInt64(&c.recvRate), true)

		/*
			// Peek into bufReader for debugging
			if numBytes := c.bufReader.Buffered(); numBytes > 0 {
				log.Info("Peek connection buffer", "numBytes", numBytes, "bytes", log15.Lazy{func() []byte {
					bytes, err := c.bufReader.Peek(MinInt(numBytes, 100))
					if err == nil {
						return bytes
					} else {
						log.Warn("Error peeking connection buffer", "error", err)
						return nil
					}
				}})
			}
		*/

		// Read packet type
		var n int64
		var err error
		pktType := wire.ReadByte(c.bufReader, &n, &err)
		c.recvMonitor.Update(int(n))
		if err != nil {
			if c.IsRunning() {
				log.Warn("Connection failed @ recvRoutine (reading byte)", "conn", c, "error", err)
				c.stopForError(err)
			}
			break FOR_LOOP
		}

		// Read more depending on packet type.
		switch pktType {
		case packetTypePing:
			// TODO: prevent abuse, as they cause flush()'s.
			log.Info("Receive Ping")
			c.pong <- struct{}{}
		case packetTypePong:
			// do nothing
			log.Info("Receive Pong")
		case packetTypeMsg:
			pkt, n, err := msgPacket{}, int64(0), error(nil)
			wire.ReadBinaryPtr(&pkt, c.bufReader, &n, &err)
			c.recvMonitor.Update(int(n))
			if err != nil {
				if c.IsRunning() {
					log.Warn("Connection failed @ recvRoutine", "conn", c, "error", err)
					c.stopForError(err)
				}
				break FOR_LOOP
			}
			channel, ok := c.channelsIdx[pkt.ChannelId]
			if !ok || channel == nil {
				PanicQ(Fmt("Unknown channel %X", pkt.ChannelId))
			}
			msgBytes, err := channel.recvMsgPacket(pkt)
			if err != nil {
				if c.IsRunning() {
					log.Warn("Connection failed @ recvRoutine", "conn", c, "error", err)
					c.stopForError(err)
				}
				break FOR_LOOP
			}
			if msgBytes != nil {
				log.Debug("Received bytes", "chId", pkt.ChannelId, "msgBytes", msgBytes)
				c.onReceive(pkt.ChannelId, msgBytes)
			}
		default:
			PanicSanity(Fmt("Unknown message type %X", pktType))
		}

		// TODO: shouldn't this go in the sendRoutine?
		// Better to send a ping packet when *we* haven't sent anything for a while.
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
	Id                 byte
	Priority           int
	SendQueueCapacity  int
	RecvBufferCapacity int
}

func (chDesc *ChannelDescriptor) FillDefaults() {
	if chDesc.SendQueueCapacity == 0 {
		chDesc.SendQueueCapacity = defaultSendQueueCapacity
	}
	if chDesc.RecvBufferCapacity == 0 {
		chDesc.RecvBufferCapacity = defaultRecvBufferCapacity
	}
}

// TODO: lowercase.
// NOTE: not goroutine-safe.
type Channel struct {
	conn          *MConnection
	desc          *ChannelDescriptor
	id            byte
	sendQueue     chan []byte
	sendQueueSize int32 // atomic.
	recving       []byte
	sending       []byte
	priority      int
	recentlySent  int64 // exponential moving average
}

func newChannel(conn *MConnection, desc *ChannelDescriptor) *Channel {
	desc.FillDefaults()
	if desc.Priority <= 0 {
		PanicSanity("Channel default priority must be a postive integer")
	}
	return &Channel{
		conn:      conn,
		desc:      desc,
		id:        desc.Id,
		sendQueue: make(chan []byte, desc.SendQueueCapacity),
		recving:   make([]byte, 0, desc.RecvBufferCapacity),
		priority:  desc.Priority,
	}
}

// Queues message to send to this channel.
// Goroutine-safe
// Times out (and returns false) after defaultSendTimeoutSeconds
func (ch *Channel) sendBytes(bytes []byte) bool {
	timeout := time.NewTimer(defaultSendTimeoutSeconds * time.Second)
	select {
	case <-timeout.C:
		// timeout
		return false
	case ch.sendQueue <- bytes:
		atomic.AddInt32(&ch.sendQueueSize, 1)
		return true
	}
}

// Queues message to send to this channel.
// Nonblocking, returns true if successful.
// Goroutine-safe
func (ch *Channel) trySendBytes(bytes []byte) bool {
	select {
	case ch.sendQueue <- bytes:
		atomic.AddInt32(&ch.sendQueueSize, 1)
		return true
	default:
		return false
	}
}

// Goroutine-safe
func (ch *Channel) loadSendQueueSize() (size int) {
	return int(atomic.LoadInt32(&ch.sendQueueSize))
}

// Goroutine-safe
// Use only as a heuristic.
func (ch *Channel) canSend() bool {
	return ch.loadSendQueueSize() < defaultSendQueueCapacity
}

// Returns true if any msgPackets are pending to be sent.
// Call before calling nextMsgPacket()
// Goroutine-safe
func (ch *Channel) isSendPending() bool {
	if len(ch.sending) == 0 {
		if len(ch.sendQueue) == 0 {
			return false
		}
		ch.sending = <-ch.sendQueue
	}
	return true
}

// Creates a new msgPacket to send.
// Not goroutine-safe
func (ch *Channel) nextMsgPacket() msgPacket {
	packet := msgPacket{}
	packet.ChannelId = byte(ch.id)
	packet.Bytes = ch.sending[:MinInt(maxMsgPacketSize, len(ch.sending))]
	if len(ch.sending) <= maxMsgPacketSize {
		packet.EOF = byte(0x01)
		ch.sending = nil
		atomic.AddInt32(&ch.sendQueueSize, -1) // decrement sendQueueSize
	} else {
		packet.EOF = byte(0x00)
		ch.sending = ch.sending[MinInt(maxMsgPacketSize, len(ch.sending)):]
	}
	return packet
}

// Writes next msgPacket to w.
// Not goroutine-safe
func (ch *Channel) writeMsgPacketTo(w io.Writer) (n int64, err error) {
	packet := ch.nextMsgPacket()
	log.Debug("Write Msg Packet", "conn", ch.conn, "packet", packet)
	wire.WriteByte(packetTypeMsg, w, &n, &err)
	wire.WriteBinary(packet, w, &n, &err)
	if err != nil {
		ch.recentlySent += n
	}
	return
}

// Handles incoming msgPackets. Returns a msg bytes if msg is complete.
// Not goroutine-safe
func (ch *Channel) recvMsgPacket(packet msgPacket) ([]byte, error) {
	log.Debug("Read Msg Packet", "conn", ch.conn, "packet", packet)
	if wire.MaxBinaryReadSize < len(ch.recving)+len(packet.Bytes) {
		return nil, wire.ErrBinaryReadSizeOverflow
	}
	ch.recving = append(ch.recving, packet.Bytes...)
	if packet.EOF == byte(0x01) {
		msgBytes := ch.recving
		ch.recving = make([]byte, 0, defaultRecvBufferCapacity)
		return msgBytes, nil
	}
	return nil, nil
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
	maxMsgPacketSize = 1024
	packetTypePing   = byte(0x01)
	packetTypePong   = byte(0x02)
	packetTypeMsg    = byte(0x03)
)

// Messages in channels are chopped into smaller msgPackets for multiplexing.
type msgPacket struct {
	ChannelId byte
	EOF       byte // 1 means message ends here.
	Bytes     []byte
}

func (p msgPacket) String() string {
	return fmt.Sprintf("MsgPacket{%X:%X T:%X}", p.ChannelId, p.Bytes, p.EOF)
}

//-----------------------------------------------------------------------------

// Convenience struct for writing typed messages.
// Reading requires a custom decoder that switches on the first type byte of a byteslice.
type TypedMessage struct {
	Type byte
	Msg  interface{}
}

func (tm TypedMessage) String() string {
	return fmt.Sprintf("TMsg{%X:%v}", tm.Type, tm.Msg)
}
