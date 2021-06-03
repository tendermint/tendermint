package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/types"
)

const (
	height int64 = 10
	base   int64 = 1
)

func TestUnsafeResync(t *testing.T) {

	mockBlockStore := &mocks.BlockStore{}
	mockStateStore := &mocks.Store{}
	mockEventSink := &mocks.EventSink{}

	env := &Environment{
		BlockStore: mockBlockStore,
		StateStore: mockStateStore,
		EventSinks: []indexer.EventSink{mockEventSink},
		Logger:     log.TestingLogger()}

	mockBlockStore.
		On("Base").Return(base).
		On("Height").Return(height).
		On("LoadBlock", base+1).Return(nil).Once().
		On("LoadBlock", base+1).Return(&types.Block{Data: types.Data{Txs: types.Txs{make(types.Tx, 1)}}})

	mockEventSink.
		On("Type").Return(indexer.NULL).Once().
		On("Type").Return(indexer.KV).
		On("IndexBlockEvents", mock.AnythingOfType("types.EventDataNewBlockHeader")).Return(errors.New("")).Once().
		On("IndexBlockEvents", mock.AnythingOfType("types.EventDataNewBlockHeader")).Return(nil).
		On("IndexTxEvents", mock.AnythingOfType("[]*types.TxResult")).Return(errors.New("")).Once().
		On("IndexTxEvents", mock.AnythingOfType("[]*types.TxResult")).Return(nil)

	dtx := abcitypes.ResponseDeliverTx{}
	abciResp := &tmstate.ABCIResponses{
		DeliverTxs: []*abcitypes.ResponseDeliverTx{&dtx},
		EndBlock:   &abcitypes.ResponseEndBlock{},
		BeginBlock: &abcitypes.ResponseBeginBlock{},
	}

	mockStateStore.
		On("LoadABCIResponses", base+1).Return(nil, errors.New("")).Once().
		On("LoadABCIResponses", base+1).Return(abciResp, nil)

	testCases := []struct {
		startHeight int64
		endHeight   int64
		enableSink  bool
		isErr       bool
	}{
		{base, 0, false, true},              // the start height less equal than the base height
		{height + 1, 0, false, true},        // the start height greater than the store height
		{base + 1, base, false, true},       // the end height less equal than the base height
		{height, height - 1, false, true},   // the start height greater than the end height
		{base + 1, height + 1, false, true}, // the end height will be the same as the store height and no eventsink error
		{base + 1, base + 1, true, true},    // LoadBlock error
		{base + 1, base + 1, true, true},    // LoadABCIResponses error
		{base + 1, base + 1, true, true},    // index block event error
		{base + 1, base + 1, true, true},    // index tx event error
		{base + 1, base + 1, true, false},
	}

	for _, tc := range testCases {
		res, err := env.UnsafeResync(&rpctypes.Context{}, tc.startHeight, tc.endHeight)
		if tc.isErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, res)
		}
	}
}
