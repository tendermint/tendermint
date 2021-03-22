package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

func TestBlockchainInfo(t *testing.T) {
	cases := []struct {
		min, max     uint64
		base, height uint64
		limit        uint64
		resultLength uint64
		wantErr      bool
	}{

		// min > max
		{0, 0, 0, 0, 10, 0, true},  // min set to 1
		{0, 1, 0, 0, 10, 0, true},  // max set to height (0)
		{0, 0, 0, 1, 10, 1, false}, // max set to height (1)
		{2, 0, 0, 1, 10, 0, true},  // max set to height (1)
		{2, 1, 0, 5, 10, 0, true},

		// negative
		{1, 10, 0, 14, 10, 10, false}, // control

		// check base
		{1, 1, 1, 1, 1, 1, false},
		{2, 5, 3, 5, 5, 3, false},

		// check limit and height
		{1, 1, 0, 1, 10, 1, false},
		{1, 1, 0, 5, 10, 1, false},
		{2, 2, 0, 5, 10, 1, false},
		{1, 2, 0, 5, 10, 2, false},
		{1, 5, 0, 1, 10, 1, false},
		{1, 5, 0, 10, 10, 5, false},
		{1, 15, 0, 10, 10, 10, false},
		{1, 15, 0, 10, 10, 10, false}, // what changed here?
		{1, 15, 0, 15, 20, 15, false},
		{1, 20, 0, 15, 20, 15, false},
		{1, 20, 0, 20, 20, 20, false},
	}

	for i, c := range cases {
		caseString := fmt.Sprintf("test %d failed", i)
		min, max, err := filterMinMax(c.base, c.height, c.min, c.max, c.limit)
		if c.wantErr {
			require.Error(t, err, caseString)
		} else {
			require.NoError(t, err, caseString)
			require.Equal(t, int64(1+max-min), int64(c.resultLength), caseString)
		}
	}
}

func TestBlockResults(t *testing.T) {
	results := &tmstate.ABCIResponses{
		DeliverTxs: []*abci.ResponseDeliverTx{
			{Code: 0, Data: []byte{0x01}, Log: "ok"},
			{Code: 0, Data: []byte{0x02}, Log: "ok"},
			{Code: 1, Log: "not ok"},
		},
		EndBlock:   &abci.ResponseEndBlock{},
		BeginBlock: &abci.ResponseBeginBlock{},
	}

	env = &Environment{}
	env.StateStore = sm.NewStore(dbm.NewMemDB())
	err := env.StateStore.SaveABCIResponses(100, results)
	require.NoError(t, err)
	env.BlockStore = mockBlockStore{height: 100}

	testCases := []struct {
		height  uint64
		wantErr bool
		wantRes *ctypes.ResultBlockResults
	}{
		{0, true, nil},
		{101, true, nil},
		{100, false, &ctypes.ResultBlockResults{
			Height:                100,
			TxsResults:            results.DeliverTxs,
			BeginBlockEvents:      results.BeginBlock.Events,
			EndBlockEvents:        results.EndBlock.Events,
			ValidatorUpdates:      results.EndBlock.ValidatorUpdates,
			ConsensusParamUpdates: results.EndBlock.ConsensusParamUpdates,
		}},
	}

	for _, tc := range testCases {
		res, err := BlockResults(&rpctypes.Context{}, &tc.height)
		if tc.wantErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRes, res)
		}
	}
}

type mockBlockStore struct {
	height uint64
}

func (mockBlockStore) Base() uint64                                       { return 1 }
func (store mockBlockStore) Height() uint64                               { return store.height }
func (store mockBlockStore) Size() uint64                                 { return store.height }
func (mockBlockStore) LoadBaseMeta() *types.BlockMeta                     { return nil }
func (mockBlockStore) LoadBlockMeta(height uint64) *types.BlockMeta       { return nil }
func (mockBlockStore) LoadBlock(height uint64) *types.Block               { return nil }
func (mockBlockStore) LoadBlockByHash(hash []byte) *types.Block           { return nil }
func (mockBlockStore) LoadBlockPart(height uint64, index int) *types.Part { return nil }
func (mockBlockStore) LoadBlockCommit(height uint64) *types.Commit        { return nil }
func (mockBlockStore) LoadSeenCommit(height uint64) *types.Commit         { return nil }
func (mockBlockStore) PruneBlocks(height uint64) (uint64, error)          { return 0, nil }
func (mockBlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
}
