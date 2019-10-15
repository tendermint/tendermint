package v2

import (
	"fmt"

	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

type processorContext interface {
	applyBlock(state state.State, blockID types.BlockID, block *types.Block) (state.State, error)
	verifyCommit(chainID string, blockID types.BlockID, height int64, commit *types.Commit) error
	saveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit)
}

// nolint:unused
type pContext struct {
	store    *store.BlockStore
	executor *state.BlockExecutor
	state    *state.State
}

// nolint:unused,deadcode
func newProcessorContext(st *store.BlockStore, ex *state.BlockExecutor, s *state.State) *pContext {
	return &pContext{
		store:    st,
		executor: ex,
		state:    s,
	}
}

func (pc *pContext) applyBlock(state state.State, blockID types.BlockID, block *types.Block) (state.State, error) {
	return pc.executor.ApplyBlock(state, blockID, block)
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
