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

There are two types of transactions -- consensus txs (e.g. bonding / unbonding /
timeout / dupeout txs) and everything else. They are stored separately to allow
nodes to only request the kind they need.
TODO: make use of this potential feature when the time comes.

For simplicity we evaluate the consensus transactions after everything else.
*/

//-----------------------------------------------------------------------------

type Mempool struct {
	mtx   sync.Mutex
	state *State
	txs   []Tx // Regular transactions
	ctxs  []Tx // Validator related transactions
}

func NewMempool(state *State) *Mempool {
	return &Mempool{
		state: state,
	}
}

func (mem *Mempool) AddTx(tx Tx) bool {
	mem.mtx.Lock()
	defer mem.mtx.Unlock()
	if tx.IsConsensus() {
		// Remember consensus tx for later staging.
		// We only keep 1 tx for each validator. TODO what? what about bonding?
		// TODO talk about prioritization.
		mem.ctxs = append(mem.ctxs, tx)
	} else {
		mem.txs = append(mem.txs, tx)
	}
}

func (mem *Mempool) CollectForState() {
}
