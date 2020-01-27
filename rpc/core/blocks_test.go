package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

func TestBlockchainInfo(t *testing.T) {
	cases := []struct {
		min, max     int64
		height       int64
		limit        int64
		resultLength int64
		wantErr      bool
	}{

		// min > max
		{0, 0, 0, 10, 0, true},  // min set to 1
		{0, 1, 0, 10, 0, true},  // max set to height (0)
		{0, 0, 1, 10, 1, false}, // max set to height (1)
		{2, 0, 1, 10, 0, true},  // max set to height (1)
		{2, 1, 5, 10, 0, true},

		// negative
		{1, 10, 14, 10, 10, false}, // control
		{-1, 10, 14, 10, 0, true},
		{1, -10, 14, 10, 0, true},
		{-9223372036854775808, -9223372036854775788, 100, 20, 0, true},

		// check limit and height
		{1, 1, 1, 10, 1, false},
		{1, 1, 5, 10, 1, false},
		{2, 2, 5, 10, 1, false},
		{1, 2, 5, 10, 2, false},
		{1, 5, 1, 10, 1, false},
		{1, 5, 10, 10, 5, false},
		{1, 15, 10, 10, 10, false},
		{1, 15, 15, 10, 10, false},
		{1, 15, 15, 20, 15, false},
		{1, 20, 15, 20, 15, false},
		{1, 20, 20, 20, 20, false},
	}

	for i, c := range cases {
		caseString := fmt.Sprintf("test %d failed", i)
		min, max, err := filterMinMax(c.height, c.min, c.max, c.limit)
		if c.wantErr {
			require.Error(t, err, caseString)
		} else {
			require.NoError(t, err, caseString)
			require.Equal(t, 1+max-min, c.resultLength, caseString)
		}
	}
}

func TestBlockResults(t *testing.T) {
	results := &sm.ABCIResponses{
		DeliverTxs: []*abci.ResponseDeliverTx{
			{Code: 0, Data: []byte{0x01}, Log: "ok"},
			{Code: 0, Data: []byte{0x02}, Log: "ok"},
			{Code: 1, Log: "not ok"},
		},
		EndBlock:   &abci.ResponseEndBlock{},
		BeginBlock: &abci.ResponseBeginBlock{},
	}

	stateDB = dbm.NewMemDB()
	sm.SaveABCIResponses(stateDB, 100, results)
	blockStore = mockBlockStore{height: 100}

	testCases := []struct {
		height  int64
		wantErr bool
		wantRes *ctypes.ResultBlockResults
	}{
		{-1, true, nil},
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
	height int64
}

func (store mockBlockStore) Height() int64                               { return store.height }
func (mockBlockStore) LoadBlockMeta(height int64) *types.BlockMeta       { return nil }
func (mockBlockStore) LoadBlock(height int64) *types.Block               { return nil }
func (mockBlockStore) LoadBlockByHash(hash []byte) *types.Block          { return nil }
func (mockBlockStore) LoadBlockPart(height int64, index int) *types.Part { return nil }
func (mockBlockStore) LoadBlockCommit(height int64) *types.Commit        { return nil }
func (mockBlockStore) LoadSeenCommit(height int64) *types.Commit         { return nil }
func (mockBlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
}
