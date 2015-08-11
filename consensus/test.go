package consensus

import (
	bc "github.com/tendermint/tendermint/blockchain"
	dbm "github.com/tendermint/tendermint/db"
	mempl "github.com/tendermint/tendermint/mempool"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

func randConsensusState() (*ConsensusState, []*types.PrivValidator) {
	state, _, privValidators := sm.RandGenesisState(20, false, 1000, 10, false, 1000)
	blockStore := bc.NewBlockStore(dbm.NewMemDB())
	mempool := mempl.NewMempool(state)
	mempoolReactor := mempl.NewMempoolReactor(mempool)
	cs := NewConsensusState(state, blockStore, mempoolReactor)
	return cs, privValidators
}
