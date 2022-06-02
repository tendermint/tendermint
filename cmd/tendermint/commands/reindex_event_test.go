package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/mocks"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"

	_ "github.com/lib/pq" // for the psql sink
)

const (
	height int64 = 10
	base   int64 = 2
)

func setupReIndexEventCmd(ctx context.Context, conf *config.Config, logger log.Logger) *cobra.Command {
	cmd := MakeReindexEventCommand(conf, logger)

	reIndexEventCmd := &cobra.Command{
		Use: cmd.Use,
		Run: func(cmd *cobra.Command, args []string) {},
	}

	_ = reIndexEventCmd.ExecuteContext(ctx)

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
		err := checkValidHeight(mockBlockStore, checkValidHeightArgs{startHeight: tc.startHeight, endHeight: tc.endHeight})
		if tc.validHeight {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestLoadEventSink(t *testing.T) {
	testCases := []struct {
		sinks   []string
		connURL string
		loadErr bool
	}{
		{[]string{}, "", true},
		{[]string{"NULL"}, "", true},
		{[]string{"KV"}, "", false},
		{[]string{"KV", "KV"}, "", true},
		{[]string{"PSQL"}, "", true},         // true because empty connect url
		{[]string{"PSQL"}, "wrongUrl", true}, // true because wrong connect url
		// skip to test PSQL connect with correct url
		{[]string{"UnsupportedSinkType"}, "wrongUrl", true},
	}

	for _, tc := range testCases {
		cfg := config.TestConfig()
		cfg.TxIndex.Indexer = tc.sinks
		cfg.TxIndex.PsqlConn = tc.connURL
		_, err := loadEventSinks(cfg)
		if tc.loadErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestLoadBlockStore(t *testing.T) {
	testCfg, err := config.ResetTestRoot(t.TempDir(), t.Name())
	require.NoError(t, err)
	testCfg.DBBackend = "goleveldb"
	_, _, err = loadStateAndBlockStore(testCfg)
	// we should return an error because the state store and block store
	// don't yet exist
	require.Error(t, err)

	dbType := dbm.BackendType(testCfg.DBBackend)
	bsdb, err := dbm.NewDB("blockstore", dbType, testCfg.DBDir())
	require.NoError(t, err)
	bsdb.Close()

	ssdb, err := dbm.NewDB("state", dbType, testCfg.DBDir())
	require.NoError(t, err)
	ssdb.Close()

	bs, ss, err := loadStateAndBlockStore(testCfg)
	require.NoError(t, err)
	require.NotNil(t, bs)
	require.NotNil(t, ss)
}

func TestReIndexEvent(t *testing.T) {
	mockBlockStore := &mocks.BlockStore{}
	mockStateStore := &mocks.Store{}
	mockEventSink := &mocks.EventSink{}

	mockBlockStore.
		On("Base").Return(base).
		On("Height").Return(height).
		On("LoadBlock", base).Return(nil).Once().
		On("LoadBlock", base).Return(&types.Block{Data: types.Data{Txs: types.Txs{make(types.Tx, 1)}}}).
		On("LoadBlock", height).Return(&types.Block{Data: types.Data{Txs: types.Txs{make(types.Tx, 1)}}})

	mockEventSink.
		On("Type").Return(indexer.KV).
		On("IndexBlockEvents", mock.AnythingOfType("types.EventDataNewBlockHeader")).Return(errors.New("")).Once().
		On("IndexBlockEvents", mock.AnythingOfType("types.EventDataNewBlockHeader")).Return(nil).
		On("IndexTxEvents", mock.AnythingOfType("[]*types.TxResult")).Return(errors.New("")).Once().
		On("IndexTxEvents", mock.AnythingOfType("[]*types.TxResult")).Return(nil)

	dtx := abcitypes.ExecTxResult{}
	abciResp := &abcitypes.ResponseFinalizeBlock{
		TxResults: []*abcitypes.ExecTxResult{&dtx},
	}

	mockStateStore.
		On("LoadFinalizeBlockResponses", base).Return(nil, errors.New("")).Once().
		On("LoadFinalizeBlockResponses", base).Return(abciResp, nil).
		On("LoadFinalizeBlockResponses", height).Return(abciResp, nil)

	testCases := []struct {
		startHeight int64
		endHeight   int64
		reIndexErr  bool
	}{
		{base, height, true}, // LoadBlock error
		{base, height, true}, // LoadFinalizeBlockResponses error
		{base, height, true}, // index block event error
		{base, height, true}, // index tx event error
		{base, base, false},
		{height, height, false},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := log.NewNopLogger()
	conf := config.DefaultConfig()

	for _, tc := range testCases {
		err := eventReIndex(
			setupReIndexEventCmd(ctx, conf, logger),
			eventReIndexArgs{
				sinks:       []indexer.EventSink{mockEventSink},
				blockStore:  mockBlockStore,
				stateStore:  mockStateStore,
				startHeight: tc.startHeight,
				endHeight:   tc.endHeight,
			})

		if tc.reIndexErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}
