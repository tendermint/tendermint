/*
Mempool receives new transactions and applies them to the latest committed state.
If the transaction is acceptable, then it broadcasts the tx to peers.

When this node happens to be the next proposer, it simply uses the recently
modified state (and the associated transactions) to construct a proposal.
*/

package mempool

import (
	"sync"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type Mempool struct {
	mtx   sync.Mutex
	state *sm.State
	cache *sm.BlockCache
	txs   []types.Tx
}

func NewMempool(state *sm.State) *Mempool {
	return &Mempool{
		state: state,
		cache: sm.NewBlockCache(state),
	}
}

func (mem *Mempool) GetState() *sm.State {
	return mem.state
}

func (mem *Mempool) GetCache() *sm.BlockCache {
	return mem.cache
}

// Apply tx to the state and remember it.
func (mem *Mempool) AddTx(tx types.Tx) (err error) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	err = sm.ExecTx(mem.cache, tx, false, nil)
	if err != nil {
		log.Info("AddTx() error", "tx", tx, "error", err)
		return err
	} else {
		log.Info("AddTx() success", "tx", tx)
		mem.txs = append(mem.txs, tx)
		return nil
	}
}

func (mem *Mempool) GetProposalTxs() []types.Tx {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	log.Info("GetProposalTxs:", "txs", mem.txs)
	return mem.txs
}

// "block" is the new block being committed.
// "state" is the result of state.AppendBlock("block").
// Txs that are present in "block" are discarded from mempool.
// Txs that have become invalid in the new "state" are also discarded.
func (mem *Mempool) ResetForBlockAndState(block *types.Block, state *sm.State) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	mem.state = state.Copy()
	mem.cache = sm.NewBlockCache(mem.state)

	// First, create a lookup map of txns in new block.
	blockTxsMap := make(map[string]struct{})
	for _, tx := range block.Data.Txs {
		blockTxsMap[string(types.TxID(state.ChainID, tx))] = struct{}{}
	}

	// Next, filter all txs from mem.txs that are in blockTxsMap
	txs := []types.Tx{}
	for _, tx := range mem.txs {
		txID := types.TxID(state.ChainID, tx)
		if _, ok := blockTxsMap[string(txID)]; ok {
			log.Info("Filter out, already committed", "tx", tx, "txID", txID)
			continue
		} else {
			log.Info("Filter in, still new", "tx", tx, "txID", txID)
			txs = append(txs, tx)
		}
	}

	// Next, filter all txs that aren't valid given new state.
	validTxs := []types.Tx{}
	for _, tx := range txs {
		err := sm.ExecTx(mem.cache, tx, false, nil)
		if err == nil {
			log.Info("Filter in, valid", "tx", tx)
			validTxs = append(validTxs, tx)
		} else {
			// tx is no longer valid.
			log.Info("Filter out, no longer valid", "tx", tx, "error", err)
		}
	}

	// We're done!
	log.Info("New txs", "txs", validTxs, "oldTxs", mem.txs)
	mem.txs = validTxs
}
