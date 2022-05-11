package factory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/types"
)

func MakeBlocks(ctx context.Context, t *testing.T, n int, state *sm.State, privVal types.PrivValidator) []*types.Block {
	t.Helper()

	blocks := make([]*types.Block, n)

	var (
		prevBlock     *types.Block
		prevBlockMeta *types.BlockMeta
	)

	appHeight := byte(0x01)
	for i := 0; i < n; i++ {
		height := int64(i + 1)

		block, parts := makeBlockAndPartSet(ctx, t, *state, prevBlock, prevBlockMeta, privVal, height)

		blocks[i] = block

		prevBlock = block
		prevBlockMeta = types.NewBlockMeta(block, parts)

		// update state
		state.AppHash = []byte{appHeight}
		appHeight++
		state.LastBlockHeight = height
	}

	return blocks
}

func MakeBlock(state sm.State, height int64, c *types.Commit) *types.Block {
	return state.MakeBlock(
		height,
		factory.MakeNTxs(state.LastBlockHeight, 10),
		c,
		nil,
		state.Validators.GetProposer().Address,
	)
}

func makeBlockAndPartSet(
	ctx context.Context,
	t *testing.T,
	state sm.State,
	lastBlock *types.Block,
	lastBlockMeta *types.BlockMeta,
	privVal types.PrivValidator,
	height int64,
) (*types.Block, *types.PartSet) {
	t.Helper()

	lastCommit := &types.Commit{Height: height - 1}
	if height > 1 {
		vote, err := factory.MakeVote(
			ctx,
			privVal,
			lastBlock.Header.ChainID,
			1, lastBlock.Header.Height, 0, 2,
			lastBlockMeta.BlockID,
			time.Now())
		require.NoError(t, err)
		lastCommit = &types.Commit{
			Height:     vote.Height,
			Round:      vote.Round,
			BlockID:    lastBlock.LastBlockID,
			Signatures: []types.CommitSig{vote.CommitSig()},
		}
	}

	block := state.MakeBlock(height, []types.Tx{}, lastCommit, nil, state.Validators.GetProposer().Address)
	partSet, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)

	return block, partSet
}
