package mempool

import (
	"bytes"
	"fmt"
	"sync/atomic"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	"github.com/tendermint/tendermint/p2p"
)

var (
	MempoolCh = byte(0x30)
)

// MempoolReactor handles mempool tx broadcasting amongst peers.
type MempoolReactor struct {
	sw      *p2p.Switch
	quit    chan struct{}
	started uint32
	stopped uint32

	Mempool *Mempool
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
			Id:       MempoolCh,
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
	_, msg_ := decodeMessage(msgBytes)
	log.Info("MempoolReactor received %v", msg_)

	switch msg_.(type) {
	case *TxMessage:
		msg := msg_.(*TxMessage)
		err := memR.Mempool.AddTx(msg.Tx)
		if err != nil {
			// Bad, seen, or conflicting tx.
			log.Debug("Could not add tx %v", msg.Tx)
			return
		} else {
			log.Debug("Added valid tx %V", msg.Tx)
		}
		// Share tx.
		// We use a simple shotgun approach for now.
		// TODO: improve efficiency
		for _, peer := range memR.sw.Peers().List() {
			if peer.Key == src.Key {
				continue
			}
			peer.TrySend(MempoolCh, msg)
		}

	default:
		// Ignore unknown message
	}
}

func (memR *MempoolReactor) BroadcastTx(tx Tx) error {
	err := memR.Mempool.AddTx(tx)
	if err != nil {
		return err
	}
	msg := &TxMessage{Tx: tx}
	memR.sw.Broadcast(MempoolCh, msg)
	return nil
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeUnknown = byte(0x00)
	msgTypeTx      = byte(0x10)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz []byte) (msgType byte, msg interface{}) {
	n, err := new(int64), new(error)
	msgType = bz[0]
	switch msgType {
	case msgTypeTx:
		msg = ReadBinary(&TxMessage{}, bytes.NewReader(bz[1:]), n, err)
	default:
		msg = nil
	}
	return
}

//-------------------------------------

type TxMessage struct {
	Tx Tx
}

func (m *TxMessage) TypeByte() byte { return msgTypeTx }

func (m *TxMessage) String() string {
	return fmt.Sprintf("[TxMessage %v]", m.Tx)
}
