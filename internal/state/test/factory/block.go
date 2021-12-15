package factory

import (
	"encoding/binary"
	"fmt"
	"github.com/tendermint/tendermint/crypto"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/types"
)

func MakeBlocks(n int, state *sm.State, privVal types.PrivValidator) ([]*types.Block, error) {
	blocks := make([]*types.Block, 0)

	var (
		prevBlock     *types.Block
		prevBlockMeta *types.BlockMeta
	)

	appHeight := byte(0x01)
	for i := 0; i < n; i++ {
		height := int64(i + 1)

		block, parts, err := makeBlockAndPartSet(*state, prevBlock, prevBlockMeta, privVal, height)
		if err != nil {
			return nil, fmt.Errorf("error making blocks %v", err)
		}
		blocks = append(blocks, block)

		prevBlock = block
		prevBlockMeta = types.NewBlockMeta(block, parts)

		// update state
		state.LastStateID = state.StateID()
		state.AppHash = make([]byte, crypto.DefaultAppHashSize)
		binary.BigEndian.PutUint64(state.AppHash, uint64(height))
		appHeight++
		state.LastBlockHeight = height
	}

	return blocks, nil
}

func MakeBlock(state sm.State, height int64, c *types.Commit, coreChainLock *types.CoreChainLock,  proposedAppVersion uint64) (*types.Block, error) {

	if state.LastBlockHeight != (height - 1) {
		return nil, fmt.Errorf("requested height %d should be 1 more than last block height %d",
			height, state.LastBlockHeight)
	}

	block, _ := state.MakeBlock(
		height,
		coreChainLock,
		factory.MakeTenTxs(state.LastBlockHeight),
		c,
		nil,
		state.Validators.GetProposer().ProTxHash,
		proposedAppVersion,
	)
	return block, nil
}

func makeBlockAndPartSet(state sm.State, lastBlock *types.Block, lastBlockMeta *types.BlockMeta,
	privVal types.PrivValidator, height int64) (*types.Block, *types.PartSet, error) {

	lastCommit := types.NewCommit(height-1, 0, types.BlockID{}, state.StateID(), state.LastValidators.QuorumHash,
		nil, nil)
	if height > state.InitialHeight {
		vote, err := factory.MakeVote(
			privVal,
			state.Validators,
			lastBlock.Header.ChainID,
			1, lastBlock.Header.Height, 0, 2,
			lastBlockMeta.BlockID,
			state.LastStateID,
			)
		if err != nil {
			return nil, nil, fmt.Errorf("error when creating vote at height %d: %s", height, err)
		}
		lastCommit = types.NewCommit(vote.Height, vote.Round,
			lastBlockMeta.BlockID, state.StateID(), state.LastValidators.QuorumHash, vote.BlockSignature, vote.StateSignature)
	}
	block, partSet := state.MakeBlock(height, nil, []types.Tx{}, lastCommit, nil, state.Validators.GetProposer().ProTxHash, 0)
	return block, partSet, nil
}
