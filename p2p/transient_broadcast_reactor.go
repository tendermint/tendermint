package p2p

import (
	"bytes"
	"fmt"
	"reflect"
	"time"

	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/common"
)

const (
	TBChannel = byte(0xf0)

	maxMessageSize = 1048576

	MaximumAmountOfMessageHops = 10

	MessageMapCleanupInterval = 30 * time.Second

	MessageKeepAliveTime = 15000
)

type TransientBroadcastReactor struct {
	BaseReactor
	evsw     types.EventSwitch
	messages *common.CMap
}

func NewTransientBroadCastReactor() *TransientBroadcastReactor {
	r := &TransientBroadcastReactor{}
	r.messages = common.NewCMap()
	r.BaseReactor = *NewBaseReactor("TransientBroadcastReactor", r)
	return r
}

func (r *TransientBroadcastReactor) SetEventSwitch(eventSwitch types.EventSwitch) {
	r.evsw = eventSwitch
}

func (r *TransientBroadcastReactor) OnStart() error {
	r.BaseReactor.OnStart()
	go r.ensureCleanupRoutine()
	return nil
}

//func(r *TransientBroadcastReactor) SetLogger(logger log.Logger) {
//	r.Logger = logger
//	logger.Debug("Set logger on TransientBroadcastReactor")
//}

func (r *TransientBroadcastReactor) GetChannels() []*ChannelDescriptor {
	return []*ChannelDescriptor{
		&ChannelDescriptor{
			ID:                TBChannel,
			Priority:          1,
			SendQueueCapacity: 10,
		},
	}
}

func (r *TransientBroadcastReactor) Receive(chID byte, src Peer, msgBytes []byte) {

	_, msg, err := DecodeTransientMessage(msgBytes)

	if (err != nil) {
		r.Logger.Error("Error decoding message", "err", err)
		return
	}

	switch msg := msg.(type) {
	case *TransientTxMessage:
		r.ReceiveTTM(msg)
	default:
		r.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}

}

func (r *TransientBroadcastReactor) ReceiveTTM(msg *TransientTxMessage) {

	r.Logger.Debug("Received Message", "tx", msg.Tx, "Hops", msg.Hop)

	if (msg.Hop < MaximumAmountOfMessageHops) {
		bytestring := BytesToString(msg.Hash)

		if (r.messages.Has(bytestring)) {
			// we have seen this before
			// what should we do?
			// do we try to re-broadcast to peers in case we have a new one?
		} else {
			// first time seen, mark it ,increase hop count, resend
			r.messages.Set(bytestring, CurrentTimestamp())

			msg.Hop++

			// we dont fire an event here, it will be fired in broadcastTransientMessage
			r.broadcastTransientMessage(msg)
		}

	} else {
		// already exceeded msg hop count
		// dont care what happens anymore
		bytestring := BytesToString(msg.Hash)
		r.messages.Delete(bytestring)
	}

}

/**
 * Broadcast a transient message to all peers
 * RPC-Endpoint Function
 */
func (r *TransientBroadcastReactor) BroadcastTransientMessage(tx types.Tx) {
	r.Logger.Debug("calling broadcastTransientMessage", "msg", tx, "Hop", 1)

	hash := tx.Hash()
	r.messages.Set(BytesToString(hash), CurrentTimestamp())
	r.broadcastTransientMessage(&TransientTxMessage{Tx: tx, Hop: 1, Hash: hash})
}

/**
 * actual broadcast impl
 */
func (r *TransientBroadcastReactor) broadcastTransientMessage(tm *TransientTxMessage) {
	r.Logger.Debug("Sending Transient Message", "msg", tm.Tx)

	// Send to own WSClients, can be multiple
	r.Logger.Debug("Re-Broadcasting over EventSwitch to own subscribers")
	types.FireEventTransientTx(r.evsw, types.EventDataTransientTx{tm.Tx})

	if (r.Switch != nil) { // we should always have a switch
		// send to peers
		r.Logger.Debug("Broadcasting to peers", "peer_size", r.Switch.peers.Size())
		for _, peer := range r.Switch.peers.List() {
			go func(peer Peer) {
				peer.Send(TBChannel, struct{ TransientMessage }{tm})
			}(peer)
		}
	}
}

func (r *TransientBroadcastReactor) ensureCleanupRoutine() {
	ticker := time.NewTicker(MessageMapCleanupInterval)
	for {
		select {
		case <-ticker.C:
			r.performMsgCleanup()
		case <-r.Quit:
			ticker.Stop()
			return
		}
	}
}

func (r *TransientBroadcastReactor) performMsgCleanup() {

	cursize := r.messages.Size()
	curtime := CurrentTimestamp()
	for _, key := range r.messages.Keys() {

		msgtime, ok := r.messages.Get(key).(int64)
		if (ok && curtime-msgtime > MessageKeepAliveTime) {
			r.messages.Delete(key)
		}
	}
	r.Logger.Debug("Finished Cleanup of MessageMap", "beginsize", cursize, "endsize", r.messages.Size())
}

//func (r *TransientBroadcastReactor) performMsgCleanupAlternative() {
//	curtime := CurrentTimestamp()
//
//	r.messages.RemoveIf(func(key string, value interface{}) bool {
//		msgtime, ok := r.messages.Get(key).(int64)
//		return ok && curtime-msgtime > MessageKeepAliveTime
//	});
//}

func BytesToString(bytes []byte) string {
	return fmt.Sprintf("%X", bytes)
}

func CurrentTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeTransientMessage = byte(0x01)
)

// TransientMessage is a message sent or received by the TransientMessageBroadcastReactor.
type TransientMessage interface{}

var _ = wire.RegisterInterface(
	struct{ TransientMessage }{},
	wire.ConcreteType{&TransientTxMessage{}, msgTypeTransientMessage},
)

// DecodeMessage decodes a byte-array into a TransientMessage.
func DecodeTransientMessage(bz []byte) (msgType byte, msg TransientMessage, err error) {
	msgType = bz[0]
	n := new(int)
	r := bytes.NewReader(bz)
	msg = wire.ReadBinary(struct{ TransientMessage }{}, r, maxMessageSize, n, &err).(struct{ TransientMessage }).TransientMessage
	return
}

//-------------------------------------

// TransientTxMessage is a TransientMessage containing a transaction.
type TransientTxMessage struct {
	Tx   types.Tx
	Hop  int8
	Hash []byte
}

// String returns a string representation of the TxMessage.
func (m *TransientTxMessage) String() string {
	return fmt.Sprintf("[TransientTxMessage %v %d]", m.Tx, m.Hop)
}
