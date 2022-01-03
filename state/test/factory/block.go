package factory

import (
	"time"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/test/factory"
	"github.com/tendermint/tendermint/types"
)

func MakeBlocks(n int, state *sm.State, privVal types.PrivValidator) ([]*types.Block, error) {
	blocks := make([]*types.Block, n)

	var (
		prevBlock     *types.Block
		prevBlockMeta *types.BlockMeta
	)

	appHeight := byte(0x01)
	for i := 0; i < n; i++ {
		height := int64(i + 1)

		block, parts, err := makeBlockAndPartSet(*state, prevBlock, prevBlockMeta, privVal, height)
		if err != nil {
			return nil, err
		}

		blocks[i] = block

		prevBlock = block
		prevBlockMeta = types.NewBlockMeta(block, parts)

		// update state
		state.AppHash = []byte{appHeight}
		appHeight++
		state.LastBlockHeight = height
	}

	return blocks, nil
}

func MakeBlock(state sm.State, height int64, c *types.Commit) (*types.Block, error) {
	block, _, err := state.MakeBlock(
		height,
		factory.MakeTenTxs(state.LastBlockHeight),
		c,
		nil,
		state.Validators.GetProposer().Address,
	)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func makeBlockAndPartSet(
	state sm.State,
	lastBlock *types.Block,
	lastBlockMeta *types.BlockMeta,
	privVal types.PrivValidator,
	height int64,
) (*types.Block, *types.PartSet, error) {
	lastCommit := types.NewCommit(height-1, 0, types.BlockID{}, nil)
	if height > 1 {
		vote, _ := types.MakeVote(
			lastBlock.Header.Height,
			lastBlockMeta.BlockID,
			state.Validators,
			privVal,
			lastBlock.Header.ChainID,
			time.Now())
		lastCommit = types.NewCommit(vote.Height, vote.Round,
			lastBlockMeta.BlockID, []types.CommitSig{vote.CommitSig()})
	}

	return state.MakeBlock(height, []types.Tx{}, lastCommit, nil, state.Validators.GetProposer().Address)
}
