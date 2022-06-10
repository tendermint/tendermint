package mock_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/mock"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

func TestABCIMock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	key, value := []byte("foo"), []byte("bar")
	height := int64(10)
	goodTx := types.Tx{0x01, 0xff}
	badTx := types.Tx{0x12, 0x21}

	m := mock.ABCIMock{
		Info: mock.Call{Error: errors.New("foobar")},
		Query: mock.Call{Response: abci.ResponseQuery{
			Key:    key,
			Value:  value,
			Height: height,
		}},
		// Broadcast commit depends on call
		BroadcastCommit: mock.Call{
			Args: goodTx,
			Response: &coretypes.ResultBroadcastTxCommit{
				CheckTx:  abci.ResponseCheckTx{Data: bytes.HexBytes("stand")},
				TxResult: abci.ExecTxResult{Data: bytes.HexBytes("deliver")},
			},
			Error: errors.New("bad tx"),
		},
		Broadcast: mock.Call{Error: errors.New("must commit")},
	}

	// now, let's try to make some calls
	_, err := m.ABCIInfo(ctx)
	require.Error(t, err)
	assert.Equal(t, "foobar", err.Error())

	// query always returns the response
	_query, err := m.ABCIQueryWithOptions(ctx, "/", nil, client.ABCIQueryOptions{Prove: false})
	query := _query.Response
	require.NoError(t, err)
	require.NotNil(t, query)
	assert.EqualValues(t, key, query.Key)
	assert.EqualValues(t, value, query.Value)
	assert.Equal(t, height, query.Height)

	// non-commit calls always return errors
	_, err = m.BroadcastTxSync(ctx, goodTx)
	require.Error(t, err)
	assert.Equal(t, "must commit", err.Error())
	_, err = m.BroadcastTxAsync(ctx, goodTx)
	require.Error(t, err)
	assert.Equal(t, "must commit", err.Error())

	// commit depends on the input
	_, err = m.BroadcastTxCommit(ctx, badTx)
	require.Error(t, err)
	assert.Equal(t, "bad tx", err.Error())
	bres, err := m.BroadcastTxCommit(ctx, goodTx)
	require.NoError(t, err, "%+v", err)
	assert.EqualValues(t, 0, bres.CheckTx.Code)
	assert.EqualValues(t, "stand", bres.CheckTx.Data)
	assert.EqualValues(t, "deliver", bres.TxResult.Data)
}

func TestABCIRecorder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// This mock returns errors on everything but Query
	m := mock.ABCIMock{
		Info: mock.Call{Response: abci.ResponseInfo{
			Data:    "data",
			Version: "v0.9.9",
		}},
		Query:           mock.Call{Error: errors.New("query")},
		Broadcast:       mock.Call{Error: errors.New("broadcast")},
		BroadcastCommit: mock.Call{Error: errors.New("broadcast_commit")},
	}
	r := mock.NewABCIRecorder(m)

	require.Equal(t, 0, len(r.Calls))

	_, err := r.ABCIInfo(ctx)
	assert.NoError(t, err, "expected no err on info")

	_, err = r.ABCIQueryWithOptions(
		ctx,
		"path",
		bytes.HexBytes("data"),
		client.ABCIQueryOptions{Prove: false},
	)
	assert.Error(t, err, "expected error on query")
	require.Equal(t, 2, len(r.Calls))

	info := r.Calls[0]
	assert.Equal(t, "abci_info", info.Name)
	assert.Nil(t, info.Error)
	assert.Nil(t, info.Args)
	require.NotNil(t, info.Response)
	ir, ok := info.Response.(*coretypes.ResultABCIInfo)
	require.True(t, ok)
	assert.Equal(t, "data", ir.Response.Data)
	assert.Equal(t, "v0.9.9", ir.Response.Version)

	query := r.Calls[1]
	assert.Equal(t, "abci_query", query.Name)
	assert.Nil(t, query.Response)
	require.NotNil(t, query.Error)
	assert.Equal(t, "query", query.Error.Error())
	require.NotNil(t, query.Args)
	qa, ok := query.Args.(mock.QueryArgs)
	require.True(t, ok)
	assert.Equal(t, "path", qa.Path)
	assert.EqualValues(t, "data", qa.Data)
	assert.False(t, qa.Prove)

	// now add some broadcasts (should all err)
	txs := []types.Tx{{1}, {2}, {3}}
	_, err = r.BroadcastTxCommit(ctx, txs[0])
	assert.Error(t, err, "expected err on broadcast")
	_, err = r.BroadcastTxSync(ctx, txs[1])
	assert.Error(t, err, "expected err on broadcast")
	_, err = r.BroadcastTxAsync(ctx, txs[2])
	assert.Error(t, err, "expected err on broadcast")

	require.Equal(t, 5, len(r.Calls))

	bc := r.Calls[2]
	assert.Equal(t, "broadcast_tx_commit", bc.Name)
	assert.Nil(t, bc.Response)
	require.NotNil(t, bc.Error)
	assert.EqualValues(t, bc.Args, txs[0])

	bs := r.Calls[3]
	assert.Equal(t, "broadcast_tx_sync", bs.Name)
	assert.Nil(t, bs.Response)
	require.NotNil(t, bs.Error)
	assert.EqualValues(t, bs.Args, txs[1])

	ba := r.Calls[4]
	assert.Equal(t, "broadcast_tx_async", ba.Name)
	assert.Nil(t, ba.Response)
	require.NotNil(t, ba.Error)
	assert.EqualValues(t, ba.Args, txs[2])
}

func TestABCIApp(t *testing.T) {
	app := kvstore.NewApplication()
	m := mock.ABCIApp{app}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// get some info
	info, err := m.ABCIInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, `{"size":0}`, info.Response.GetData())

	// add a key
	key, value := "foo", "bar"
	tx := fmt.Sprintf("%s=%s", key, value)
	res, err := m.BroadcastTxCommit(ctx, types.Tx(tx))
	require.NoError(t, err)
	assert.True(t, res.CheckTx.IsOK())
	require.NotNil(t, res.TxResult)
	assert.True(t, res.TxResult.IsOK())

	// commit
	// TODO: This may not be necessary in the future
	if res.Height == -1 {
		_, err := m.App.Commit(ctx)
		require.NoError(t, err)
	}

	// check the key
	_qres, err := m.ABCIQueryWithOptions(
		ctx,
		"/key",
		bytes.HexBytes(key),
		client.ABCIQueryOptions{Prove: true},
	)
	qres := _qres.Response
	require.NoError(t, err)
	assert.EqualValues(t, value, qres.Value)

	// XXX Check proof
}
