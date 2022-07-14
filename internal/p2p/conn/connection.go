package conn

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/internal/libs/flowrate"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	"github.com/tendermint/tendermint/internal/libs/timer"
	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/libs/service"
	tmp2p "github.com/tendermint/tendermint/proto/tendermint/p2p"
)

const (
	// mirrors MaxPacketMsgPayloadSize from config/config.go
	defaultMaxPacketMsgPayloadSize = 1400

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
	defaultPongTimeout         = 90 * time.Second
)

type receiveCbFunc func(ctx context.Context, chID ChannelID, msgBytes []byte)
type errorCbFunc func(context.Context, interface{})

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

`Send(chID, msgBytes)` is a blocking call that waits until `msg` is
successfully queued for the channel with the given id byte `chID`, or until the
request times out.  The message `msg` is serialized using Protobuf.

Inbound message bytes are handled with an onReceive callback function.
*/
type MConnection struct {
	service.BaseService
	logger log.Logger

	conn          net.Conn
	bufConnReader *bufio.Reader
	bufConnWriter *bufio.Writer
	sendMonitor   *flowrate.Monitor
	recvMonitor   *flowrate.Monitor
	send          chan struct{}
	pong          chan struct{}
	channels      []*channel
	channelsIdx   map[ChannelID]*channel
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
	stopMtx    sync.Mutex
	stopSignal <-chan struct{}

	cancel context.CancelFunc

	flushTimer *timer.ThrottleTimer // flush writes as necessary but throttled.
	pingTimer  *time.Ticker         // send pings periodically

	// close conn if pong is not received in pongTimeout
	lastMsgRecv struct {
		sync.Mutex
		at time.Time
	}

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

	// Process/Transport Start time
	StartTime time.Time `mapstructure:",omitempty"`
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
		StartTime:               time.Now(),
	}
}

// NewMConnection wraps net.Conn and creates multiplex connection with a config
func NewMConnection(
	logger log.Logger,
	conn net.Conn,
	chDescs []*ChannelDescriptor,
	onReceive receiveCbFunc,
	onError errorCbFunc,
	config MConnConfig,
) *MConnection {
	mconn := &MConnection{
		logger:        logger,
		conn:          conn,
		bufConnReader: bufio.NewReaderSize(conn, minReadBufferSize),
		bufConnWriter: bufio.NewWriterSize(conn, minWriteBufferSize),
		sendMonitor:   flowrate.New(config.StartTime, 0, 0),
		recvMonitor:   flowrate.New(config.StartTime, 0, 0),
		send:          make(chan struct{}, 1),
		pong:          make(chan struct{}, 1),
		onReceive:     onReceive,
		onError:       onError,
		config:        config,
		created:       time.Now(),
		cancel:        func() {},
	}

	mconn.BaseService = *service.NewBaseService(logger, "MConnection", mconn)

	// Create channels
	var channelsIdx = map[ChannelID]*channel{}
	var channels = []*channel{}

	for _, desc := range chDescs {
		channel := newChannel(mconn, *desc)
		channelsIdx[channel.desc.ID] = channel
		channels = append(channels, channel)
	}
	mconn.channels = channels
	mconn.channelsIdx = channelsIdx

	// maxPacketMsgSize() is a bit heavy, so call just once
	mconn._maxPacketMsgSize = mconn.maxPacketMsgSize()

	return mconn
}

// OnStart implements BaseService
func (c *MConnection) OnStart(ctx context.Context) error {
	c.flushTimer = timer.NewThrottleTimer("flush", c.config.FlushThrottle)
	c.pingTimer = time.NewTicker(c.config.PingInterval)
	c.chStatsTimer = time.NewTicker(updateStats)
	c.quitSendRoutine = make(chan struct{})
	c.doneSendRoutine = make(chan struct{})
	c.quitRecvRoutine = make(chan struct{})
	c.stopSignal = ctx.Done()
	c.setRecvLastMsgAt(time.Now())
	go c.sendRoutine(ctx)
	go c.recvRoutine(ctx)
	return nil
}

func (c *MConnection) setRecvLastMsgAt(t time.Time) {
	c.lastMsgRecv.Lock()
	defer c.lastMsgRecv.Unlock()
	c.lastMsgRecv.at = t
}

func (c *MConnection) getLastMessageAt() time.Time {
	c.lastMsgRecv.Lock()
	defer c.lastMsgRecv.Unlock()
	return c.lastMsgRecv.at
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

	c.flushTimer.Stop()
	c.pingTimer.Stop()
	c.chStatsTimer.Stop()

	// inform the recvRouting that we are shutting down
	close(c.quitRecvRoutine)
	close(c.quitSendRoutine)
	return false
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
	c.logger.Debug("Flush", "conn", c)
	err := c.bufConnWriter.Flush()
	if err != nil {
		c.logger.Debug("MConnection flush failed", "err", err)
	}
}

// Catch panics, usually caused by remote disconnects.
func (c *MConnection) _recover(ctx context.Context) {
	if r := recover(); r != nil {
		c.logger.Error("MConnection panicked", "err", r, "stack", string(debug.Stack()))
		c.stopForError(ctx, fmt.Errorf("recovered from panic: %v", r))
	}
}

func (c *MConnection) stopForError(ctx context.Context, r interface{}) {
	c.Stop()

	if atomic.CompareAndSwapUint32(&c.errored, 0, 1) {
		if c.onError != nil {
			c.onError(ctx, r)
		}
	}
}

// Queues a message to be sent to channel.
func (c *MConnection) Send(chID ChannelID, msgBytes []byte) bool {
	if !c.IsRunning() {
		return false
	}

	c.logger.Debug("Send", "channel", chID, "conn", c, "msgBytes", msgBytes)

	// Send message to channel.
	channel, ok := c.channelsIdx[chID]
	if !ok {
		c.logger.Error("Cannot send bytes to unknown channel", "channel", chID)
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
		c.logger.Debug("Send failed", "channel", chID, "conn", c, "msgBytes", msgBytes)
	}
	return success
}

// sendRoutine polls for packets to send from channels.
func (c *MConnection) sendRoutine(ctx context.Context) {
	defer c._recover(ctx)
	protoWriter := protoio.NewDelimitedWriter(c.bufConnWriter)

	pongTimeout := time.NewTicker(c.config.PongTimeout)
	defer pongTimeout.Stop()
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
			_n, err = protoWriter.WriteMsg(mustWrapPacket(&tmp2p.PacketPing{}))
			if err != nil {
				c.logger.Error("Failed to send PacketPing", "err", err)
				break SELECTION
			}
			c.sendMonitor.Update(_n)
			c.flush()
		case <-c.pong:
			_n, err = protoWriter.WriteMsg(mustWrapPacket(&tmp2p.PacketPong{}))
			if err != nil {
				c.logger.Error("Failed to send PacketPong", "err", err)
				break SELECTION
			}
			c.sendMonitor.Update(_n)
			c.flush()
		case <-ctx.Done():
			break FOR_LOOP
		case <-c.quitSendRoutine:
			break FOR_LOOP
		case <-pongTimeout.C:
			// the point of the pong timer is to check to
			// see if we've seen a message recently, so we
			// want to make sure that we escape this
			// select statement on an interval to ensure
			// that we avoid hanging on to dead
			// connections for too long.
			break SELECTION
		case <-c.send:
			// Send some PacketMsgs
			eof := c.sendSomePacketMsgs(ctx)
			if !eof {
				// Keep sendRoutine awake.
				select {
				case c.send <- struct{}{}:
				default:
				}
			}
		}

		if time.Since(c.getLastMessageAt()) > c.config.PongTimeout {
			err = errors.New("pong timeout")
		}

		if err != nil {
			c.logger.Error("Connection failed @ sendRoutine", "conn", c, "err", err)
			c.stopForError(ctx, err)
			break FOR_LOOP
		}
		if !c.IsRunning() {
			break FOR_LOOP
		}
	}

	// Cleanup
	close(c.doneSendRoutine)
}

// Returns true if messages from channels were exhausted.
// Blocks in accordance to .sendMonitor throttling.
func (c *MConnection) sendSomePacketMsgs(ctx context.Context) bool {
	// Block until .sendMonitor says we can write.
	// Once we're ready we send more than we asked for,
	// but amortized it should even out.
	c.sendMonitor.Limit(c._maxPacketMsgSize, c.config.SendRate, true)

	// Now send some PacketMsgs.
	for i := 0; i < numBatchPacketMsgs; i++ {
		if c.sendPacketMsg(ctx) {
			return true
		}
	}
	return false
}

// Returns true if messages from channels were exhausted.
func (c *MConnection) sendPacketMsg(ctx context.Context) bool {
	// Choose a channel to create a PacketMsg from.
	// The chosen channel will be the one whose recentlySent/priority is the least.
	var leastRatio float32 = math.MaxFloat32
	var leastChannel *channel
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
	// c.logger.Info("Found a msgPacket to send")

	// Make & send a PacketMsg from this channel
	_n, err := leastChannel.writePacketMsgTo(c.bufConnWriter)
	if err != nil {
		c.logger.Error("Failed to write PacketMsg", "err", err)
		c.stopForError(ctx, err)
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
func (c *MConnection) recvRoutine(ctx context.Context) {
	defer c._recover(ctx)

	protoReader := protoio.NewDelimitedReader(c.bufConnReader, c._maxPacketMsgSize)

FOR_LOOP:
	for {
		select {
		case <-ctx.Done():
			break FOR_LOOP
		case <-c.doneSendRoutine:
			break FOR_LOOP
		default:
		}

		// Block until .recvMonitor says we can read.
		c.recvMonitor.Limit(c._maxPacketMsgSize, c.config.RecvRate, true)

		// Peek into bufConnReader for debugging
		/*
			if numBytes := c.bufConnReader.Buffered(); numBytes > 0 {
				bz, err := c.bufConnReader.Peek(tmmath.MinInt(numBytes, 100))
				if err == nil {
					// return
				} else {
					c.logger.Debug("error peeking connection buffer", "err", err)
					// return nil
				}
				c.logger.Info("Peek connection buffer", "numBytes", numBytes, "bz", bz)
			}
		*/

		// Read packet type
		var packet tmp2p.Packet

		_n, err := protoReader.ReadMsg(&packet)
		c.recvMonitor.Update(_n)
		if err != nil {
			// stopServices was invoked and we are shutting down
			// receiving is excpected to fail since we will close the connection
			select {
			case <-ctx.Done():
			case <-c.quitRecvRoutine:
				break FOR_LOOP
			default:
			}

			if c.IsRunning() {
				if err == io.EOF {
					c.logger.Info("Connection is closed @ recvRoutine (likely by the other side)", "conn", c)
				} else {
					c.logger.Debug("Connection failed @ recvRoutine (reading byte)", "conn", c, "err", err)
				}
				c.stopForError(ctx, err)
			}
			break FOR_LOOP
		}

		// record for pong/heartbeat
		c.setRecvLastMsgAt(time.Now())

		// Read more depending on packet type.
		switch pkt := packet.Sum.(type) {
		case *tmp2p.Packet_PacketPing:
			// TODO: prevent abuse, as they cause flush()'s.
			// https://github.com/tendermint/tendermint/issues/1190
			select {
			case c.pong <- struct{}{}:
			default:
				// never block
			}
		case *tmp2p.Packet_PacketPong:
			// do nothing, we updated the "last message
			// received" timestamp above, so we can ignore
			// this message
		case *tmp2p.Packet_PacketMsg:
			channelID := ChannelID(pkt.PacketMsg.ChannelID)
			channel, ok := c.channelsIdx[channelID]
			if pkt.PacketMsg.ChannelID < 0 || pkt.PacketMsg.ChannelID > math.MaxUint8 || !ok || channel == nil {
				err := fmt.Errorf("unknown channel %X", pkt.PacketMsg.ChannelID)
				c.logger.Debug("Connection failed @ recvRoutine", "conn", c, "err", err)
				c.stopForError(ctx, err)
				break FOR_LOOP
			}

			msgBytes, err := channel.recvPacketMsg(*pkt.PacketMsg)
			if err != nil {
				if c.IsRunning() {
					c.logger.Debug("Connection failed @ recvRoutine", "conn", c, "err", err)
					c.stopForError(ctx, err)
				}
				break FOR_LOOP
			}
			if msgBytes != nil {
				c.logger.Debug("Received bytes", "chID", channelID, "msgBytes", msgBytes)
				// NOTE: This means the reactor.Receive runs in the same thread as the p2p recv routine
				c.onReceive(ctx, channelID, msgBytes)
			}
		default:
			err := fmt.Errorf("unknown message type %v", reflect.TypeOf(packet))
			c.logger.Error("Connection failed @ recvRoutine", "conn", c, "err", err)
			c.stopForError(ctx, err)
			break FOR_LOOP
		}
	}

	// Cleanup
	close(c.pong)
	for range c.pong {
		// Drain
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

type ChannelStatus struct {
	ID                byte
	SendQueueCapacity int
	SendQueueSize     int
	Priority          int
	RecentlySent      int64
}

//-----------------------------------------------------------------------------
// ChannelID is an arbitrary channel ID.
type ChannelID uint16

type ChannelDescriptor struct {
	ID       ChannelID
	Priority int

	MessageType proto.Message

	// TODO: Remove once p2p refactor is complete.
	SendQueueCapacity   int
	RecvMessageCapacity int

	// RecvBufferCapacity defines the max buffer size of inbound messages for a
	// given p2p Channel queue.
	RecvBufferCapacity int

	// Human readable name of the channel, used in logging and
	// diagnostics.
	Name string
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

// NOTE: not goroutine-safe.
type channel struct {
	// Exponential moving average.
	// This field must be accessed atomically.
	// It is first in the struct to ensure correct alignment.
	// See https://github.com/tendermint/tendermint/issues/7000.
	recentlySent int64

	conn          *MConnection
	desc          ChannelDescriptor
	sendQueue     chan []byte
	sendQueueSize int32 // atomic.
	recving       []byte
	sending       []byte

	maxPacketMsgPayloadSize int

	logger log.Logger
}

func newChannel(conn *MConnection, desc ChannelDescriptor) *channel {
	desc = desc.FillDefaults()
	if desc.Priority <= 0 {
		panic("Channel default priority must be a positive integer")
	}
	return &channel{
		conn:                    conn,
		desc:                    desc,
		sendQueue:               make(chan []byte, desc.SendQueueCapacity),
		recving:                 make([]byte, 0, desc.RecvBufferCapacity),
		maxPacketMsgPayloadSize: conn.config.MaxPacketMsgPayloadSize,
		logger:                  conn.logger,
	}
}

// Queues message to send to this channel.
// Goroutine-safe
// Times out (and returns false) after defaultSendTimeout
func (ch *channel) sendBytes(bytes []byte) bool {
	select {
	case ch.sendQueue <- bytes:
		atomic.AddInt32(&ch.sendQueueSize, 1)
		return true
	case <-time.After(defaultSendTimeout):
		return false
	case <-ch.conn.stopSignal:
		return false
	}
}

// Returns true if any PacketMsgs are pending to be sent.
// Call before calling nextPacketMsg()
// Goroutine-safe
func (ch *channel) isSendPending() bool {
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
func (ch *channel) nextPacketMsg() tmp2p.PacketMsg {
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
func (ch *channel) writePacketMsgTo(w io.Writer) (n int, err error) {
	packet := ch.nextPacketMsg()
	n, err = protoio.NewDelimitedWriter(w).WriteMsg(mustWrapPacket(&packet))
	atomic.AddInt64(&ch.recentlySent, int64(n))
	return
}

// Handles incoming PacketMsgs. It returns a message bytes if message is
// complete, which is owned by the caller and will not be modified.
// Not goroutine-safe
func (ch *channel) recvPacketMsg(packet tmp2p.PacketMsg) ([]byte, error) {
	ch.logger.Debug("Read PacketMsg", "conn", ch.conn, "packet", packet)
	var recvCap, recvReceived = ch.desc.RecvMessageCapacity, len(ch.recving) + len(packet.Data)
	if recvCap < recvReceived {
		return nil, fmt.Errorf("received message exceeds available capacity: %v < %v", recvCap, recvReceived)
	}
	ch.recving = append(ch.recving, packet.Data...)
	if packet.EOF {
		msgBytes := ch.recving
		ch.recving = make([]byte, 0, ch.desc.RecvBufferCapacity)
		return msgBytes, nil
	}
	return nil, nil
}

// Call this periodically to update stats for throttling purposes.
// Not goroutine-safe
func (ch *channel) updateStats() {
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
