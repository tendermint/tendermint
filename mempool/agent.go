package mempool

import (
	"bytes"
	"fmt"
	"io"
	"sync/atomic"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	"github.com/tendermint/tendermint/p2p"
)

var (
	MempoolCh = byte(0x30)
)

// MempoolAgent handles mempool tx broadcasting amongst peers.
type MempoolAgent struct {
	sw       *p2p.Switch
	swEvents chan interface{}
	quit     chan struct{}
	started  uint32
	stopped  uint32

	mempool *Mempool
}

func NewMempoolAgent(sw *p2p.Switch, mempool *Mempool) *MempoolAgent {
	swEvents := make(chan interface{})
	sw.AddEventListener("MempoolAgent.swEvents", swEvents)
	memA := &MempoolAgent{
		sw:       sw,
		swEvents: swEvents,
		quit:     make(chan struct{}),
		mempool:  mempool,
	}
	return memA
}

func (memA *MempoolAgent) Start() {
	if atomic.CompareAndSwapUint32(&memA.started, 0, 1) {
		log.Info("Starting MempoolAgent")
		go memA.switchEventsRoutine()
		go memA.gossipTxRoutine()
	}
}

func (memA *MempoolAgent) Stop() {
	if atomic.CompareAndSwapUint32(&memA.stopped, 0, 1) {
		log.Info("Stopping MempoolAgent")
		close(memA.quit)
		close(memA.swEvents)
	}
}

func (memA *MempoolAgent) BroadcastTx(tx Tx) error {
	err := memA.mempool.AddTx(tx)
	if err != nil {
		return err
	}
	msg := &TxMessage{Tx: tx}
	memA.sw.Broadcast(MempoolCh, msg)
	return nil
}

// Handle peer new/done events
func (memA *MempoolAgent) switchEventsRoutine() {
	for {
		swEvent, ok := <-memA.swEvents
		if !ok {
			break
		}
		switch swEvent.(type) {
		case p2p.SwitchEventNewPeer:
			// event := swEvent.(p2p.SwitchEventNewPeer)
		case p2p.SwitchEventDonePeer:
			// event := swEvent.(p2p.SwitchEventDonePeer)
		default:
			log.Warning("Unhandled switch event type")
		}
	}
}

func (memA *MempoolAgent) gossipTxRoutine() {
OUTER_LOOP:
	for {
		// Receive incoming message on MempoolCh
		inMsg, ok := memA.sw.Receive(MempoolCh)
		if !ok {
			break OUTER_LOOP // Client has stopped
		}
		_, msg_ := decodeMessage(inMsg.Bytes)
		log.Info("gossipTxRoutine received %v", msg_)

		switch msg_.(type) {
		case *TxMessage:
			msg := msg_.(*TxMessage)
			err := memA.mempool.AddTx(msg.Tx)
			if err != nil {
				// Bad, seen, or conflicting tx.
				log.Debug("Could not add tx %v", msg.Tx)
				continue OUTER_LOOP
			} else {
				log.Debug("Added valid tx %V", msg.Tx)
			}
			// Share tx.
			// We use a simple shotgun approach for now.
			// TODO: improve efficiency
			for _, peer := range memA.sw.Peers().List() {
				if peer.Key == inMsg.MConn.Peer.Key {
					continue
				}
				peer.TrySend(MempoolCh, msg)
			}

		default:
			// Ignore unknown message
			// memA.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
		}
	}

	// Cleanup
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
	// log.Debug("decoding msg bytes: %X", bz)
	msgType = bz[0]
	switch msgType {
	case msgTypeTx:
		msg = readTxMessage(bytes.NewReader(bz[1:]), n, err)
	// case ...:
	default:
		msg = nil
	}
	return
}

//-------------------------------------

type TxMessage struct {
	Tx Tx
}

func readTxMessage(r io.Reader, n *int64, err *error) *TxMessage {
	return &TxMessage{
		Tx: ReadTx(r, n, err),
	}
}

func (m *TxMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeTx, &n, &err)
	WriteBinary(w, m.Tx, &n, &err)
	return
}

func (m *TxMessage) String() string {
	return fmt.Sprintf("[TxMessage %v]", m.Tx)
}
