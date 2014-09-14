package mempool

import (
	"bytes"
	"fmt"
	"io"
	"sync/atomic"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/p2p"
)

var (
	MempoolCh = byte(0x30)
)

// MempoolReactor handles mempool tx broadcasting amongst peers.
type MempoolReactor struct {
	sw      *Switch
	quit    chan struct{}
	started uint32
	stopped uint32

	mempool *Mempool
}

func NewMempoolReactor(sw *Switch, mempool *Mempool) *MempoolReactor {
	memR := &MempoolReactor{
		sw:      sw,
		quit:    make(chan struct{}),
		mempool: mempool,
	}
	return memR
}

func (memR *MempoolReactor) Start() {
	if atomic.CompareAndSwapUint32(&memR.started, 0, 1) {
		log.Info("Starting MempoolReactor")
	}
}

func (memR *MempoolReactor) Stop() {
	if atomic.CompareAndSwapUint32(&memR.stopped, 0, 1) {
		log.Info("Stopping MempoolReactor")
		close(memR.quit)
	}
}

func (memR *MempoolReactor) BroadcastTx(tx Tx) error {
	err := memR.mempool.AddTx(tx)
	if err != nil {
		return err
	}
	msg := &TxMessage{Tx: tx}
	memR.sw.Broadcast(MempoolCh, msg)
	return nil
}

// Implements Reactor
func (pexR *MempoolReactor) AddPeer(peer *Peer) {
}

// Implements Reactor
func (pexR *MempoolReactor) RemovePeer(peer *Peer, err error) {
}

func (memR *MempoolReactor) Receive(chId byte, src *Peer, msgBytes []byte) {
	_, msg_ := decodeMessage(msgBytes)
	log.Info("MempoolReactor received %v", msg_)

	switch msg_.(type) {
	case *TxMessage:
		msg := msg_.(*TxMessage)
		err := memR.mempool.AddTx(msg.Tx)
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
