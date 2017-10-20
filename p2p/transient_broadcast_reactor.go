package p2p

import (
	"bytes"
	"fmt"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	"reflect"
)

const (
	TBChannel = byte(0x00)

	maxMessageSize = 1048576
)

type TransientBroadcastReactor struct {
	BaseReactor
	evsw types.EventSwitch
}

func NewTransientBroadCastReactor() *TransientBroadcastReactor {
	r := &TransientBroadcastReactor{}
	r.BaseReactor = *NewBaseReactor("TransientBroadcastReactor", r)
	return r
}

func (r *TransientBroadcastReactor) SetEventSwitch(eventSwitch types.EventSwitch) {
	r.evsw = eventSwitch
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

	r.Logger.Debug("Received message", "msg", msg)

	switch msg := msg.(type) {
	case *TransientTxMessage:
		r.Logger.Debug("Received tx, broadcasting over EventSwitch now", "tx", msg.Tx)
		types.FireEventTransientTx(r.evsw, types.EventDataTransientTx{msg.Tx})
	default:
		r.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}

}

/**
 * Broadcast a transient message to all peers
 */
func (r *TransientBroadcastReactor) BroadcastTransientMessage(tx types.Tx) {
	r.Logger.Debug("Sending Transient Message", "msg", tx)

	if (r.Switch == nil) {
		r.Logger.Error("PeerSwitch is nil, this shouldnt be")
		return
	}

	// Send to own WSClients, can be multiple
	r.Logger.Debug("Re-Broadcasting over EventSwitch to own subscribers")
	types.FireEventTransientTx(r.evsw, types.EventDataTransientTx{tx})

	// send to peers
	r.Logger.Debug("Broadcasting to peers", "peer_size", r.Switch.peers.Size())
	for _, peer := range r.Switch.peers.List() {
		go func(peer Peer) {
			peer.Send(TBChannel, struct{ TransientMessage }{&TransientTxMessage{Tx: tx}})
		}(peer)
	}

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
	Tx types.Tx
}

// String returns a string representation of the TxMessage.
func (m *TransientTxMessage) String() string {
	return fmt.Sprintf("[TransientTxMessage %v]", m.Tx)
}
