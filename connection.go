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

	cmn "github.com/tendermint/go-common"
	flow "github.com/tendermint/go-flowrate/flowrate"
	wire "github.com/tendermint/go-wire"
)

const (
	numBatchMsgPackets = 10
	minReadBufferSize  = 1024
	minWriteBufferSize = 65536
	updateState        = 2 * time.Second
	pingTimeout        = 40 * time.Second
	flushThrottle      = 100 * time.Millisecond

	defaultSendQueueCapacity   = 1
	defaultSendRate            = int64(512000) // 500KB/s
	defaultRecvBufferCapacity  = 4096
	defaultRecvMessageCapacity = 22020096      // 21MB
	defaultRecvRate            = int64(512000) // 500KB/s
	defaultSendTimeout         = 10 * time.Second
)

type receiveCbFunc func(chID byte, msgBytes []byte)
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
	func (m MConnection) Send(chID byte, msg interface{}) bool {}
	func (m MConnection) TrySend(chID byte, msg interface{}) bool {}

`Send(chID, msg)` is a blocking call that waits until `msg` is successfully queued
for the channel with the given id byte `chID`, or until the request times out.
The message `msg` is serialized using the `tendermint/wire` submodule's
`WriteBinary()` reflection routine.

`TrySend(chID, msg)` is a nonblocking call that returns false if the channel's
queue is full.

Inbound message bytes are handled with an onReceive callback function.
*/
type MConnection struct {
	cmn.BaseService

	conn        net.Conn
	bufReader   *bufio.Reader
	bufWriter   *bufio.Writer
	sendMonitor *flow.Monitor
	recvMonitor *flow.Monitor
	send        chan struct{}
	pong        chan struct{}
	channels    []*Channel
	channelsIdx map[byte]*Channel
	onReceive   receiveCbFunc
	onError     errorCbFunc
	errored     uint32
	config      *MConnConfig

	quit         chan struct{}
	flushTimer   *cmn.ThrottleTimer // flush writes as necessary but throttled.
	pingTimer    *cmn.RepeatTimer   // send pings periodically
	chStatsTimer *cmn.RepeatTimer   // update channel stats periodically

	LocalAddress  *NetAddress
	RemoteAddress *NetAddress
}

// MConnConfig is a MConnection configuration.
type MConnConfig struct {
	SendRate int64
	RecvRate int64
}

// DefaultMConnConfig returns the default config.
func DefaultMConnConfig() *MConnConfig {
	return &MConnConfig{
		SendRate: defaultSendRate,
		RecvRate: defaultRecvRate,
	}
}

// NewMConnection wraps net.Conn and creates multiplex connection
func NewMConnection(conn net.Conn, chDescs []*ChannelDescriptor, onReceive receiveCbFunc, onError errorCbFunc) *MConnection {
	return NewMConnectionWithConfig(
		conn,
		chDescs,
		onReceive,
		onError,
		DefaultMConnConfig())
}

// NewMConnectionWithConfig wraps net.Conn and creates multiplex connection with a config
func NewMConnectionWithConfig(conn net.Conn, chDescs []*ChannelDescriptor, onReceive receiveCbFunc, onError errorCbFunc, config *MConnConfig) *MConnection {
	mconn := &MConnection{
		conn:        conn,
		bufReader:   bufio.NewReaderSize(conn, minReadBufferSize),
		bufWriter:   bufio.NewWriterSize(conn, minWriteBufferSize),
		sendMonitor: flow.New(0, 0),
		recvMonitor: flow.New(0, 0),
		send:        make(chan struct{}, 1),
		pong:        make(chan struct{}),
		onReceive:   onReceive,
		onError:     onError,
		config:      config,

		LocalAddress:  NewNetAddress(conn.LocalAddr()),
		RemoteAddress: NewNetAddress(conn.RemoteAddr()),
	}

	// Create channels
	var channelsIdx = map[byte]*Channel{}
	var channels = []*Channel{}

	for _, desc := range chDescs {
		descCopy := *desc // copy the desc else unsafe access across connections
		channel := newChannel(mconn, &descCopy)
		channelsIdx[channel.id] = channel
		channels = append(channels, channel)
	}
	mconn.channels = channels
	mconn.channelsIdx = channelsIdx

	mconn.BaseService = *cmn.NewBaseService(log, "MConnection", mconn)

	return mconn
}

func (c *MConnection) OnStart() error {
	c.BaseService.OnStart()
	c.quit = make(chan struct{})
	c.flushTimer = cmn.NewThrottleTimer("flush", flushThrottle)
	c.pingTimer = cmn.NewRepeatTimer("ping", pingTimeout)
	c.chStatsTimer = cmn.NewRepeatTimer("chStats", updateState)
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
		err := cmn.StackError{r, stack}
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
func (c *MConnection) Send(chID byte, msg interface{}) bool {
	if !c.IsRunning() {
		return false
	}

	log.Debug("Send", "channel", chID, "conn", c, "msg", msg) //, "bytes", wire.BinaryBytes(msg))

	// Send message to channel.
	channel, ok := c.channelsIdx[chID]
	if !ok {
		log.Error(cmn.Fmt("Cannot send bytes, unknown channel %X", chID))
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
		log.Warn("Send failed", "channel", chID, "conn", c, "msg", msg)
	}
	return success
}

// Queues a message to be sent to channel.
// Nonblocking, returns true if successful.
func (c *MConnection) TrySend(chID byte, msg interface{}) bool {
	if !c.IsRunning() {
		return false
	}

	log.Debug("TrySend", "channel", chID, "conn", c, "msg", msg)

	// Send message to channel.
	channel, ok := c.channelsIdx[chID]
	if !ok {
		log.Error(cmn.Fmt("Cannot send bytes, unknown channel %X", chID))
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

// CanSend returns true if you can send more data onto the chID, false
// otherwise. Use only as a heuristic.
func (c *MConnection) CanSend(chID byte) bool {
	if !c.IsRunning() {
		return false
	}

	channel, ok := c.channelsIdx[chID]
	if !ok {
		log.Error(cmn.Fmt("Unknown channel %X", chID))
		return false
	}
	return channel.canSend()
}

// sendRoutine polls for packets to send from channels.
func (c *MConnection) sendRoutine() {
	defer c._recover()

FOR_LOOP:
	for {
		var n int
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
			log.Debug("Send Ping")
			wire.WriteByte(packetTypePing, c.bufWriter, &n, &err)
			c.sendMonitor.Update(int(n))
			c.flush()
		case <-c.pong:
			log.Debug("Send Pong")
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
	c.sendMonitor.Limit(maxMsgPacketTotalSize, atomic.LoadInt64(&c.config.SendRate), true)

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
		c.recvMonitor.Limit(maxMsgPacketTotalSize, atomic.LoadInt64(&c.config.RecvRate), true)

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
		var n int
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
			log.Debug("Receive Ping")
			c.pong <- struct{}{}
		case packetTypePong:
			// do nothing
			log.Debug("Receive Pong")
		case packetTypeMsg:
			pkt, n, err := msgPacket{}, int(0), error(nil)
			wire.ReadBinaryPtr(&pkt, c.bufReader, maxMsgPacketTotalSize, &n, &err)
			c.recvMonitor.Update(int(n))
			if err != nil {
				if c.IsRunning() {
					log.Warn("Connection failed @ recvRoutine", "conn", c, "error", err)
					c.stopForError(err)
				}
				break FOR_LOOP
			}
			channel, ok := c.channelsIdx[pkt.ChannelID]
			if !ok || channel == nil {
				cmn.PanicQ(cmn.Fmt("Unknown channel %X", pkt.ChannelID))
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
				log.Debug("Received bytes", "chID", pkt.ChannelID, "msgBytes", msgBytes)
				c.onReceive(pkt.ChannelID, msgBytes)
			}
		default:
			cmn.PanicSanity(cmn.Fmt("Unknown message type %X", pktType))
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

type ConnectionStatus struct {
	SendMonitor flow.Status
	RecvMonitor flow.Status
	Channels    []ChannelStatus
}

type ChannelStatus struct {
	ID                byte
	SendQueueCapacity int
	SendQueueSize     int
	Priority          int
	RecentlySent      int64
}

func (c *MConnection) Status() ConnectionStatus {
	var status ConnectionStatus
	status.SendMonitor = c.sendMonitor.Status()
	status.RecvMonitor = c.recvMonitor.Status()
	status.Channels = make([]ChannelStatus, len(c.channels))
	for i, channel := range c.channels {
		status.Channels[i] = ChannelStatus{
			ID:                channel.id,
			SendQueueCapacity: cap(channel.sendQueue),
			SendQueueSize:     int(channel.sendQueueSize), // TODO use atomic
			Priority:          channel.priority,
			RecentlySent:      channel.recentlySent,
		}
	}
	return status
}

//-----------------------------------------------------------------------------

type ChannelDescriptor struct {
	ID                  byte
	Priority            int
	SendQueueCapacity   int
	RecvBufferCapacity  int
	RecvMessageCapacity int
}

func (chDesc *ChannelDescriptor) FillDefaults() {
	if chDesc.SendQueueCapacity == 0 {
		chDesc.SendQueueCapacity = defaultSendQueueCapacity
	}
	if chDesc.RecvBufferCapacity == 0 {
		chDesc.RecvBufferCapacity = defaultRecvBufferCapacity
	}
	if chDesc.RecvMessageCapacity == 0 {
		chDesc.RecvMessageCapacity = defaultRecvMessageCapacity
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
		cmn.PanicSanity("Channel default priority must be a postive integer")
	}
	return &Channel{
		conn:      conn,
		desc:      desc,
		id:        desc.ID,
		sendQueue: make(chan []byte, desc.SendQueueCapacity),
		recving:   make([]byte, 0, desc.RecvBufferCapacity),
		priority:  desc.Priority,
	}
}

// Queues message to send to this channel.
// Goroutine-safe
// Times out (and returns false) after defaultSendTimeout
func (ch *Channel) sendBytes(bytes []byte) bool {
	select {
	case ch.sendQueue <- bytes:
		atomic.AddInt32(&ch.sendQueueSize, 1)
		return true
	case <-time.After(defaultSendTimeout):
		return false
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
	packet.ChannelID = byte(ch.id)
	packet.Bytes = ch.sending[:cmn.MinInt(maxMsgPacketPayloadSize, len(ch.sending))]
	if len(ch.sending) <= maxMsgPacketPayloadSize {
		packet.EOF = byte(0x01)
		ch.sending = nil
		atomic.AddInt32(&ch.sendQueueSize, -1) // decrement sendQueueSize
	} else {
		packet.EOF = byte(0x00)
		ch.sending = ch.sending[cmn.MinInt(maxMsgPacketPayloadSize, len(ch.sending)):]
	}
	return packet
}

// Writes next msgPacket to w.
// Not goroutine-safe
func (ch *Channel) writeMsgPacketTo(w io.Writer) (n int, err error) {
	packet := ch.nextMsgPacket()
	log.Debug("Write Msg Packet", "conn", ch.conn, "packet", packet)
	wire.WriteByte(packetTypeMsg, w, &n, &err)
	wire.WriteBinary(packet, w, &n, &err)
	if err == nil {
		ch.recentlySent += int64(n)
	}
	return
}

// Handles incoming msgPackets. Returns a msg bytes if msg is complete.
// Not goroutine-safe
func (ch *Channel) recvMsgPacket(packet msgPacket) ([]byte, error) {
	// log.Debug("Read Msg Packet", "conn", ch.conn, "packet", packet)
	if ch.desc.RecvMessageCapacity < len(ch.recving)+len(packet.Bytes) {
		return nil, wire.ErrBinaryReadOverflow
	}
	ch.recving = append(ch.recving, packet.Bytes...)
	if packet.EOF == byte(0x01) {
		msgBytes := ch.recving
		// clear the slice without re-allocating.
		// http://stackoverflow.com/questions/16971741/how-do-you-clear-a-slice-in-go
		//   suggests this could be a memory leak, but we might as well keep the memory for the channel until it closes,
		//	at which point the recving slice stops being used and should be garbage collected
		ch.recving = ch.recving[:0] // make([]byte, 0, ch.desc.RecvBufferCapacity)
		return msgBytes, nil
	}
	return nil, nil
}

// Call this periodically to update stats for throttling purposes.
// Not goroutine-safe
func (ch *Channel) updateStats() {
	// Exponential decay of stats.
	// TODO: optimize.
	ch.recentlySent = int64(float64(ch.recentlySent) * 0.8)
}

//-----------------------------------------------------------------------------

const (
	maxMsgPacketPayloadSize  = 1024
	maxMsgPacketOverheadSize = 10 // It's actually lower but good enough
	maxMsgPacketTotalSize    = maxMsgPacketPayloadSize + maxMsgPacketOverheadSize
	packetTypePing           = byte(0x01)
	packetTypePong           = byte(0x02)
	packetTypeMsg            = byte(0x03)
)

// Messages in channels are chopped into smaller msgPackets for multiplexing.
type msgPacket struct {
	ChannelID byte
	EOF       byte // 1 means message ends here.
	Bytes     []byte
}

func (p msgPacket) String() string {
	return fmt.Sprintf("MsgPacket{%X:%X T:%X}", p.ChannelID, p.Bytes, p.EOF)
}
