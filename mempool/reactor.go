package mempool

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/events"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var (
	MempoolChannel = byte(0x30)

	checkExecutedTxsMilliseconds = 1       // check for new mempool txs to send to peer
	txsToSendPerCheck            = 64      // send up to this many txs from the mempool per check
	newBlockChCapacity           = 100     // queue to process this many ResetInfos per peer
	maxMempoolMessageSize        = 1048576 // 1MB TODO make it configurable
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
	newBlockChan := make(chan ResetInfo, newBlockChCapacity)
	peer.Data.Set(types.PeerMempoolChKey, newBlockChan)
	timer := time.NewTicker(time.Millisecond * time.Duration(checkExecutedTxsMilliseconds))
	go memR.broadcastTxRoutine(timer.C, newBlockChan, peer)
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

// "block" is the new block being committed.
// "state" is the result of state.AppendBlock("block").
// Txs that are present in "block" are discarded from mempool.
// Txs that have become invalid in the new "state" are also discarded.
func (memR *MempoolReactor) ResetForBlockAndState(block *types.Block, state *sm.State) {
	ri := memR.Mempool.ResetForBlockAndState(block, state)
	for _, peer := range memR.Switch.Peers().List() {
		peerMempoolCh := peer.Data.Get(types.PeerMempoolChKey).(chan ResetInfo)
		select {
		case peerMempoolCh <- ri:
		default:
			memR.Switch.StopPeerForError(peer, errors.New("Peer's mempool push channel full"))
		}
	}
}

// Just an alias for AddTx since broadcasting happens in peer routines
func (memR *MempoolReactor) BroadcastTx(tx types.Tx) error {
	return memR.Mempool.AddTx(tx)
}

type PeerState interface {
	GetHeight() int
}

type Peer interface {
	IsRunning() bool
	Send(byte, interface{}) bool
	Get(string) interface{}
}

// send new mempool txs to peer, strictly in order we applied them to our state.
// new blocks take chunks out of the mempool, but we've already sent some txs to the peer.
// so we wait to hear that the peer has progressed to the new height, and then continue sending txs from where we left off
func (memR *MempoolReactor) broadcastTxRoutine(tickerChan <-chan time.Time, newBlockChan chan ResetInfo, peer Peer) {
	var height = memR.Mempool.GetHeight()
	var txsSent int // new txs sent for height. (reset every new height)

	for {
		select {
		case <-tickerChan:
			if !peer.IsRunning() {
				return
			}

			// make sure the peer is up to date
			if peerState_i := peer.Get(types.PeerStateKey); peerState_i != nil {
				peerState := peerState_i.(PeerState)
				if peerState.GetHeight() < height {
					continue
				}
			} else {
				continue
			}

			// check the mempool for new transactions
			newTxs := memR.getNewTxs(height)
			txsSentLoop := 0
			start := time.Now()

		TX_LOOP:
			for i := txsSent; i < len(newTxs) && txsSentLoop < txsToSendPerCheck; i++ {
				tx := newTxs[i]
				msg := &TxMessage{Tx: tx}
				success := peer.Send(MempoolChannel, msg)
				if !success {
					break TX_LOOP
				} else {
					txsSentLoop += 1
				}
			}

			if txsSentLoop > 0 {
				txsSent += txsSentLoop
				log.Info("Sent txs to peer", "txsSentLoop", txsSentLoop,
					"took", time.Since(start), "txsSent", txsSent, "newTxs", len(newTxs))
			}

		case ri := <-newBlockChan:
			height = ri.Height

			// find out how many txs below what we've sent were included in a block and how many became invalid
			included := tallyRangesUpTo(ri.Included, txsSent)
			invalidated := tallyRangesUpTo(ri.Invalid, txsSent)

			txsSent -= included + invalidated
		}
	}
}

// fetch new txs from the mempool
func (memR *MempoolReactor) getNewTxs(height int) (txs []types.Tx) {
	memR.Mempool.mtx.Lock()
	defer memR.Mempool.mtx.Unlock()

	// if the mempool got ahead of us just return empty txs
	if memR.Mempool.state.LastBlockHeight != height {
		return
	}
	return memR.Mempool.txs
}

// return the size of ranges less than upTo
func tallyRangesUpTo(ranger []Range, upTo int) int {
	totalUpTo := 0
	for _, r := range ranger {
		if r.Start >= upTo {
			break
		}
		if r.Start+r.Length >= upTo {
			totalUpTo += upTo - r.Start
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
	n := new(int)
	r := bytes.NewReader(bz)
	msg = wire.ReadBinary(struct{ MempoolMessage }{}, r, maxMempoolMessageSize, n, &err).(struct{ MempoolMessage }).MempoolMessage
	return
}

//-------------------------------------

type TxMessage struct {
	Tx types.Tx
}

func (m *TxMessage) String() string {
	return fmt.Sprintf("[TxMessage %v]", m.Tx)
}
