package mempool

import (
	"bytes"
	"fmt"
	"reflect"
	"sync/atomic"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

var (
	MempoolChannel = byte(0x30)
)

// MempoolReactor handles mempool tx broadcasting amongst peers.
type MempoolReactor struct {
	sw      *p2p.Switch
	quit    chan struct{}
	started uint32
	stopped uint32

	Mempool *Mempool

	evsw events.Fireable
}

func NewMempoolReactor(mempool *Mempool) *MempoolReactor {
	memR := &MempoolReactor{
		quit:    make(chan struct{}),
		Mempool: mempool,
	}
	return memR
}

// Implements Reactor
func (memR *MempoolReactor) Start(sw *p2p.Switch) {
	if atomic.CompareAndSwapUint32(&memR.started, 0, 1) {
		memR.sw = sw
		log.Info("Starting MempoolReactor")
	}
}

// Implements Reactor
func (memR *MempoolReactor) Stop() {
	if atomic.CompareAndSwapUint32(&memR.stopped, 0, 1) {
		log.Info("Stopping MempoolReactor")
		close(memR.quit)
	}
}

// Implements Reactor
func (memR *MempoolReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			Id:       MempoolChannel,
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
func (memR *MempoolReactor) Receive(chId byte, src *p2p.Peer, msgBytes []byte) {
	_, msg_, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "error", err)
		return
	}
	log.Info("MempoolReactor received message", "msg", msg_)

	switch msg := msg_.(type) {
	case *TxMessage:
		err := memR.Mempool.AddTx(msg.Tx)
		if err != nil {
			// Bad, seen, or conflicting tx.
			log.Debug("Could not add tx", "tx", msg.Tx)
			return
		} else {
			log.Debug("Added valid tx", "tx", msg.Tx)
		}
		// Share tx.
		// We use a simple shotgun approach for now.
		// TODO: improve efficiency
		for _, peer := range memR.sw.Peers().List() {
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
	memR.sw.Broadcast(MempoolChannel, msg)
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

var _ = binary.RegisterInterface(
	struct{ MempoolMessage }{},
	binary.ConcreteType{&TxMessage{}, msgTypeTx},
)

func DecodeMessage(bz []byte) (msgType byte, msg MempoolMessage, err error) {
	msgType = bz[0]
	n := new(int64)
	r := bytes.NewReader(bz)
	msg = binary.ReadBinary(struct{ MempoolMessage }{}, r, n, &err).(struct{ MempoolMessage }).MempoolMessage
	return
}

//-------------------------------------

type TxMessage struct {
	Tx types.Tx
}

func (m *TxMessage) String() string {
	return fmt.Sprintf("[TxMessage %v]", m.Tx)
}
