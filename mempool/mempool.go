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
	txs   []types.Tx // TODO: we need to add a map to facilitate replace-by-fee
}

func NewMempool(state *sm.State) *Mempool {
	return &Mempool{
		state: state,
	}
}

func (mem *Mempool) GetState() *sm.State {
	return mem.state
}

func (mem *Mempool) GetHeight() int {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	return mem.state.LastBlockHeight
}

// Apply tx to the state and remember it.
func (mem *Mempool) AddTx(tx types.Tx) (err error) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	err = sm.ExecTx(mem.state, tx, nil)
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

// We use this to inform peer routines of how the mempool has been updated
type ResetInfo struct {
	Height   int
	Included []Range
	Invalid  []Range
}

type Range struct {
	Start  int
	Length int
}

// "block" is the new block being committed.
// "state" is the result of state.AppendBlock("block").
// Txs that are present in "block" are discarded from mempool.
// Txs that have become invalid in the new "state" are also discarded.
func (mem *Mempool) ResetForBlockAndState(block *types.Block, state *sm.State) ResetInfo {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	mem.state = state.Copy()

	// First, create a lookup map of txns in new block.
	blockTxsMap := make(map[string]struct{})
	for _, tx := range block.Data.Txs {
		blockTxsMap[string(tx)] = struct{}{}
	}

	// Now we filter all txs from mem.txs that are in blockTxsMap,
	// and ExecTx on what remains. Only valid txs are kept.
	// We track the ranges of txs included in the block and invalidated by it
	// so we can tell peer routines
	var ri = ResetInfo{Height: block.Height}
	var validTxs []types.Tx
	includedStart, invalidStart := -1, -1
	for i, tx := range mem.txs {
		if _, ok := blockTxsMap[string(tx)]; ok {
			startRange(&includedStart, i)           // start counting included txs
			endRange(&invalidStart, i, &ri.Invalid) // stop counting invalid txs
			log.Info("Filter out, already committed", "tx", tx)
		} else {
			endRange(&includedStart, i, &ri.Included) // stop counting included txs
			err := sm.ExecTx(mem.state, tx, nil)
			if err != nil {
				startRange(&invalidStart, i) // start counting invalid txs
				log.Info("Filter out, no longer valid", "tx", tx, "error", err)
			} else {
				endRange(&invalidStart, i, &ri.Invalid) // stop counting invalid txs
				log.Info("Filter in, new, valid", "tx", tx)
				validTxs = append(validTxs, tx)
			}
		}
	}
	endRange(&includedStart, len(mem.txs)-1, &ri.Included) // stop counting included txs
	endRange(&invalidStart, len(mem.txs)-1, &ri.Invalid)   // stop counting invalid txs

	// We're done!
	log.Info("New txs", "txs", validTxs, "oldTxs", mem.txs)
	mem.txs = validTxs
	return ri
}

func startRange(start *int, i int) {
	if *start < 0 {
		*start = i
	}
}

func endRange(start *int, i int, ranger *[]Range) {
	if *start >= 0 {
		length := i - *start
		*ranger = append(*ranger, Range{*start, length})
		*start = -1
	}
}
