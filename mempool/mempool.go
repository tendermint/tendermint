package mempool

import (
	"sync"

	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/state"
)

/*
Mempool receives new transactions and applies them to the latest committed state.
If the transaction is acceptable, then it broadcasts a fingerprint to peers.

The transaction fingerprint is a short sequence of bytes (shorter than a full hash).
Each peer connection uses a different algorithm for turning the tx hash into a
fingerprint in order to prevent transaction blocking attacks. Upon inspecting a
tx fingerprint, the receiver may query the source for the full tx bytes.

When this node happens to be the next proposer, it simply takes the recently
modified state (and the associated transactions) and use that as the proposal.
*/

//-----------------------------------------------------------------------------

type Mempool struct {
	mtx   sync.Mutex
	state *State
	txs   []Tx
}

func NewMempool(state *State) *Mempool {
	return &Mempool{
		state: state,
	}
}

func (mem *Mempool) AddTx(tx Tx) (err error) {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	// Add the tx to the state.
	err = mem.state.CommitTx(tx)
	if err != nil {
		return err
	} else {
		mem.txs = append(mem.txs, tx)
		return nil
	}
}
