package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

const (
	DefaultTestChainID = "test-chain"
)

var (
	DefaultTestTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
)

func RandomAddress() []byte {
	return crypto.CRandBytes(crypto.AddressSize)
}

func RandomHash() []byte {
	return crypto.CRandBytes(tmhash.Size)
}

func MakeBlockID() types.BlockID {
	return MakeBlockIDWithHash(RandomHash())
}

func MakeBlockIDWithHash(hash []byte) types.BlockID {
	return types.BlockID{
		Hash: hash,
		PartSetHeader: types.PartSetHeader{
			Total: 100,
			Hash:  RandomHash(),
		},
	}
}

// MakeHeader fills the rest of the contents of the header such that it passes
// validate basic
func MakeHeader(t *testing.T, h *types.Header) *types.Header {
	t.Helper()
	if h.Version.Block == 0 {
		h.Version.Block = version.BlockProtocol
	}
	if h.Height == 0 {
		h.Height = 1
	}
	if h.LastBlockID.IsZero() {
		h.LastBlockID = MakeBlockID()
	}
	if h.ChainID == "" {
		h.ChainID = DefaultTestChainID
	}
	if len(h.LastCommitHash) == 0 {
		h.LastCommitHash = RandomHash()
	}
	if len(h.DataHash) == 0 {
		h.DataHash = RandomHash()
	}
	if len(h.ValidatorsHash) == 0 {
		h.ValidatorsHash = RandomHash()
	}
	if len(h.NextValidatorsHash) == 0 {
		h.NextValidatorsHash = RandomHash()
	}
	if len(h.ConsensusHash) == 0 {
		h.ConsensusHash = RandomHash()
	}
	if len(h.AppHash) == 0 {
		h.AppHash = RandomHash()
	}
	if len(h.LastResultsHash) == 0 {
		h.LastResultsHash = RandomHash()
	}
	if len(h.EvidenceHash) == 0 {
		h.EvidenceHash = RandomHash()
	}
	if len(h.ProposerAddress) == 0 {
		h.ProposerAddress = RandomAddress()
	}

	require.NoError(t, h.ValidateBasic())

	return h
}

func MakeBlock(state sm.State) *types.Block {
	return state.MakeBlock(state.LastBlockHeight+1, MakeNTxs(state.LastBlockHeight+1, 10), new(types.Commit), nil, state.NextValidators.Proposer.Address)
}

func MakeBlocks(n int, state sm.State, privVals []types.PrivValidator) ([]*types.Block, error) {
	blockID := MakeBlockID()
	blocks := make([]*types.Block, n)

	for i := 0; i < n; i++ {
		height := state.LastBlockHeight + 1 + int64(i)
		lastCommit, err := MakeCommit(blockID, height-1, 0, state.LastValidators, privVals, state.ChainID, state.LastBlockTime)
		if err != nil {
			return nil, err
		}
		block := state.MakeBlock(height, MakeNTxs(height, 10), lastCommit, nil, state.LastValidators.Proposer.Address)
		blocks[i] = block
		state.LastBlockID = blockID
		state.LastBlockHeight = height
		state.LastBlockTime = state.LastBlockTime.Add(1 * time.Second)
		state.LastValidators = state.Validators.Copy()
		state.Validators = state.NextValidators.Copy()
		state.NextValidators = state.NextValidators.CopyIncrementProposerPriority(1)
		state.AppHash = RandomHash()

		blockID = MakeBlockIDWithHash(block.Hash())
	}

	return blocks, nil
}
