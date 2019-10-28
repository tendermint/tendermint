package v2

import (
	"fmt"

	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type processorContext interface {
	applyBlock(state state.State, blockID types.BlockID, block *types.Block) (state.State, error)
	verifyCommit(chainID string, blockID types.BlockID, height int64, commit *types.Commit) error
	saveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit)
}

type pContext struct {
	store   blockStore
	applier blockApplier
	state   state.State
}

func newProcessorContext(st blockStore, ex blockApplier, s state.State) *pContext {
	return &pContext{
		store:   st,
		applier: ex,
		state:   s,
	}
}

func (pc *pContext) applyBlock(state state.State, blockID types.BlockID, block *types.Block) (state.State, error) {
	state, err := pc.applier.ApplyBlock(state, blockID, block)
	return state, err
}

func (pc *pContext) verifyCommit(chainID string, blockID types.BlockID, height int64, commit *types.Commit) error {
	return pc.state.Validators.VerifyCommit(chainID, blockID, height, commit)
}

func (pc *pContext) saveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	pc.store.SaveBlock(block, blockParts, seenCommit)
}

type mockPContext struct {
	applicationBL  []int64
	verificationBL []int64
}

func newMockProcessorContext(verificationBlackList []int64, applicationBlackList []int64) *mockPContext {
	return &mockPContext{
		applicationBL:  applicationBlackList,
		verificationBL: verificationBlackList,
	}
}

func (mpc *mockPContext) applyBlock(state state.State, blockID types.BlockID, block *types.Block) (state.State, error) {
	for _, h := range mpc.applicationBL {
		if h == block.Height {
			return state, fmt.Errorf("generic application error")
		}
	}
	state.LastBlockHeight = block.Height
	return state, nil
}

func (mpc *mockPContext) verifyCommit(chainID string, blockID types.BlockID, height int64, commit *types.Commit) error {
	for _, h := range mpc.verificationBL {
		if h == height {
			return fmt.Errorf("generic verification error")
		}
	}
	return nil
}

func (mpc *mockPContext) saveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
}
