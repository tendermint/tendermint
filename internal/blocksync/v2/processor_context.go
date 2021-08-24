package v2

import (
	"fmt"

	cons "github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/pkg/block"
	"github.com/tendermint/tendermint/pkg/metadata"
	"github.com/tendermint/tendermint/state"
)

type processorContext interface {
	applyBlock(blockID metadata.BlockID, block *block.Block) error
	verifyCommit(chainID string, blockID metadata.BlockID, height int64, commit *metadata.Commit) error
	saveBlock(block *block.Block, blockParts *metadata.PartSet, seenCommit *metadata.Commit)
	tmState() state.State
	setState(state.State)
	recordConsMetrics(block *block.Block)
}

type pContext struct {
	store   blockStore
	applier blockApplier
	state   state.State
	metrics *cons.Metrics
}

func newProcessorContext(st blockStore, ex blockApplier, s state.State, m *cons.Metrics) *pContext {
	return &pContext{
		store:   st,
		applier: ex,
		state:   s,
		metrics: m,
	}
}

func (pc *pContext) applyBlock(blockID metadata.BlockID, block *block.Block) error {
	newState, err := pc.applier.ApplyBlock(pc.state, blockID, block)
	pc.state = newState
	return err
}

func (pc pContext) tmState() state.State {
	return pc.state
}

func (pc *pContext) setState(state state.State) {
	pc.state = state
}

func (pc pContext) verifyCommit(chainID string, blockID metadata.BlockID, height int64, commit *metadata.Commit) error {
	return pc.state.Validators.VerifyCommitLight(chainID, blockID, height, commit)
}

func (pc *pContext) saveBlock(block *block.Block, blockParts *metadata.PartSet, seenCommit *metadata.Commit) {
	pc.store.SaveBlock(block, blockParts, seenCommit)
}

func (pc *pContext) recordConsMetrics(block *block.Block) {
	pc.metrics.RecordConsMetrics(block)
}

type mockPContext struct {
	applicationBL  []int64
	verificationBL []int64
	state          state.State
}

func newMockProcessorContext(
	state state.State,
	verificationBlackList []int64,
	applicationBlackList []int64) *mockPContext {
	return &mockPContext{
		applicationBL:  applicationBlackList,
		verificationBL: verificationBlackList,
		state:          state,
	}
}

func (mpc *mockPContext) applyBlock(blockID metadata.BlockID, block *block.Block) error {
	for _, h := range mpc.applicationBL {
		if h == block.Height {
			return fmt.Errorf("generic application error")
		}
	}
	mpc.state.LastBlockHeight = block.Height
	return nil
}

func (mpc *mockPContext) verifyCommit(chainID string, blockID metadata.BlockID, height int64, commit *metadata.Commit) error {
	for _, h := range mpc.verificationBL {
		if h == height {
			return fmt.Errorf("generic verification error")
		}
	}
	return nil
}

func (mpc *mockPContext) saveBlock(block *block.Block, blockParts *metadata.PartSet, seenCommit *metadata.Commit) {

}

func (mpc *mockPContext) setState(state state.State) {
	mpc.state = state
}

func (mpc *mockPContext) tmState() state.State {
	return mpc.state
}

func (mpc *mockPContext) recordConsMetrics(block *block.Block) {

}
