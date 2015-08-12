package mempool

import (
	"bytes"
	"fmt"
	"reflect"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/wire"
)

var (
	MempoolChannel = byte(0x30)
)

// MempoolReactor handles mempool tx broadcasting amongst peers.
type MempoolReactor struct {
	p2p.BaseReactor

	Mempool *Mempool

	evsw events.Fireable
}

func NewMempoolReactor(mempool *Mempool) *MempoolReactor {
	memR := &MempoolReactor{
		Mempool: mempool,
	}
	memR.BaseReactor = *p2p.NewBaseReactor(log, "MempoolReactor", memR)
	return memR
}

// Implements Reactor
func (memR *MempoolReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			ID:       MempoolChannel,
			Priority: 5,
		},
	}
}

// Implements Reactor
func (pexR *MempoolReactor) AddPeer(peer *p2p.Peer) {
}

// Implements Reactor
func (pexR *MempoolReactor) RemovePeer(peer *p2p.Peer, reason interface{}) {
}

// Implements Reactor
func (memR *MempoolReactor) Receive(chID byte, src *p2p.Peer, msgBytes []byte) {
	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "error", err)
		return
	}
	log.Notice("MempoolReactor received message", "msg", msg)

	switch msg := msg.(type) {
	case *TxMessage:
		err := memR.Mempool.AddTx(msg.Tx)
		if err != nil {
			// Bad, seen, or conflicting tx.
			log.Info("Could not add tx", "tx", msg.Tx)
			return
		} else {
			log.Info("Added valid tx", "tx", msg.Tx)
		}
		// Share tx.
		// We use a simple shotgun approach for now.
		// TODO: improve efficiency
		for _, peer := range memR.Switch.Peers().List() {
			if peer.Key == src.Key {
				continue
			}
			peer.TrySend(MempoolChannel, msg)
		}

	default:
		log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

func (memR *MempoolReactor) BroadcastTx(tx types.Tx) error {
	err := memR.Mempool.AddTx(tx)
	if err != nil {
		return err
	}
	msg := &TxMessage{Tx: tx}
	memR.Switch.Broadcast(MempoolChannel, msg)
	return nil
}

// implements events.Eventable
func (memR *MempoolReactor) SetFireable(evsw events.Fireable) {
	memR.evsw = evsw
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeTx = byte(0x01)
)

type MempoolMessage interface{}

var _ = wire.RegisterInterface(
	struct{ MempoolMessage }{},
	wire.ConcreteType{&TxMessage{}, msgTypeTx},
)

func DecodeMessage(bz []byte) (msgType byte, msg MempoolMessage, err error) {
	msgType = bz[0]
	n := new(int64)
	r := bytes.NewReader(bz)
	msg = wire.ReadBinary(struct{ MempoolMessage }{}, r, n, &err).(struct{ MempoolMessage }).MempoolMessage
	return
}

//-------------------------------------

type TxMessage struct {
	Tx types.Tx
}

func (m *TxMessage) String() string {
	return fmt.Sprintf("[TxMessage %v]", m.Tx)
}
