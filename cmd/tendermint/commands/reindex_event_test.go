package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmcfg "github.com/tendermint/tendermint/config"
	prototmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	blockmocks "github.com/tendermint/tendermint/state/indexer/mocks"
	"github.com/tendermint/tendermint/state/mocks"
	txmocks "github.com/tendermint/tendermint/state/txindex/mocks"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

const (
	height int64 = 10
	base   int64 = 2
)

func setupReIndexEventCmd() *cobra.Command {
	reIndexEventCmd := &cobra.Command{
		Use: ReIndexEventCmd.Use,
		Run: func(cmd *cobra.Command, args []string) {},
	}

	_ = reIndexEventCmd.ExecuteContext(context.Background())

	return reIndexEventCmd
}

func TestReIndexEventCheckHeight(t *testing.T) {
	mockBlockStore := &mocks.BlockStore{}
	mockBlockStore.
		On("Base").Return(base).
		On("Height").Return(height)

	testCases := []struct {
		startHeight int64
		endHeight   int64
		validHeight bool
	}{
		{0, 0, true},
		{0, base, true},
		{0, base - 1, false},
		{0, height, true},
		{0, height + 1, true},
		{0, 0, true},
		{base - 1, 0, false},
		{base, 0, true},
		{base, base, true},
		{base, base - 1, false},
		{base, height, true},
		{base, height + 1, true},
		{height, 0, true},
		{height, base, false},
		{height, height - 1, false},
		{height, height, true},
		{height, height + 1, true},
		{height + 1, 0, false},
	}

	for _, tc := range testCases {
		startHeight = tc.startHeight
		endHeight = tc.endHeight

		err := checkValidHeight(mockBlockStore)
		if tc.validHeight {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestLoadEventSink(t *testing.T) {
	testCases := []struct {
		sinks   string
		connURL string
		loadErr bool
	}{
		{"", "", true},
		{"NULL", "", true},
		{"KV", "", false},
		{"PSQL", "", true}, // true because empty connect url
		// skip to test PSQL connect with correct url
		{"UnsupportedSinkType", "wrongUrl", true},
	}

	for idx, tc := range testCases {
		cfg := tmcfg.TestConfig()
		cfg.TxIndex.Indexer = tc.sinks
		cfg.TxIndex.PsqlConn = tc.connURL
		_, _, err := loadEventSinks(cfg)
		if tc.loadErr {
			require.Error(t, err, idx)
		} else {
			require.NoError(t, err, idx)
		}
	}
}

func TestLoadBlockStore(t *testing.T) {
	cfg := tmcfg.TestConfig()
	cfg.DBPath = t.TempDir()
	_, _, err := loadStateAndBlockStore(cfg)
	require.Error(t, err)

	_, err = dbm.NewDB("blockstore", dbm.GoLevelDBBackend, cfg.DBDir())
	require.NoError(t, err)

	// Get StateStore
	_, err = dbm.NewDB("state", dbm.GoLevelDBBackend, cfg.DBDir())
	require.NoError(t, err)

	bs, ss, err := loadStateAndBlockStore(cfg)
	require.NoError(t, err)
	require.NotNil(t, bs)
	require.NotNil(t, ss)

}

func TestReIndexEvent(t *testing.T) {
	mockBlockStore := &mocks.BlockStore{}
	mockStateStore := &mocks.Store{}
	mockBlockIndexer := &blockmocks.BlockIndexer{}
	mockTxIndexer := &txmocks.TxIndexer{}

	mockBlockStore.
		On("Base").Return(base).
		On("Height").Return(height).
		On("LoadBlock", base).Return(nil).Once().
		On("LoadBlock", base).Return(&types.Block{Data: types.Data{Txs: types.Txs{make(types.Tx, 1)}}}).
		On("LoadBlock", height).Return(&types.Block{Data: types.Data{Txs: types.Txs{make(types.Tx, 1)}}})

	dtx := abcitypes.ResponseDeliverTx{}
	abciResp := &prototmstate.ABCIResponses{
		DeliverTxs: []*abcitypes.ResponseDeliverTx{&dtx},
		EndBlock:   &abcitypes.ResponseEndBlock{},
		BeginBlock: &abcitypes.ResponseBeginBlock{},
	}

	mockBlockIndexer.
		On("Index", mock.AnythingOfType("types.EventDataNewBlockHeader")).Return(errors.New("")).Once().
		On("Index", mock.AnythingOfType("types.EventDataNewBlockHeader")).Return(nil)

	mockTxIndexer.
		On("AddBatch", mock.AnythingOfType("*txindex.Batch")).Return(errors.New("")).Once().
		On("AddBatch", mock.AnythingOfType("*txindex.Batch")).Return(nil)

	mockStateStore.
		On("LoadABCIResponses", base).Return(nil, errors.New("")).Once().
		On("LoadABCIResponses", base).Return(abciResp, nil).
		On("LoadABCIResponses", height).Return(abciResp, nil)

	testCases := []struct {
		startHeight int64
		endHeight   int64
		reIndexErr  bool
	}{
		{base, height, true}, // LoadBlock error
		{base, height, true}, // LoadABCIResponses error
		{base, height, true}, // index block event error
		{base, height, true}, // index tx event error
		{base, base, false},
		{height, height, false},
	}

	for _, tc := range testCases {
		args := eventReIndexArgs{
			startHeight:  tc.startHeight,
			endHeight:    tc.endHeight,
			blockIndexer: mockBlockIndexer,
			txIndexer:    mockTxIndexer,
			blockStore:   mockBlockStore,
			stateStore:   mockStateStore,
		}

		err := eventReIndex(setupReIndexEventCmd(), args)
		if tc.reIndexErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}
