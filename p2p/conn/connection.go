package conn

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"

	flow "github.com/tendermint/tendermint/libs/flowrate"
	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/libs/protoio"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/libs/timer"
	tmp2p "github.com/tendermint/tendermint/proto/tendermint/p2p"
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
request times out.  The message `msg` is serialized using Protobuf.

`TrySend(chID, msgBytes)` is a nonblocking call that returns false if the
channel's queue is full.

Inbound message bytes are handled with an onReceive callback function.
*/
type MConnection struct {
	service.BaseService

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

	// Closing quitSendRoutine will cause the sendRoutine to eventually quit.
	// doneSendRoutine is closed when the sendRoutine actually quits.
	quitSendRoutine chan struct{}
	doneSendRoutine chan struct{}

	// Closing quitRecvRouting will cause the recvRouting to eventually quit.
	quitRecvRoutine chan struct{}

	// used to ensure FlushStop and OnStop
	// are safe to call concurrently.
	stopMtx tmsync.Mutex

	flushTimer *timer.ThrottleTimer // flush writes as necessary but throttled.
	pingTimer  *time.Ticker         // send pings periodically

	// close conn if pong is not received in pongTimeout
	pongTimer     *time.Timer
	pongTimeoutCh chan bool // true - timeout, false - peer sent pong

	chStatsTimer *time.Ticker // update channel stats periodically

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
func NewMConnection(
	conn net.Conn,
	chDescs []*ChannelDescriptor,
	onReceive receiveCbFunc,
	onError errorCbFunc,
) *MConnection {
	return NewMConnectionWithConfig(
		conn,
		chDescs,
		onReceive,
		onError,
		DefaultMConnConfig())
}

// NewMConnectionWithConfig wraps net.Conn and creates multiplex connection with a config
func NewMConnectionWithConfig(
	conn net.Conn,
	chDescs []*ChannelDescriptor,
	onReceive receiveCbFunc,
	onError errorCbFunc,
	config MConnConfig,
) *MConnection {
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
		created:       time.Now(),
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

	mconn.BaseService = *service.NewBaseService(nil, "MConnection", mconn)

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
	c.flushTimer = timer.NewThrottleTimer("flush", c.config.FlushThrottle)
	c.pingTimer = time.NewTicker(c.config.PingInterval)
	c.pongTimeoutCh = make(chan bool, 1)
	c.chStatsTimer = time.NewTicker(updateStats)
	c.quitSendRoutine = make(chan struct{})
	c.doneSendRoutine = make(chan struct{})
	c.quitRecvRoutine = make(chan struct{})
	go c.sendRoutine()
	go c.recvRoutine()
	return nil
}

// stopServices stops the BaseService and timers and closes the quitSendRoutine.
// if the quitSendRoutine was already closed, it returns true, otherwise it returns false.
// It uses the stopMtx to ensure only one of FlushStop and OnStop can do this at a time.
func (c *MConnection) stopServices() (alreadyStopped bool) {
	c.stopMtx.Lock()
	defer c.stopMtx.Unlock()

	select {
	case <-c.quitSendRoutine:
		// already quit
		return true
	default:
	}

	select {
	case <-c.quitRecvRoutine:
		// already quit
		return true
	default:
	}

	c.BaseService.OnStop()
	c.flushTimer.Stop()
	c.pingTimer.Stop()
	c.chStatsTimer.Stop()

	// inform the recvRouting that we are shutting down
	close(c.quitRecvRoutine)
	close(c.quitSendRoutine)
	return false
}

// FlushStop replicates the logic of OnStop.
// It additionally ensures that all successful
// .Send() calls will get flushed before closing
// the connection.
func (c *MConnection) FlushStop() {
	if c.stopServices() {
		return
	}

	// this block is unique to FlushStop
	{
		// wait until the sendRoutine exits
		// so we dont race on calling sendSomePacketMsgs
		<-c.doneSendRoutine

		// Send and flush all pending msgs.
		// Since sendRoutine has exited, we can call this
		// safely
		eof := c.sendSomePacketMsgs()
		for !eof {
			eof = c.sendSomePacketMsgs()
		}
		c.flush()

		// Now we can close the connection
	}

	c.conn.Close()

	// We can't close pong safely here because
	// recvRoutine may write to it after we've stopped.
	// Though it doesn't need to get closed at all,
	// we close it @ recvRoutine.

	// c.Stop()
}

// OnStop implements BaseService
func (c *MConnection) OnStop() {
	if c.stopServices() {
		return
	}

	c.conn.Close()

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
		c.Logger.Debug("MConnection flush failed", "err", err)
	}
}

// Catch panics, usually caused by remote disconnects.
func (c *MConnection) _recover() {
	if r := recover(); r != nil {
		c.Logger.Error("MConnection panicked", "err", r, "stack", string(debug.Stack()))
		c.stopForError(fmt.Errorf("recovered from panic: %v", r))
	}
}

func (c *MConnection) stopForError(r interface{}) {
	if err := c.Stop(); err != nil {
		c.Logger.Error("Error stopping connection", "err", err)
	}
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
		c.Logger.Debug("Send failed", "channel", chID, "conn", c, "msgBytes", fmt.Sprintf("%X", msgBytes))
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

	protoWriter := protoio.NewDelimitedWriter(c.bufConnWriter)

FOR_LOOP:
	for {
		var _n int
		var err error
	SELECTION:
		select {
		case <-c.flushTimer.Ch:
			// NOTE: flushTimer.Set() must be called every time
			// something is written to .bufConnWriter.
			c.flush()
		case <-c.chStatsTimer.C:
			for _, channel := range c.channels {
				channel.updateStats()
			}
		case <-c.pingTimer.C:
			c.Logger.Debug("Send Ping")
			_n, err = protoWriter.WriteMsg(mustWrapPacket(&tmp2p.PacketPing{}))
			if err != nil {
				c.Logger.Error("Failed to send PacketPing", "err", err)
				break SELECTION
			}
			c.sendMonitor.Update(_n)
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
			_n, err = protoWriter.WriteMsg(mustWrapPacket(&tmp2p.PacketPong{}))
			if err != nil {
				c.Logger.Error("Failed to send PacketPong", "err", err)
				break SELECTION
			}
			c.sendMonitor.Update(_n)
			c.flush()
		case <-c.quitSendRoutine:
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
	close(c.doneSendRoutine)
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
	c.sendMonitor.Update(_n)
	c.flushTimer.Set()
	return false
}

// recvRoutine reads PacketMsgs and reconstructs the message using the channels' "recving" buffer.
// After a whole message has been assembled, it's pushed to onReceive().
// Blocks depending on how the connection is throttled.
// Otherwise, it never blocks.
func (c *MConnection) recvRoutine() {
	defer c._recover()

	protoReader := protoio.NewDelimitedReader(c.bufConnReader, c._maxPacketMsgSize)

FOR_LOOP:
	for {
		// Block until .recvMonitor says we can read.
		c.recvMonitor.Limit(c._maxPacketMsgSize, atomic.LoadInt64(&c.config.RecvRate), true)

		// Peek into bufConnReader for debugging
		/*
			if numBytes := c.bufConnReader.Buffered(); numBytes > 0 {
				bz, err := c.bufConnReader.Peek(tmmath.MinInt(numBytes, 100))
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
		var packet tmp2p.Packet

		err := protoReader.ReadMsg(&packet)
		if err != nil {
			// stopServices was invoked and we are shutting down
			// receiving is excpected to fail since we will close the connection
			select {
			case <-c.quitRecvRoutine:
				break FOR_LOOP
			default:
			}

			if c.IsRunning() {
				if err == io.EOF {
					c.Logger.Info("Connection is closed @ recvRoutine (likely by the other side)", "conn", c)
				} else {
					c.Logger.Debug("Connection failed @ recvRoutine (reading byte)", "conn", c, "err", err)
				}
				c.stopForError(err)
			}
			break FOR_LOOP
		}

		// Read more depending on packet type.
		switch pkt := packet.Sum.(type) {
		case *tmp2p.Packet_PacketPing:
			// TODO: prevent abuse, as they cause flush()'s.
			// https://github.com/tendermint/tendermint/issues/1190
			c.Logger.Debug("Receive Ping")
			select {
			case c.pong <- struct{}{}:
			default:
				// never block
			}
		case *tmp2p.Packet_PacketPong:
			c.Logger.Debug("Receive Pong")
			select {
			case c.pongTimeoutCh <- false:
			default:
				// never block
			}
		case *tmp2p.Packet_PacketMsg:
			channel, ok := c.channelsIdx[byte(pkt.PacketMsg.ChannelID)]
			if !ok || channel == nil {
				err := fmt.Errorf("unknown channel %X", pkt.PacketMsg.ChannelID)
				c.Logger.Debug("Connection failed @ recvRoutine", "conn", c, "err", err)
				c.stopForError(err)
				break FOR_LOOP
			}

			msgBytes, err := channel.recvPacketMsg(*pkt.PacketMsg)
			if err != nil {
				if c.IsRunning() {
					c.Logger.Debug("Connection failed @ recvRoutine", "conn", c, "err", err)
					c.stopForError(err)
				}
				break FOR_LOOP
			}
			if msgBytes != nil {
				c.Logger.Debug("Received bytes", "chID", pkt.PacketMsg.ChannelID, "msgBytes", fmt.Sprintf("%X", msgBytes))
				// NOTE: This means the reactor.Receive runs in the same thread as the p2p recv routine
				c.onReceive(byte(pkt.PacketMsg.ChannelID), msgBytes)
			}
		default:
			err := fmt.Errorf("unknown message type %v", reflect.TypeOf(packet))
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

// maxPacketMsgSize returns a maximum size of PacketMsg
func (c *MConnection) maxPacketMsgSize() int {
	bz, err := proto.Marshal(mustWrapPacket(&tmp2p.PacketMsg{
		ChannelID: 0x01,
		EOF:       true,
		Data:      make([]byte, c.config.MaxPacketMsgPayloadSize),
	}))
	if err != nil {
		panic(err)
	}
	return len(bz)
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
			SendQueueSize:     int(atomic.LoadInt32(&channel.sendQueueSize)),
			Priority:          channel.desc.Priority,
			RecentlySent:      atomic.LoadInt64(&channel.recentlySent),
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
		panic("Channel default priority must be a positive integer")
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
func (ch *Channel) nextPacketMsg() tmp2p.PacketMsg {
	packet := tmp2p.PacketMsg{ChannelID: int32(ch.desc.ID)}
	maxSize := ch.maxPacketMsgPayloadSize
	packet.Data = ch.sending[:tmmath.MinInt(maxSize, len(ch.sending))]
	if len(ch.sending) <= maxSize {
		packet.EOF = true
		ch.sending = nil
		atomic.AddInt32(&ch.sendQueueSize, -1) // decrement sendQueueSize
	} else {
		packet.EOF = false
		ch.sending = ch.sending[tmmath.MinInt(maxSize, len(ch.sending)):]
	}
	return packet
}

// Writes next PacketMsg to w and updates c.recentlySent.
// Not goroutine-safe
func (ch *Channel) writePacketMsgTo(w io.Writer) (n int, err error) {
	packet := ch.nextPacketMsg()
	n, err = protoio.NewDelimitedWriter(w).WriteMsg(mustWrapPacket(&packet))
	atomic.AddInt64(&ch.recentlySent, int64(n))
	return
}

// Handles incoming PacketMsgs. It returns a message bytes if message is
// complete. NOTE message bytes may change on next call to recvPacketMsg.
// Not goroutine-safe
func (ch *Channel) recvPacketMsg(packet tmp2p.PacketMsg) ([]byte, error) {
	ch.Logger.Debug("Read PacketMsg", "conn", ch.conn, "packet", packet)
	var recvCap, recvReceived = ch.desc.RecvMessageCapacity, len(ch.recving) + len(packet.Data)
	if recvCap < recvReceived {
		return nil, fmt.Errorf("received message exceeds available capacity: %v < %v", recvCap, recvReceived)
	}
	ch.recving = append(ch.recving, packet.Data...)
	if packet.EOF {
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
	atomic.StoreInt64(&ch.recentlySent, int64(float64(atomic.LoadInt64(&ch.recentlySent))*0.8))
}

//----------------------------------------
// Packet

// mustWrapPacket takes a packet kind (oneof) and wraps it in a tmp2p.Packet message.
func mustWrapPacket(pb proto.Message) *tmp2p.Packet {
	var msg tmp2p.Packet

	switch pb := pb.(type) {
	case *tmp2p.Packet: // already a packet
		msg = *pb
	case *tmp2p.PacketPing:
		msg = tmp2p.Packet{
			Sum: &tmp2p.Packet_PacketPing{
				PacketPing: pb,
			},
		}
	case *tmp2p.PacketPong:
		msg = tmp2p.Packet{
			Sum: &tmp2p.Packet_PacketPong{
				PacketPong: pb,
			},
		}
	case *tmp2p.PacketMsg:
		msg = tmp2p.Packet{
			Sum: &tmp2p.Packet_PacketMsg{
				PacketMsg: pb,
			},
		}
	default:
		panic(fmt.Errorf("unknown packet type %T", pb))
	}

	return &msg
}
