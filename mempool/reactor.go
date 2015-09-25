package mempool

import (
	"bytes"
	"fmt"
	"reflect"
	"time"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/wire"
)

var (
	MempoolChannel = byte(0x30)

	checkExecutedTxsMilliseconds = 1  // check for new mempool txs to send to peer
	txsToSendPerCheck            = 64 // send up to this many txs from the mempool per check
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
func (memR *MempoolReactor) AddPeer(peer *p2p.Peer) {
	// Each peer gets a go routine on which we broadcast transactions in the same order we applied them to our state.
	go memR.broadcastTxRoutine(peer)
}

// Implements Reactor
func (memR *MempoolReactor) RemovePeer(peer *p2p.Peer, reason interface{}) {
	// broadcast routine checks if peer is gone and returns
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
		// broadcasting happens from go routines per peer
	default:
		log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Just an alias for AddTx since broadcasting happens in peer routines
func (memR *MempoolReactor) BroadcastTx(tx types.Tx) error {
	return memR.Mempool.AddTx(tx)
}

type PeerState interface {
	GetHeight() int
}

// send new mempool txs to peer, strictly in order we applied them to our state.
// new blocks take chunks out of the mempool, but we've already sent some txs to the peer.
// so we wait to hear that the peer has progressed to the new height, and then continue sending txs from where we left off
func (memR *MempoolReactor) broadcastTxRoutine(peer *p2p.Peer) {
	newBlockChan := make(chan ResetInfo)
	memR.evsw.(*events.EventSwitch).AddListenerForEvent("broadcastRoutine:"+peer.Key, types.EventStringNewBlock(), func(data types.EventData) {
		// no lock needed because consensus is blocking on this
		// and the mempool is reset before this event fires
		newBlockChan <- memR.Mempool.resetInfo
	})
	timer := time.NewTicker(time.Millisecond * time.Duration(checkExecutedTxsMilliseconds))
	currentHeight := memR.Mempool.state.LastBlockHeight
	var nTxs, txsSent int
	var txs []types.Tx
	for {
		select {
		case <-timer.C:
			if !peer.IsRunning() {
				return
			}

			// make sure the peer is up to date
			peerState := peer.Data.Get(types.PeerStateKey).(PeerState)
			if peerState.GetHeight() < currentHeight {
				continue
			}

			// check the mempool for new transactions
			nTxs, txs = memR.getNewTxs(txsSent, currentHeight)

			theseTxsSent := 0
			start := time.Now()
		TX_LOOP:
			for _, tx := range txs {
				// send tx to peer.
				msg := &TxMessage{Tx: tx}
				success := peer.Send(MempoolChannel, msg)
				if !success {
					break TX_LOOP
				} else {
					theseTxsSent += 1
				}
			}
			if theseTxsSent > 0 {
				txsSent += theseTxsSent
				log.Warn("Sent txs to peer", "ntxs", theseTxsSent, "took", time.Since(start), "total_sent", txsSent, "total_exec", nTxs)
			}

		case ri := <-newBlockChan:
			currentHeight = ri.Height

			// find out how many txs below what we've sent were included in a block and how many became invalid
			included := tallyRangesUpTo(ri.Included, txsSent)
			invalidated := tallyRangesUpTo(ri.Invalid, txsSent)

			txsSent -= included + invalidated
		}
	}
}

// fetch new txs from the mempool
func (memR *MempoolReactor) getNewTxs(txsSent, height int) (nTxs int, txs []types.Tx) {
	memR.Mempool.mtx.Lock()
	defer memR.Mempool.mtx.Unlock()

	// if the mempool got ahead of us just return empty txs
	if memR.Mempool.state.LastBlockHeight != height {
		return
	}

	nTxs = len(memR.Mempool.txs)
	if txsSent < nTxs {
		if nTxs > txsSent+txsToSendPerCheck {
			txs = memR.Mempool.txs[txsSent : txsSent+txsToSendPerCheck]
		} else {
			txs = memR.Mempool.txs[txsSent:]
		}
	}
	return
}

// return the size of ranges less than upTo
func tallyRangesUpTo(ranger []Range, upTo int) int {
	totalUpTo := 0
	for _, r := range ranger {
		if r.Start >= upTo {
			break
		}
		if r.Start+r.Length-1 > upTo {
			totalUpTo += upTo - r.Start - 1
			break
		}
		totalUpTo += r.Length
	}
	return totalUpTo
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
