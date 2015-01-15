/*
Mempool receives new transactions and applies them to the latest committed state.
If the transaction is acceptable, then it broadcasts the tx to peers.

When this node happens to be the next proposer, it simply uses the recently
modified state (and the associated transactions) to construct a proposal.
*/

package mempool

import (
	"sync"

	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/block"
	sm "github.com/tendermint/tendermint/state"
)

type Mempool struct {
	mtx   sync.Mutex
	state *sm.State
	txs   []block.Tx
}

func NewMempool(state *sm.State) *Mempool {
	return &Mempool{
		state: state,
	}
}

// Apply tx to the state and remember it.
func (mem *Mempool) AddTx(tx block.Tx) (err error) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	err = mem.state.ExecTx(tx)
	if err != nil {
		log.Debug("AddTx() error", "tx", tx, "error", err)
		return err
	} else {
		log.Debug("AddTx() success", "tx", tx)
		mem.txs = append(mem.txs, tx)
		return nil
	}
}

func (mem *Mempool) GetProposalTxs() []block.Tx {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	log.Debug("GetProposalTxs:", "txs", mem.txs)
	return mem.txs
}

// "block" is the new block being committed.
// "state" is the result of state.AppendBlock("block").
// Txs that are present in "block" are discarded from mempool.
// Txs that have become invalid in the new "state" are also discarded.
func (mem *Mempool) ResetForBlockAndState(block_ *block.Block, state *sm.State) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	mem.state = state.Copy()

	// First, create a lookup map of txns in new block.
	blockTxsMap := make(map[string]struct{})
	for _, tx := range block_.Data.Txs {
		txHash := binary.BinarySha256(tx)
		blockTxsMap[string(txHash)] = struct{}{}
	}

	// Next, filter all txs from mem.txs that are in blockTxsMap
	txs := []block.Tx{}
	for _, tx := range mem.txs {
		txHash := binary.BinarySha256(tx)
		if _, ok := blockTxsMap[string(txHash)]; ok {
			log.Debug("Filter out, already committed", "tx", tx, "txHash", txHash)
			continue
		} else {
			log.Debug("Filter in, still new", "tx", tx, "txHash", txHash)
			txs = append(txs, tx)
		}
	}

	// Next, filter all txs that aren't valid given new state.
	validTxs := []block.Tx{}
	for _, tx := range txs {
		err := mem.state.ExecTx(tx)
		if err == nil {
			log.Debug("Filter in, valid", "tx", tx)
			validTxs = append(validTxs, tx)
		} else {
			// tx is no longer valid.
			log.Debug("Filter out, no longer valid", "tx", tx, "error", err)
		}
	}

	// We're done!
	log.Debug("New txs", "txs", validTxs, "oldTxs", mem.txs)
	mem.txs = validTxs
}
