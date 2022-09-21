package kvstore

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"

	abcicli "github.com/tendermint/tendermint/abci/client"
	abciserver "github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
)

const (
	testKey   = "abc"
	testValue = "def"
)

func TestKVStoreKV(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kvstore := NewInMemoryApplication()
	key := testKey
	value := key
	tx := []byte(key)
	testKVStore(ctx, t, kvstore, tx, key, value)

	value = testValue
	tx = []byte(key + "=" + value)
	testKVStore(ctx, t, kvstore, tx, key, value)
}

func testKVStore(ctx context.Context, t *testing.T, app types.Application, tx []byte, key, value string) {
	req := &types.RequestFinalizeBlock{Txs: [][]byte{tx}}
	ar, err := app.FinalizeBlock(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, len(ar.TxResults))
	require.False(t, ar.TxResults[0].IsErr())
	// repeating tx doesn't raise error
	ar, err = app.FinalizeBlock(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, len(ar.TxResults))
	require.False(t, ar.TxResults[0].IsErr())
	// commit
	_, err = app.Commit(ctx, &types.RequestCommit{})
	require.NoError(t, err)

	info, err := app.Info(ctx, &types.RequestInfo{})
	require.NoError(t, err)
	require.NotZero(t, info.LastBlockHeight)

	// make sure query is fine
	resQuery, err := app.Query(ctx, &types.RequestQuery{
		Path: "/store",
		Data: []byte(key),
	})
	require.NoError(t, err)
	require.Equal(t, CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)

	// make sure proof is fine
	resQuery, err = app.Query(ctx, &types.RequestQuery{
		Path:  "/store",
		Data:  []byte(key),
		Prove: true,
	})
	require.NoError(t, err)
	require.EqualValues(t, CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)
}

func TestPersistentKVStoreKV(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kvstore := NewPersistentApplication(t.TempDir())
	key := testKey
	value := key
	tx := []byte(key)
	testKVStore(ctx, t, kvstore, tx, key, value)

	value = testValue
	tx = []byte(key + "=" + value)
	testKVStore(ctx, t, kvstore, tx, key, value)
}

func TestPersistentKVStoreInfo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kvstore := NewPersistentApplication(t.TempDir())
	require.NoError(t, InitKVStore(ctx, kvstore))
	height := int64(0)

	resInfo, err := kvstore.Info(ctx, &types.RequestInfo{})
	require.NoError(t, err)
	if resInfo.LastBlockHeight != height {
		t.Fatalf("expected height of %d, got %d", height, resInfo.LastBlockHeight)
	}

	// make and apply block
	height = int64(1)
	hash := []byte("foo")
	if _, err := kvstore.FinalizeBlock(ctx, &types.RequestFinalizeBlock{Hash: hash, Height: height}); err != nil {
		t.Fatal(err)
	}

	_, err = kvstore.Commit(ctx, &types.RequestCommit{})
	require.NoError(t, err)

	resInfo, err = kvstore.Info(ctx, &types.RequestInfo{})
	require.NoError(t, err)
	require.Equal(t, height, resInfo.LastBlockHeight)

}

// add a validator, remove a validator, update a validator
func TestValUpdates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kvstore := NewInMemoryApplication()

	// init with some validators
	total := 10
	nInit := 5
	vals := RandVals(total)
	// initialize with the first nInit
	_, err := kvstore.InitChain(ctx, &types.RequestInitChain{
		Validators: vals[:nInit],
	})
	require.NoError(t, err)

	vals1, vals2 := vals[:nInit], kvstore.Validators()
	valsEqual(t, vals1, vals2)

	var v1, v2, v3 types.ValidatorUpdate

	// add some validators
	v1, v2 = vals[nInit], vals[nInit+1]
	diff := []types.ValidatorUpdate{v1, v2}
	tx1 := MakeValSetChangeTx(v1.PubKey, v1.Power)
	tx2 := MakeValSetChangeTx(v2.PubKey, v2.Power)

	makeApplyBlock(ctx, t, kvstore, 1, diff, tx1, tx2)

	vals1, vals2 = vals[:nInit+2], kvstore.Validators()
	valsEqual(t, vals1, vals2)

	// remove some validators
	v1, v2, v3 = vals[nInit-2], vals[nInit-1], vals[nInit]
	v1.Power = 0
	v2.Power = 0
	v3.Power = 0
	diff = []types.ValidatorUpdate{v1, v2, v3}
	tx1 = MakeValSetChangeTx(v1.PubKey, v1.Power)
	tx2 = MakeValSetChangeTx(v2.PubKey, v2.Power)
	tx3 := MakeValSetChangeTx(v3.PubKey, v3.Power)

	makeApplyBlock(ctx, t, kvstore, 2, diff, tx1, tx2, tx3)

	vals1 = append(vals[:nInit-2], vals[nInit+1]) //nolint: gocritic
	vals2 = kvstore.Validators()
	valsEqual(t, vals1, vals2)

	// update some validators
	v1 = vals[0]
	if v1.Power == 5 {
		v1.Power = 6
	} else {
		v1.Power = 5
	}
	diff = []types.ValidatorUpdate{v1}
	tx1 = MakeValSetChangeTx(v1.PubKey, v1.Power)

	makeApplyBlock(ctx, t, kvstore, 3, diff, tx1)

	vals1 = append([]types.ValidatorUpdate{v1}, vals1[1:]...)
	vals2 = kvstore.Validators()
	valsEqual(t, vals1, vals2)

}

func TestClientServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// set up socket app
	kvstore := NewInMemoryApplication()
	client, _, err := makeSocketClientServer(t, kvstore, "kvstore-socket")
	require.NoError(t, err)

	runClientTests(ctx, t, client)

	// set up grpc app
	kvstore = NewInMemoryApplication()
	gclient, _, err := makeGRPCClientServer(kvstore, "/tmp/kvstore-grpc")
	require.NoError(t, err)

	runClientTests(ctx, t, gclient)
}

func makeApplyBlock(
	ctx context.Context,
	t *testing.T,
	kvstore types.Application,
	heightInt int,
	diff []types.ValidatorUpdate,
	txs ...[]byte) {
	// make and apply block
	height := int64(heightInt)
	hash := []byte("foo")
	resFinalizeBlock, err := kvstore.FinalizeBlock(ctx, &types.RequestFinalizeBlock{
		Hash:   hash,
		Height: height,
		Txs:    txs,
	})
	require.NoError(t, err)

	_, err = kvstore.Commit(ctx, &types.RequestCommit{})
	require.NoError(t, err)

	valsEqual(t, diff, resFinalizeBlock.ValidatorUpdates)

}

// order doesn't matter
func valsEqual(t *testing.T, vals1, vals2 []types.ValidatorUpdate) {
	t.Helper()
	if len(vals1) != len(vals2) {
		t.Fatalf("vals dont match in len. got %d, expected %d", len(vals2), len(vals1))
	}
	sort.Sort(types.ValidatorUpdates(vals1))
	sort.Sort(types.ValidatorUpdates(vals2))
	for i, v1 := range vals1 {
		v2 := vals2[i]
		if !v1.PubKey.Equal(v2.PubKey) ||
			v1.Power != v2.Power {
			t.Fatalf("vals dont match at index %d. got %X/%d , expected %X/%d", i, v2.PubKey, v2.Power, v1.PubKey, v1.Power)
		}
	}
}

func makeSocketClientServer(t *testing.T, app types.Application, name string) (abcicli.Client, service.Service, error) {
	// Start the listener
	socket := fmt.Sprintf("unix://%s.sock", name)
	logger := log.TestingLogger()

	server := abciserver.NewSocketServer(socket, app)
	server.SetLogger(logger.With("module", "abci-server"))
	if err := server.Start(); err != nil {
		return nil, nil, err
	}

	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Error(err)
		}
	})

	// Connect to the socket
	client := abcicli.NewSocketClient(socket, false)
	client.SetLogger(logger.With("module", "abci-client"))
	if err := client.Start(); err != nil {
		return nil, nil, err
	}

	t.Cleanup(func() {
		if err := client.Stop(); err != nil {
			t.Error(err)
		}
	})

	return client, server, nil
}

func makeGRPCClientServer(app types.Application, name string) (abcicli.Client, service.Service, error) {
	// Start the listener
	socket := fmt.Sprintf("unix://%s.sock", name)
	logger := log.TestingLogger()

	server := abciserver.NewGRPCServer(socket, app)
	server.SetLogger(logger.With("module", "abci-server"))
	if err := server.Start(); err != nil {
		return nil, nil, err
	}

	client := abcicli.NewGRPCClient(socket, true)
	client.SetLogger(logger.With("module", "abci-client"))
	if err := client.Start(); err != nil {
		if err := server.Stop(); err != nil {
			return nil, nil, err
		}
		return nil, nil, err
	}
	return client, server, nil
}

func runClientTests(ctx context.Context, t *testing.T, client abcicli.Client) {
	// run some tests....
	key := testKey
	value := key
	tx := []byte(key)
	testClient(ctx, t, client, tx, key, value)

	value = testValue
	tx = []byte(key + "=" + value)
	testClient(ctx, t, client, tx, key, value)
}

func testClient(ctx context.Context, t *testing.T, app abcicli.Client, tx []byte, key, value string) {
	ar, err := app.FinalizeBlock(ctx, &types.RequestFinalizeBlock{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.Equal(t, 1, len(ar.TxResults))
	require.False(t, ar.TxResults[0].IsErr())
	// repeating FinalizeBlock doesn't raise error
	ar, err = app.FinalizeBlock(ctx, &types.RequestFinalizeBlock{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.Equal(t, 1, len(ar.TxResults))
	require.False(t, ar.TxResults[0].IsErr())
	// commit
	_, err = app.Commit(ctx, &types.RequestCommit{})
	require.NoError(t, err)

	info, err := app.Info(ctx, &types.RequestInfo{})
	require.NoError(t, err)
	require.NotZero(t, info.LastBlockHeight)

	// make sure query is fine
	resQuery, err := app.Query(ctx, &types.RequestQuery{
		Path: "/store",
		Data: []byte(key),
	})
	require.NoError(t, err)
	require.Equal(t, CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)

	// make sure proof is fine
	resQuery, err = app.Query(ctx, &types.RequestQuery{
		Path:  "/store",
		Data:  []byte(key),
		Prove: true,
	})
	require.NoError(t, err)
	require.Equal(t, CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)
}
