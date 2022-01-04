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
	assert, require := assert.New(t), require.New(t)

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
				CheckTx:   abci.ResponseCheckTx{Data: bytes.HexBytes("stand")},
				DeliverTx: abci.ResponseDeliverTx{Data: bytes.HexBytes("deliver")},
			},
			Error: errors.New("bad tx"),
		},
		Broadcast: mock.Call{Error: errors.New("must commit")},
	}

	// now, let's try to make some calls
	_, err := m.ABCIInfo(ctx)
	require.NotNil(err)
	assert.Equal("foobar", err.Error())

	// query always returns the response
	_query, err := m.ABCIQueryWithOptions(ctx, "/", nil, client.ABCIQueryOptions{Prove: false})
	query := _query.Response
	require.Nil(err)
	require.NotNil(query)
	assert.EqualValues(key, query.Key)
	assert.EqualValues(value, query.Value)
	assert.Equal(height, query.Height)

	// non-commit calls always return errors
	_, err = m.BroadcastTxSync(ctx, goodTx)
	require.NotNil(err)
	assert.Equal("must commit", err.Error())
	_, err = m.BroadcastTxAsync(ctx, goodTx)
	require.NotNil(err)
	assert.Equal("must commit", err.Error())

	// commit depends on the input
	_, err = m.BroadcastTxCommit(ctx, badTx)
	require.NotNil(err)
	assert.Equal("bad tx", err.Error())
	bres, err := m.BroadcastTxCommit(ctx, goodTx)
	require.Nil(err, "%+v", err)
	assert.EqualValues(0, bres.CheckTx.Code)
	assert.EqualValues("stand", bres.CheckTx.Data)
	assert.EqualValues("deliver", bres.DeliverTx.Data)
}

func TestABCIRecorder(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

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

	require.Equal(0, len(r.Calls))

	_, err := r.ABCIInfo(ctx)
	assert.Nil(err, "expected no err on info")

	_, err = r.ABCIQueryWithOptions(
		ctx,
		"path",
		bytes.HexBytes("data"),
		client.ABCIQueryOptions{Prove: false},
	)
	assert.NotNil(err, "expected error on query")
	require.Equal(2, len(r.Calls))

	info := r.Calls[0]
	assert.Equal("abci_info", info.Name)
	assert.Nil(info.Error)
	assert.Nil(info.Args)
	require.NotNil(info.Response)
	ir, ok := info.Response.(*coretypes.ResultABCIInfo)
	require.True(ok)
	assert.Equal("data", ir.Response.Data)
	assert.Equal("v0.9.9", ir.Response.Version)

	query := r.Calls[1]
	assert.Equal("abci_query", query.Name)
	assert.Nil(query.Response)
	require.NotNil(query.Error)
	assert.Equal("query", query.Error.Error())
	require.NotNil(query.Args)
	qa, ok := query.Args.(mock.QueryArgs)
	require.True(ok)
	assert.Equal("path", qa.Path)
	assert.EqualValues("data", qa.Data)
	assert.False(qa.Prove)

	// now add some broadcasts (should all err)
	txs := []types.Tx{{1}, {2}, {3}}
	_, err = r.BroadcastTxCommit(ctx, txs[0])
	assert.NotNil(err, "expected err on broadcast")
	_, err = r.BroadcastTxSync(ctx, txs[1])
	assert.NotNil(err, "expected err on broadcast")
	_, err = r.BroadcastTxAsync(ctx, txs[2])
	assert.NotNil(err, "expected err on broadcast")

	require.Equal(5, len(r.Calls))

	bc := r.Calls[2]
	assert.Equal("broadcast_tx_commit", bc.Name)
	assert.Nil(bc.Response)
	require.NotNil(bc.Error)
	assert.EqualValues(bc.Args, txs[0])

	bs := r.Calls[3]
	assert.Equal("broadcast_tx_sync", bs.Name)
	assert.Nil(bs.Response)
	require.NotNil(bs.Error)
	assert.EqualValues(bs.Args, txs[1])

	ba := r.Calls[4]
	assert.Equal("broadcast_tx_async", ba.Name)
	assert.Nil(ba.Response)
	require.NotNil(ba.Error)
	assert.EqualValues(ba.Args, txs[2])
}

func TestABCIApp(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	app := kvstore.NewApplication()
	m := mock.ABCIApp{app}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// get some info
	info, err := m.ABCIInfo(ctx)
	require.Nil(err)
	assert.Equal(`{"size":0}`, info.Response.GetData())

	// add a key
	key, value := "foo", "bar"
	tx := fmt.Sprintf("%s=%s", key, value)
	res, err := m.BroadcastTxCommit(ctx, types.Tx(tx))
	require.Nil(err)
	assert.True(res.CheckTx.IsOK())
	require.NotNil(res.DeliverTx)
	assert.True(res.DeliverTx.IsOK())

	// commit
	// TODO: This may not be necessary in the future
	if res.Height == -1 {
		m.App.Commit()
	}

	// check the key
	_qres, err := m.ABCIQueryWithOptions(
		ctx,
		"/key",
		bytes.HexBytes(key),
		client.ABCIQueryOptions{Prove: true},
	)
	qres := _qres.Response
	require.Nil(err)
	assert.EqualValues(value, qres.Value)

	// XXX Check proof
}
