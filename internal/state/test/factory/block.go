package factory

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/types"
)

func MakeBlocks(ctx context.Context, t *testing.T, n int, state *sm.State, privVal types.PrivValidator, proposedAppVersion uint64) []*types.Block {
	t.Helper()

	blocks := make([]*types.Block, n)

	var (
		prevBlock     *types.Block
		prevBlockMeta *types.BlockMeta
	)

	appHeight := byte(0x01)
	for i := 0; i < n; i++ {
		height := int64(i + 1)

		block, parts := makeBlockAndPartSet(ctx, t, *state, prevBlock, prevBlockMeta, privVal, height, proposedAppVersion)
		blocks = append(blocks, block)

		prevBlock = block
		prevBlockMeta = types.NewBlockMeta(block, parts)

		// update state
		appHash := make([]byte, crypto.DefaultAppHashSize)
		binary.BigEndian.PutUint64(appHash, uint64(height))
		changes, err := state.NewStateChangeset(ctx, &abci.ResponsePrepareProposal{
			AppHash: appHash,
		})
		require.NoError(t, err)
		err = changes.UpdateState(ctx, state)
		assert.NoError(t, err)
		appHeight++
		state.LastBlockHeight = height
	}

	return blocks
}

func MakeBlock(state sm.State, height int64, c *types.Commit, proposedAppVersion uint64) (*types.Block, error) {
	if state.LastBlockHeight != (height - 1) {
		return nil, fmt.Errorf("requested height %d should be 1 more than last block height %d", height, state.LastBlockHeight)
	}
	block := state.MakeBlock(
		height,
		factory.MakeNTxs(state.LastBlockHeight, 10),
		c,
		nil,
		state.Validators.GetProposer().ProTxHash,
		proposedAppVersion,
	)
	var err error
	block.AppHash = make([]byte, crypto.DefaultAppHashSize)
	if block.ResultsHash, err = abci.TxResultsHash(factory.ExecTxResults(block.Txs)); err != nil {
		return nil, err
	}

	return block, nil
}

func makeBlockAndPartSet(
	ctx context.Context,
	t *testing.T,
	state sm.State,
	lastBlock *types.Block,
	lastBlockMeta *types.BlockMeta,
	privVal types.PrivValidator,
	height int64,
	proposedAppVersion uint64,
) (*types.Block, *types.PartSet) {
	t.Helper()

	quorumSigns := &types.CommitSigns{QuorumHash: state.LastValidators.QuorumHash}
	lastCommit := types.NewCommit(height-1, 0, types.BlockID{}, state.LastStateID, quorumSigns)
	if height > 1 {
		vote, err := factory.MakeVote(
			ctx,
			privVal,
			state.Validators,
			lastBlock.Header.ChainID,
			1, lastBlock.Header.Height, 0, 2,
			lastBlockMeta.BlockID,
			lastBlock.StateID(),
		)
		require.NoError(t, err)
		thresholdSigns, err := types.NewSignsRecoverer([]*types.Vote{vote}).Recover()
		require.NoError(t, err)
		lastCommit = types.NewCommit(
			vote.Height,
			vote.Round,
			lastBlockMeta.BlockID,
			state.LastStateID,
			&types.CommitSigns{
				QuorumSigns: *thresholdSigns,
				QuorumHash:  state.LastValidators.QuorumHash,
			},
		)
	}

	block := state.MakeBlock(height, []types.Tx{}, lastCommit, nil, state.Validators.GetProposer().ProTxHash, proposedAppVersion)
	partSet, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)

	return block, partSet
}
