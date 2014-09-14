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
	. "github.com/tendermint/tendermint/state"
)

type Mempool struct {
	mtx       sync.Mutex
	lastBlock *Block
	state     *State
	txs       []Tx
}

func NewMempool(lastBlock *Block, state *State) *Mempool {
	return &Mempool{
		lastBlock: lastBlock,
		state:     state,
	}
}

// Apply tx to the state and remember it.
func (mem *Mempool) AddTx(tx Tx) (err error) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	err = mem.state.CommitTx(tx)
	if err != nil {
		return err
	} else {
		mem.txs = append(mem.txs, tx)
		return nil
	}
}

// Returns a new block from the current state and associated transactions.
// The block's Validation is empty, and some parts of the header too.
func (mem *Mempool) MakeProposalBlock() (*Block, *State) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	nextBlock := mem.lastBlock.MakeNextBlock()
	nextBlock.Data.Txs = mem.txs
	return nextBlock, mem.state
}

// Txs that are present in block are discarded from mempool.
// Txs that have become invalid in the new state are also discarded.
func (mem *Mempool) ResetForBlockAndState(block *Block, state *State) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	mem.lastBlock = block
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
		err := mem.state.CommitTx(tx)
		if err != nil {
			validTxs = append(validTxs, tx)
		} else {
			// tx is no longer valid.
		}
	}

	// We're done!
	mem.txs = validTxs
}
