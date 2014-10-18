package consensus

import (
	"testing"
	"time"

	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/state"
)

func randAccountDetail(id uint64, status byte) (*state.AccountDetail, *state.PrivAccount) {
	privAccount := state.GenPrivAccount()
	privAccount.Id = id
	account := privAccount.Account
	return &state.AccountDetail{
		Account:  account,
		Sequence: RandUInt(),
		Balance:  RandUInt64() + 1000, // At least 1000.
		Status:   status,
	}, privAccount
}

// The first numValidators accounts are validators.
func randGenesisState(numAccounts int, numValidators int) (*state.State, []*state.PrivAccount) {
	db := db_.NewMemDB()
	accountDetails := make([]*state.AccountDetail, numAccounts)
	privAccounts := make([]*state.PrivAccount, numAccounts)
	for i := 0; i < numAccounts; i++ {
		if i < numValidators {
			accountDetails[i], privAccounts[i] =
				randAccountDetail(uint64(i), state.AccountStatusBonded)
		} else {
			accountDetails[i], privAccounts[i] =
				randAccountDetail(uint64(i), state.AccountStatusNominal)
		}
	}
	s0 := state.GenesisState(db, time.Now(), accountDetails)
	s0.Save(time.Now())
	return s0, privAccounts
}

func makeConsensusState() (*ConsensusState, []*state.PrivAccount) {
	state, privAccounts := randGenesisState(20, 10)
	blockStore := NewBlockStore(db_.NewMemDB())
	mempool := mempool.NewMempool(nil, state)
	cs := NewConsensusState(state, blockStore, mempool)
	return cs, privAccounts
}

func TestUnit(t *testing.T) {
	cs, privAccounts := makeConsensusState()
	rs := cs.GetRoundState()
	t.Log(rs)
	if false {
		t.Log(privAccounts)
	}
}
