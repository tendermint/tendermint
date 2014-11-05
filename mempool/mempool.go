/*
Mempool receives new transactions and applies them to the latest committed state.
If the transaction is acceptable, then it broadcasts the tx to peers.

When this node happens to be the next proposer, it simply takes the recently
modified state (and the associated transactions) and use that as the proposal.
*/

package mempool

import (
	"sync"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	"github.com/tendermint/tendermint/state"
)

type Mempool struct {
	mtx   sync.Mutex
	state *state.State
	txs   []Tx
}

func NewMempool(state *state.State) *Mempool {
	return &Mempool{
		state: state,
	}
}

// Apply tx to the state and remember it.
func (mem *Mempool) AddTx(tx Tx) (err error) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	err = mem.state.ExecTx(tx)
	if err != nil {
		return err
	} else {
		mem.txs = append(mem.txs, tx)
		return nil
	}
}

func (mem *Mempool) GetProposalTxs() []Tx {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	return mem.txs
}

// "block" is the new block being committed.
// "state" is the result of state.AppendBlock("block").
// Txs that are present in "block" are discarded from mempool.
// Txs that have become invalid in the new "state" are also discarded.
func (mem *Mempool) ResetForBlockAndState(block *Block, state *state.State) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	mem.state = state.Copy()

	// First, create a lookup map of txns in new block.
	blockTxsMap := make(map[string]struct{})
	for _, tx := range block.Data.Txs {
		txHash := BinaryHash(tx)
		blockTxsMap[string(txHash)] = struct{}{}
	}

	// Next, filter all txs from mem.txs that are in blockTxsMap
	txs := []Tx{}
	for _, tx := range mem.txs {
		txHash := BinaryHash(tx)
		if _, ok := blockTxsMap[string(txHash)]; ok {
			continue
		} else {
			txs = append(txs, tx)
		}
	}

	// Next, filter all txs that aren't valid given new state.
	validTxs := []Tx{}
	for _, tx := range txs {
		err := mem.state.ExecTx(tx)
		if err != nil {
			validTxs = append(validTxs, tx)
		} else {
			// tx is no longer valid.
		}
	}

	// We're done!
	mem.txs = validTxs
}
