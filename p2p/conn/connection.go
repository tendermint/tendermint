package conn

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"sync/atomic"
	"time"

	amino "github.com/tendermint/go-amino"
	cmn "github.com/tendermint/tendermint/libs/common"
	flow "github.com/tendermint/tendermint/libs/flowrate"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	defaultMaxPacketMsgPayloadSize = 1024

	numBatchPacketMsgs = 10
	minReadBufferSize  = 1024
	minWriteBufferSize = 65536
	updateStats        = 2 * time.Second

	// some of these defaults are written in the user config
	// flushThrottle, sendRate, recvRate
	// TODO: remove values present in config
	defaultFlushThrottle = 100 * time.Millisecond

	defaultSendQueueCapacity   = 1
	defaultRecvBufferCapacity  = 4096
	defaultRecvMessageCapacity = 22020096      // 21MB
	defaultSendRate            = int64(512000) // 500KB/s
	defaultRecvRate            = int64(512000) // 500KB/s
	defaultSendTimeout         = 10 * time.Second
	defaultPingInterval        = 60 * time.Second
	defaultPongTimeout         = 45 * time.Second
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
	func (m MConnection) Send(chID byte, msgBytes []byte) bool {}
	func (m MConnection) TrySend(chID byte, msgBytes []byte}) bool {}

`Send(chID, msgBytes)` is a blocking call that waits until `msg` is
successfully queued for the channel with the given id byte `chID`, or until the
request times out.  The message `msg` is serialized using Go-Amino.

`TrySend(chID, msgBytes)` is a nonblocking call that returns false if the
channel's queue is full.

Inbound message bytes are handled with an onReceive callback function.
*/
type MConnection struct {
	cmn.BaseService

	conn          net.Conn
	bufConnReader *bufio.Reader
	bufConnWriter *bufio.Writer
	sendMonitor   *flow.Monitor
	recvMonitor   *flow.Monitor
	send          chan struct{}
	pong          chan struct{}
	channels      []*Channel
	channelsIdx   map[byte]*Channel
	onReceive     receiveCbFunc
	onError       errorCbFunc
	errored       uint32
	config        MConnConfig

	quit       chan struct{}
	flushTimer *cmn.ThrottleTimer // flush writes as necessary but throttled.
	pingTimer  *cmn.RepeatTimer   // send pings periodically

	// close conn if pong is not received in pongTimeout
	pongTimer     *time.Timer
	pongTimeoutCh chan bool // true - timeout, false - peer sent pong

	chStatsTimer *cmn.RepeatTimer // update channel stats periodically

	created time.Time // time of creation

	_maxPacketMsgSize int
}

// MConnConfig is a MConnection configuration.
type MConnConfig struct {
	SendRate int64 `mapstructure:"send_rate"`
	RecvRate int64 `mapstructure:"recv_rate"`

	// Maximum payload size
	MaxPacketMsgPayloadSize int `mapstructure:"max_packet_msg_payload_size"`

	// Interval to flush writes (throttled)
	FlushThrottle time.Duration `mapstructure:"flush_throttle"`

	// Interval to send pings
	PingInterval time.Duration `mapstructure:"ping_interval"`

	// Maximum wait time for pongs
	PongTimeout time.Duration `mapstructure:"pong_timeout"`
}

// DefaultMConnConfig returns the default config.
func DefaultMConnConfig() MConnConfig {
	return MConnConfig{
		SendRate:                defaultSendRate,
		RecvRate:                defaultRecvRate,
		MaxPacketMsgPayloadSize: defaultMaxPacketMsgPayloadSize,
		FlushThrottle:           defaultFlushThrottle,
		PingInterval:            defaultPingInterval,
		PongTimeout:             defaultPongTimeout,
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
func NewMConnectionWithConfig(conn net.Conn, chDescs []*ChannelDescriptor, onReceive receiveCbFunc, onError errorCbFunc, config MConnConfig) *MConnection {
	if config.PongTimeout >= config.PingInterval {
		panic("pongTimeout must be less than pingInterval (otherwise, next ping will reset pong timer)")
	}

	mconn := &MConnection{
		conn:          conn,
		bufConnReader: bufio.NewReaderSize(conn, minReadBufferSize),
		bufConnWriter: bufio.NewWriterSize(conn, minWriteBufferSize),
		sendMonitor:   flow.New(0, 0),
		recvMonitor:   flow.New(0, 0),
		send:          make(chan struct{}, 1),
		pong:          make(chan struct{}, 1),
		onReceive:     onReceive,
		onError:       onError,
		config:        config,
	}

	// Create channels
	var channelsIdx = map[byte]*Channel{}
	var channels = []*Channel{}

	for _, desc := range chDescs {
		channel := newChannel(mconn, *desc)
		channelsIdx[channel.desc.ID] = channel
		channels = append(channels, channel)
	}
	mconn.channels = channels
	mconn.channelsIdx = channelsIdx

	mconn.BaseService = *cmn.NewBaseService(nil, "MConnection", mconn)

	// maxPacketMsgSize() is a bit heavy, so call just once
	mconn._maxPacketMsgSize = mconn.maxPacketMsgSize()

	return mconn
}

func (c *MConnection) SetLogger(l log.Logger) {
	c.BaseService.SetLogger(l)
	for _, ch := range c.channels {
		ch.SetLogger(l)
	}
}

// OnStart implements BaseService
func (c *MConnection) OnStart() error {
	if err := c.BaseService.OnStart(); err != nil {
		return err
	}
	c.quit = make(chan struct{})
	c.flushTimer = cmn.NewThrottleTimer("flush", c.config.FlushThrottle)
	c.pingTimer = cmn.NewRepeatTimer("ping", c.config.PingInterval)
	c.pongTimeoutCh = make(chan bool, 1)
	c.chStatsTimer = cmn.NewRepeatTimer("chStats", updateStats)
	go c.sendRoutine()
	go c.recvRoutine()
	return nil
}

// OnStop implements BaseService
func (c *MConnection) OnStop() {
	c.BaseService.OnStop()
	c.flushTimer.Stop()
	c.pingTimer.Stop()
	c.chStatsTimer.Stop()
	if c.quit != nil {
		close(c.quit)
	}
	c.conn.Close() // nolint: errcheck

	// We can't close pong safely here because
	// recvRoutine may write to it after we've stopped.
	// Though it doesn't need to get closed at all,
	// we close it @ recvRoutine.
}

func (c *MConnection) String() string {
	return fmt.Sprintf("MConn{%v}", c.conn.RemoteAddr())
}

func (c *MConnection) flush() {
	c.Logger.Debug("Flush", "conn", c)
	err := c.bufConnWriter.Flush()
	if err != nil {
		c.Logger.Error("MConnection flush failed", "err", err)
	}
}

// Catch panics, usually caused by remote disconnects.
func (c *MConnection) _recover() {
	if r := recover(); r != nil {
		err := cmn.ErrorWrap(r, "recovered panic in MConnection")
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
func (c *MConnection) Send(chID byte, msgBytes []byte) bool {
	if !c.IsRunning() {
		return false
	}

	c.Logger.Debug("Send", "channel", chID, "conn", c, "msgBytes", fmt.Sprintf("%X", msgBytes))

	// Send message to channel.
	channel, ok := c.channelsIdx[chID]
	if !ok {
		c.Logger.Error(fmt.Sprintf("Cannot send bytes, unknown channel %X", chID))
		return false
	}

	success := channel.sendBytes(msgBytes)
	if success {
		// Wake up sendRoutine if necessary
		select {
		case c.send <- struct{}{}:
		default:
		}
	} else {
		c.Logger.Error("Send failed", "channel", chID, "conn", c, "msgBytes", fmt.Sprintf("%X", msgBytes))
	}
	return success
}

// Queues a message to be sent to channel.
// Nonblocking, returns true if successful.
func (c *MConnection) TrySend(chID byte, msgBytes []byte) bool {
	if !c.IsRunning() {
		return false
	}

	c.Logger.Debug("TrySend", "channel", chID, "conn", c, "msgBytes", fmt.Sprintf("%X", msgBytes))

	// Send message to channel.
	channel, ok := c.channelsIdx[chID]
	if !ok {
		c.Logger.Error(fmt.Sprintf("Cannot send bytes, unknown channel %X", chID))
		return false
	}

	ok = channel.trySendBytes(msgBytes)
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
		c.Logger.Error(fmt.Sprintf("Unknown channel %X", chID))
		return false
	}
	return channel.canSend()
}

// sendRoutine polls for packets to send from channels.
func (c *MConnection) sendRoutine() {
	defer c._recover()

FOR_LOOP:
	for {
		var _n int64
		var err error
	SELECTION:
		select {
		case <-c.flushTimer.Ch:
			// NOTE: flushTimer.Set() must be called every time
			// something is written to .bufConnWriter.
			c.flush()
		case <-c.chStatsTimer.Chan():
			for _, channel := range c.channels {
				channel.updateStats()
			}
		case <-c.pingTimer.Chan():
			c.Logger.Debug("Send Ping")
			_n, err = cdc.MarshalBinaryWriter(c.bufConnWriter, PacketPing{})
			if err != nil {
				break SELECTION
			}
			c.sendMonitor.Update(int(_n))
			c.Logger.Debug("Starting pong timer", "dur", c.config.PongTimeout)
			c.pongTimer = time.AfterFunc(c.config.PongTimeout, func() {
				select {
				case c.pongTimeoutCh <- true:
				default:
				}
			})
			c.flush()
		case timeout := <-c.pongTimeoutCh:
			if timeout {
				c.Logger.Debug("Pong timeout")
				err = errors.New("pong timeout")
			} else {
				c.stopPongTimer()
			}
		case <-c.pong:
			c.Logger.Debug("Send Pong")
			_n, err = cdc.MarshalBinaryWriter(c.bufConnWriter, PacketPong{})
			if err != nil {
				break SELECTION
			}
			c.sendMonitor.Update(int(_n))
			c.flush()
		case <-c.quit:
			break FOR_LOOP
		case <-c.send:
			// Send some PacketMsgs
			eof := c.sendSomePacketMsgs()
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
			c.Logger.Error("Connection failed @ sendRoutine", "conn", c, "err", err)
			c.stopForError(err)
			break FOR_LOOP
		}
	}

	// Cleanup
	c.stopPongTimer()
}

// Returns true if messages from channels were exhausted.
// Blocks in accordance to .sendMonitor throttling.
func (c *MConnection) sendSomePacketMsgs() bool {
	// Block until .sendMonitor says we can write.
	// Once we're ready we send more than we asked for,
	// but amortized it should even out.
	c.sendMonitor.Limit(c._maxPacketMsgSize, atomic.LoadInt64(&c.config.SendRate), true)

	// Now send some PacketMsgs.
	for i := 0; i < numBatchPacketMsgs; i++ {
		if c.sendPacketMsg() {
			return true
		}
	}
	return false
}

// Returns true if messages from channels were exhausted.
func (c *MConnection) sendPacketMsg() bool {
	// Choose a channel to create a PacketMsg from.
	// The chosen channel will be the one whose recentlySent/priority is the least.
	var leastRatio float32 = math.MaxFloat32
	var leastChannel *Channel
	for _, channel := range c.channels {
		// If nothing to send, skip this channel
		if !channel.isSendPending() {
			continue
		}
		// Get ratio, and keep track of lowest ratio.
		ratio := float32(channel.recentlySent) / float32(channel.desc.Priority)
		if ratio < leastRatio {
			leastRatio = ratio
			leastChannel = channel
		}
	}

	// Nothing to send?
	if leastChannel == nil {
		return true
	}
	// c.Logger.Info("Found a msgPacket to send")

	// Make & send a PacketMsg from this channel
	_n, err := leastChannel.writePacketMsgTo(c.bufConnWriter)
	if err != nil {
		c.Logger.Error("Failed to write PacketMsg", "err", err)
		c.stopForError(err)
		return true
	}
	c.sendMonitor.Update(int(_n))
	c.flushTimer.Set()
	return false
}

// recvRoutine reads PacketMsgs and reconstructs the message using the channels' "recving" buffer.
// After a whole message has been assembled, it's pushed to onReceive().
// Blocks depending on how the connection is throttled.
// Otherwise, it never blocks.
func (c *MConnection) recvRoutine() {
	defer c._recover()

FOR_LOOP:
	for {
		// Block until .recvMonitor says we can read.
		c.recvMonitor.Limit(c._maxPacketMsgSize, atomic.LoadInt64(&c.config.RecvRate), true)

		// Peek into bufConnReader for debugging
		/*
			if numBytes := c.bufConnReader.Buffered(); numBytes > 0 {
				bz, err := c.bufConnReader.Peek(cmn.MinInt(numBytes, 100))
				if err == nil {
					// return
				} else {
					c.Logger.Debug("Error peeking connection buffer", "err", err)
					// return nil
				}
				c.Logger.Info("Peek connection buffer", "numBytes", numBytes, "bz", bz)
			}
		*/

		// Read packet type
		var packet Packet
		var _n int64
		var err error
		_n, err = cdc.UnmarshalBinaryReader(c.bufConnReader, &packet, int64(c._maxPacketMsgSize))
		c.recvMonitor.Update(int(_n))
		if err != nil {
			if c.IsRunning() {
				c.Logger.Error("Connection failed @ recvRoutine (reading byte)", "conn", c, "err", err)
				c.stopForError(err)
			}
			break FOR_LOOP
		}

		// Read more depending on packet type.
		switch pkt := packet.(type) {
		case PacketPing:
			// TODO: prevent abuse, as they cause flush()'s.
			// https://github.com/tendermint/tendermint/issues/1190
			c.Logger.Debug("Receive Ping")
			select {
			case c.pong <- struct{}{}:
			default:
				// never block
			}
		case PacketPong:
			c.Logger.Debug("Receive Pong")
			select {
			case c.pongTimeoutCh <- false:
			default:
				// never block
			}
		case PacketMsg:
			channel, ok := c.channelsIdx[pkt.ChannelID]
			if !ok || channel == nil {
				err := fmt.Errorf("Unknown channel %X", pkt.ChannelID)
				c.Logger.Error("Connection failed @ recvRoutine", "conn", c, "err", err)
				c.stopForError(err)
				break FOR_LOOP
			}

			msgBytes, err := channel.recvPacketMsg(pkt)
			if err != nil {
				if c.IsRunning() {
					c.Logger.Error("Connection failed @ recvRoutine", "conn", c, "err", err)
					c.stopForError(err)
				}
				break FOR_LOOP
			}
			if msgBytes != nil {
				c.Logger.Debug("Received bytes", "chID", pkt.ChannelID, "msgBytes", fmt.Sprintf("%X", msgBytes))
				// NOTE: This means the reactor.Receive runs in the same thread as the p2p recv routine
				c.onReceive(pkt.ChannelID, msgBytes)
			}
		default:
			err := fmt.Errorf("Unknown message type %v", reflect.TypeOf(packet))
			c.Logger.Error("Connection failed @ recvRoutine", "conn", c, "err", err)
			c.stopForError(err)
			break FOR_LOOP
		}
	}

	// Cleanup
	close(c.pong)
	for range c.pong {
		// Drain
	}
}

// not goroutine-safe
func (c *MConnection) stopPongTimer() {
	if c.pongTimer != nil {
		_ = c.pongTimer.Stop()
		c.pongTimer = nil
	}
}

// maxPacketMsgSize returns a maximum size of PacketMsg, including the overhead
// of amino encoding.
func (c *MConnection) maxPacketMsgSize() int {
	return len(cdc.MustMarshalBinary(PacketMsg{
		ChannelID: 0x01,
		EOF:       1,
		Bytes:     make([]byte, c.config.MaxPacketMsgPayloadSize),
	})) + 10 // leave room for changes in amino
}

type ConnectionStatus struct {
	Duration    time.Duration
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
	status.Duration = time.Since(c.created)
	status.SendMonitor = c.sendMonitor.Status()
	status.RecvMonitor = c.recvMonitor.Status()
	status.Channels = make([]ChannelStatus, len(c.channels))
	for i, channel := range c.channels {
		status.Channels[i] = ChannelStatus{
			ID:                channel.desc.ID,
			SendQueueCapacity: cap(channel.sendQueue),
			SendQueueSize:     int(channel.sendQueueSize), // TODO use atomic
			Priority:          channel.desc.Priority,
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

func (chDesc ChannelDescriptor) FillDefaults() (filled ChannelDescriptor) {
	if chDesc.SendQueueCapacity == 0 {
		chDesc.SendQueueCapacity = defaultSendQueueCapacity
	}
	if chDesc.RecvBufferCapacity == 0 {
		chDesc.RecvBufferCapacity = defaultRecvBufferCapacity
	}
	if chDesc.RecvMessageCapacity == 0 {
		chDesc.RecvMessageCapacity = defaultRecvMessageCapacity
	}
	filled = chDesc
	return
}

// TODO: lowercase.
// NOTE: not goroutine-safe.
type Channel struct {
	conn          *MConnection
	desc          ChannelDescriptor
	sendQueue     chan []byte
	sendQueueSize int32 // atomic.
	recving       []byte
	sending       []byte
	recentlySent  int64 // exponential moving average

	maxPacketMsgPayloadSize int

	Logger log.Logger
}

func newChannel(conn *MConnection, desc ChannelDescriptor) *Channel {
	desc = desc.FillDefaults()
	if desc.Priority <= 0 {
		cmn.PanicSanity("Channel default priority must be a positive integer")
	}
	return &Channel{
		conn:                    conn,
		desc:                    desc,
		sendQueue:               make(chan []byte, desc.SendQueueCapacity),
		recving:                 make([]byte, 0, desc.RecvBufferCapacity),
		maxPacketMsgPayloadSize: conn.config.MaxPacketMsgPayloadSize,
	}
}

func (ch *Channel) SetLogger(l log.Logger) {
	ch.Logger = l
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

// Returns true if any PacketMsgs are pending to be sent.
// Call before calling nextPacketMsg()
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

// Creates a new PacketMsg to send.
// Not goroutine-safe
func (ch *Channel) nextPacketMsg() PacketMsg {
	packet := PacketMsg{}
	packet.ChannelID = byte(ch.desc.ID)
	maxSize := ch.maxPacketMsgPayloadSize
	packet.Bytes = ch.sending[:cmn.MinInt(maxSize, len(ch.sending))]
	if len(ch.sending) <= maxSize {
		packet.EOF = byte(0x01)
		ch.sending = nil
		atomic.AddInt32(&ch.sendQueueSize, -1) // decrement sendQueueSize
	} else {
		packet.EOF = byte(0x00)
		ch.sending = ch.sending[cmn.MinInt(maxSize, len(ch.sending)):]
	}
	return packet
}

// Writes next PacketMsg to w and updates c.recentlySent.
// Not goroutine-safe
func (ch *Channel) writePacketMsgTo(w io.Writer) (n int64, err error) {
	var packet = ch.nextPacketMsg()
	n, err = cdc.MarshalBinaryWriter(w, packet)
	ch.recentlySent += n
	return
}

// Handles incoming PacketMsgs. It returns a message bytes if message is
// complete. NOTE message bytes may change on next call to recvPacketMsg.
// Not goroutine-safe
func (ch *Channel) recvPacketMsg(packet PacketMsg) ([]byte, error) {
	ch.Logger.Debug("Read PacketMsg", "conn", ch.conn, "packet", packet)
	var recvCap, recvReceived = ch.desc.RecvMessageCapacity, len(ch.recving) + len(packet.Bytes)
	if recvCap < recvReceived {
		return nil, fmt.Errorf("Received message exceeds available capacity: %v < %v", recvCap, recvReceived)
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

//----------------------------------------
// Packet

type Packet interface {
	AssertIsPacket()
}

func RegisterPacket(cdc *amino.Codec) {
	cdc.RegisterInterface((*Packet)(nil), nil)
	cdc.RegisterConcrete(PacketPing{}, "tendermint/p2p/PacketPing", nil)
	cdc.RegisterConcrete(PacketPong{}, "tendermint/p2p/PacketPong", nil)
	cdc.RegisterConcrete(PacketMsg{}, "tendermint/p2p/PacketMsg", nil)
}

func (_ PacketPing) AssertIsPacket() {}
func (_ PacketPong) AssertIsPacket() {}
func (_ PacketMsg) AssertIsPacket()  {}

type PacketPing struct {
}

type PacketPong struct {
}

type PacketMsg struct {
	ChannelID byte
	EOF       byte // 1 means message ends here.
	Bytes     []byte
}

func (mp PacketMsg) String() string {
	return fmt.Sprintf("PacketMsg{%X:%X T:%X}", mp.ChannelID, mp.Bytes, mp.EOF)
}
